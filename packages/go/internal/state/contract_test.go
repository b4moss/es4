package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/b4moss/es4/packages/go/internal/state"
)

// storeFactories drive the same Store contract against Memory, SQLite, and Redis.
// Firestore is exercised in firestore_contract_test.go (emulator required).
func storeFactories(t *testing.T) map[string]func(t *testing.T) state.Store {
	t.Helper()
	return map[string]func(t *testing.T) state.Store{
		"Memory": func(t *testing.T) state.Store {
			t.Helper()
			m := state.NewMemory()
			t.Cleanup(func() { _ = m.Close() })
			return m
		},
		"SQLite": func(t *testing.T) state.Store {
			t.Helper()
			s, err := state.OpenSQLite(filepath.Join(t.TempDir(), "contract.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = s.Close() })
			return s
		},
		"Redis": func(t *testing.T) state.Store {
			t.Helper()
			mr := miniredis.RunT(t)
			// Unique HASH per test name so parallel contract cases do not collide.
			st, err := state.OpenRedis(context.Background(), "redis://"+mr.Addr(), "contract/"+t.Name()+"/")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = st.Close() })
			return st
		},
	}
}

func forEachStore(t *testing.T, name string, fn func(t *testing.T, st state.Store)) {
	t.Helper()
	for backend, open := range storeFactories(t) {
		t.Run(backend+"/"+name, func(t *testing.T) {
			t.Parallel()
			fn(t, open(t))
		})
	}
}

func TestStoreContract_CRUD(t *testing.T) {
	t.Parallel()
	forEachStore(t, "crud", func(t *testing.T, st state.Store) {
		ctx := context.Background()
		val := json.RawMessage(`{"n":1}`)

		if err := st.Set(ctx, "a/b", val); err != nil {
			t.Fatal(err)
		}
		got, err := st.Get(ctx, "a/b")
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(val) {
			t.Fatalf("get: got %s want %s", got, val)
		}
		ok, err := st.Exists(ctx, "a/b")
		if err != nil || !ok {
			t.Fatalf("exists: ok=%v err=%v", ok, err)
		}
		if err := st.Delete(ctx, "a/b"); err != nil {
			t.Fatal(err)
		}
		ok, err = st.Exists(ctx, "a/b")
		if err != nil || ok {
			t.Fatalf("exists after delete: ok=%v err=%v", ok, err)
		}
		_, err = st.Get(ctx, "a/b")
		if !errors.Is(err, state.ErrNotFound) {
			t.Fatalf("get after delete: %v", err)
		}

		_ = st.Set(ctx, "x", json.RawMessage(`1`))
		_ = st.Set(ctx, "y", json.RawMessage(`2`))
		if err := st.Clear(ctx); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"x", "y"} {
			ok, err = st.Exists(ctx, k)
			if err != nil || ok {
				t.Fatalf("exists after clear %q: ok=%v err=%v", k, ok, err)
			}
		}
		exp, err := st.Export(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(exp) != 0 {
			t.Fatalf("export after clear: want empty, got %d keys", len(exp))
		}
	})
}

func TestStoreContract_Replace(t *testing.T) {
	t.Parallel()
	forEachStore(t, "replace", func(t *testing.T, st state.Store) {
		ctx := context.Background()
		_ = st.Set(ctx, "old", json.RawMessage(`0`))
		_ = st.Set(ctx, "keep-me-gone", json.RawMessage(`9`))

		entries := map[string]json.RawMessage{
			"new":  json.RawMessage(`{"a":1}`),
			"also": json.RawMessage(`true`),
		}
		if err := st.Replace(ctx, entries); err != nil {
			t.Fatal(err)
		}
		got, err := st.Get(ctx, "new")
		if err != nil || string(got) != `{"a":1}` {
			t.Fatalf("new: %s err=%v", got, err)
		}
		got, err = st.Get(ctx, "also")
		if err != nil || string(got) != `true` {
			t.Fatalf("also: %s err=%v", got, err)
		}
		_, err = st.Get(ctx, "old")
		if !errors.Is(err, state.ErrNotFound) {
			t.Fatalf("old should be gone: %v", err)
		}
		exp, err := st.Export(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(exp) != 2 {
			t.Fatalf("export size: got %d want 2", len(exp))
		}
		if _, ok := exp["old"]; ok {
			t.Fatal("export still has old")
		}

		if err := st.Replace(ctx, map[string]json.RawMessage{}); err != nil {
			t.Fatal(err)
		}
		exp, err = st.Export(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(exp) != 0 {
			t.Fatalf("empty replace: want empty export, got %d", len(exp))
		}
		_, err = st.Get(ctx, "new")
		if !errors.Is(err, state.ErrNotFound) {
			t.Fatalf("after empty replace: %v", err)
		}
	})
}

func TestStoreContract_BeginTxCommit(t *testing.T) {
	t.Parallel()
	forEachStore(t, "begin_tx_commit", func(t *testing.T, st state.Store) {
		ctx := context.Background()
		tx, err := st.BeginTx(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Set(ctx, "txk", json.RawMessage(`42`)); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		got, err := st.Get(ctx, "txk")
		if err != nil || string(got) != `42` {
			t.Fatalf("got %s err=%v", got, err)
		}
	})
}

func TestStoreContract_InvalidKeyValue(t *testing.T) {
	t.Parallel()
	invalidKeys := []string{"", "/a", "a/", "a//b"}
	forEachStore(t, "invalid_keys", func(t *testing.T, st state.Store) {
		ctx := context.Background()
		for _, key := range invalidKeys {
			err := st.Set(ctx, key, json.RawMessage(`1`))
			if !errors.Is(err, state.ErrInvalidKey) {
				t.Fatalf("Set(%q): got %v want ErrInvalidKey", key, err)
			}
			_, err = st.Get(ctx, key)
			if !errors.Is(err, state.ErrInvalidKey) {
				t.Fatalf("Get(%q): got %v want ErrInvalidKey", key, err)
			}
			err = st.Delete(ctx, key)
			if !errors.Is(err, state.ErrInvalidKey) {
				t.Fatalf("Delete(%q): got %v want ErrInvalidKey", key, err)
			}
			_, err = st.Exists(ctx, key)
			if !errors.Is(err, state.ErrInvalidKey) {
				t.Fatalf("Exists(%q): got %v want ErrInvalidKey", key, err)
			}
		}

		for _, bad := range []json.RawMessage{nil, {}, []byte(`{`)} {
			err := st.Set(ctx, "k", bad)
			if !errors.Is(err, state.ErrInvalidValue) {
				t.Fatalf("Set invalid %q: got %v want ErrInvalidValue", bad, err)
			}
		}

		for _, good := range []json.RawMessage{
			[]byte(`null`),
			[]byte(`[1,2]`),
			[]byte(`{"o":true}`),
			[]byte(`3`),
			[]byte(`"s"`),
		} {
			if err := st.Set(ctx, "good", good); err != nil {
				t.Fatalf("valid JSON %s: %v", good, err)
			}
		}
	})
}

func TestStoreContract_ExportDeepCopy(t *testing.T) {
	t.Parallel()
	forEachStore(t, "export_deep_copy", func(t *testing.T, st state.Store) {
		ctx := context.Background()
		orig := json.RawMessage(`{"n":1}`)
		if err := st.Set(ctx, "k", orig); err != nil {
			t.Fatal(err)
		}

		entries, err := st.Export(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if string(entries["k"]) != string(orig) {
			t.Fatalf("export: %s", entries["k"])
		}

		// Mutate returned RawMessage bytes.
		entries["k"][0] = 'X'
		entries["extra"] = json.RawMessage(`9`)
		delete(entries, "k")

		got, err := st.Get(ctx, "k")
		if err != nil || string(got) != string(orig) {
			t.Fatalf("store after mutate export: got %s err=%v", got, err)
		}
		again, err := st.Export(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if string(again["k"]) != string(orig) {
			t.Fatalf("re-export after mutate: %s", again["k"])
		}
		if _, ok := again["extra"]; ok {
			t.Fatal("mutated map key leaked into store")
		}

		// Store mutation must not change previously returned map.
		snapshot := again
		if err := st.Set(ctx, "k", json.RawMessage(`{"n":2}`)); err != nil {
			t.Fatal(err)
		}
		if string(snapshot["k"]) != string(orig) {
			t.Fatalf("prior export mutated by store Set: %s", snapshot["k"])
		}
	})
}

func TestStoreContract_ReplaceAtomicity(t *testing.T) {
	t.Parallel()
	forEachStore(t, "replace_atomicity", func(t *testing.T, st state.Store) {
		ctx := context.Background()
		oldSet := map[string]json.RawMessage{
			"old-a": json.RawMessage(`1`),
			"old-b": json.RawMessage(`2`),
		}
		newSet := map[string]json.RawMessage{
			"new-a": json.RawMessage(`3`),
			"new-b": json.RawMessage(`4`),
		}
		if err := st.Replace(ctx, oldSet); err != nil {
			t.Fatal(err)
		}

		var (
			wg       sync.WaitGroup
			stop     = make(chan struct{})
			mixedMu  sync.Mutex
			sawMixed bool
		)
		reader := func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				exp, err := st.Export(ctx)
				if err != nil {
					if errors.Is(err, state.ErrClosed) {
						return
					}
					continue
				}
				hasOld := false
				hasNew := false
				for k := range exp {
					if k == "old-a" || k == "old-b" {
						hasOld = true
					}
					if k == "new-a" || k == "new-b" {
						hasNew = true
					}
				}
				if hasOld && hasNew {
					mixedMu.Lock()
					sawMixed = true
					mixedMu.Unlock()
				}
				// Successful observation must be all-old or all-new (or empty mid-flight only if atomic empty — not expected here).
				if len(exp) > 0 && !( (hasOld && !hasNew && len(exp) == 2) || (hasNew && !hasOld && len(exp) == 2) ) {
					// Allow only pure old or pure new sets of size 2.
					mixedMu.Lock()
					sawMixed = true
					mixedMu.Unlock()
				}
			}
		}

		wg.Add(2)
		go reader()
		go reader()

		for i := 0; i < 40; i++ {
			if err := st.Replace(ctx, newSet); err != nil {
				t.Fatal(err)
			}
			if err := st.Replace(ctx, oldSet); err != nil {
				t.Fatal(err)
			}
		}
		if err := st.Replace(ctx, newSet); err != nil {
			t.Fatal(err)
		}

		close(stop)
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("readers did not stop")
		}

		mixedMu.Lock()
		mixed := sawMixed
		mixedMu.Unlock()
		if mixed {
			t.Fatal("observed torn mixed old+new entry set during Replace")
		}

		got, err := st.Get(ctx, "new-a")
		if err != nil || string(got) != `3` {
			t.Fatalf("final new-a: %s err=%v", got, err)
		}
		_, err = st.Get(ctx, "old-a")
		if !errors.Is(err, state.ErrNotFound) {
			t.Fatalf("final old-a: %v", err)
		}
	})
}

func TestStoreContract_AfterClose(t *testing.T) {
	t.Parallel()
	for backend, open := range storeFactories(t) {
		t.Run(backend+"/after_close", func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			st := open(t)
			if err := st.Close(); err != nil {
				t.Fatal(err)
			}
			// Double Close: Memory and SQLite are both idempotent (nil error).
			_ = st.Close()

			cases := []struct {
				name string
				fn   func() error
			}{
				{"Set", func() error {
					return st.Set(ctx, "k", json.RawMessage(`1`))
				}},
				{"Get", func() error {
					_, err := st.Get(ctx, "k")
					return err
				}},
				{"Delete", func() error {
					return st.Delete(ctx, "k")
				}},
				{"Exists", func() error {
					_, err := st.Exists(ctx, "k")
					return err
				}},
				{"Clear", func() error {
					return st.Clear(ctx)
				}},
				{"Export", func() error {
					_, err := st.Export(ctx)
					return err
				}},
				{"Replace", func() error {
					return st.Replace(ctx, map[string]json.RawMessage{"k": json.RawMessage(`1`)})
				}},
				{"BeginTx", func() error {
					_, err := st.BeginTx(ctx)
					return err
				}},
			}
			for _, tc := range cases {
				if err := tc.fn(); !errors.Is(err, state.ErrClosed) {
					t.Fatalf("%s after Close: got %v want ErrClosed", tc.name, err)
				}
			}
		})
	}
}
