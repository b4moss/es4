// Package es4 is the public library entry for Phase 1–3.
//
// The public surface is the State API (SET / GET / DELETE / EXISTS / CLEAR)
// and the Tx API (BeginTx). Snapshot and Recovery are internal and driven by
// Open / Close lifecycle.
package es4

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/b4moss/es4/packages/go/internal/recovery"
	"github.com/b4moss/es4/packages/go/internal/snapshot"
	"github.com/b4moss/es4/packages/go/internal/state"
	"github.com/b4moss/es4/packages/go/pkg/options"
)

// Re-export State sentinel errors for callers.
var (
	ErrNotFound     = state.ErrNotFound
	ErrInvalidKey   = state.ErrInvalidKey
	ErrInvalidValue = state.ErrInvalidValue
	ErrClosed       = state.ErrClosed
	ErrTxDone       = state.ErrTxDone
	ErrNestedTx     = state.ErrNestedTx
)

// Tx is the public transactional State surface (separate from the non-Tx API).
type Tx = state.Tx

// DB is an open Es4 handle. Methods implement the public State API and Tx API.
// Snapshot / Recovery run internally according to Options.Effective().
type DB struct {
	opts     options.Options // Effective values
	state    state.Store
	recovery recovery.Store // nil when recovery R/W disabled
	snap     *snapshot.Manager

	mu       sync.Mutex
	closed   bool
	restoreN sync.WaitGroup // tracks in-flight startup restore
	ready    atomic.Bool    // true after startup restore finishes (or when skipped)
}

// OpenConfig allows tests and advanced wiring to inject adapters.
// Production callers normally use Open with Options only.
type OpenConfig struct {
	Options  options.Options
	State    state.Store    // default: selected from Effective Options
	Recovery recovery.Store // default: built from Effective recovery_* Options
	// SkipAsyncRestore, when true, runs startup restore synchronously inside Open
	// (still does not gate the State API afterward). Used by tests that need
	// deterministic restore completion without waiting.
	SkipAsyncRestore bool
	// DisableRestore skips startup restore entirely (overrides RestoreOnStartup).
	DisableRestore bool
}

// Open builds a DB from Options. Consumes opts.Effective().
//
// Backend selection (when State is not injected via OpenWith):
//  1. memory_only → Memory (state_path / state_backend / redis / Firestore ignored)
//  2. state_backend=redis|valkey → Redis adapter (state_redis_url required)
//  3. state_backend=firestore → Firestore (project_id + collection required)
//  4. state_backend=memory → Memory
//  5. state_path non-empty (or state_backend=sqlite) → on-disk SQLite
//  6. empty state_path → Memory (compat)
//
// Recovery selection (when Recovery is not injected via OpenWith):
//   - memory_only → no Recovery (settings ignored)
//   - recovery_backend file|libsql|object (empty + recovery_path → file)
//
// Lifecycle:
//   - restore_on_startup (Effective): load recovery if present/readable;
//     missing/unreadable → empty State and continue (not fatal). Restore runs
//     asynchronously; State API is accepted before it completes.
//   - snapshot_interval (Effective) > 0: start periodic snapshots; stop on Close.
//   - memory_only (Effective): no periodic snapshot / recovery writes.
func Open(ctx context.Context, opts options.Options) (*DB, error) {
	return open(ctx, OpenConfig{Options: opts})
}

// OpenWith opens with explicit adapter injection (swappable State / Recovery).
// Injected State / Recovery take priority over Options-based selection.
func OpenWith(ctx context.Context, cfg OpenConfig) (*DB, error) {
	return open(ctx, cfg)
}

func open(ctx context.Context, cfg OpenConfig) (*DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	eff := cfg.Options.Effective()
	if err := eff.Validate(); err != nil {
		return nil, err
	}

	st := cfg.State
	if st == nil {
		var err error
		st, err = openDefaultState(eff)
		if err != nil {
			return nil, err
		}
	}

	var rec recovery.Store
	if cfg.Recovery != nil {
		// OpenWith injection wins over Options-based Recovery selection.
		rec = cfg.Recovery
	} else if !eff.MemoryOnly {
		var err error
		rec, err = recovery.OpenFromOptions(ctx, eff)
		if err != nil {
			_ = st.Close()
			return nil, err
		}
	}

	// Persistence writes only when recovery is configured and not memory_only.
	var snapRec recovery.Store
	if !eff.MemoryOnly && rec != nil {
		snapRec = rec
	}

	interval := eff.SnapshotInterval
	if eff.MemoryOnly {
		interval = 0
	}

	mgr := snapshot.NewManager(snapshot.Config{
		State:    st,
		Recovery: snapRec,
		Interval: interval,
	})

	db := &DB{
		opts:     eff,
		state:    st,
		recovery: rec,
		snap:     mgr,
	}

	doRestore := !cfg.DisableRestore && eff.RestoreOnStartup && !eff.MemoryOnly && rec != nil
	if doRestore {
		if cfg.SkipAsyncRestore {
			_ = db.restoreOnce(ctx)
			db.ready.Store(true)
		} else {
			db.restoreN.Add(1)
			go func() {
				defer db.restoreN.Done()
				_ = db.restoreOnce(context.Background())
				db.ready.Store(true)
			}()
		}
	} else {
		db.ready.Store(true)
	}

	mgr.Start()
	return db, nil
}

func openDefaultState(eff options.Options) (state.Store, error) {
	if eff.MemoryOnly {
		return state.NewMemory(), nil
	}
	if eff.IsRedisStateBackend() {
		st, err := state.OpenRedis(context.Background(), eff.StateRedisURL, eff.StateRedisKeyPrefix)
		if err != nil {
			return nil, fmt.Errorf("es4: open redis state: %w", err)
		}
		return st, nil
	}
	if eff.IsFirestoreStateBackend() {
		st, err := state.OpenFirestore(context.Background(),
			eff.StateFirestoreProjectID,
			eff.StateFirestoreDatabaseID,
			eff.StateFirestoreCollection,
		)
		if err != nil {
			return nil, fmt.Errorf("es4: open firestore state: %w", err)
		}
		return st, nil
	}
	if eff.StateBackend == options.StateBackendMemory {
		return state.NewMemory(), nil
	}
	if eff.StatePath != "" || eff.StateBackend == options.StateBackendSQLite {
		st, err := state.OpenSQLite(eff.StatePath)
		if err != nil {
			return nil, fmt.Errorf("es4: open sqlite state: %w", err)
		}
		return st, nil
	}
	return state.NewMemory(), nil
}

func (db *DB) restoreOnce(ctx context.Context) error {
	if db.recovery == nil {
		return nil
	}
	data, err := db.recovery.Load(ctx)
	if err != nil {
		// Missing or unreadable → empty State, continue (not fatal).
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	if err := snapshot.RestoreInto(ctx, db.state, data); err != nil {
		// Unreadable / corrupt envelope → empty State, continue.
		return nil
	}
	return nil
}

// Set stores a JSON value at key.
func (db *DB) Set(ctx context.Context, key string, value json.RawMessage) error {
	if err := db.guard(ctx); err != nil {
		return err
	}
	return db.state.Set(ctx, key, value)
}

// Get returns the JSON value at key. Missing key → ErrNotFound.
func (db *DB) Get(ctx context.Context, key string) (json.RawMessage, error) {
	if err := db.guard(ctx); err != nil {
		return nil, err
	}
	return db.state.Get(ctx, key)
}

// Delete removes key. Missing key → ErrNotFound.
func (db *DB) Delete(ctx context.Context, key string) error {
	if err := db.guard(ctx); err != nil {
		return err
	}
	return db.state.Delete(ctx, key)
}

// Exists reports whether key is present. Missing key → false, nil.
func (db *DB) Exists(ctx context.Context, key string) (bool, error) {
	if err := db.guard(ctx); err != nil {
		return false, err
	}
	return db.state.Exists(ctx, key)
}

// Clear removes all keys. Succeeds on an already empty store.
func (db *DB) Clear(ctx context.Context) error {
	if err := db.guard(ctx); err != nil {
		return err
	}
	return db.state.Clear(ctx)
}

// BeginTx starts a transaction (Tx API; separate from the non-Tx State API).
// Nested Begin returns ErrNestedTx. Uncommitted transactions are rolled back
// on Close.
func (db *DB) BeginTx(ctx context.Context) (Tx, error) {
	if err := db.guard(ctx); err != nil {
		return nil, err
	}
	return db.state.BeginTx(ctx)
}

// Ready reports whether startup Restore has finished (or was skipped).
// The library State / Tx API still accepts calls before Ready; Server
// readiness (/readyz) and HTTP State/Tx gates use this flag.
func (db *DB) Ready() bool {
	return db.ready.Load()
}

// Close stops periodic snapshots, rolls back any open Tx, and releases resources.
func (db *DB) Close() error {
	db.mu.Lock()
	if db.closed {
		db.mu.Unlock()
		return nil
	}
	db.closed = true
	db.mu.Unlock()

	db.snap.Stop()
	db.restoreN.Wait()
	if c, ok := db.recovery.(io.Closer); ok && c != nil {
		_ = c.Close()
	}
	return db.state.Close()
}

// snapshotNow takes an internal explicit Snapshot and resets the interval timer.
func (db *DB) snapshotNow(ctx context.Context) error {
	if err := db.guard(ctx); err != nil {
		return err
	}
	return db.snap.Take(ctx)
}

func (db *DB) waitRestore() {
	db.restoreN.Wait()
}

func (db *DB) guard(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		return ErrClosed
	}
	return nil
}

// IsNotFound reports whether err is or wraps ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
