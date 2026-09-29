package recovery_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/internal/recovery"
	_ "modernc.org/sqlite"
)

func TestLibSQL_SaveLoad_TTL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openTestSQLite(t)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	store, err := recovery.NewLibSQL(db, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	store.Clock = func() time.Time { return now }

	if err := store.Save(ctx, []byte(`a`)); err != nil {
		t.Fatal(err)
	}
	now = now.Add(10 * time.Minute)
	if err := store.Save(ctx, []byte(`b`)); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(ctx)
	if err != nil || string(got) != "b" {
		t.Fatalf("got %q err=%v", got, err)
	}
	assertLibSQLCount(t, db, 2)

	now = now.Add(2 * time.Hour)
	if err := store.Save(ctx, []byte(`c`)); err != nil {
		t.Fatal(err)
	}
	assertLibSQLCount(t, db, 1)
	got, err = store.Load(ctx)
	if err != nil || string(got) != "c" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestLibSQL_TTLZero_KeepsLatestOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openTestSQLite(t)
	store, err := recovery.NewLibSQL(db, 0)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store.Clock = func() time.Time { return now }
	if err := store.Save(ctx, []byte(`1`)); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if err := store.Save(ctx, []byte(`2`)); err != nil {
		t.Fatal(err)
	}
	assertLibSQLCount(t, db, 1)
	got, err := store.Load(ctx)
	if err != nil || string(got) != "2" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestLibSQL_LoadMissing(t *testing.T) {
	t.Parallel()
	store, err := recovery.NewLibSQL(openTestSQLite(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Load(context.Background())
	if !errors.Is(err, recovery.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestOpenLibSQL_FileURL(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "rec.db")
	store, err := recovery.OpenLibSQL("file:"+path, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()
	if err := store.Save(ctx, []byte(`x`)); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(ctx)
	if err != nil || string(got) != "x" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func openTestSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func assertLibSQLCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM es4_recovery`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != want {
		t.Fatalf("count=%d want %d", n, want)
	}
}
