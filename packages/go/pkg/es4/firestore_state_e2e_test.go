//go:build e2e

package es4_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/pkg/es4"
	"github.com/b4moss/es4/packages/go/pkg/options"
	"github.com/google/uuid"
)

// Firestore State Backend E2E — remote State persistence
// (docs/tests/e2e/e2e-spec.md §F). Catalog C1–C5 on State itself
// (Close → re-Open; no SnapshotNow / Recovery required).
//
// Env:
//   FIRESTORE_EMULATOR_HOST (required in CI)
//   ES4_E2E_FIRESTORE_PROJECT_ID (default demo-es4)
// Skip when HOST unset outside CI; CI layer=firestore sets ES4_E2E_REQUIRE_FIRESTORE=1.

func firestoreE2EProject(t *testing.T) string {
	t.Helper()
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		if os.Getenv("ES4_E2E_REQUIRE_FIRESTORE") == "1" {
			t.Fatal("FIRESTORE_EMULATOR_HOST required when ES4_E2E_REQUIRE_FIRESTORE=1")
		}
		t.Skip("FIRESTORE_EMULATOR_HOST unset; start docker/e2e firestore service")
	}
	project := os.Getenv("ES4_E2E_FIRESTORE_PROJECT_ID")
	if project == "" {
		project = "demo-es4"
	}
	return project
}

type firestoreStateFixture struct {
	t          *testing.T
	projectID  string
	collection string
}

func newFirestoreStateFixture(t *testing.T) *firestoreStateFixture {
	t.Helper()
	f := &firestoreStateFixture{
		t:          t,
		projectID:  firestoreE2EProject(t),
		collection: "e2e_" + uuid.NewString(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := f.open(ctx)
	if err := db.Clear(ctx); err != nil {
		_ = db.Close()
		t.Fatalf("start clear: %v", err)
	}
	_ = db.Close()
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ccancel()
		cdb := f.open(cctx)
		_ = cdb.Clear(cctx)
		_ = cdb.Clear(cctx) // F-C5: cleanup twice, errors none
		_ = cdb.Close()
	})
	return f
}

func (f *firestoreStateFixture) opts() options.Options {
	return options.Options{
		StateBackend:             options.StateBackendFirestore,
		StateFirestoreProjectID:  f.projectID,
		StateFirestoreDatabaseID: "(default)",
		StateFirestoreCollection: f.collection,
		SnapshotInterval:         0,
		RestoreOnStartup:         false,
		MemoryOnly:               false,
	}
}

func (f *firestoreStateFixture) open(ctx context.Context) *es4.DB {
	f.t.Helper()
	db, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options:          f.opts(),
		DisableRestore:   true,
		SkipAsyncRestore: true,
	})
	if err != nil {
		f.t.Fatalf("OpenWith firestore state: %v", err)
	}
	return db
}

// TestE2E_FirestoreState_SaveRestartRestore — F-C1
func TestE2E_FirestoreState_SaveRestartRestore(t *testing.T) {
	ctx := context.Background()
	f := newFirestoreStateFixture(t)

	db1 := f.open(ctx)
	val := json.RawMessage(`{"hello":"firestore","n":1}`)
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

// TestE2E_FirestoreState_MultiKeyHierarchy — F-C2
func TestE2E_FirestoreState_MultiKeyHierarchy(t *testing.T) {
	ctx := context.Background()
	f := newFirestoreStateFixture(t)

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

// TestE2E_FirestoreState_UncommittedNotRestored — F-C3
func TestE2E_FirestoreState_UncommittedNotRestored(t *testing.T) {
	ctx := context.Background()
	f := newFirestoreStateFixture(t)

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
	_ = db1.Close()

	db2 := f.open(ctx)
	t.Cleanup(func() { _ = db2.Close() })
	_, err = db2.Get(ctx, "ephemeral")
	if !errors.Is(err, es4.ErrNotFound) {
		t.Fatalf("want ErrNotFound after rollback, got %v", err)
	}
}

// TestE2E_FirestoreState_EmptyCollection — F-C4
func TestE2E_FirestoreState_EmptyCollection(t *testing.T) {
	ctx := context.Background()
	f := newFirestoreStateFixture(t)

	db := f.open(ctx)
	t.Cleanup(func() { _ = db.Close() })
	ok, err := db.Exists(ctx, "anything")
	if err != nil || ok {
		t.Fatalf("want empty: ok=%v err=%v", ok, err)
	}
	if err := db.Set(ctx, "first", json.RawMessage(`true`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Clear(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestE2E_FirestoreState_CleanupTwice — F-C5
func TestE2E_FirestoreState_CleanupTwice(t *testing.T) {
	ctx := context.Background()
	f := newFirestoreStateFixture(t)
	db := f.open(ctx)
	_ = db.Set(ctx, "tmp", json.RawMessage(`1`))
	if err := db.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
}

// TestE2E_FirestoreState_HierarchyEncode — F-N6
func TestE2E_FirestoreState_HierarchyEncode(t *testing.T) {
	ctx := context.Background()
	f := newFirestoreStateFixture(t)

	db1 := f.open(ctx)
	val := json.RawMessage(`{"path":"x/y/z"}`)
	if err := db1.Set(ctx, "x/y/z", val); err != nil {
		t.Fatal(err)
	}
	_ = db1.Close()

	db2 := f.open(ctx)
	t.Cleanup(func() { _ = db2.Close() })
	got, err := db2.Get(ctx, "x/y/z")
	if err != nil || string(got) != string(val) {
		t.Fatalf("got %s err=%v", got, err)
	}
}

// TestE2E_FirestoreState_ParallelCollections — F-N7
func TestE2E_FirestoreState_ParallelCollections(t *testing.T) {
	ctx := context.Background()
	f1 := newFirestoreStateFixture(t)
	f2 := newFirestoreStateFixture(t)

	db1 := f1.open(ctx)
	db2 := f2.open(ctx)
	t.Cleanup(func() {
		_ = db1.Close()
		_ = db2.Close()
	})
	_ = db1.Set(ctx, "shared-name", json.RawMessage(`1`))
	_ = db2.Set(ctx, "shared-name", json.RawMessage(`2`))

	got1, err := db1.Get(ctx, "shared-name")
	if err != nil || string(got1) != `1` {
		t.Fatalf("f1: %s err=%v", got1, err)
	}
	got2, err := db2.Get(ctx, "shared-name")
	if err != nil || string(got2) != `2` {
		t.Fatalf("f2: %s err=%v", got2, err)
	}
}

// TestE2E_FirestoreState_InvalidKey — F-E3
func TestE2E_FirestoreState_InvalidKey(t *testing.T) {
	ctx := context.Background()
	f := newFirestoreStateFixture(t)
	db := f.open(ctx)
	t.Cleanup(func() { _ = db.Close() })
	err := db.Set(ctx, "/bad", json.RawMessage(`1`))
	if !errors.Is(err, es4.ErrInvalidKey) {
		t.Fatalf("got %v", err)
	}
}

// TestE2E_FirestoreState_GetMissing — F-E4
func TestE2E_FirestoreState_GetMissing(t *testing.T) {
	ctx := context.Background()
	f := newFirestoreStateFixture(t)
	db := f.open(ctx)
	t.Cleanup(func() { _ = db.Close() })
	_, err := db.Get(ctx, "missing")
	if !errors.Is(err, es4.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
