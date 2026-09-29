package snapshot_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/internal/recovery"
	"github.com/b4moss/es4/packages/go/internal/snapshot"
	"github.com/b4moss/es4/packages/go/internal/state"
)

func TestEnvelope_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := state.NewMemory()
	t.Cleanup(func() { _ = st.Close() })
	_ = st.Set(ctx, "a/b", json.RawMessage(`{"x":true}`))

	path := filepath.Join(t.TempDir(), "rp.json")
	rec := recovery.NewFile(path)
	mgr := snapshot.NewManager(snapshot.Config{
		State:    st,
		Recovery: rec,
		Interval: 0,
		Clock:    func() time.Time { return time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC) },
	})
	if err := mgr.Take(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := rec.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	env, err := snapshot.DecodeEnvelope(data)
	if err != nil {
		t.Fatal(err)
	}
	if env.Version != snapshot.CurrentVersion {
		t.Fatalf("version: %d", env.Version)
	}
	if env.CreatedAt.UTC() != time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("created_at: %v", env.CreatedAt)
	}
	payload, err := snapshot.DecodeMemoryPayload(env.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload.Entries["a/b"]) != `{"x":true}` {
		t.Fatalf("payload: %#v", payload.Entries)
	}
}

func TestTake_NilRecovery_NoError(t *testing.T) {
	t.Parallel()
	st := state.NewMemory()
	t.Cleanup(func() { _ = st.Close() })
	mgr := snapshot.NewManager(snapshot.Config{State: st, Recovery: nil, Interval: 0})
	if err := mgr.Take(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPeriodic_StartStop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := state.NewMemory()
	t.Cleanup(func() { _ = st.Close() })
	_ = st.Set(ctx, "k", json.RawMessage(`1`))

	path := filepath.Join(t.TempDir(), "rp.json")
	rec := recovery.NewFile(path)
	mgr := snapshot.NewManager(snapshot.Config{
		State:    st,
		Recovery: rec,
		Interval: 20 * time.Millisecond,
	})
	mgr.Start()
	defer mgr.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for {
		_, err := rec.Load(ctx)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for periodic snapshot")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestExplicit_ResetsIntervalTimer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := state.NewMemory()
	t.Cleanup(func() { _ = st.Close() })
	_ = st.Set(ctx, "k", json.RawMessage(`1`))

	path := filepath.Join(t.TempDir(), "rp.json")
	rec := recovery.NewFile(path)
	interval := 80 * time.Millisecond
	mgr := snapshot.NewManager(snapshot.Config{
		State:    st,
		Recovery: rec,
		Interval: interval,
	})
	mgr.Start()
	defer mgr.Stop()

	// Wait for first periodic write.
	waitFile(t, rec, 2*time.Second)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	// Explicit Take should reset the ticker — next periodic write should not
	// appear until roughly one full interval after the reset.
	if err := mgr.Take(ctx); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	waitFile(t, rec, 2*time.Second)
	elapsed := time.Since(start)
	if elapsed < interval/2 {
		t.Fatalf("periodic fired too soon after reset: elapsed=%v interval=%v", elapsed, interval)
	}
}

func TestRestoreInto(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := state.NewMemory()
	t.Cleanup(func() { _ = src.Close() })
	_ = src.Set(ctx, "a", json.RawMessage(`"hi"`))
	path := filepath.Join(t.TempDir(), "rp.json")
	rec := recovery.NewFile(path)
	mgr := snapshot.NewManager(snapshot.Config{State: src, Recovery: rec})
	if err := mgr.Take(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := rec.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dst := state.NewMemory()
	t.Cleanup(func() { _ = dst.Close() })
	if err := snapshot.RestoreInto(ctx, dst, data); err != nil {
		t.Fatal(err)
	}
	got, err := dst.Get(ctx, "a")
	if err != nil || string(got) != `"hi"` {
		t.Fatalf("got %s err=%v", got, err)
	}
}

func waitFile(t *testing.T, rec *recovery.File, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if _, err := rec.Load(context.Background()); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for recovery file")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
