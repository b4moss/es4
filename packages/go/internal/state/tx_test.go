package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/b4moss/es4/packages/go/internal/state"
)

func runTxSuite(t *testing.T, open func(t *testing.T) state.Store) {
	t.Helper()
	ctx := context.Background()

	t.Run("commit_visible", func(t *testing.T) {
		t.Parallel()
		st := open(t)
		tx, err := st.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Set(ctx, "k", json.RawMessage(`1`)); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		got, err := st.Get(ctx, "k")
		if err != nil || string(got) != "1" {
			t.Fatalf("got %s err=%v", got, err)
		}
	})

	t.Run("rollback_missing", func(t *testing.T) {
		t.Parallel()
		st := open(t)
		tx, err := st.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Set(ctx, "k", json.RawMessage(`1`)); err != nil {
			t.Fatal(err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		_, err = st.Get(ctx, "k")
		if !errors.Is(err, state.ErrNotFound) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("tx_visibility", func(t *testing.T) {
		t.Parallel()
		st := open(t)
		_ = st.Set(ctx, "base", json.RawMessage(`0`))
		tx, err := st.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if err := tx.Set(ctx, "txk", json.RawMessage(`2`)); err != nil {
			t.Fatal(err)
		}
		got, err := tx.Get(ctx, "txk")
		if err != nil || string(got) != "2" {
			t.Fatalf("tx get: %s err=%v", got, err)
		}
		ok, err := tx.Exists(ctx, "txk")
		if err != nil || !ok {
			t.Fatalf("tx exists: %v %v", ok, err)
		}
		ok, err = tx.Exists(ctx, "missing")
		if err != nil || ok {
			t.Fatalf("tx missing exists: %v %v", ok, err)
		}
		if err := tx.Delete(ctx, "base"); err != nil {
			t.Fatal(err)
		}
		ok, err = tx.Exists(ctx, "base")
		if err != nil || ok {
			t.Fatalf("tx delete visibility: %v %v", ok, err)
		}
		if err := tx.Clear(ctx); err != nil {
			t.Fatal(err)
		}
		ok, err = tx.Exists(ctx, "txk")
		if err != nil || ok {
			t.Fatalf("tx clear: %v %v", ok, err)
		}
	})

	t.Run("reuse_after_commit", func(t *testing.T) {
		t.Parallel()
		st := open(t)
		tx, err := st.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_ = tx.Set(ctx, "k", json.RawMessage(`1`))
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		err = tx.Set(ctx, "k", json.RawMessage(`2`))
		if !errors.Is(err, state.ErrTxDone) {
			t.Fatalf("got %v want ErrTxDone", err)
		}
		if err := tx.Commit(); !errors.Is(err, state.ErrTxDone) {
			t.Fatalf("commit reuse: %v", err)
		}
		if err := tx.Rollback(); !errors.Is(err, state.ErrTxDone) {
			t.Fatalf("rollback reuse: %v", err)
		}
	})

	t.Run("nested_begin", func(t *testing.T) {
		t.Parallel()
		st := open(t)
		tx, err := st.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		_, err = st.BeginTx(ctx)
		if !errors.Is(err, state.ErrNestedTx) {
			t.Fatalf("got %v want ErrNestedTx", err)
		}
	})

	t.Run("close_rolls_back", func(t *testing.T) {
		t.Parallel()
		st := open(t)
		tx, err := st.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Set(ctx, "k", json.RawMessage(`1`)); err != nil {
			t.Fatal(err)
		}
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
		// Reopen for durable backends to confirm rollback; Memory is gone.
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		st := open(t)
		tx, err := st.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		_, err = tx.Get(ctx, "missing")
		if !errors.Is(err, state.ErrNotFound) {
			t.Fatalf("get: %v", err)
		}
		err = tx.Delete(ctx, "missing")
		if !errors.Is(err, state.ErrNotFound) {
			t.Fatalf("delete: %v", err)
		}
		err = tx.Set(ctx, "", json.RawMessage(`1`))
		if !errors.Is(err, state.ErrInvalidKey) {
			t.Fatalf("key: %v", err)
		}
		err = tx.Set(ctx, "k", json.RawMessage(`{`))
		if !errors.Is(err, state.ErrInvalidValue) {
			t.Fatalf("value: %v", err)
		}
		cctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := tx.Set(cctx, "k", json.RawMessage(`1`)); !errors.Is(err, context.Canceled) {
			t.Fatalf("ctx: %v", err)
		}
	})

	t.Run("concurrent_smoke", func(t *testing.T) {
		t.Parallel()
		st := open(t)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				key := "c/" + string(rune('a'+i))
				_ = st.Set(ctx, key, json.RawMessage(`1`))
				tx, err := st.BeginTx(ctx)
				if err != nil {
					// nested contention may yield ErrNestedTx; that's fine
					return
				}
				_ = tx.Set(ctx, key, json.RawMessage(`2`))
				if i%2 == 0 {
					_ = tx.Commit()
				} else {
					_ = tx.Rollback()
				}
				_, _ = st.Get(ctx, key)
			}(i)
		}
		wg.Wait()
	})
}

func TestTx_Memory(t *testing.T) {
	t.Parallel()
	runTxSuite(t, func(t *testing.T) state.Store {
		t.Helper()
		m := state.NewMemory()
		t.Cleanup(func() { _ = m.Close() })
		return m
	})
}

func TestTx_SQLite(t *testing.T) {
	t.Parallel()
	runTxSuite(t, func(t *testing.T) state.Store {
		t.Helper()
		s, err := state.OpenSQLite(filepath.Join(t.TempDir(), "tx.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		return s
	})
}

func TestTx_SQLite_CloseRollsBackDurable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rollback.db")
	s, err := state.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Set(ctx, "k", json.RawMessage(`1`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := state.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	_, err = s2.Get(ctx, "k")
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("uncommitted close must rollback: %v", err)
	}
}
