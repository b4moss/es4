package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/b4moss/es4/packages/go/internal/state"
	"github.com/google/uuid"
)

// requireFirestoreEmulator skips locally when FIRESTORE_EMULATOR_HOST is unset;
// fails when ES4_TEST_REQUIRE_FIRESTORE=1 (CI contract / E2E).
func requireFirestoreEmulator(t *testing.T) (projectID string) {
	t.Helper()
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		if os.Getenv("ES4_TEST_REQUIRE_FIRESTORE") == "1" || os.Getenv("ES4_E2E_REQUIRE_FIRESTORE") == "1" {
			t.Fatal("FIRESTORE_EMULATOR_HOST required when ES4_TEST_REQUIRE_FIRESTORE=1")
		}
		t.Skip("FIRESTORE_EMULATOR_HOST unset; start docker/e2e firestore service")
	}
	projectID = os.Getenv("ES4_E2E_FIRESTORE_PROJECT_ID")
	if projectID == "" {
		projectID = "demo-es4"
	}
	return projectID
}

func openFirestoreStore(t *testing.T, collection string) *state.Firestore {
	t.Helper()
	projectID := requireFirestoreEmulator(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	st, err := state.OpenFirestore(ctx, projectID, "(default)", collection)
	if err != nil {
		t.Fatalf("OpenFirestore: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})
	return st
}

func TestFirestoreContract_CRUD(t *testing.T) {
	st := openFirestoreStore(t, "contract_"+uuid.NewString())
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
}

func TestFirestoreContract_JSONKinds(t *testing.T) {
	st := openFirestoreStore(t, "json_"+uuid.NewString())
	ctx := context.Background()
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
		got, err := st.Get(ctx, "good")
		if err != nil || string(got) != string(good) {
			t.Fatalf("round-trip %s: got %s err=%v", good, got, err)
		}
	}
}

func TestFirestoreContract_InvalidKeyValue(t *testing.T) {
	st := openFirestoreStore(t, "inv_"+uuid.NewString())
	ctx := context.Background()
	for _, key := range []string{"", "/a", "a/", "a//b"} {
		err := st.Set(ctx, key, json.RawMessage(`1`))
		if !errors.Is(err, state.ErrInvalidKey) {
			t.Fatalf("Set(%q): got %v want ErrInvalidKey", key, err)
		}
	}
	for _, bad := range []json.RawMessage{nil, {}, []byte(`{`)} {
		err := st.Set(ctx, "k", bad)
		if !errors.Is(err, state.ErrInvalidValue) {
			t.Fatalf("Set invalid %q: got %v", bad, err)
		}
	}
}

func TestFirestoreContract_ExportReplaceTx(t *testing.T) {
	st := openFirestoreStore(t, "ert_"+uuid.NewString())
	ctx := context.Background()

	_ = st.Set(ctx, "old", json.RawMessage(`0`))
	entries := map[string]json.RawMessage{
		"new":  json.RawMessage(`{"a":1}`),
		"a/b":  json.RawMessage(`true`),
	}
	if err := st.Replace(ctx, entries); err != nil {
		t.Fatal(err)
	}
	exp, err := st.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(exp) != 2 {
		t.Fatalf("export size %d", len(exp))
	}
	if _, ok := exp["old"]; ok {
		t.Fatal("old should be gone")
	}
	if _, ok := exp["a/b"]; !ok {
		t.Fatal("export keys must be logical (K-N5)")
	}

	if err := st.Replace(ctx, map[string]json.RawMessage{}); err != nil {
		t.Fatal(err)
	}
	exp, err = st.Export(ctx)
	if err != nil || len(exp) != 0 {
		t.Fatalf("empty replace: %v len=%d", err, len(exp))
	}

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

	tx2, err := st.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_ = tx2.Set(ctx, "roll", json.RawMessage(`1`))
	if err := tx2.Rollback(); err != nil {
		t.Fatal(err)
	}
	_, err = st.Get(ctx, "roll")
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("rollback: %v", err)
	}

	txOpen, err := st.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.BeginTx(ctx)
	if !errors.Is(err, state.ErrNestedTx) {
		t.Fatalf("nested: %v", err)
	}
	if err := txOpen.Rollback(); err != nil {
		t.Fatal(err)
	}
}

func TestFirestoreContract_ClearIsolation(t *testing.T) {
	// S-N16 / S-N17: Clear / empty Replace must not touch other collections.
	projectID := requireFirestoreEmulator(t)
	otherCol := "other_" + uuid.NewString()
	targetCol := "target_" + uuid.NewString()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := firestore.NewClientWithDatabase(ctx, projectID, "(default)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// Seed other collection directly.
	_, err = client.Collection(otherCol).Doc("keep").Set(ctx, map[string]string{"value": `"survive"`})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = client.Collection(otherCol).Doc("keep").Delete(context.Background())
	})

	st, err := state.OpenFirestoreConfig(ctx, state.FirestoreConfig{
		ProjectID:  projectID,
		DatabaseID: "(default)",
		Collection: targetCol,
		Client:     client,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if err := st.Set(ctx, "victim", json.RawMessage(`1`)); err != nil {
		t.Fatal(err)
	}
	if err := st.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	ok, err := st.Exists(ctx, "victim")
	if err != nil || ok {
		t.Fatalf("target should be empty: ok=%v err=%v", ok, err)
	}

	snap, err := client.Collection(otherCol).Doc("keep").Get(ctx)
	if err != nil {
		t.Fatalf("other collection wiped by Clear: %v", err)
	}
	if snap == nil || !snap.Exists() {
		t.Fatal("other collection document missing after Clear")
	}

	_ = st.Set(ctx, "again", json.RawMessage(`2`))
	if err := st.Replace(ctx, map[string]json.RawMessage{}); err != nil {
		t.Fatal(err)
	}
	snap, err = client.Collection(otherCol).Doc("keep").Get(ctx)
	if err != nil || snap == nil || !snap.Exists() {
		t.Fatalf("other collection wiped by empty Replace: %v", err)
	}
}

func TestFirestoreContract_PersistenceAndIsolation(t *testing.T) {
	projectID := requireFirestoreEmulator(t)
	col := "persist_" + uuid.NewString()
	col2 := "persist2_" + uuid.NewString()
	ctx := context.Background()

	st1, err := state.OpenFirestore(ctx, projectID, "(default)", col)
	if err != nil {
		t.Fatal(err)
	}
	if err := st1.Set(ctx, "k", json.RawMessage(`"alive"`)); err != nil {
		t.Fatal(err)
	}
	if err := st1.Close(); err != nil {
		t.Fatal(err)
	}

	st2, err := state.OpenFirestore(ctx, projectID, "(default)", col)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = st2.Clear(context.Background())
		_ = st2.Close()
	})
	got, err := st2.Get(ctx, "k")
	if err != nil || string(got) != `"alive"` {
		t.Fatalf("persist: %s err=%v", got, err)
	}

	other, err := state.OpenFirestore(ctx, projectID, "(default)", col2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = other.Clear(context.Background())
		_ = other.Close()
	})
	_, err = other.Get(ctx, "k")
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("other collection should not see k: %v", err)
	}
}

func TestFirestoreContract_AfterClose(t *testing.T) {
	st := openFirestoreStore(t, "close_"+uuid.NewString())
	ctx := context.Background()
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	if err := st.Set(ctx, "k", json.RawMessage(`1`)); !errors.Is(err, state.ErrClosed) {
		t.Fatalf("Set after Close: %v", err)
	}
	_, err := st.BeginTx(ctx)
	if !errors.Is(err, state.ErrClosed) {
		t.Fatalf("BeginTx after Close: %v", err)
	}
}

func TestFirestoreContract_CanceledContext(t *testing.T) {
	st := openFirestoreStore(t, "ctx_"+uuid.NewString())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := st.Set(ctx, "k", json.RawMessage(`1`))
	if err == nil {
		t.Fatal("want context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestFirestoreContract_ExportRoundTripToMemory(t *testing.T) {
	st := openFirestoreStore(t, "rt_"+uuid.NewString())
	ctx := context.Background()
	_ = st.Set(ctx, "a/b", json.RawMessage(`{"x":1}`))
	exp, err := st.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mem := state.NewMemory()
	t.Cleanup(func() { _ = mem.Close() })
	if err := mem.Replace(ctx, exp); err != nil {
		t.Fatal(err)
	}
	got, err := mem.Get(ctx, "a/b")
	if err != nil || string(got) != `{"x":1}` {
		t.Fatalf("got %s err=%v", got, err)
	}
}

func TestFirestoreContract_TxDone(t *testing.T) {
	st := openFirestoreStore(t, "txdone_"+uuid.NewString())
	ctx := context.Background()
	tx, err := st.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	err = tx.Set(ctx, "k", json.RawMessage(`1`))
	if !errors.Is(err, state.ErrTxDone) {
		t.Fatalf("reuse committed tx: %v", err)
	}
}

func TestFirestore_OpenUnreachable(t *testing.T) {
	t.Setenv("FIRESTORE_EMULATOR_HOST", "127.0.0.1:1")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	st, err := state.OpenFirestore(ctx, "demo-es4", "(default)", fmt.Sprintf("u_%s", uuid.NewString()))
	if err != nil {
		// Open failed — acceptable for W-E3
		return
	}
	t.Cleanup(func() { _ = st.Close() })
	err = st.Set(ctx, "k", json.RawMessage(`1`))
	if err == nil {
		t.Fatal("want error against unreachable emulator")
	}
}
