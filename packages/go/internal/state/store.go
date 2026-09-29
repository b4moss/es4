// Package state defines the State Store adapter contract and the in-memory
// JSON implementation used by Phase 1.
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

// Store is the swappable State adapter surface.
type Store interface {
	Set(ctx context.Context, key string, value json.RawMessage) error
	Get(ctx context.Context, key string) (json.RawMessage, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context) error
	// Export returns a deep copy of all entries (for Snapshot payload).
	Export(ctx context.Context) (map[string]json.RawMessage, error)
	// Replace atomically replaces all entries (for Restore).
	Replace(ctx context.Context, entries map[string]json.RawMessage) error
	Close() error
}

// Memory is an in-process JSON document store.
type Memory struct {
	mu      sync.RWMutex
	entries map[string]json.RawMessage
	closed  bool
}

// NewMemory returns an empty in-memory State adapter.
func NewMemory() *Memory {
	return &Memory{entries: make(map[string]json.RawMessage)}
}

// ValidateKey checks Phase 1 hierarchical key rules (`/` separator).
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

	m.mu.RLock()
	defer m.mu.RUnlock()
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

	m.mu.RLock()
	defer m.mu.RUnlock()
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

	m.mu.RLock()
	defer m.mu.RUnlock()
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

func (m *Memory) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	m.entries = nil
	return nil
}
