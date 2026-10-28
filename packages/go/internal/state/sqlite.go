package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS kv (
	key TEXT PRIMARY KEY NOT NULL,
	value TEXT NOT NULL,
	expires_at TEXT
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
	if err := migrateSQLiteExpiresAt(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &SQLite{db: db, path: path}
	s.txCond = sync.NewCond(&s.mu)
	return s, nil
}

func migrateSQLiteExpiresAt(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(kv)`)
	if err != nil {
		return fmt.Errorf("state: sqlite pragma table_info: %w", err)
	}
	defer rows.Close()
	has := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("state: sqlite pragma scan: %w", err)
		}
		if name == "expires_at" {
			has = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if has {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE kv ADD COLUMN expires_at TEXT`); err != nil {
		return fmt.Errorf("state: sqlite add expires_at: %w", err)
	}
	return nil
}

func sqliteExpiresArg(exp *time.Time) any {
	if exp == nil {
		return nil
	}
	return FormatExpiresAt(*exp)
}

func scanSQLiteExpires(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	return ParseExpiresAt(ns.String)
}

func (s *SQLite) waitNoActiveTxLocked() {
	for s.active != nil {
		s.txCond.Wait()
	}
}

func (s *SQLite) Set(ctx context.Context, key string, value json.RawMessage, meta ...SetMeta) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	exp := CloneTimePtr(FirstMeta(meta).ExpiresAt)

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
INSERT INTO kv(key, value, expires_at) VALUES(?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value, expires_at=excluded.expires_at
`, key, string(value), sqliteExpiresArg(exp))
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
	var expNS sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT value, expires_at FROM kv WHERE key = ?`, key).Scan(&v, &expNS)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("state: sqlite get: %w", err)
	}
	exp, err := scanSQLiteExpires(expNS)
	if err != nil {
		return nil, fmt.Errorf("state: sqlite get expires: %w", err)
	}
	if IsExpired(exp) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM kv WHERE key = ?`, key)
		return nil, ErrNotFound
	}
	return append(json.RawMessage(nil), v...), nil
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
	var expNS sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT expires_at FROM kv WHERE key = ?`, key).Scan(&expNS)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("state: sqlite delete lookup: %w", err)
	}
	exp, err := scanSQLiteExpires(expNS)
	if err != nil {
		return fmt.Errorf("state: sqlite delete expires: %w", err)
	}
	if IsExpired(exp) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM kv WHERE key = ?`, key)
		return ErrNotFound
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
	var expNS sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT expires_at FROM kv WHERE key = ? LIMIT 1`, key).Scan(&expNS)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("state: sqlite exists: %w", err)
	}
	exp, err := scanSQLiteExpires(expNS)
	if err != nil {
		return false, fmt.Errorf("state: sqlite exists expires: %w", err)
	}
	if IsExpired(exp) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM kv WHERE key = ?`, key)
		return false, nil
	}
	return true, nil
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

func (s *SQLite) Scan(ctx context.Context, prefix string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateScanPrefix(prefix); err != nil {
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
	rows, err := s.db.QueryContext(ctx, `SELECT key, expires_at FROM kv`)
	if err != nil {
		return nil, fmt.Errorf("state: sqlite scan: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var k string
		var expNS sql.NullString
		if err := rows.Scan(&k, &expNS); err != nil {
			return nil, fmt.Errorf("state: sqlite scan row: %w", err)
		}
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		exp, err := scanSQLiteExpires(expNS)
		if err != nil {
			return nil, fmt.Errorf("state: sqlite scan expires: %w", err)
		}
		if IsExpired(exp) {
			continue
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: sqlite scan rows: %w", err)
	}
	return out, nil
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
	rows, err := s.db.QueryContext(ctx, `SELECT key, value, expires_at FROM kv`)
	if err != nil {
		return nil, fmt.Errorf("state: sqlite export: %w", err)
	}
	defer rows.Close()

	out := make(map[string]json.RawMessage)
	for rows.Next() {
		var k, v string
		var expNS sql.NullString
		if err := rows.Scan(&k, &v, &expNS); err != nil {
			return nil, fmt.Errorf("state: sqlite export scan: %w", err)
		}
		exp, err := scanSQLiteExpires(expNS)
		if err != nil {
			return nil, fmt.Errorf("state: sqlite export expires: %w", err)
		}
		if IsExpired(exp) {
			continue
		}
		// Deep copy: values only (no expiry meta).
		out[k] = append(json.RawMessage(nil), v...)
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
		if _, err := tx.ExecContext(ctx, `INSERT INTO kv(key, value, expires_at) VALUES(?, ?, NULL)`, k, string(v)); err != nil {
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

func (t *sqliteTx) Set(ctx context.Context, key string, value json.RawMessage, meta ...SetMeta) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	exp := CloneTimePtr(FirstMeta(meta).ExpiresAt)
	return t.withLock(ctx, func() error {
		_, err := t.tx.ExecContext(ctx, `
INSERT INTO kv(key, value, expires_at) VALUES(?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value, expires_at=excluded.expires_at
`, key, string(value), sqliteExpiresArg(exp))
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
		var expNS sql.NullString
		err := t.tx.QueryRowContext(ctx, `SELECT value, expires_at FROM kv WHERE key = ?`, key).Scan(&v, &expNS)
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("state: sqlite tx get: %w", err)
		}
		exp, err := scanSQLiteExpires(expNS)
		if err != nil {
			return fmt.Errorf("state: sqlite tx get expires: %w", err)
		}
		if IsExpired(exp) {
			return ErrNotFound
		}
		out = append(json.RawMessage(nil), v...)
		return nil
	})
	return out, err
}

func (t *sqliteTx) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	return t.withLock(ctx, func() error {
		var expNS sql.NullString
		err := t.tx.QueryRowContext(ctx, `SELECT expires_at FROM kv WHERE key = ?`, key).Scan(&expNS)
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("state: sqlite tx delete lookup: %w", err)
		}
		exp, err := scanSQLiteExpires(expNS)
		if err != nil {
			return fmt.Errorf("state: sqlite tx delete expires: %w", err)
		}
		if IsExpired(exp) {
			return ErrNotFound
		}
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
		var expNS sql.NullString
		err := t.tx.QueryRowContext(ctx, `SELECT expires_at FROM kv WHERE key = ? LIMIT 1`, key).Scan(&expNS)
		if err == sql.ErrNoRows {
			ok = false
			return nil
		}
		if err != nil {
			return fmt.Errorf("state: sqlite tx exists: %w", err)
		}
		exp, err := scanSQLiteExpires(expNS)
		if err != nil {
			return fmt.Errorf("state: sqlite tx exists expires: %w", err)
		}
		ok = !IsExpired(exp)
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

func (t *sqliteTx) Scan(ctx context.Context, prefix string) ([]string, error) {
	if err := ValidateScanPrefix(prefix); err != nil {
		return nil, err
	}
	var out []string
	err := t.withLock(ctx, func() error {
		rows, err := t.tx.QueryContext(ctx, `SELECT key, expires_at FROM kv`)
		if err != nil {
			return fmt.Errorf("state: sqlite tx scan: %w", err)
		}
		defer rows.Close()
		out = make([]string, 0)
		for rows.Next() {
			var k string
			var expNS sql.NullString
			if err := rows.Scan(&k, &expNS); err != nil {
				return fmt.Errorf("state: sqlite tx scan row: %w", err)
			}
			if !strings.HasPrefix(k, prefix) {
				continue
			}
			exp, err := scanSQLiteExpires(expNS)
			if err != nil {
				return fmt.Errorf("state: sqlite tx scan expires: %w", err)
			}
			if IsExpired(exp) {
				continue
			}
			out = append(out, k)
		}
		return rows.Err()
	})
	return out, err
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
