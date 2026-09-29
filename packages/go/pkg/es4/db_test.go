package es4_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/internal/recovery"
	"github.com/b4moss/es4/packages/go/internal/state"
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

func TestOpen_BackendSelection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("memory_only", func(t *testing.T) {
		t.Parallel()
		db, err := es4.Open(ctx, options.Options{
			MemoryOnly: true,
			StatePath:  filepath.Join(t.TempDir(), "ignored.db"),
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if _, ok := db.StateForTest().(*state.Memory); !ok {
			t.Fatalf("want Memory, got %T", db.StateForTest())
		}
	})

	t.Run("state_path", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "state.db")
		db, err := es4.Open(ctx, options.Options{
			StatePath:        path,
			SnapshotInterval: 0,
			RestoreOnStartup: false,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if _, ok := db.StateForTest().(*state.SQLite); !ok {
			t.Fatalf("want SQLite, got %T", db.StateForTest())
		}
	})

	t.Run("empty_state_path", func(t *testing.T) {
		t.Parallel()
		db, err := es4.Open(ctx, options.Options{SnapshotInterval: 0, RestoreOnStartup: false})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if _, ok := db.StateForTest().(*state.Memory); !ok {
			t.Fatalf("want Memory, got %T", db.StateForTest())
		}
	})

	t.Run("open_with_injection", func(t *testing.T) {
		t.Parallel()
		injected := state.NewMemory()
		db, err := es4.OpenWith(ctx, es4.OpenConfig{
			Options: options.Options{
				StatePath:        filepath.Join(t.TempDir(), "would-be-sqlite.db"),
				SnapshotInterval: 0,
				RestoreOnStartup: false,
			},
			State: injected,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if db.StateForTest() != injected {
			t.Fatal("OpenWith injection must take priority")
		}
	})
}

func TestOpen_SQLite_RestoreOnStartup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.db")
	recPath := filepath.Join(dir, "rp.json")
	opts := options.Options{
		StatePath:        statePath,
		RecoveryPath:     recPath,
		RestoreOnStartup: true,
		SnapshotInterval: 0,
	}
	db1, err := es4.OpenWith(ctx, es4.OpenConfig{Options: opts, SkipAsyncRestore: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = db1.Set(ctx, "a", json.RawMessage(`{"ok":true}`))
	if err := db1.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}
	_ = db1.Close()

	// New empty sqlite + restore from recovery.
	_ = os.Remove(statePath)
	db2, err := es4.OpenWith(ctx, es4.OpenConfig{Options: opts, SkipAsyncRestore: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db2.Close() })
	got, err := db2.Get(ctx, "a")
	if err != nil || string(got) != `{"ok":true}` {
		t.Fatalf("got %s err=%v", got, err)
	}
}

func TestOpen_SQLite_RestoreMissing_Continues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	db, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options: options.Options{
			StatePath:        filepath.Join(dir, "s.db"),
			RecoveryPath:     filepath.Join(dir, "missing.json"),
			RestoreOnStartup: true,
			SnapshotInterval: 0,
		},
		SkipAsyncRestore: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ok, err := db.Exists(ctx, "x")
	if err != nil || ok {
		t.Fatalf("want empty: ok=%v err=%v", ok, err)
	}
}

func TestSnapshot_SQLite_EntriesEnvelope_NotDBCopy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.db")
	recPath := filepath.Join(dir, "rp.json")
	db, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options: options.Options{
			StatePath:        statePath,
			RecoveryPath:     recPath,
			RestoreOnStartup: false,
			SnapshotInterval: 0,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_ = db.Set(ctx, "users/1", json.RawMessage(`{"n":7}`))
	if err := db.SnapshotNow(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatal(err)
	}
	// Must be JSON envelope with entries — not a SQLite DB binary.
	if len(data) < 2 || data[0] != '{' {
		t.Fatalf("recovery must be JSON envelope, got prefix %q", data[:min(16, len(data))])
	}
	if !json.Valid(data) {
		t.Fatal("recovery must be valid JSON")
	}
	var env struct {
		Version   int             `json:"version"`
		CreatedAt time.Time       `json:"created_at"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	if env.Version != 1 {
		t.Fatalf("version: %d", env.Version)
	}
	if env.CreatedAt.IsZero() {
		t.Fatal("created_at required")
	}
	var payload struct {
		Entries map[string]json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if string(payload.Entries["users/1"]) != `{"n":7}` {
		t.Fatalf("entries: %#v", payload.Entries)
	}
}

func TestDB_Tx_MemoryAndSQLite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	backends := []struct {
		name string
		opts options.Options
	}{
		{"memory", options.Options{MemoryOnly: true}},
		{"sqlite", options.Options{
			StatePath:        filepath.Join(t.TempDir(), "tx.db"),
			SnapshotInterval: 0,
			RestoreOnStartup: false,
		}},
	}
	for _, bc := range backends {
		bc := bc
		t.Run(bc.name, func(t *testing.T) {
			t.Parallel()
			db, err := es4.Open(ctx, bc.opts)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })

			tx, err := db.BeginTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := tx.Set(ctx, "k", json.RawMessage(`1`)); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			got, err := db.Get(ctx, "k")
			if err != nil || string(got) != "1" {
				t.Fatalf("after commit: %s err=%v", got, err)
			}

			tx2, err := db.BeginTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_ = tx2.Set(ctx, "r", json.RawMessage(`2`))
			if err := tx2.Rollback(); err != nil {
				t.Fatal(err)
			}
			_, err = db.Get(ctx, "r")
			if !errors.Is(err, es4.ErrNotFound) {
				t.Fatalf("after rollback: %v", err)
			}

			tx3, err := db.BeginTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_ = tx3.Set(ctx, "x", json.RawMessage(`3`))
			if err := tx3.Commit(); err != nil {
				t.Fatal(err)
			}
			if err := tx3.Set(ctx, "x", json.RawMessage(`4`)); !errors.Is(err, es4.ErrTxDone) {
				t.Fatalf("reuse: %v", err)
			}

			tx4, err := db.BeginTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.BeginTx(ctx)
			if !errors.Is(err, es4.ErrNestedTx) {
				t.Fatalf("nested: %v", err)
			}
			_ = tx4.Rollback()
		})
	}
}

func TestDB_Tx_CloseRollsBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "close-rb.db")
	db, err := es4.Open(ctx, options.Options{
		StatePath:        path,
		SnapshotInterval: 0,
		RestoreOnStartup: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Set(ctx, "k", json.RawMessage(`1`))
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db2, err := es4.Open(ctx, options.Options{
		StatePath:        path,
		SnapshotInterval: 0,
		RestoreOnStartup: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db2.Close() })
	_, err = db2.Get(ctx, "k")
	if !errors.Is(err, es4.ErrNotFound) {
		t.Fatalf("close must rollback open tx: %v", err)
	}
}

func TestDB_Tx_ConcurrentSmoke(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := es4.Open(ctx, options.Options{
		StatePath:        filepath.Join(t.TempDir(), "conc.db"),
		SnapshotInterval: 0,
		RestoreOnStartup: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "k/" + string(rune('a'+i%8))
			_ = db.Set(ctx, key, json.RawMessage(`1`))
			tx, err := db.BeginTx(ctx)
			if err != nil {
				return
			}
			_ = tx.Set(ctx, key, json.RawMessage(`2`))
			if i%2 == 0 {
				_ = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
		}(i)
	}
	wg.Wait()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestOpen_RecoveryBackendSelection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("empty_backend_with_path_is_file", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "rp.json")
		db, err := es4.OpenWith(ctx, es4.OpenConfig{
			Options: options.Options{
				RecoveryPath:     path,
				SnapshotInterval: 0,
				RestoreOnStartup: false,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		rec := db.RecoveryForTest()
		if _, ok := rec.(*recovery.File); !ok {
			t.Fatalf("want *recovery.File, got %T", rec)
		}
	})

	t.Run("libsql_file_url", func(t *testing.T) {
		t.Parallel()
		dbPath := filepath.Join(t.TempDir(), "rec.db")
		db, err := es4.OpenWith(ctx, es4.OpenConfig{
			Options: options.Options{
				RecoveryBackend:  "libsql",
				RecoveryLibSQLURL: "file:" + dbPath,
				SnapshotInterval: 0,
				RestoreOnStartup: false,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if _, ok := db.RecoveryForTest().(*recovery.LibSQL); !ok {
			t.Fatalf("want *recovery.LibSQL, got %T", db.RecoveryForTest())
		}
		_ = db.Set(ctx, "k", json.RawMessage(`1`))
		if err := db.SnapshotNow(ctx); err != nil {
			t.Fatal(err)
		}
		db2, err := es4.OpenWith(ctx, es4.OpenConfig{
			Options: options.Options{
				RecoveryBackend:   "libsql",
				RecoveryLibSQLURL: "file:" + dbPath,
				SnapshotInterval:  0,
				RestoreOnStartup:  true,
			},
			SkipAsyncRestore: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db2.Close() })
		got, err := db2.Get(ctx, "k")
		if err != nil || string(got) != "1" {
			t.Fatalf("got %s err=%v", got, err)
		}
	})

	t.Run("libsql_url_required", func(t *testing.T) {
		t.Parallel()
		_, err := es4.Open(ctx, options.Options{RecoveryBackend: "libsql", SnapshotInterval: 0})
		if err == nil {
			t.Fatal("want error when libsql url missing")
		}
	})

	t.Run("object_bucket_required", func(t *testing.T) {
		t.Parallel()
		_, err := es4.Open(ctx, options.Options{RecoveryBackend: "object", SnapshotInterval: 0})
		if err == nil {
			t.Fatal("want error when bucket missing")
		}
	})

	t.Run("object_injection_openwith", func(t *testing.T) {
		t.Parallel()
		fake := recovery.NewMemoryObject()
		store, err := recovery.NewObject(recovery.ObjectConfig{
			Bucket: "b",
			Prefix: "p",
			Client: fake,
		})
		if err != nil {
			t.Fatal(err)
		}
		db, err := es4.OpenWith(ctx, es4.OpenConfig{
			Options: options.Options{
				RecoveryBackend:  "object",
				RecoveryS3Bucket: "would-use-aws",
				SnapshotInterval: 0,
				RestoreOnStartup: false,
			},
			Recovery: store,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if db.RecoveryForTest() != store {
			t.Fatal("OpenWith recovery injection must win")
		}
		_ = db.Set(ctx, "a", json.RawMessage(`true`))
		if err := db.SnapshotNow(ctx); err != nil {
			t.Fatal(err)
		}
		keys, _ := fake.ListObjectKeys(ctx, "b", "p/")
		if len(keys) != 1 {
			t.Fatalf("want 1 object, got %v", keys)
		}
	})

	t.Run("memory_only_ignores_recovery", func(t *testing.T) {
		t.Parallel()
		db, err := es4.Open(ctx, options.Options{
			MemoryOnly:        true,
			RecoveryBackend:   "libsql",
			RecoveryLibSQLURL: "file:" + filepath.Join(t.TempDir(), "x.db"),
			RecoveryPath:      filepath.Join(t.TempDir(), "r.json"),
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if db.RecoveryForTest() != nil {
			t.Fatal("memory_only must skip recovery adapter")
		}
	})

	t.Run("file_ttl_generations_restore", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "gens")
		opts := options.Options{
			RecoveryPath:     dir,
			RecoveryBackend:  "file",
			RecoveryTTL:      time.Hour,
			SnapshotInterval: 0,
			RestoreOnStartup: true,
		}
		db1, err := es4.OpenWith(ctx, es4.OpenConfig{Options: opts, SkipAsyncRestore: true})
		if err != nil {
			t.Fatal(err)
		}
		_ = db1.Set(ctx, "g", json.RawMessage(`9`))
		if err := db1.SnapshotNow(ctx); err != nil {
			t.Fatal(err)
		}
		_ = db1.Close()

		db2, err := es4.OpenWith(ctx, es4.OpenConfig{Options: opts, SkipAsyncRestore: true})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db2.Close() })
		got, err := db2.Get(ctx, "g")
		if err != nil || string(got) != "9" {
			t.Fatalf("got %s err=%v", got, err)
		}
	})
}
