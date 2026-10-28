package es4_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/internal/state"
	"github.com/b4moss/es4/packages/go/pkg/es4"
	"github.com/b4moss/es4/packages/go/pkg/options"
)

func TestDB_WithExpiresAt(t *testing.T) {
	// Not parallel: SetNowFunc is process-global.
	ctx := context.Background()
	db, err := es4.Open(ctx, options.Options{MemoryOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	fixed := time.Date(2032, 3, 1, 10, 0, 0, 0, time.UTC)
	state.SetNowFunc(func() time.Time { return fixed })
	t.Cleanup(func() { state.SetNowFunc(nil) })

	future := fixed.Add(30 * time.Minute)
	past := fixed.Add(-time.Second)
	val := json.RawMessage(`{"ok":true}`)

	if err := db.Set(ctx, "a", val, es4.WithExpiresAt(future)); err != nil {
		t.Fatal(err)
	}
	got, err := db.Get(ctx, "a")
	if err != nil || string(got) != string(val) {
		t.Fatalf("get: %s err=%v", got, err)
	}

	if err := db.Set(ctx, "b", val, es4.WithExpiresAt(past)); err != nil {
		t.Fatal(err)
	}
	_, err = db.Get(ctx, "b")
	if !errors.Is(err, es4.ErrNotFound) {
		t.Fatalf("past: %v", err)
	}

	tx, err := db.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Set(ctx, "c", val, es4.WithExpiresAt(future)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	ok, err := db.Exists(ctx, "c")
	if err != nil || !ok {
		t.Fatalf("tx commit exists: %v %v", ok, err)
	}
}
