//go:build e2e

package es4_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/internal/e2e"
	"github.com/b4moss/es4/packages/go/internal/recovery"
	"github.com/b4moss/es4/packages/go/pkg/es4"
	"github.com/b4moss/es4/packages/go/pkg/options"
)

// Object Recovery × RustFS E2E (docs/tests/e2e/e2e-spec.md §5.1–§5.5).
//
// Requires RustFS from docker/e2e/docker-compose.yml.
// State uses in-process Memory (empty state_path) so Close→re-Open proves
// Recovery → Restore only — not SQLite durability.

var (
	e2eClient *e2e.Client
	e2eCfg    e2e.Config
)

func TestMain(m *testing.M) {
	e2eCfg = e2e.ConfigFromEnv()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := e2e.WaitHealthy(ctx, e2eCfg.Endpoint); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: RustFS not reachable (%v)\n", err)
		fmt.Fprintf(os.Stderr, "e2e: start with: docker compose -f docker/e2e/docker-compose.yml up -d\n")
		os.Exit(1)
	}
	e2eClient = e2e.NewClient(e2eCfg)
	os.Exit(m.Run())
}

// fixture prepares a unique bucket+prefix, clears at start, and registers
// t.Cleanup that clears on success and failure.
type fixture struct {
	t      *testing.T
	bucket string
	prefix string
	client *e2e.Client
}

func newFixture(t *testing.T, caseName string) *fixture {
	t.Helper()
	ctx := context.Background()
	f := &fixture{
		t:      t,
		bucket: e2e.UniqueBucket("es4-e2e"),
		prefix: e2e.UniquePrefix(caseName),
		client: e2eClient,
	}
	if err := f.client.EnsureBucket(ctx, f.bucket); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}
	if err := f.client.ClearPrefix(ctx, f.bucket, f.prefix); err != nil {
		t.Fatalf("start clear: %v", err)
	}
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := f.client.ClearPrefix(cctx, f.bucket, f.prefix); err != nil {
			t.Logf("cleanup clear prefix %s/%s: %v", f.bucket, f.prefix, err)
		}
		if err := f.client.DeleteBucketEmpty(cctx, f.bucket); err != nil {
			// Bucket may still have other prefixes from parallel cases — log only.
			t.Logf("cleanup delete bucket %s: %v", f.bucket, err)
		}
	})
	return f
}

func (f *fixture) open(ctx context.Context) *es4.DB {
	f.t.Helper()
	store, err := recovery.NewObject(recovery.ObjectConfig{
		Bucket:   f.bucket,
		Prefix:   f.prefix,
		Region:   e2eCfg.Region,
		Endpoint: e2eCfg.Endpoint,
		Client:   f.client,
	})
	if err != nil {
		f.t.Fatalf("NewObject: %v", err)
	}
	opts := options.Options{
		RecoveryBackend:   options.RecoveryBackendObject,
		RecoveryS3Bucket:  f.bucket,
		RecoveryS3Prefix:  f.prefix,
		RecoveryS3Region:  e2eCfg.Region,
		RecoveryS3Endpoint: e2eCfg.Endpoint,
		RestoreOnStartup:  true,
		MemoryOnly:        false,
		StatePath:         "", // Memory State; Recovery proves restart restore
		SnapshotInterval:  0,  // explicit SnapshotNow only
	}
	db, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options:          opts,
		Recovery:         store,
		SkipAsyncRestore: true,
	})
	if err != nil {
		f.t.Fatalf("OpenWith: %v", err)
	}
	return db
}

// TestE2E_ObjectRecovery_SaveRestartRestore — §5.1
// After Snapshot Save to Object, Close→re-Open restores only Snapshot-saved state.
func TestE2E_ObjectRecovery_SaveRestartRestore(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, "save-restart")

	db1 := f.open(ctx)
	val := json.RawMessage(`{"hello":"rustfs","n":1}`)
	if err := db1.Set(ctx, "users/1", val); err != nil {
		t.Fatal(err)
	}
	// Explicit Snapshot Save (not interval-dependent).
	if err := db1.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}
	keys, err := f.client.ListKeys(ctx, f.bucket, f.prefix)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) < 1 {
		t.Fatalf("want ≥1 object under prefix after Save, got %v", keys)
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

// TestE2E_ObjectRecovery_MultiKeyHierarchy — §5.2
func TestE2E_ObjectRecovery_MultiKeyHierarchy(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, "multi-key")

	db1 := f.open(ctx)
	want := map[string]json.RawMessage{
		"plain":     json.RawMessage(`"a"`),
		"a/b/c":     json.RawMessage(`{"deep":true}`),
		"x/y":       json.RawMessage(`[1,2,3]`),
		"solo":      json.RawMessage(`42`),
	}
	for k, v := range want {
		if err := db1.Set(ctx, k, v); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}
	// Tx path: Commit then Snapshot.
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

// TestE2E_ObjectRecovery_UnsavedNotRestored — §5.3
// Commit/Set without Snapshot Save must NOT come back after restart.
func TestE2E_ObjectRecovery_UnsavedNotRestored(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, "unsaved")

	db1 := f.open(ctx)
	if err := db1.Set(ctx, "ephemeral", json.RawMessage(`{"saved":false}`)); err != nil {
		t.Fatal(err)
	}
	// Intentionally no SnapshotNow — Close before any Save.
	if err := db1.Close(); err != nil {
		t.Fatal(err)
	}
	keys, err := f.client.ListKeys(ctx, f.bucket, f.prefix)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("prefix should be empty without Save, got %v", keys)
	}

	db2 := f.open(ctx)
	t.Cleanup(func() { _ = db2.Close() })
	_, err = db2.Get(ctx, "ephemeral")
	if !es4.IsNotFound(err) {
		t.Fatalf("unsaved key must be ErrNotFound after restart, got %v", err)
	}
}

// TestE2E_ObjectRecovery_EmptyBucketStartup — §5.4
func TestE2E_ObjectRecovery_EmptyBucketStartup(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, "empty-start")
	// Prefix already cleared by fixture.
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

// TestE2E_ObjectRecovery_CleanupIdempotent — §5.5
func TestE2E_ObjectRecovery_CleanupIdempotent(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, "cleanup-idem")

	db := f.open(ctx)
	_ = db.Set(ctx, "k", json.RawMessage(`1`))
	if err := db.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	keys, err := f.client.ListKeys(ctx, f.bucket, f.prefix)
	if err != nil || len(keys) < 1 {
		t.Fatalf("want objects before cleanup, keys=%v err=%v", keys, err)
	}
	if err := f.client.ClearPrefix(ctx, f.bucket, f.prefix); err != nil {
		t.Fatalf("first clear: %v", err)
	}
	// Second clear on already-empty prefix must not error (idempotent).
	if err := f.client.ClearPrefix(ctx, f.bucket, f.prefix); err != nil {
		t.Fatalf("second clear: %v", err)
	}
	keys, err = f.client.ListKeys(ctx, f.bucket, f.prefix)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("want empty after clear, got %v", keys)
	}
}
