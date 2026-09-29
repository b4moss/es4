package recovery_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/internal/recovery"
)

func TestFile_SaveLoad_AtomicReplace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "rp.json")
	f := recovery.NewFile(path)
	ctx := context.Background()

	if err := f.Save(ctx, []byte(`{"v":1}`)); err != nil {
		t.Fatal(err)
	}
	got, err := f.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"v":1}` {
		t.Fatalf("got %s", got)
	}

	if err := f.Save(ctx, []byte(`{"v":2}`)); err != nil {
		t.Fatal(err)
	}
	got, err = f.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"v":2}` {
		t.Fatalf("got %s", got)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
	// TTL=0: single file only.
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestFile_LoadMissing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing.json")
	f := recovery.NewFile(path)
	_, err := f.Load(context.Background())
	if !errors.Is(err, recovery.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestFile_LoadUnreadable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "dir-as-file")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	f := recovery.NewFile(path)
	_, err := f.Load(context.Background())
	if err == nil {
		t.Fatal("want error for unreadable path")
	}
	if errors.Is(err, recovery.ErrNotFound) {
		t.Fatal("want non-NotFound read error")
	}
}

func TestFile_SaveNestedDir(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "deep", "rp.json")
	f := recovery.NewFile(path)
	if err := f.Save(context.Background(), []byte(`ok`)); err != nil {
		t.Fatal(err)
	}
	got, err := f.Load(context.Background())
	if err != nil || string(got) != "ok" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestFile_TTL_Generations(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	f := recovery.NewFileTTL(dir, time.Hour)
	f.Clock = func() time.Time { return now }

	if err := f.Save(ctx, []byte(`g1`)); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Minute)
	if err := f.Save(ctx, []byte(`g2`)); err != nil {
		t.Fatal(err)
	}
	// Both within TTL.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var snaps int
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".snap" {
			snaps++
		}
	}
	if snaps != 2 {
		t.Fatalf("want 2 generations within TTL, got %d", snaps)
	}
	got, err := f.Load(ctx)
	if err != nil || string(got) != "g2" {
		t.Fatalf("got %q err=%v", got, err)
	}

	// Advance past TTL of g1; Save g3 and prune g1.
	now = now.Add(2 * time.Hour)
	if err := f.Save(ctx, []byte(`g3`)); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	snaps = 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".snap" {
			snaps++
		}
	}
	if snaps != 1 {
		t.Fatalf("want 1 generation after prune, got %d", snaps)
	}
	got, err = f.Load(ctx)
	if err != nil || string(got) != "g3" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestFile_TTL_LoadMissing(t *testing.T) {
	t.Parallel()
	f := recovery.NewFileTTL(t.TempDir(), time.Minute)
	_, err := f.Load(context.Background())
	if !errors.Is(err, recovery.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
