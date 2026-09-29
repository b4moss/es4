// Package snapshot will hold Snapshot envelope / periodic snapshot logic.
//
// Not implemented in the Options slice. snapshot_interval and memory_only
// Effective() semantics come from pkg/options; when MemoryOnly is on,
// snapshot/recovery settings are ignored.
package snapshot
