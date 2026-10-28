package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/b4moss/es4/packages/go/internal/state"
	"github.com/google/uuid"
)

func TestFirestoreContract_ExpiresAt_Field(t *testing.T) {
	st := openFirestoreStore(t, "exp_"+uuid.NewString())
	ctx := context.Background()
	fixed := time.Date(2034, 5, 1, 0, 0, 0, 0, time.UTC)
	state.SetNowFunc(func() time.Time { return fixed })
	t.Cleanup(func() { state.SetNowFunc(nil) })

	future := fixed.Add(2 * time.Hour)
	val := json.RawMessage(`{"fs":1}`)

	if err := st.Set(ctx, "with", val, state.SetMeta{ExpiresAt: &future}); err != nil {
		t.Fatal(err)
	}
	if err := st.Set(ctx, "without", val); err != nil {
		t.Fatal(err)
	}

	// Inspect raw documents via client.
	projectID := os.Getenv("ES4_E2E_FIRESTORE_PROJECT_ID")
	if projectID == "" {
		projectID = "demo-es4"
	}
	client, err := firestore.NewClientWithDatabase(ctx, projectID, st.DatabaseID())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	withID, err := state.EncodeFirestoreDocID("with")
	if err != nil {
		t.Fatal(err)
	}
	snap, err := client.Collection(st.Collection()).Doc(withID).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data := snap.Data()
	rawExp, ok := data["expires_at"]
	if !ok {
		t.Fatal("expires_at missing on timed doc")
	}
	if _, ok := rawExp.(time.Time); !ok {
		// Firestore client may return time.Time
		t.Logf("expires_at type=%T value=%v", rawExp, rawExp)
		if rawExp == nil {
			t.Fatal("expires_at nil")
		}
	}

	withoutID, err := state.EncodeFirestoreDocID("without")
	if err != nil {
		t.Fatal(err)
	}
	snap2, err := client.Collection(st.Collection()).Doc(withoutID).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap2.Data()["expires_at"]; ok {
		t.Fatal("expires_at must be omitted when no expiry")
	}

	// Overwrite timed → untimed clears field.
	if err := st.Set(ctx, "with", json.RawMessage(`{"fs":2}`)); err != nil {
		t.Fatal(err)
	}
	snap3, err := client.Collection(st.Collection()).Doc(withID).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap3.Data()["expires_at"]; ok {
		t.Fatal("expires_at should be removed after untimed overwrite")
	}

	past := fixed.Add(-time.Minute)
	if err := st.Set(ctx, "past", val, state.SetMeta{ExpiresAt: &past}); err != nil {
		t.Fatal(err)
	}
	_, err = st.Get(ctx, "past")
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("past get: %v", err)
	}

	// Persistence: Close → re-Open keeps future expiry.
	col := st.Collection()
	_ = st.Close()
	st2, err := state.OpenFirestore(ctx, projectID, "(default)", col)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st2.Close() })
	if err := st2.Set(ctx, "persist", val, state.SetMeta{ExpiresAt: &future}); err != nil {
		t.Fatal(err)
	}
	_ = st2.Close()
	st3, err := state.OpenFirestore(ctx, projectID, "(default)", col)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = st3.Clear(context.Background())
		_ = st3.Close()
	})
	got, err := st3.Get(ctx, "persist")
	if err != nil || string(got) != string(val) {
		t.Fatalf("persist get: %s err=%v", got, err)
	}
	// Advance past expiry.
	state.SetNowFunc(func() time.Time { return future.Add(time.Second) })
	_, err = st3.Get(ctx, "persist")
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("persist after expiry: %v", err)
	}

}
