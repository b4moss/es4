//go:build e2e

package es4_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/b4moss/es4/packages/go/pkg/es4"
	"github.com/b4moss/es4/packages/go/pkg/options"
	_ "modernc.org/sqlite"
)

// File-backed libSQL Recovery E2E — local durable Recovery
// (docs/tests/e2e/e2e-spec.md §L.F).
//
// Maps common catalog C1–C5 to recovery_backend=libsql with a local
// file: DSN. Empty state_path (Memory State) so Close→re-Open proves
// Recovery → Restore only (same isolation idea as Object E2E).
// Explicit SnapshotNow; temp-dir isolation with start/failure cleanup.
// Does not require RustFS.

type libsqlFileFixture struct {
	t          *testing.T
	dir        string
	recoveryDB string
}

func newLibSQLFileFixture(t *testing.T) *libsqlFileFixture {
	t.Helper()
	dir, err := os.MkdirTemp("", "es4-e2e-libsql-file-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	f := &libsqlFileFixture{
		t:          t,
		dir:        dir,
		recoveryDB: filepath.Join(dir, "recovery.db"),
	}
	if err := f.clear(); err != nil {
		t.Fatalf("start clear: %v", err)
	}
	t.Cleanup(func() {
		if err := f.clear(); err != nil {
			t.Logf("cleanup clear: %v", err)
		}
		if err := os.RemoveAll(f.dir); err != nil {
			t.Logf("cleanup remove dir: %v", err)
		}
	})
	return f
}

// clear removes libSQL Recovery artifacts under the fixture dir (idempotent).
func (f *libsqlFileFixture) clear() error {
	for _, p := range []string{
		f.recoveryDB,
		f.recoveryDB + "-wal",
		f.recoveryDB + "-shm",
		f.recoveryDB + "-journal",
	} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (f *libsqlFileFixture) opts() options.Options {
	return options.Options{
		StatePath:         "", // Memory State; restart proof is Recovery→Restore
		RecoveryBackend:   options.RecoveryBackendLibSQL,
		RecoveryLibSQLURL: "file:" + f.recoveryDB,
		RestoreOnStartup:  true,
		MemoryOnly:        false,
		SnapshotInterval:  0, // explicit SnapshotNow only
	}
}

func (f *libsqlFileFixture) open(ctx context.Context) *es4.DB {
	f.t.Helper()
	db, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options:          f.opts(),
		SkipAsyncRestore: true,
	})
	if err != nil {
		f.t.Fatalf("OpenWith: %v", err)
	}
	return db
}

func (f *libsqlFileFixture) recoveryHasRows() bool {
	// Stat first: sql.Open("sqlite", path) would recreate a missing file.
	if _, err := os.Stat(f.recoveryDB); err != nil {
		return false
	}
	db, err := sql.Open("sqlite", f.recoveryDB)
	if err != nil {
		return false
	}
	defer db.Close()
	var n int
	err = db.QueryRow(`SELECT COUNT(*) FROM es4_recovery`).Scan(&n)
	return err == nil && n > 0
}

// TestE2E_LibSQLFile_SaveRestartRestore — C1
func TestE2E_LibSQLFile_SaveRestartRestore(t *testing.T) {
	ctx := context.Background()
	f := newLibSQLFileFixture(t)

	db1 := f.open(ctx)
	val := json.RawMessage(`{"hello":"libsql-file","n":1}`)
	if err := db1.Set(ctx, "users/1", val); err != nil {
		t.Fatal(err)
	}
	if err := db1.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}
	if !f.recoveryHasRows() {
		t.Fatal("want libSQL Recovery rows after SnapshotNow")
	}
	if err := db1.Close(); err != nil {
		t.Fatal(err)
	}

	db2 := f.open(ctx)
	t.Cleanup(func() { _ = db2.Close() })
	got, err := db2.Get(ctx, "users/1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(val) {
		t.Fatalf("restored %s want %s", got, val)
	}
	ok, err := db2.Exists(ctx, "users/1")
	if err != nil || !ok {
		t.Fatalf("exists: %v %v", ok, err)
	}
}

// TestE2E_LibSQLFile_MultiKeyHierarchy — C2
func TestE2E_LibSQLFile_MultiKeyHierarchy(t *testing.T) {
	ctx := context.Background()
	f := newLibSQLFileFixture(t)

	db1 := f.open(ctx)
	want := map[string]json.RawMessage{
		"plain": json.RawMessage(`"a"`),
		"a/b/c": json.RawMessage(`{"deep":true}`),
		"x/y":   json.RawMessage(`[1,2,3]`),
		"solo":  json.RawMessage(`42`),
	}
	for k, v := range want {
		if err := db1.Set(ctx, k, v); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}
	tx, err := db1.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Set(ctx, "tx/key", json.RawMessage(`{"tx":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	want["tx/key"] = json.RawMessage(`{"tx":true}`)

	if err := db1.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}
	_ = db1.Close()

	db2 := f.open(ctx)
	t.Cleanup(func() { _ = db2.Close() })
	for k, v := range want {
		got, err := db2.Get(ctx, k)
		if err != nil {
			t.Fatalf("get %s: %v", k, err)
		}
		if string(got) != string(v) {
			t.Fatalf("%s: got %s want %s", k, got, v)
		}
	}
}

// TestE2E_LibSQLFile_UnsavedNotRestored — C3
func TestE2E_LibSQLFile_UnsavedNotRestored(t *testing.T) {
	ctx := context.Background()
	f := newLibSQLFileFixture(t)

	db1 := f.open(ctx)
	if err := db1.Set(ctx, "ephemeral", json.RawMessage(`{"saved":false}`)); err != nil {
		t.Fatal(err)
	}
	// Intentionally no SnapshotNow.
	if err := db1.Close(); err != nil {
		t.Fatal(err)
	}
	if f.recoveryHasRows() {
		t.Fatal("recovery rows must be absent without SnapshotNow")
	}

	db2 := f.open(ctx)
	t.Cleanup(func() { _ = db2.Close() })
	_, err := db2.Get(ctx, "ephemeral")
	if !es4.IsNotFound(err) {
		t.Fatalf("unsaved key must be ErrNotFound after restart, got %v", err)
	}
}

// TestE2E_LibSQLFile_EmptyStartup — C4
func TestE2E_LibSQLFile_EmptyStartup(t *testing.T) {
	ctx := context.Background()
	f := newLibSQLFileFixture(t)
	// Paths cleared by fixture; recovery missing → empty State continue.
	db := f.open(ctx)
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	ok, err := db.Exists(ctx, "any")
	if err != nil || ok {
		t.Fatalf("empty state: exists=%v err=%v", ok, err)
	}
	_, err = db.Get(ctx, "any")
	if !errors.Is(err, es4.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// TestE2E_LibSQLFile_CleanupIdempotent — C5
func TestE2E_LibSQLFile_CleanupIdempotent(t *testing.T) {
	ctx := context.Background()
	f := newLibSQLFileFixture(t)

	db := f.open(ctx)
	_ = db.Set(ctx, "k", json.RawMessage(`1`))
	if err := db.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	if !f.recoveryHasRows() {
		t.Fatal("want recovery before cleanup")
	}
	if err := f.clear(); err != nil {
		t.Fatalf("first clear: %v", err)
	}
	// Second clear on already-empty paths must not error (idempotent).
	if err := f.clear(); err != nil {
		t.Fatalf("second clear: %v", err)
	}
	if f.recoveryHasRows() {
		t.Fatal("want empty recovery after clear")
	}
	if _, err := os.Stat(f.recoveryDB); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want recovery db removed, err=%v", err)
	}
}
