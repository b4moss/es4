package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/b4moss/es4/packages/go/internal/state"
)

func TestMemory_SetGetExistsDeleteClear(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := state.NewMemory()
	t.Cleanup(func() { _ = m.Close() })

	val := json.RawMessage(`{"n":1}`)
	if err := m.Set(ctx, "a/b", val); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get(ctx, "a/b")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(val) {
		t.Fatalf("get: got %s want %s", got, val)
	}
	ok, err := m.Exists(ctx, "a/b")
	if err != nil || !ok {
		t.Fatalf("exists: ok=%v err=%v", ok, err)
	}
	if err := m.Delete(ctx, "a/b"); err != nil {
		t.Fatal(err)
	}
	ok, err = m.Exists(ctx, "a/b")
	if err != nil || ok {
		t.Fatalf("exists after delete: ok=%v err=%v", ok, err)
	}
	if err := m.Set(ctx, "x", json.RawMessage(`1`)); err != nil {
		t.Fatal(err)
	}
	if err := m.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	ok, err = m.Exists(ctx, "x")
	if err != nil || ok {
		t.Fatalf("exists after clear: ok=%v err=%v", ok, err)
	}
}

func TestMemory_GetMissing(t *testing.T) {
	t.Parallel()
	m := state.NewMemory()
	t.Cleanup(func() { _ = m.Close() })
	_, err := m.Get(context.Background(), "missing")
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestMemory_DeleteMissing(t *testing.T) {
	t.Parallel()
	m := state.NewMemory()
	t.Cleanup(func() { _ = m.Close() })
	err := m.Delete(context.Background(), "missing")
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestMemory_ExistsMissing(t *testing.T) {
	t.Parallel()
	m := state.NewMemory()
	t.Cleanup(func() { _ = m.Close() })
	ok, err := m.Exists(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("want false")
	}
}

func TestValidateKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		key string
		ok  bool
	}{
		{"a", true},
		{"a/b", true},
		{"a/b/c", true},
		{"", false},
		{"/a", false},
		{"a/", false},
		{"a//b", false},
		{"/", false},
	}
	for _, tc := range cases {
		err := state.ValidateKey(tc.key)
		if tc.ok && err != nil {
			t.Fatalf("key %q: unexpected err %v", tc.key, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("key %q: want error", tc.key)
		}
	}
}

func TestMemory_InvalidValue(t *testing.T) {
	t.Parallel()
	m := state.NewMemory()
	t.Cleanup(func() { _ = m.Close() })
	err := m.Set(context.Background(), "k", json.RawMessage(`{`))
	if !errors.Is(err, state.ErrInvalidValue) {
		t.Fatalf("got %v want ErrInvalidValue", err)
	}
}

func TestMemory_ExportReplace(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := state.NewMemory()
	t.Cleanup(func() { _ = m.Close() })
	_ = m.Set(ctx, "a", json.RawMessage(`1`))
	exp, err := m.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m2 := state.NewMemory()
	t.Cleanup(func() { _ = m2.Close() })
	if err := m2.Replace(ctx, exp); err != nil {
		t.Fatal(err)
	}
	got, err := m2.Get(ctx, "a")
	if err != nil || string(got) != "1" {
		t.Fatalf("got %s err=%v", got, err)
	}
}

func TestMemory_ContextCanceled(t *testing.T) {
	t.Parallel()
	m := state.NewMemory()
	t.Cleanup(func() { _ = m.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.Set(ctx, "a", json.RawMessage(`1`)); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestSQLite_APIParity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := state.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	val := json.RawMessage(`{"n":1}`)
	if err := s.Set(ctx, "a/b", val); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "a/b")
	if err != nil || string(got) != string(val) {
		t.Fatalf("get: %s err=%v", got, err)
	}
	ok, err := s.Exists(ctx, "a/b")
	if err != nil || !ok {
		t.Fatalf("exists: %v %v", ok, err)
	}
	ok, err = s.Exists(ctx, "missing")
	if err != nil || ok {
		t.Fatalf("missing exists: %v %v", ok, err)
	}
	_, err = s.Get(ctx, "missing")
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("get missing: %v", err)
	}
	err = s.Delete(ctx, "missing")
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
	if err := s.Delete(ctx, "a/b"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, "x", json.RawMessage(`true`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	ok, err = s.Exists(ctx, "x")
	if err != nil || ok {
		t.Fatalf("after clear: %v %v", ok, err)
	}
	err = s.Set(ctx, "", json.RawMessage(`1`))
	if !errors.Is(err, state.ErrInvalidKey) {
		t.Fatalf("invalid key: %v", err)
	}
	err = s.Set(ctx, "k", json.RawMessage(`{`))
	if !errors.Is(err, state.ErrInvalidValue) {
		t.Fatalf("invalid value: %v", err)
	}
}

func TestSQLite_Persistence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.db")

	s1, err := state.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Set(ctx, "keep", json.RawMessage(`{"v":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := state.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	got, err := s2.Get(ctx, "keep")
	if err != nil || string(got) != `{"v":1}` {
		t.Fatalf("got %s err=%v", got, err)
	}
}

func TestSQLite_SeparatePaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	a, err := state.OpenSQLite(filepath.Join(dir, "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	b, err := state.OpenSQLite(filepath.Join(dir, "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	_ = a.Set(ctx, "only-a", json.RawMessage(`1`))
	_ = b.Set(ctx, "only-b", json.RawMessage(`2`))
	ok, err := b.Exists(ctx, "only-a")
	if err != nil || ok {
		t.Fatalf("paths must not interfere: ok=%v err=%v", ok, err)
	}
}

func TestSQLite_MissingFileCreatesEmpty(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "subdir", "new.db")
	s, err := state.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ok, err := s.Exists(context.Background(), "x")
	if err != nil || ok {
		t.Fatalf("want empty: ok=%v err=%v", ok, err)
	}
}

func TestSQLite_UnopenablePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create a directory where a file is expected → open/schema fails.
	bad := filepath.Join(dir, "not-a-file")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := state.OpenSQLite(bad)
	if err == nil {
		t.Fatal("want open error for directory path")
	}
}

func TestSQLite_ExportReplace_RoundTripWithMemory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sq, err := state.OpenSQLite(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	_ = sq.Set(ctx, "a/b", json.RawMessage(`{"x":true}`))
	exp, err := sq.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := exp["a/b"]; !ok {
		t.Fatal("export missing key")
	}
	mem := state.NewMemory()
	t.Cleanup(func() { _ = mem.Close() })
	if err := mem.Replace(ctx, exp); err != nil {
		t.Fatal(err)
	}
	got, err := mem.Get(ctx, "a/b")
	if err != nil || string(got) != `{"x":true}` {
		t.Fatalf("got %s err=%v", got, err)
	}
	back, err := mem.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sq2, err := state.OpenSQLite(filepath.Join(t.TempDir(), "y.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq2.Close() })
	if err := sq2.Replace(ctx, back); err != nil {
		t.Fatal(err)
	}
	got, err = sq2.Get(ctx, "a/b")
	if err != nil || string(got) != `{"x":true}` {
		t.Fatalf("sqlite replace: %s err=%v", got, err)
	}
}

func TestSQLite_ContextCanceled(t *testing.T) {
	t.Parallel()
	s, err := state.OpenSQLite(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Set(ctx, "a", json.RawMessage(`1`)); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
