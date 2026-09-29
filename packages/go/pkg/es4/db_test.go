package es4_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/pkg/es4"
	"github.com/b4moss/es4/packages/go/pkg/options"
)

func TestDB_StateAPI(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := es4.Open(ctx, options.Options{MemoryOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	val := json.RawMessage(`{"hello":"world"}`)
	if err := db.Set(ctx, "users/1", val); err != nil {
		t.Fatal(err)
	}
	got, err := db.Get(ctx, "users/1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(val) {
		t.Fatalf("got %s", got)
	}
	ok, err := db.Exists(ctx, "users/1")
	if err != nil || !ok {
		t.Fatalf("exists: %v %v", ok, err)
	}
	ok, err = db.Exists(ctx, "users/2")
	if err != nil || ok {
		t.Fatalf("missing exists: %v %v", ok, err)
	}
	_, err = db.Get(ctx, "users/2")
	if !es4.IsNotFound(err) {
		t.Fatalf("get missing: %v", err)
	}
	err = db.Delete(ctx, "users/2")
	if !errors.Is(err, es4.ErrNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
	if err := db.Delete(ctx, "users/1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Clear(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDB_InvalidKeys(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := es4.Open(ctx, options.Options{MemoryOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, key := range []string{"", "/a", "a/", "a//b"} {
		err := db.Set(ctx, key, json.RawMessage(`1`))
		if !errors.Is(err, es4.ErrInvalidKey) {
			t.Fatalf("key %q: got %v", key, err)
		}
	}
}

func TestDB_RestoreOnStartup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "rp.json")

	opts := options.Options{
		SnapshotInterval: 0,
		RestoreOnStartup: true,
		MemoryOnly:       false,
		RecoveryPath:     path,
	}
	db1, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options:          opts,
		SkipAsyncRestore: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db1.Set(ctx, "a/b", json.RawMessage(`{"n":42}`)); err != nil {
		t.Fatal(err)
	}
	if err := db1.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db1.Close(); err != nil {
		t.Fatal(err)
	}

	db2, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options:          opts,
		SkipAsyncRestore: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db2.Close() })
	got, err := db2.Get(ctx, "a/b")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"n":42}` {
		t.Fatalf("got %s", got)
	}
}

func TestDB_RestoreMissing_ContinuesEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "missing.json")
	db, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options: options.Options{
			RestoreOnStartup: true,
			RecoveryPath:     path,
			SnapshotInterval: 0,
		},
		SkipAsyncRestore: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ok, err := db.Exists(ctx, "anything")
	if err != nil || ok {
		t.Fatalf("want empty state: ok=%v err=%v", ok, err)
	}
}

func TestDB_RestoreUnreadable_ContinuesEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`not-json{{{`), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options: options.Options{
			RestoreOnStartup: true,
			RecoveryPath:     path,
			SnapshotInterval: 0,
		},
		SkipAsyncRestore: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// API accepted; state remains empty after failed restore.
	if err := db.Set(ctx, "ok", json.RawMessage(`true`)); err != nil {
		t.Fatal(err)
	}
}

func TestDB_AcceptAPIBeforeAsyncRestoreCompletes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "rp.json")

	// Seed a recovery point.
	seed, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options: options.Options{
			RestoreOnStartup: false,
			RecoveryPath:     path,
			SnapshotInterval: 0,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = seed.Set(ctx, "seeded", json.RawMessage(`1`))
	if err := seed.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}
	_ = seed.Close()

	db, err := es4.Open(ctx, options.Options{
		RestoreOnStartup: true,
		RecoveryPath:     path,
		SnapshotInterval: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Immediately accept State API (may or may not see seeded yet).
	if err := db.Set(ctx, "live", json.RawMessage(`2`)); err != nil {
		t.Fatal(err)
	}
	ok, err := db.Exists(ctx, "live")
	if err != nil || !ok {
		t.Fatalf("live key: ok=%v err=%v", ok, err)
	}
	db.WaitRestore()
}

func TestDB_MemoryOnly_NoRecoveryWrite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "should-not-exist.json")
	db, err := es4.Open(ctx, options.Options{
		MemoryOnly:       true,
		RecoveryPath:     path,
		SnapshotInterval: 30 * time.Second,
		RestoreOnStartup: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Set(ctx, "a", json.RawMessage(`1`)); err != nil {
		t.Fatal(err)
	}
	if err := db.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("recovery file should not exist, err=%v", err)
	}
}

func TestDB_RestoreOnStartupFalse_SkipRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rp.json")

	seed, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options: options.Options{RecoveryPath: path, RestoreOnStartup: false, SnapshotInterval: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = seed.Set(ctx, "hidden", json.RawMessage(`9`))
	_ = seed.SnapshotNow(ctx)
	_ = seed.Close()

	db, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options: options.Options{
			RecoveryPath:     path,
			RestoreOnStartup: false,
			SnapshotInterval: 0,
		},
		SkipAsyncRestore: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ok, err := db.Exists(ctx, "hidden")
	if err != nil || ok {
		t.Fatalf("should skip restore: ok=%v err=%v", ok, err)
	}
}

func TestDB_PeriodicSnapshot_WritesRecovery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rp.json")
	db, err := es4.Open(ctx, options.Options{
		RecoveryPath:     path,
		RestoreOnStartup: false,
		SnapshotInterval: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_ = db.Set(ctx, "tick", json.RawMessage(`true`))

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for periodic recovery write")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDB_CloseIdempotent(t *testing.T) {
	t.Parallel()
	db, err := es4.Open(context.Background(), options.Options{MemoryOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	err = db.Set(context.Background(), "a", json.RawMessage(`1`))
	if !errors.Is(err, es4.ErrClosed) {
		t.Fatalf("got %v", err)
	}
}

func TestDB_EffectiveConsumed(t *testing.T) {
	t.Parallel()
	// Raw Options with MemoryOnly true + recovery fields set must be Effective()'d.
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rp.json")
	db, err := es4.Open(ctx, options.Options{
		MemoryOnly:       true,
		RecoveryPath:     path,
		RestoreOnStartup: true,
		SnapshotInterval: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("Effective memory_only must skip recovery writes")
	}
}
