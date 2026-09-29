package recovery_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
