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

// In-memory layer E2E — no persistence expected (docs/tests/e2e/e2e-spec.md §M).
//
// Adopts common catalog C2, C4, C5 only (no C1/C3 restart/restore).
// memory_only=true; flow is open → basic multi-key ops. Still //go:build e2e
// so real runtime + library integration is exercised. No RustFS.

func openMemory(t *testing.T, ctx context.Context) *es4.DB {
	t.Helper()
	db, err := es4.Open(ctx, options.Options{
		MemoryOnly:       true,
		SnapshotInterval: 0,
		// Recovery / state_path ignored via Effective when MemoryOnly.
		RecoveryPath: "",
		StatePath:    "",
	})
	if err != nil {
		t.Fatalf("Open memory_only: %v", err)
	}
	return db
}

// TestE2E_Memory_MultiKeyOps — C2 (no restart; basic multi-key after open)
func TestE2E_Memory_MultiKeyOps(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t, ctx)
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

// TestE2E_Memory_EmptyStartup — C4 (no restart/recovery; empty + basic ops)
func TestE2E_Memory_EmptyStartup(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t, ctx)
	t.Cleanup(func() { _ = db.Close() })

	ok, err := db.Exists(ctx, "any")
	if err != nil || ok {
		t.Fatalf("empty state: exists=%v err=%v", ok, err)
	}
	_, err = db.Get(ctx, "any")
	if !errors.Is(err, es4.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	// Basic ops after open on empty store.
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

// TestE2E_Memory_CleanupIdempotent — C5 (minimal in-process cleanup)
func TestE2E_Memory_CleanupIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t, ctx)

	_ = db.Set(ctx, "k", json.RawMessage(`1`))
	if err := db.Clear(ctx); err != nil {
		t.Fatalf("first clear: %v", err)
	}
	// Second Clear on already-empty store must not error (idempotent).
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
	// Second Close is idempotent on DB.
	if err := db.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}
