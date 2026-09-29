// Package options is the canonical configuration surface for the Es4 library.
//
// Setting keys use snake_case. The same concept names and defaults are shared
// with later Server environment variables (prefix ES4_ + SCREAMING_SNAKE).
//
// When MemoryOnly is on, recovery-related settings are ignored (not rejected).
// Downstream Snapshot / Recovery should consume Effective() values.
package options

import "time"

// EnvPrefix is the Server / env overlay prefix (Phase 4 mapping).
const EnvPrefix = "ES4_"

// DefaultSnapshotInterval is the Phase 1 default for snapshot_interval.
const DefaultSnapshotInterval = 30 * time.Second

// Options holds library configuration. Options are the source of truth;
// an optional config file and env overlay only help build them.
type Options struct {
	// SnapshotInterval is snapshot_interval (default 30s).
	SnapshotInterval time.Duration

	// RestoreOnStartup is restore_on_startup (default on).
	RestoreOnStartup bool

	// MemoryOnly is memory_only (default off). When on, recovery-related
	// settings are ignored — see Effective.
	MemoryOnly bool

	// RecoveryPath is recovery_path (file Recovery location). Empty by default.
	// Consumed by Recovery once implemented; ignored when MemoryOnly is on.
	RecoveryPath string
}

// Defaults returns Phase 1 default Options.
func Defaults() Options {
	return Options{
		SnapshotInterval: DefaultSnapshotInterval,
		RestoreOnStartup: true,
		MemoryOnly:       false,
		RecoveryPath:     "",
	}
}

// Effective returns Options with recovery-related fields cleared when
// MemoryOnly is on. Cleared fields: SnapshotInterval, RestoreOnStartup,
// RecoveryPath. Does not error if those fields were set while MemoryOnly is on.
func (o Options) Effective() Options {
	if !o.MemoryOnly {
		return o
	}
	return Options{
		SnapshotInterval: 0,
		RestoreOnStartup: false,
		MemoryOnly:       true,
		RecoveryPath:     "",
	}
}

// UsesRecovery reports whether recovery settings should be honored.
func (o Options) UsesRecovery() bool {
	return !o.MemoryOnly
}
