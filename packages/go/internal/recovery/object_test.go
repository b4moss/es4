package recovery_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/internal/recovery"
)

func TestObject_SaveLoad_Prefix_TTL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := recovery.NewMemoryObject()
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	store, err := recovery.NewObject(recovery.ObjectConfig{
		Bucket: "bkt",
		Prefix: "es4/rec",
		TTL:    time.Hour,
		Client: fake,
		Clock:  func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Save(ctx, []byte(`o1`)); err != nil {
		t.Fatal(err)
	}
	now = now.Add(20 * time.Minute)
	if err := store.Save(ctx, []byte(`o2`)); err != nil {
		t.Fatal(err)
	}

	keys, err := fake.ListObjectKeys(ctx, "bkt", "es4/rec/")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("want 2 keys, got %v", keys)
	}
	for _, k := range keys {
		if !strings.HasPrefix(k, "es4/rec/gen-") {
			t.Fatalf("unexpected key %q", k)
		}
	}

	got, err := store.Load(ctx)
	if err != nil || string(got) != "o2" {
		t.Fatalf("got %q err=%v", got, err)
	}

	now = now.Add(2 * time.Hour)
	if err := store.Save(ctx, []byte(`o3`)); err != nil {
		t.Fatal(err)
	}
	keys, err = fake.ListObjectKeys(ctx, "bkt", "es4/rec/")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("want 1 key after prune, got %v", keys)
	}
	got, err = store.Load(ctx)
	if err != nil || string(got) != "o3" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestObject_TTLZero_KeepsLatestOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := recovery.NewMemoryObject()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store, err := recovery.NewObject(recovery.ObjectConfig{
		Bucket: "b",
		Client: fake,
		TTL:    0,
		Clock:  func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Save(ctx, []byte(`1`))
	now = now.Add(time.Second)
	_ = store.Save(ctx, []byte(`2`))
	keys, _ := fake.ListObjectKeys(ctx, "b", "gen-")
	if len(keys) != 1 {
		t.Fatalf("want 1, got %v", keys)
	}
	got, err := store.Load(ctx)
	if err != nil || string(got) != "2" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestObject_LoadMissing(t *testing.T) {
	t.Parallel()
	store, err := recovery.NewObject(recovery.ObjectConfig{
		Bucket: "empty",
		Client: recovery.NewMemoryObject(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Load(context.Background())
	if !errors.Is(err, recovery.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestObject_BucketRequired(t *testing.T) {
	t.Parallel()
	_, err := recovery.NewObject(recovery.ObjectConfig{Client: recovery.NewMemoryObject()})
	if err == nil {
		t.Fatal("want error for empty bucket")
	}
}
