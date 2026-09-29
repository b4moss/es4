package es4

import (
	"context"

	"github.com/b4moss/es4/packages/go/internal/recovery"
	"github.com/b4moss/es4/packages/go/internal/state"
)

// Test helpers (compiled only in tests of this package).

// SnapshotNow exposes the internal explicit snapshot for tests.
func (db *DB) SnapshotNow(ctx context.Context) error {
	return db.snapshotNow(ctx)
}

// WaitRestore waits for async startup restore (tests).
func (db *DB) WaitRestore() {
	db.waitRestore()
}

// StateForTest returns the underlying State adapter.
func (db *DB) StateForTest() state.Store { return db.state }

// RecoveryForTest returns the underlying Recovery adapter (may be nil).
func (db *DB) RecoveryForTest() recovery.Store { return db.recovery }
