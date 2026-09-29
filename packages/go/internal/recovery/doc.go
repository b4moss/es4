// Package recovery will hold file-based Recovery Storage (Phase 1).
//
// Not implemented in the Options slice. The recovery file path is supplied via
// options.Options.RecoveryPath (snake_case key recovery_path / env
// ES4_RECOVERY_PATH). When options.Options.MemoryOnly is on, recovery-related
// settings are ignored — callers should use Options.Effective().
package recovery
