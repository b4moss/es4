// Package recovery defines the Recovery Storage adapter and the single-file
// implementation used by Phase 1.
package recovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotFound indicates the recovery file is missing.
var ErrNotFound = errors.New("recovery: not found")

// Store is the swappable Recovery adapter surface.
type Store interface {
	// Save persists data via an atomic replace (temp + rename).
	Save(ctx context.Context, data []byte) error
	// Load reads the recovery point. Returns ErrNotFound when missing.
	Load(ctx context.Context) ([]byte, error)
}

// File is a single-file Recovery Storage at Path.
type File struct {
	Path string
}

// NewFile returns a file-backed Recovery adapter.
func NewFile(path string) *File {
	return &File{Path: path}
}

// Save writes data atomically: write temp file in the same directory, then rename.
func (f *File) Save(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.Path == "" {
		return fmt.Errorf("recovery: empty path")
	}
	dir := filepath.Dir(f.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("recovery: mkdir %q: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".es4-recovery-*.tmp")
	if err != nil {
		return fmt.Errorf("recovery: create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("recovery: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("recovery: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("recovery: close temp: %w", err)
	}
	if err := os.Rename(tmpName, f.Path); err != nil {
		return fmt.Errorf("recovery: rename: %w", err)
	}
	cleanup = false
	return nil
}

// Load reads the recovery file. Missing file → ErrNotFound.
// Other read errors are returned as-is (caller may treat as empty State).
func (f *File) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.Path == "" {
		return nil, ErrNotFound
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("recovery: read %q: %w", f.Path, err)
	}
	return data, nil
}
