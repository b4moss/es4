package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/b4moss/es4/packages/go/internal/state"
)

func TestRedis_PrefixIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mr := miniredis.RunT(t)
	url := "redis://" + mr.Addr()

	a, err := state.OpenRedis(ctx, url, "ns/a/")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	b, err := state.OpenRedis(ctx, url, "ns/b/")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })

	if err := a.Set(ctx, "k", json.RawMessage(`{"from":"a"}`)); err != nil {
		t.Fatal(err)
	}
	if err := b.Set(ctx, "k", json.RawMessage(`{"from":"b"}`)); err != nil {
		t.Fatal(err)
	}

	gotA, err := a.Get(ctx, "k")
	if err != nil || string(gotA) != `{"from":"a"}` {
		t.Fatalf("a: %s err=%v", gotA, err)
	}
	gotB, err := b.Get(ctx, "k")
	if err != nil || string(gotB) != `{"from":"b"}` {
		t.Fatalf("b: %s err=%v", gotB, err)
	}
	if err := a.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = a.Get(ctx, "k")
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("a after clear: %v", err)
	}
	gotB, err = b.Get(ctx, "k")
	if err != nil || string(gotB) != `{"from":"b"}` {
		t.Fatalf("b must be untouched: %s err=%v", gotB, err)
	}
}

func TestRedis_CloseReopenPersistence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mr := miniredis.RunT(t)
	url := "redis://" + mr.Addr()
	prefix := "persist/case/"

	st1, err := state.OpenRedis(ctx, url, prefix)
	if err != nil {
		t.Fatal(err)
	}
	val := json.RawMessage(`{"n":7}`)
	if err := st1.Set(ctx, "users/1", val); err != nil {
		t.Fatal(err)
	}
	tx, err := st1.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Set(ctx, "tx/k", json.RawMessage(`1`)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := st1.Close(); err != nil {
		t.Fatal(err)
	}

	st2, err := state.OpenRedis(ctx, url, prefix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st2.Close() })
	got, err := st2.Get(ctx, "users/1")
	if err != nil || string(got) != string(val) {
		t.Fatalf("persist users/1: %s err=%v", got, err)
	}
	got, err = st2.Get(ctx, "tx/k")
	if err != nil || string(got) != `1` {
		t.Fatalf("persist tx/k: %s err=%v", got, err)
	}
}

func TestRedis_TxRollbackNotVisible(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mr := miniredis.RunT(t)
	st, err := state.OpenRedis(ctx, "redis://"+mr.Addr(), "tx/roll/")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	tx, err := st.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Set(ctx, "ephemeral", json.RawMessage(`true`)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	_, err = st.Get(ctx, "ephemeral")
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("want ErrNotFound after rollback, got %v", err)
	}
}

func TestRedis_DefaultHashKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mr := miniredis.RunT(t)
	st, err := state.OpenRedis(ctx, "redis://"+mr.Addr(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if st.HashKey() != state.DefaultRedisHashKey {
		t.Fatalf("hash key: got %q want %q", st.HashKey(), state.DefaultRedisHashKey)
	}
	if err := st.Set(ctx, "k", json.RawMessage(`1`)); err != nil {
		t.Fatal(err)
	}
	if !mr.Exists(state.DefaultRedisHashKey) {
		t.Fatal("want default hash key present in miniredis")
	}
}
