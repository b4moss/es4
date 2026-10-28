package state

import (
	"sync"
	"time"
)

// SetMeta carries optional write metadata for Store.Set / Tx.Set.
// A nil ExpiresAt means no expiry (and clears any prior expiry on overwrite).
type SetMeta struct {
	ExpiresAt *time.Time
}

// FirstMeta returns meta[0] when present, otherwise a zero SetMeta (no expiry).
func FirstMeta(meta []SetMeta) SetMeta {
	if len(meta) == 0 {
		return SetMeta{}
	}
	return meta[0]
}

var (
	nowMu   sync.RWMutex
	nowFunc = func() time.Time { return time.Now().UTC() }
)

// Now returns the current UTC time (overridable in tests via SetNowFunc).
func Now() time.Time {
	nowMu.RLock()
	f := nowFunc
	nowMu.RUnlock()
	return f()
}

// SetNowFunc overrides the clock used for expiry checks. Pass nil to restore
// time.Now().UTC. Intended for tests.
func SetNowFunc(f func() time.Time) {
	nowMu.Lock()
	defer nowMu.Unlock()
	if f == nil {
		nowFunc = func() time.Time { return time.Now().UTC() }
		return
	}
	nowFunc = f
}

// IsExpired reports whether expiresAt is set and not after Now().
// nil expiresAt is never expired.
func IsExpired(expiresAt *time.Time) bool {
	if expiresAt == nil {
		return false
	}
	return !expiresAt.After(Now())
}

// CloneTimePtr returns a UTC copy of t, or nil.
func CloneTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// FormatExpiresAt formats t as RFC3339Nano UTC for SQLite / Redis meta storage.
func FormatExpiresAt(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// ParseExpiresAt parses an RFC3339 / RFC3339Nano expiry string.
func ParseExpiresAt(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, err
		}
	}
	u := t.UTC()
	return &u, nil
}
