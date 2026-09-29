package recovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	genFilePrefix = "gen-"
	genFileSuffix = ".snap"
)

// File is a file-backed Recovery Storage.
//
// When TTL <= 0, Path is a single file overwritten atomically (Phase 1 compat).
// When TTL > 0, Path is a directory of generation files; Load returns the latest
// and Save prunes generations older than TTL.
type File struct {
	Path  string
	TTL   time.Duration
	Clock func() time.Time
}

// NewFile returns a file-backed Recovery adapter (TTL=0 single-file compat).
func NewFile(path string) *File {
	return NewFileTTL(path, 0)
}

// NewFileTTL returns a file-backed Recovery adapter with the given TTL.
func NewFileTTL(path string, ttl time.Duration) *File {
	return &File{
		Path:  path,
		TTL:   ttl,
		Clock: time.Now,
	}
}

func (f *File) now() time.Time {
	if f.Clock != nil {
		return f.Clock()
	}
	return time.Now()
}

// Save writes a new generation. TTL<=0 atomically replaces the single file;
// TTL>0 writes a generation file under Path and prunes older ones.
func (f *File) Save(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.Path == "" {
		return fmt.Errorf("recovery: empty path")
	}
	if f.TTL <= 0 {
		return f.saveSingle(data)
	}
	return f.saveGeneration(ctx, data)
}

func (f *File) saveSingle(data []byte) error {
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

func (f *File) saveGeneration(ctx context.Context, data []byte) error {
	if err := os.MkdirAll(f.Path, 0o755); err != nil {
		return fmt.Errorf("recovery: mkdir %q: %w", f.Path, err)
	}
	id := genID(f.now())
	name := genFilePrefix + id + genFileSuffix
	dest := filepath.Join(f.Path, name)
	tmp, err := os.CreateTemp(f.Path, ".es4-recovery-*.tmp")
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
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("recovery: rename: %w", err)
	}
	cleanup = false
	return f.pruneGenerations(ctx)
}

func (f *File) pruneGenerations(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	gens, err := f.listGenerations()
	if err != nil {
		return err
	}
	if len(gens) == 0 {
		return nil
	}
	now := f.now()
	// Always keep the newest; drop others outside TTL (or all-but-newest when TTL<=0).
	newest := gens[len(gens)-1]
	for _, g := range gens {
		if g.id == newest.id {
			continue
		}
		created, err := parseGenID(g.id)
		if err != nil {
			_ = os.Remove(g.path)
			continue
		}
		if keepGeneration(created, now, f.TTL) {
			continue
		}
		_ = os.Remove(g.path)
	}
	return nil
}

type fileGen struct {
	id   string
	path string
}

func (f *File) listGenerations() ([]fileGen, error) {
	entries, err := os.ReadDir(f.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("recovery: readdir %q: %w", f.Path, err)
	}
	var gens []fileGen
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, genFilePrefix) || !strings.HasSuffix(name, genFileSuffix) {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, genFilePrefix), genFileSuffix)
		if _, err := parseGenID(id); err != nil {
			continue
		}
		gens = append(gens, fileGen{id: id, path: filepath.Join(f.Path, name)})
	}
	sort.Slice(gens, func(i, j int) bool { return gens[i].id < gens[j].id })
	return gens, nil
}

// Load reads the recovery point. Missing → ErrNotFound.
func (f *File) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.Path == "" {
		return nil, ErrNotFound
	}
	if f.TTL <= 0 {
		return f.loadSingle()
	}
	return f.loadLatestGeneration()
}

func (f *File) loadSingle() ([]byte, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("recovery: read %q: %w", f.Path, err)
	}
	return data, nil
}

func (f *File) loadLatestGeneration() ([]byte, error) {
	gens, err := f.listGenerations()
	if err != nil {
		return nil, err
	}
	if len(gens) == 0 {
		return nil, ErrNotFound
	}
	latest := gens[len(gens)-1]
	data, err := os.ReadFile(latest.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("recovery: read %q: %w", latest.path, err)
	}
	return data, nil
}
