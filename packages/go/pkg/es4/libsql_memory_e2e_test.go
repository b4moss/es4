//go:build e2e

package es4_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/b4moss/es4/packages/go/pkg/es4"
	"github.com/b4moss/es4/packages/go/pkg/options"
)

// In-memory libSQL Recovery E2E — process-local Recovery DB
// (docs/tests/e2e/e2e-spec.md §L.M).
//
// Adopts common catalog C2, C4, C5 only (no C1/C3 restart/restore).
// recovery_backend=libsql with :memory: DSN; empty state_path (Memory State).
// SnapshotNow/Save is exercised in-process. No RustFS.

func openLibSQLMemory(t *testing.T, ctx context.Context) *es4.DB {
	t.Helper()
	db, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options: options.Options{
			StatePath:         "",
			RecoveryBackend:   options.RecoveryBackendLibSQL,
			RecoveryLibSQLURL: ":memory:",
			RestoreOnStartup:  true,
			MemoryOnly:        false,
			SnapshotInterval:  0,
		},
		SkipAsyncRestore: true,
	})
	if err != nil {
		t.Fatalf("OpenWith libsql :memory:: %v", err)
	}
	return db
}

// TestE2E_LibSQLMemory_MultiKeyOps — C2 (no restart; SnapshotNow in-process)
func TestE2E_LibSQLMemory_MultiKeyOps(t *testing.T) {
	ctx := context.Background()
	db := openLibSQLMemory(t, ctx)
	t.Cleanup(func() { _ = db.Close() })

	want := map[string]json.RawMessage{
		"plain": json.RawMessage(`"a"`),
		"a/b/c": json.RawMessage(`{"deep":true}`),
		"x/y":   json.RawMessage(`[1,2,3]`),
		"solo":  json.RawMessage(`42`),
	}
	for k, v := range want {
		if err := db.Set(ctx, k, v); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}
	tx, err := db.BeginTx(ctx)
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

	// Exercise Recovery Save path in-process (DB dies with process / connection).
	if err := db.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}

	for k, v := range want {
		got, err := db.Get(ctx, k)
		if err != nil {
			t.Fatalf("get %s: %v", k, err)
		}
		if string(got) != string(v) {
			t.Fatalf("%s: got %s want %s", k, got, v)
		}
	}
}

// TestE2E_LibSQLMemory_EmptyStartup — C4 (no restart/recovery restore)
func TestE2E_LibSQLMemory_EmptyStartup(t *testing.T) {
	ctx := context.Background()
	db := openLibSQLMemory(t, ctx)
	t.Cleanup(func() { _ = db.Close() })

	ok, err := db.Exists(ctx, "any")
	if err != nil || ok {
		t.Fatalf("empty state: exists=%v err=%v", ok, err)
	}
	_, err = db.Get(ctx, "any")
	if !errors.Is(err, es4.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	val := json.RawMessage(`{"n":1}`)
	if err := db.Set(ctx, "k", val); err != nil {
		t.Fatal(err)
	}
	got, err := db.Get(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(val) {
		t.Fatalf("got %s want %s", got, val)
	}
}

// TestE2E_LibSQLMemory_CleanupIdempotent — C5 (in-process Clear/Close)
func TestE2E_LibSQLMemory_CleanupIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openLibSQLMemory(t, ctx)

	_ = db.Set(ctx, "k", json.RawMessage(`1`))
	if err := db.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Clear(ctx); err != nil {
		t.Fatalf("first clear: %v", err)
	}
	if err := db.Clear(ctx); err != nil {
		t.Fatalf("second clear: %v", err)
	}
	ok, err := db.Exists(ctx, "k")
	if err != nil || ok {
		t.Fatalf("want empty after clear: ok=%v err=%v", ok, err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}
