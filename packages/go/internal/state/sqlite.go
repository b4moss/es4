package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS kv (
	key TEXT PRIMARY KEY NOT NULL,
	value TEXT NOT NULL
);
`

// SQLite is an on-disk SQLite State adapter (pure Go driver, no CGO).
type SQLite struct {
	mu     sync.Mutex
	txCond *sync.Cond
	db     *sql.DB
	path   string
	closed bool
	active *sqliteTx
}

// OpenSQLite opens (or creates) an on-disk SQLite State at path.
// Missing parent directories are created. A missing DB file starts empty.
func OpenSQLite(path string) (*SQLite, error) {
	if path == "" {
		return nil, fmt.Errorf("state: empty sqlite path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("state: mkdir %q: %w", dir, err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("state: open sqlite %q: %w", path, err)
	}
	// Serialize access at the driver level as well as with mu.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(sqliteSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("state: init schema %q: %w", path, err)
	}
	s := &SQLite{db: db, path: path}
	s.txCond = sync.NewCond(&s.mu)
	return s, nil
}

func (s *SQLite) waitNoActiveTxLocked() {
	for s.active != nil {
		s.txCond.Wait()
	}
}

func (s *SQLite) Set(ctx context.Context, key string, value json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	s.waitNoActiveTxLocked()
	if s.closed {
		return ErrClosed
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO kv(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value
`, key, string(value))
	if err != nil {
		return fmt.Errorf("state: sqlite set: %w", err)
	}
	return nil
}

func (s *SQLite) Get(ctx context.Context, key string) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateKey(key); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	s.waitNoActiveTxLocked()
	if s.closed {
		return nil, ErrClosed
	}
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM kv WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("state: sqlite get: %w", err)
	}
	return json.RawMessage(v), nil
}

func (s *SQLite) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	s.waitNoActiveTxLocked()
	if s.closed {
		return ErrClosed
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM kv WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("state: sqlite delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("state: sqlite delete rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLite) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := ValidateKey(key); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false, ErrClosed
	}
	s.waitNoActiveTxLocked()
	if s.closed {
		return false, ErrClosed
	}
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM kv WHERE key = ?`, key).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("state: sqlite exists: %w", err)
	}
	return n > 0, nil
}

func (s *SQLite) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	s.waitNoActiveTxLocked()
	if s.closed {
		return ErrClosed
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM kv`); err != nil {
		return fmt.Errorf("state: sqlite clear: %w", err)
	}
	return nil
}

func (s *SQLite) Export(ctx context.Context) (map[string]json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	s.waitNoActiveTxLocked()
	if s.closed {
		return nil, ErrClosed
	}
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM kv`)
	if err != nil {
		return nil, fmt.Errorf("state: sqlite export: %w", err)
	}
	defer rows.Close()

	out := make(map[string]json.RawMessage)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("state: sqlite export scan: %w", err)
		}
		out[k] = json.RawMessage(v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: sqlite export rows: %w", err)
	}
	return out, nil
}

func (s *SQLite) Replace(ctx context.Context, entries map[string]json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for k, v := range entries {
		if err := ValidateKey(k); err != nil {
			return err
		}
		if err := ValidateValue(v); err != nil {
			return err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	s.waitNoActiveTxLocked()
	if s.closed {
		return ErrClosed
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("state: sqlite replace begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM kv`); err != nil {
		return fmt.Errorf("state: sqlite replace clear: %w", err)
	}
	for k, v := range entries {
		if _, err := tx.ExecContext(ctx, `INSERT INTO kv(key, value) VALUES(?, ?)`, k, string(v)); err != nil {
			return fmt.Errorf("state: sqlite replace insert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("state: sqlite replace commit: %w", err)
	}
	return nil
}

func (s *SQLite) BeginTx(ctx context.Context) (Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	if s.active != nil {
		return nil, ErrNestedTx
	}
	sqlTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("state: sqlite begin: %w", err)
	}
	tx := &sqliteTx{s: s, tx: sqlTx}
	s.active = tx
	return tx, nil
}

func (s *SQLite) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		_ = s.active.tx.Rollback()
		s.active.done = true
		s.active = nil
		s.txCond.Broadcast()
	}
	s.closed = true
	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
}

// Path returns the on-disk SQLite path (tests / diagnostics).
func (s *SQLite) Path() string { return s.path }

type sqliteTx struct {
	s    *SQLite
	tx   *sql.Tx
	done bool
}

func (t *sqliteTx) withLock(ctx context.Context, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t.s.mu.Lock()
	defer t.s.mu.Unlock()
	if t.done || t.s.active != t {
		return ErrTxDone
	}
	if t.s.closed {
		return ErrClosed
	}
	return fn()
}

func (t *sqliteTx) Set(ctx context.Context, key string, value json.RawMessage) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	return t.withLock(ctx, func() error {
		_, err := t.tx.ExecContext(ctx, `
INSERT INTO kv(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value
`, key, string(value))
		if err != nil {
			return fmt.Errorf("state: sqlite tx set: %w", err)
		}
		return nil
	})
}

func (t *sqliteTx) Get(ctx context.Context, key string) (json.RawMessage, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	var out json.RawMessage
	err := t.withLock(ctx, func() error {
		var v string
		err := t.tx.QueryRowContext(ctx, `SELECT value FROM kv WHERE key = ?`, key).Scan(&v)
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("state: sqlite tx get: %w", err)
		}
		out = json.RawMessage(v)
		return nil
	})
	return out, err
}

func (t *sqliteTx) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	return t.withLock(ctx, func() error {
		res, err := t.tx.ExecContext(ctx, `DELETE FROM kv WHERE key = ?`, key)
		if err != nil {
			return fmt.Errorf("state: sqlite tx delete: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("state: sqlite tx delete rows: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (t *sqliteTx) Exists(ctx context.Context, key string) (bool, error) {
	if err := ValidateKey(key); err != nil {
		return false, err
	}
	var ok bool
	err := t.withLock(ctx, func() error {
		var n int
		err := t.tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM kv WHERE key = ?`, key).Scan(&n)
		if err != nil {
			return fmt.Errorf("state: sqlite tx exists: %w", err)
		}
		ok = n > 0
		return nil
	})
	return ok, err
}

func (t *sqliteTx) Clear(ctx context.Context) error {
	return t.withLock(ctx, func() error {
		if _, err := t.tx.ExecContext(ctx, `DELETE FROM kv`); err != nil {
			return fmt.Errorf("state: sqlite tx clear: %w", err)
		}
		return nil
	})
}

func (t *sqliteTx) Commit() error {
	t.s.mu.Lock()
	defer t.s.mu.Unlock()
	if t.done || t.s.active != t {
		return ErrTxDone
	}
	if t.s.closed {
		return ErrClosed
	}
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("state: sqlite tx commit: %w", err)
	}
	t.done = true
	t.s.active = nil
	t.s.txCond.Broadcast()
	return nil
}

func (t *sqliteTx) Rollback() error {
	t.s.mu.Lock()
	defer t.s.mu.Unlock()
	if t.done || t.s.active != t {
		return ErrTxDone
	}
	_ = t.tx.Rollback()
	t.done = true
	t.s.active = nil
	t.s.txCond.Broadcast()
	return nil
}
