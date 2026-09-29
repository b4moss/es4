package recovery

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

const libsqlSchema = `
CREATE TABLE IF NOT EXISTS es4_recovery (
  gen_id TEXT PRIMARY KEY NOT NULL,
  created_at INTEGER NOT NULL,
  payload BLOB NOT NULL
);
`

// LibSQL stores recovery generations in a libSQL / SQLite-compatible database.
type LibSQL struct {
	db    *sql.DB
	TTL   time.Duration
	Clock func() time.Time
	owns  bool // close DB on Close when opened by OpenLibSQL
}

// NewLibSQL wraps an existing *sql.DB (tests may inject modernc sqlite).
func NewLibSQL(db *sql.DB, ttl time.Duration) (*LibSQL, error) {
	if db == nil {
		return nil, fmt.Errorf("recovery: nil libsql db")
	}
	s := &LibSQL{db: db, TTL: ttl, Clock: time.Now}
	if err := s.ensureSchema(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

// OpenLibSQL opens a libSQL Recovery adapter from URL + optional auth token.
// Local file URLs (`file:` or plain paths) use the pure-Go SQLite driver.
// Remote `libsql://` / `http(s)://` URLs use the libSQL client driver.
func OpenLibSQL(dsn, authToken string, ttl time.Duration) (*LibSQL, error) {
	db, err := openLibSQLDB(dsn, authToken)
	if err != nil {
		return nil, err
	}
	s, err := NewLibSQL(db, ttl)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	s.owns = true
	return s, nil
}

func openLibSQLDB(dsn, authToken string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("recovery: empty libsql url")
	}
	switch {
	case strings.HasPrefix(dsn, "libsql://"),
		strings.HasPrefix(dsn, "https://"),
		strings.HasPrefix(dsn, "http://"):
		remote := dsn
		if authToken != "" {
			remote = appendAuthToken(remote, authToken)
		}
		db, err := sql.Open("libsql", remote)
		if err != nil {
			return nil, fmt.Errorf("recovery: open libsql: %w", err)
		}
		return db, nil
	default:
		path := dsn
		if strings.HasPrefix(dsn, "file:") {
			path = strings.TrimPrefix(dsn, "file:")
			// Allow file:/abs and file:./rel; strip query if present.
			if i := strings.IndexByte(path, '?'); i >= 0 {
				path = path[:i]
			}
		}
		db, err := sql.Open("sqlite", path)
		if err != nil {
			return nil, fmt.Errorf("recovery: open local libsql/sqlite: %w", err)
		}
		return db, nil
	}
}

func appendAuthToken(dsn, token string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		return dsn + sep + "authToken=" + url.QueryEscape(token)
	}
	q := u.Query()
	q.Set("authToken", token)
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *LibSQL) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}

func (s *LibSQL) ensureSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, libsqlSchema); err != nil {
		return fmt.Errorf("recovery: libsql schema: %w", err)
	}
	return nil
}

// Close closes the underlying DB when opened via OpenLibSQL.
func (s *LibSQL) Close() error {
	if s == nil || s.db == nil || !s.owns {
		return nil
	}
	return s.db.Close()
}

// Save inserts a new generation and prunes per TTL.
func (s *LibSQL) Save(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := s.now().UTC()
	id := genID(now)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO es4_recovery (gen_id, created_at, payload) VALUES (?, ?, ?)`,
		id, now.UnixNano(), data,
	)
	if err != nil {
		return fmt.Errorf("recovery: libsql insert: %w", err)
	}
	return s.prune(ctx, now)
}

func (s *LibSQL) prune(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT gen_id, created_at FROM es4_recovery ORDER BY gen_id ASC`)
	if err != nil {
		return fmt.Errorf("recovery: libsql list: %w", err)
	}
	defer rows.Close()

	type row struct {
		id        string
		createdAt int64
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.createdAt); err != nil {
			return fmt.Errorf("recovery: libsql scan: %w", err)
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(all) == 0 {
		return nil
	}
	newest := all[len(all)-1].id
	for _, r := range all {
		if r.id == newest {
			continue
		}
		created := time.Unix(0, r.createdAt).UTC()
		if keepGeneration(created, now, s.TTL) {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM es4_recovery WHERE gen_id = ?`, r.id); err != nil {
			return fmt.Errorf("recovery: libsql prune: %w", err)
		}
	}
	return nil
}

// Load returns the latest generation payload.
func (s *LibSQL) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var payload []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT payload FROM es4_recovery ORDER BY gen_id DESC LIMIT 1`,
	).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("recovery: libsql load: %w", err)
	}
	return payload, nil
}
