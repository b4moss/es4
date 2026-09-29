package state_test

import (
	"context"
	"encoding/json"
	"errors"
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
