// Package options is the canonical configuration surface for the Es4 library.
//
// Setting keys use snake_case. The same concept names and defaults are shared
// with later Server environment variables (prefix ES4_ + SCREAMING_SNAKE).
//
// Assemble order: Defaults → optional YAML file → ES4_* env overlay.
// When MemoryOnly is true, recovery-related settings, state_path, and
// state_backend / redis settings are ignored (not rejected);
// downstream Snapshot / Recovery / Open should consume Effective() values.
package options

import (
	"fmt"
	"time"
)

// EnvPrefix is the Server / env overlay prefix (Phase 4 mapping).
const EnvPrefix = "ES4_"

// DefaultSnapshotInterval is the Phase 1 default for snapshot_interval.
const DefaultSnapshotInterval = 30 * time.Second

// Recovery backend identifiers (recovery_backend).
const (
	RecoveryBackendFile   = "file"
	RecoveryBackendLibSQL = "libsql"
	RecoveryBackendObject = "object"
)

// State backend identifiers (state_backend).
const (
	StateBackendMemory = "memory"
	StateBackendSQLite = "sqlite"
	StateBackendRedis  = "redis"
	StateBackendValkey = "valkey" // alias of redis (same adapter)
)

// Options holds library configuration. Options are the source of truth;
// an optional YAML config file and env overlay only help build them.
type Options struct {
	// SnapshotInterval is snapshot_interval (default 30s).
	SnapshotInterval time.Duration

	// RestoreOnStartup is restore_on_startup (default true).
	RestoreOnStartup bool

	// MemoryOnly is memory_only (default false). When true, recovery-related
	// settings, state_path, and state_backend / redis settings are ignored —
	// see Effective.
	MemoryOnly bool

	// RecoveryPath is recovery_path (file Recovery location). Empty by default.
	// Consumed by file Recovery; ignored when MemoryOnly is true.
	RecoveryPath string

	// RecoveryBackend is recovery_backend: file | libsql | object.
	// Empty with a non-empty RecoveryPath implies file.
	RecoveryBackend string

	// RecoveryTTL is recovery_ttl (Go duration). Default 0 keeps only the latest
	// generation. Positive values prune older generations after each Save.
	RecoveryTTL time.Duration

	// RecoveryLibSQLURL is recovery_libsql_url (required when backend=libsql).
	RecoveryLibSQLURL string

	// RecoveryLibSQLAuthToken is recovery_libsql_auth_token (optional).
	RecoveryLibSQLAuthToken string

	// RecoveryS3Bucket is recovery_s3_bucket (required when backend=object).
	RecoveryS3Bucket string

	// RecoveryS3Prefix is recovery_s3_prefix (optional object key prefix).
	RecoveryS3Prefix string

	// RecoveryS3Region is recovery_s3_region (optional; AWS default chain otherwise).
	RecoveryS3Region string

	// RecoveryS3Endpoint is recovery_s3_endpoint (optional; S3-compatible / GCS).
	RecoveryS3Endpoint string

	// StatePath is state_path (on-disk SQLite State file). Empty by default.
	// When non-empty and MemoryOnly is false (and state_backend is not
	// redis/valkey/memory), Open selects the SQLite backend.
	// Ignored when MemoryOnly is true.
	StatePath string

	// StateBackend is state_backend: "" | memory | sqlite | redis | valkey.
	// Empty preserves path / memory_only selection (compat).
	// valkey is an alias of redis (same adapter).
	StateBackend string

	// StateRedisURL is state_redis_url (required when state_backend=redis|valkey).
	StateRedisURL string

	// StateRedisKeyPrefix is state_redis_key_prefix (optional HASH key / namespace).
	// Empty uses the default Redis HASH name es4:state.
	StateRedisKeyPrefix string
}

// Defaults returns Phase 1–3 default Options (plus empty state_backend keys).
func Defaults() Options {
	return Options{
		SnapshotInterval: DefaultSnapshotInterval,
		RestoreOnStartup: true,
		MemoryOnly:       false,
		RecoveryPath:     "",
		RecoveryBackend:  "",
		RecoveryTTL:      0,
		StatePath:        "",
		StateBackend:     "",
	}
}

// Effective returns Options with recovery-related fields, state_path, and
// state_backend / redis fields cleared when MemoryOnly is true.
// Cleared fields: SnapshotInterval, RestoreOnStartup, RecoveryPath,
// RecoveryBackend, RecoveryTTL, libSQL/S3 recovery keys, StatePath,
// StateBackend, StateRedisURL, StateRedisKeyPrefix.
// Does not error if those fields were set while MemoryOnly is true.
func (o Options) Effective() Options {
	if !o.MemoryOnly {
		return o
	}
	return Options{
		SnapshotInterval:        0,
		RestoreOnStartup:        false,
		MemoryOnly:              true,
		RecoveryPath:            "",
		RecoveryBackend:         "",
		RecoveryTTL:             0,
		RecoveryLibSQLURL:       "",
		RecoveryLibSQLAuthToken: "",
		RecoveryS3Bucket:        "",
		RecoveryS3Prefix:        "",
		RecoveryS3Region:        "",
		RecoveryS3Endpoint:      "",
		StatePath:               "",
		StateBackend:            "",
		StateRedisURL:           "",
		StateRedisKeyPrefix:     "",
	}
}

// UsesRecovery reports whether recovery settings should be honored.
func (o Options) UsesRecovery() bool {
	return !o.MemoryOnly
}

// ResolvedRecoveryBackend returns the effective recovery backend name.
// Empty backend with a non-empty RecoveryPath resolves to "file".
// Empty backend and empty RecoveryPath returns "" (no Recovery adapter).
func (o Options) ResolvedRecoveryBackend() string {
	if o.RecoveryBackend != "" {
		return o.RecoveryBackend
	}
	if o.RecoveryPath != "" {
		return RecoveryBackendFile
	}
	return ""
}

// IsRedisStateBackend reports whether state_backend selects the Redis protocol
// adapter (redis or valkey alias).
func (o Options) IsRedisStateBackend() bool {
	switch o.StateBackend {
	case StateBackendRedis, StateBackendValkey:
		return true
	default:
		return false
	}
}

// Validate checks recovery_backend / recovery_ttl / state_backend and
// backend-required keys. Callers that consume Effective Options (e.g. Open)
// should Validate the Effective view. When MemoryOnly is true, recovery and
// redis required-key checks are skipped (those settings are ignored, not
// rejected).
func (o Options) Validate() error {
	if o.RecoveryTTL < 0 {
		return fmt.Errorf("options: recovery_ttl: must be >= 0")
	}
	switch o.RecoveryBackend {
	case "", RecoveryBackendFile, RecoveryBackendLibSQL, RecoveryBackendObject:
		// ok
	default:
		return fmt.Errorf("options: recovery_backend: want file|libsql|object, got %q", o.RecoveryBackend)
	}
	switch o.StateBackend {
	case "", StateBackendMemory, StateBackendSQLite, StateBackendRedis, StateBackendValkey:
		// ok
	default:
		return fmt.Errorf("options: state_backend: want memory|sqlite|redis|valkey, got %q", o.StateBackend)
	}
	if o.MemoryOnly {
		return nil
	}
	switch o.ResolvedRecoveryBackend() {
	case RecoveryBackendFile:
		if o.RecoveryPath == "" {
			return fmt.Errorf("options: recovery_path: required when recovery_backend=file")
		}
	case RecoveryBackendLibSQL:
		if o.RecoveryLibSQLURL == "" {
			return fmt.Errorf("options: recovery_libsql_url: required when recovery_backend=libsql")
		}
	case RecoveryBackendObject:
		if o.RecoveryS3Bucket == "" {
			return fmt.Errorf("options: recovery_s3_bucket: required when recovery_backend=object")
		}
	}
	if o.IsRedisStateBackend() && o.StateRedisURL == "" {
		return fmt.Errorf("options: state_redis_url: required when state_backend=%s", o.StateBackend)
	}
	if o.StateBackend == StateBackendSQLite && o.StatePath == "" {
		return fmt.Errorf("options: state_path: required when state_backend=sqlite")
	}
	return nil
}
