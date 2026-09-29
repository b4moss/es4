// Package state defines the State Store adapter contract, in-memory,
// on-disk SQLite, Redis-protocol (Redis / Valkey), and Firestore
// implementations, and the Tx surface (separate from Store).
//
// Adapter boundary (Phase 5 / SemVer v0.6.0; Redis/Valkey + Firestore in v0.8.0):
//   - All backends share this Store (+ Tx) surface. Public pkg/es4 contracts
//     must not grow backend-specific APIs.
//   - Export returns a deep copy (caller mutations of the map or RawMessage
//     bytes must not affect the Store; Store mutations must not mutate a
//     previously returned Export map).
//   - Replace is atomic: after success only the new entry set is observable;
//     concurrent readers may block on locks but must not see a torn mix of
//     old and new keys.
//   - Internal optimizations of Memory / SQLite / Redis / Firestore are
//     allowed only when they preserve these shared semantics.
package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Sentinel errors for the State API.
var (
	ErrNotFound     = errors.New("state: key not found")
	ErrInvalidKey   = errors.New("state: invalid key")
	ErrInvalidValue = errors.New("state: invalid JSON value")
	ErrClosed       = errors.New("state: closed")
)

// Store is the swappable State adapter surface shared by Memory, SQLite, and
// Redis/Valkey (and any future backend). Implementations must honor Export
// deep-copy, Replace atomicity, and ErrClosed after Close. See package docs.
type Store interface {
	Set(ctx context.Context, key string, value json.RawMessage) error
	Get(ctx context.Context, key string) (json.RawMessage, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context) error
	// Export returns a deep copy of all entries (for Snapshot payload).
	// The returned map and each RawMessage are independent of Store storage.
	Export(ctx context.Context) (map[string]json.RawMessage, error)
	// Replace atomically replaces all entries (for Restore).
	// On success, only entries is visible; an empty map clears the Store.
	Replace(ctx context.Context, entries map[string]json.RawMessage) error
	// BeginTx starts a transaction. Nested Begin returns ErrNestedTx.
	BeginTx(ctx context.Context) (Tx, error)
	// Close releases resources. Subsequent Store ops return ErrClosed
	// (or wrap it). Double Close is idempotent for Memory and SQLite.
	Close() error
}

// Memory is an in-process JSON document store.
type Memory struct {
	mu      sync.Mutex
	entries map[string]json.RawMessage
	closed  bool
	active  *memoryTx
}

// NewMemory returns an empty in-memory State adapter.
func NewMemory() *Memory {
	return &Memory{entries: make(map[string]json.RawMessage)}
}

// ValidateKey checks hierarchical key rules (`/` separator).
// Rejects empty, leading `/`, trailing `/`, and consecutive `//`.
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty", ErrInvalidKey)
	}
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("%w: leading slash", ErrInvalidKey)
	}
	if strings.HasSuffix(key, "/") {
		return fmt.Errorf("%w: trailing slash", ErrInvalidKey)
	}
	if strings.Contains(key, "//") {
		return fmt.Errorf("%w: consecutive slashes", ErrInvalidKey)
	}
	return nil
}

// ValidateValue ensures value is valid JSON (any document).
func ValidateValue(value json.RawMessage) error {
	if len(value) == 0 {
		return fmt.Errorf("%w: empty", ErrInvalidValue)
	}
	if !json.Valid(value) {
		return fmt.Errorf("%w: not valid JSON", ErrInvalidValue)
	}
	return nil
}

func (m *Memory) Set(ctx context.Context, key string, value json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	cp := append(json.RawMessage(nil), value...)

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	m.entries[key] = cp
	return nil
}

func (m *Memory) Get(ctx context.Context, key string) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateKey(key); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrClosed
	}
	v, ok := m.entries[key]
	if !ok {
		return nil, ErrNotFound
	}
	return append(json.RawMessage(nil), v...), nil
}

func (m *Memory) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	if _, ok := m.entries[key]; !ok {
		return ErrNotFound
	}
	delete(m.entries, key)
	return nil
}

func (m *Memory) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := ValidateKey(key); err != nil {
		return false, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return false, ErrClosed
	}
	_, ok := m.entries[key]
	return ok, nil
}

func (m *Memory) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	m.entries = make(map[string]json.RawMessage)
	return nil
}

func (m *Memory) Export(ctx context.Context) (map[string]json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrClosed
	}
	out := make(map[string]json.RawMessage, len(m.entries))
	for k, v := range m.entries {
		out[k] = append(json.RawMessage(nil), v...)
	}
	return out, nil
}

func (m *Memory) Replace(ctx context.Context, entries map[string]json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	next := make(map[string]json.RawMessage, len(entries))
	for k, v := range entries {
		if err := ValidateKey(k); err != nil {
			return err
		}
		if err := ValidateValue(v); err != nil {
			return err
		}
		next[k] = append(json.RawMessage(nil), v...)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	m.entries = next
	return nil
}

func (m *Memory) BeginTx(ctx context.Context) (Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrClosed
	}
	if m.active != nil {
		return nil, ErrNestedTx
	}
	tx := &memoryTx{
		m:       m,
		overlay: make(map[string]overlayEntry),
	}
	m.active = tx
	return tx, nil
}

func (m *Memory) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active != nil {
		m.active.done = true
		m.active = nil
	}
	m.closed = true
	m.entries = nil
	return nil
}

type overlayEntry struct {
	deleted bool
	value   json.RawMessage
}

type memoryTx struct {
	m       *Memory
	overlay map[string]overlayEntry
	cleared bool
	done    bool
}

func (t *memoryTx) withLock(ctx context.Context, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t.m.mu.Lock()
	defer t.m.mu.Unlock()
	if t.done || t.m.active != t {
		return ErrTxDone
	}
	if t.m.closed {
		return ErrClosed
	}
	return fn()
}

func (t *memoryTx) Set(ctx context.Context, key string, value json.RawMessage) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(value); err != nil {
		return err
	}
	cp := append(json.RawMessage(nil), value...)
	return t.withLock(ctx, func() error {
		t.overlay[key] = overlayEntry{value: cp}
		return nil
	})
}

func (t *memoryTx) Get(ctx context.Context, key string) (json.RawMessage, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	var out json.RawMessage
	err := t.withLock(ctx, func() error {
		if e, ok := t.overlay[key]; ok {
			if e.deleted {
				return ErrNotFound
			}
			out = append(json.RawMessage(nil), e.value...)
			return nil
		}
		if t.cleared {
			return ErrNotFound
		}
		v, ok := t.m.entries[key]
		if !ok {
			return ErrNotFound
		}
		out = append(json.RawMessage(nil), v...)
		return nil
	})
	return out, err
}

func (t *memoryTx) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	return t.withLock(ctx, func() error {
		if e, ok := t.overlay[key]; ok {
			if e.deleted {
				return ErrNotFound
			}
			t.overlay[key] = overlayEntry{deleted: true}
			return nil
		}
		if t.cleared {
			return ErrNotFound
		}
		if _, ok := t.m.entries[key]; !ok {
			return ErrNotFound
		}
		t.overlay[key] = overlayEntry{deleted: true}
		return nil
	})
}

func (t *memoryTx) Exists(ctx context.Context, key string) (bool, error) {
	if err := ValidateKey(key); err != nil {
		return false, err
	}
	var ok bool
	err := t.withLock(ctx, func() error {
		if e, hit := t.overlay[key]; hit {
			ok = !e.deleted
			return nil
		}
		if t.cleared {
			ok = false
			return nil
		}
		_, ok = t.m.entries[key]
		return nil
	})
	return ok, err
}

func (t *memoryTx) Clear(ctx context.Context) error {
	return t.withLock(ctx, func() error {
		t.cleared = true
		t.overlay = make(map[string]overlayEntry)
		return nil
	})
}

func (t *memoryTx) Commit() error {
	t.m.mu.Lock()
	defer t.m.mu.Unlock()
	if t.done || t.m.active != t {
		return ErrTxDone
	}
	if t.m.closed {
		return ErrClosed
	}
	if t.cleared {
		t.m.entries = make(map[string]json.RawMessage)
	}
	for k, e := range t.overlay {
		if e.deleted {
			delete(t.m.entries, k)
			continue
		}
		t.m.entries[k] = append(json.RawMessage(nil), e.value...)
	}
	t.done = true
	t.m.active = nil
	return nil
}

func (t *memoryTx) Rollback() error {
	t.m.mu.Lock()
	defer t.m.mu.Unlock()
	if t.done || t.m.active != t {
		return ErrTxDone
	}
	t.done = true
	t.m.active = nil
	return nil
}
