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

	"github.com/b4moss/es4/packages/go/pkg/es4"
	"github.com/b4moss/es4/packages/go/pkg/options"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Valkey State Backend E2E — same Redis-protocol adapter against a Valkey
// process (docs/tests/e2e/e2e-spec.md §V). Catalog C1–C5; Close→re-Open
// persistence on State itself (no SnapshotNow / Recovery).
//
// Env: ES4_E2E_VALKEY_URL (e.g. redis://127.0.0.1:6380/0).
// Skip when unset outside CI; CI layer=valkey sets ES4_E2E_REQUIRE_VALKEY=1.

func valkeyE2EURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("ES4_E2E_VALKEY_URL")
	if url == "" {
		if os.Getenv("ES4_E2E_REQUIRE_VALKEY") == "1" {
			t.Fatal("ES4_E2E_VALKEY_URL required when ES4_E2E_REQUIRE_VALKEY=1")
		}
		t.Skip("ES4_E2E_VALKEY_URL unset; start docker/e2e valkey service")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse ES4_E2E_VALKEY_URL: %v", err)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		if os.Getenv("ES4_E2E_REQUIRE_VALKEY") == "1" {
			t.Fatalf("valkey not reachable at %s: %v", url, err)
		}
		t.Skipf("valkey not reachable at %s: %v", url, err)
	}
	return url
}

type valkeyStateFixture struct {
	t      *testing.T
	url    string
	prefix string
	client *redis.Client
}

func newValkeyStateFixture(t *testing.T) *valkeyStateFixture {
	t.Helper()
	url := valkeyE2EURL(t)
	opt, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opt)
	f := &valkeyStateFixture{
		t:      t,
		url:    url,
		prefix: fmt.Sprintf("e2e/%s/%s/", time.Now().UTC().Format("150405"), uuid.NewString()),
		client: client,
	}
	if err := f.clear(); err != nil {
		t.Fatalf("start clear: %v", err)
	}
	t.Cleanup(func() {
		if err := f.clear(); err != nil {
			t.Logf("cleanup clear: %v", err)
		}
		_ = f.client.Close()
	})
	return f
}

func (f *valkeyStateFixture) clear() error {
	return f.client.Del(context.Background(), f.prefix).Err()
}

func (f *valkeyStateFixture) opts() options.Options {
	// valkey alias exercises the same Redis adapter via state_backend=valkey.
	return options.Options{
		StateBackend:        options.StateBackendValkey,
		StateRedisURL:       f.url,
		StateRedisKeyPrefix: f.prefix,
		SnapshotInterval:    0,
		RestoreOnStartup:    false,
		MemoryOnly:          false,
	}
}

func (f *valkeyStateFixture) open(ctx context.Context) *es4.DB {
	f.t.Helper()
	db, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options:          f.opts(),
		DisableRestore:   true,
		SkipAsyncRestore: true,
	})
	if err != nil {
		f.t.Fatalf("OpenWith valkey state: %v", err)
	}
	return db
}

// TestE2E_ValkeyState_SaveRestartRestore — C1
func TestE2E_ValkeyState_SaveRestartRestore(t *testing.T) {
	ctx := context.Background()
	f := newValkeyStateFixture(t)

	db1 := f.open(ctx)
	val := json.RawMessage(`{"hello":"valkey","n":1}`)
	if err := db1.Set(ctx, "users/1", val); err != nil {
		t.Fatal(err)
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

// TestE2E_ValkeyState_MultiKeyHierarchy — C2
func TestE2E_ValkeyState_MultiKeyHierarchy(t *testing.T) {
	ctx := context.Background()
	f := newValkeyStateFixture(t)

	db1 := f.open(ctx)
	want := map[string]json.RawMessage{
		"plain": json.RawMessage(`"a"`),
		"a/b/c": json.RawMessage(`{"deep":true}`),
		"x/y":   json.RawMessage(`[1,2,3]`),
		"solo":  json.RawMessage(`42`),
	}
	for k, v := range want {
		if err := db1.Set(ctx, k, v); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}
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

// TestE2E_ValkeyState_UncommittedNotRestored — C3
func TestE2E_ValkeyState_UncommittedNotRestored(t *testing.T) {
	ctx := context.Background()
	f := newValkeyStateFixture(t)

	db1 := f.open(ctx)
	tx, err := db1.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Set(ctx, "ephemeral", json.RawMessage(`{"saved":false}`)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := db1.Close(); err != nil {
		t.Fatal(err)
	}

	db2 := f.open(ctx)
	t.Cleanup(func() { _ = db2.Close() })
	_, err = db2.Get(ctx, "ephemeral")
	if !es4.IsNotFound(err) {
		t.Fatalf("rolled-back key must be ErrNotFound after restart, got %v", err)
	}
}

// TestE2E_ValkeyState_EmptyStartup — C4
func TestE2E_ValkeyState_EmptyStartup(t *testing.T) {
	ctx := context.Background()
	f := newValkeyStateFixture(t)
	db := f.open(ctx)
	t.Cleanup(func() { _ = db.Close() })

	ok, err := db.Exists(ctx, "any")
	if err != nil || ok {
		t.Fatalf("empty state: exists=%v err=%v", ok, err)
	}
	_, err = db.Get(ctx, "any")
	if !errors.Is(err, es4.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	val := json.RawMessage(`{"n":1}`)
	if err := db.Set(ctx, "k", val); err != nil {
		t.Fatal(err)
	}
	got, err := db.Get(ctx, "k")
	if err != nil || string(got) != string(val) {
		t.Fatalf("got %s err=%v", got, err)
	}
}

// TestE2E_ValkeyState_CleanupIdempotent — C5
func TestE2E_ValkeyState_CleanupIdempotent(t *testing.T) {
	ctx := context.Background()
	f := newValkeyStateFixture(t)

	db := f.open(ctx)
	_ = db.Set(ctx, "k", json.RawMessage(`1`))
	_ = db.Close()

	if err := f.clear(); err != nil {
		t.Fatalf("first clear: %v", err)
	}
	if err := f.clear(); err != nil {
		t.Fatalf("second clear: %v", err)
	}
}
