Phase 1 Options slice (this PR)

Implemented:
- packages/go/pkg/options — Defaults, Load/FromFile, ApplyEnv, Effective
- docs/specs/options, docs/tests/options

Deferred (still plans/v0.2.0):
- State API SET/GET/DELETE/EXISTS/CLEAR
- Snapshot envelope / periodic snapshots
- File Recovery restore

Charter note: full plans→specs move waits until Phase 1 minimal-core is complete;
only the Options domain is in specs/ for this slice.
