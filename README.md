# es4

[![CI](https://github.com/b4moss/es4/actions/workflows/ci.yml/badge.svg)](https://github.com/b4moss/es4/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/codecov/c/github/b4moss/es4)](https://codecov.io/gh/b4moss/es4)
[![Go Reference](https://pkg.go.dev/badge/github.com/b4moss/es4/packages/go.svg)](https://pkg.go.dev/github.com/b4moss/es4/packages/go)
[![Release](https://img.shields.io/github/v/release/b4moss/es4)](https://github.com/b4moss/es4/releases)
[![License](https://img.shields.io/github/license/b4moss/es4)](https://github.com/b4moss/es4/blob/main/LICENSE)
[![OpenSSF Scorecard](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fapi.scorecard.dev%2Fprojects%2Fgithub.com%2Fb4moss%2Fes4&query=%24.score&label=OpenSSF%20Scorecard&suffix=%2F10)](https://scorecard.dev/viewer/?uri=github.com/b4moss/es4)

Embedded server-side state store with pluggable Recovery — for when you want durable in-process state without bringing in Redis/Valkey.

- [日本語版](./README_ja.md)

Product knowledge lives under **[`docs/`](./docs/)** (OKF). This root README is the canonical English entry for install and usage; deeper specs and plans stay in `docs/`.

Current monorepo line: **Git tag `v0.7.3`** (Go library module path still `github.com/b4moss/es4/packages/go`).

## Purpose

- Keep a JSON key/value **State** in-process (Memory or on-disk SQLite)
- Persist recovery points through a **Recovery** adapter (file / libSQL / S3-compatible object)
- Drive Snapshot → Save → Restore on Open without exposing Snapshot/Recovery as a public API
- Use as an **embedded library** or as a standalone **HTTP Es4 Server** (`packages/go/cmd/es4-server`)

## Scope

**In scope (implemented in Go at `v0.7.3`):**

| Layer | Choices |
|-------|---------|
| State | Memory · on-disk SQLite (`state_path`) · Redis／Valkey (`state_backend` + `state_redis_url`) · Firestore (`state_backend=firestore`) |
| Recovery | `file` · `libsql` (local) · `object` (S3-compatible) |
| Public API | State: `Set` / `Get` / `Delete` / `Exists` / `Clear` · Tx: `BeginTx` → Commit/Rollback |
| Server | HTTP Es4 Server + Docker image workflows |
| E2E | Object (RustFS) · File SQLite · Memory · libSQL File/Memory · Redis State · Valkey State · Firestore Emulator (`workflow_dispatch`; redis／valkey／firestore **not** in `layer=all`) |

**Out of scope / not yet:**

- Node.js / TypeScript port (`packages/node` is a stub)
- Remote Turso (`libsql://`) in product E2E
- Production GCP-required Firestore E2E (Emulator only)
- Public `SnapshotNow` API (explicit flush exists only as a **test helper** in `export_test.go`)

## Install

**Requirements:** Go **1.24+** (module declares `go 1.24`; toolchain `go1.26.8` in `packages/go/go.mod`).

Module path (current, until a rename is approved):

```text
github.com/b4moss/es4/packages/go
```

Nested Go module tags (`packages/go/vX.Y.Z`) are not published yet (`packages/go/VERSION` is still `0.7.1`). Pin the monorepo commit for tag `v0.7.3`:

```bash
go get github.com/b4moss/es4/packages/go@b43247463bb76a8758829ad819f4070b9986e366
```

Import:

```go
import (
    "github.com/b4moss/es4/packages/go/pkg/es4"
    "github.com/b4moss/es4/packages/go/pkg/options"
)
```

Optional HTTP server binary (from a clone):

```bash
cd packages/go
go run ./cmd/es4-server
```

Server options load from `ES4_*` env (same snake_case keys as library Options). See [`docs/specs/es4-server/`](./docs/specs/es4-server/).

## Quick start

Minimal **memory-only** store (no Recovery):

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    "github.com/b4moss/es4/packages/go/pkg/es4"
    "github.com/b4moss/es4/packages/go/pkg/options"
)

func main() {
    ctx := context.Background()
    db, err := es4.Open(ctx, options.Options{
        MemoryOnly: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    if err := db.Set(ctx, "users/1", json.RawMessage(`{"name":"ada"}`)); err != nil {
        log.Fatal(err)
    }
    raw, err := db.Get(ctx, "users/1")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(string(raw))
}
```

Local **SQLite State + file Recovery** (periodic Snapshot; default interval 30s when unset):

```go
db, err := es4.Open(ctx, options.Options{
    StatePath:        "./data/state.db",
    RecoveryBackend:  options.RecoveryBackendFile, // or omit if RecoveryPath is set
    RecoveryPath:     "./data/recovery.snap",
    RestoreOnStartup: true,
})
```

Keys are hierarchical strings separated by `/` (no leading/trailing `/`, no `//`). Values are arbitrary JSON documents (`json.RawMessage`). Missing keys: `Get`/`Delete` → `es4.ErrNotFound`; `Exists` → `false, nil`.

Transactions (separate Tx surface):

```go
tx, err := db.BeginTx(ctx)
if err != nil {
    log.Fatal(err)
}
_ = tx.Set(ctx, "users/2", json.RawMessage(`{"name":"grace"}`))
if err := tx.Commit(); err != nil {
    log.Fatal(err)
}
```

Assemble Options from Defaults → optional YAML → `ES4_*` env with `options.Load(path, nil)`.

## Usage — Recovery backends & product E2E

Library Open selects backends from `options.Options.Effective()` (see [`docs/specs/options/`](./docs/specs/options/) and `packages/go/pkg/options`). Snapshot/Recovery are **internal**; production code relies on `snapshot_interval` (default `30s`) and `restore_on_startup` (default `true`). E2E tests set `snapshot_interval=0` and call the **test-only** `SnapshotNow` helper to flush explicitly.

### Library wiring (minimal)

**Memory (no persistence)**

```go
es4.Open(ctx, options.Options{MemoryOnly: true})
```

**File Recovery** (`recovery_backend=file` or non-empty `recovery_path`)

```go
es4.Open(ctx, options.Options{
    StatePath:       "/var/es4/state.db", // empty → Memory State
    RecoveryBackend: options.RecoveryBackendFile,
    RecoveryPath:    "/var/es4/recovery.snap",
})
```

**Object Recovery** (S3-compatible; needs credentials in the process environment / AWS default chain)

```go
es4.Open(ctx, options.Options{
    RecoveryBackend:    options.RecoveryBackendObject,
    RecoveryS3Bucket:   "my-bucket",
    RecoveryS3Prefix:   "es4/prod/",
    RecoveryS3Region:   "us-east-1",
    RecoveryS3Endpoint: "", // set for MinIO / RustFS / GCS-compatible endpoints
    RestoreOnStartup:   true,
})
```

**libSQL Recovery** (local file or `:memory:`; remote Turso not covered by product E2E)

```go
// File-backed Recovery DB
es4.Open(ctx, options.Options{
    RecoveryBackend:   options.RecoveryBackendLibSQL,
    RecoveryLibSQLURL: "file:/var/es4/recovery.db",
    RestoreOnStartup:  true,
})

// Process-local in-memory Recovery
es4.Open(ctx, options.Options{
    RecoveryBackend:   options.RecoveryBackendLibSQL,
    RecoveryLibSQLURL: ":memory:",
})
```

**Redis / Valkey State** (same adapter; `valkey` is an alias)

```go
es4.Open(ctx, options.Options{
    StateBackend:        options.StateBackendRedis, // or StateBackendValkey
    StateRedisURL:       "redis://127.0.0.1:6379/0",
    StateRedisKeyPrefix: "myapp/prod/", // optional; empty → HASH es4:state
    SnapshotInterval:    0,             // or leave default if also using Recovery
})
```

**Firestore State** (Emulator: set `FIRESTORE_EMULATOR_HOST`)

```go
es4.Open(ctx, options.Options{
    StateBackend:               options.StateBackendFirestore,
    StateFirestoreProjectID:    "demo-es4",
    StateFirestoreCollection:   "es4-state",
    // StateFirestoreDatabaseID: "", // Effective default: (default)
    SnapshotInterval:           0,
})
```

Optional: `recovery_ttl` (Go duration; `0` keeps only the latest generation), `recovery_libsql_auth_token` for authenticated DSN forms.

### Product E2E — run the workflow layers

E2E is **manual only** (`.github/workflows/e2e.yml`, `workflow_dispatch`). Input `layer`:

| `layer` | What runs | Extra service |
|---------|-----------|---------------|
| `all` | `go test -tags=e2e ./...` (every `//go:build e2e` package) | RustFS required; Redis/Valkey/Firestore skip unless env set |
| `object` | `TestE2E_ObjectRecovery_*` | RustFS |
| `file` | `TestE2E_FileSQLite_*` | No |
| `memory` | `TestE2E_Memory_*` | No |
| `libsql` | `TestE2E_LibSQL*` (File C1–C5 + Memory C2/C4/C5) | No |
| `redis` | `TestE2E_RedisState_*` | Compose `redis` (**not** in `all`) |
| `valkey` | `TestE2E_ValkeyState_*` | Compose `valkey` (**not** in `all`) |
| `firestore` | `TestE2E_FirestoreState_*` | Compose `firestore` (**not** in `all`) |

Local commands (from repo root / `packages/go`):

```bash
# Object needs RustFS first
docker compose -f docker/e2e/docker-compose.yml up -d --wait rustfs
export AWS_ACCESS_KEY_ID=es4e2eaccess
export AWS_SECRET_ACCESS_KEY=es4e2esecretkey
export ES4_E2E_S3_ENDPOINT=http://127.0.0.1:9000
export ES4_E2E_S3_REGION=us-east-1

# all (CI sets ES4_E2E_REQUIRE_RUSTFS=1 so missing RustFS fails)
cd packages/go && go test -tags=e2e ./... -count=1 -timeout 10m

# object only
go test -tags=e2e ./pkg/es4 -run 'TestE2E_ObjectRecovery_' -count=1 -timeout 10m

# file SQLite (no RustFS)
go test -tags=e2e ./pkg/es4 -run 'TestE2E_FileSQLite_' -count=1 -timeout 10m

# memory (no RustFS)
go test -tags=e2e ./pkg/es4 -run 'TestE2E_Memory_' -count=1 -timeout 10m

# Redis State
docker compose -f docker/e2e/docker-compose.yml up -d --wait redis
export ES4_E2E_REDIS_URL=redis://127.0.0.1:6379/0
go test -tags=e2e ./pkg/es4 -run 'TestE2E_RedisState_' -count=1 -timeout 10m

# Valkey State
docker compose -f docker/e2e/docker-compose.yml up -d --wait valkey
export ES4_E2E_VALKEY_URL=redis://127.0.0.1:6380/0
go test -tags=e2e ./pkg/es4 -run 'TestE2E_ValkeyState_' -count=1 -timeout 10m

# Firestore Emulator (not part of layer=all)
docker compose -f docker/e2e/docker-compose.yml up -d --wait firestore
export FIRESTORE_EMULATOR_HOST=127.0.0.1:8080
export ES4_E2E_FIRESTORE_PROJECT_ID=demo-es4
go test -tags=e2e ./pkg/es4 -run 'TestE2E_FirestoreState_' -count=1 -timeout 10m
```

Compose defaults and cleanup: [`docker/e2e/README.md`](./docker/e2e/README.md). Behavior catalog: [`docs/tests/e2e/e2e-spec.md`](./docs/tests/e2e/e2e-spec.md).

Harness files (all `//go:build e2e`):

| Layer | File |
|-------|------|
| Object | `packages/go/pkg/es4/object_recovery_e2e_test.go` |
| File SQLite | `packages/go/pkg/es4/file_sqlite_e2e_test.go` |
| Memory | `packages/go/pkg/es4/memory_e2e_test.go` |
| libSQL File / Memory | `libsql_file_e2e_test.go` · `libsql_memory_e2e_test.go` |
| Redis State | `redis_state_e2e_test.go` |
| Valkey State | `valkey_state_e2e_test.go` |
| Firestore State | `firestore_state_e2e_test.go` |

### Writing / running libSQL E2E

Spec: [`docs/tests/e2e/e2e-spec.md`](./docs/tests/e2e/e2e-spec.md) §L · plan: [`docs/plans/v0.7.3/`](./docs/plans/v0.7.3/).

| Sub-layer | Catalog | Options pattern |
|-----------|---------|-----------------|
| **File-backed** | C1–C5 | `recovery_backend=libsql`, `recovery_libsql_url=file:<temp>/recovery.db`, empty `state_path` (Memory State), `restore_on_startup=true`, `snapshot_interval=0` |
| **InMemory** | C2 / C4 / C5 only | same backend, `recovery_libsql_url=:memory:` (no restart/restore scenarios) |

Run:

```bash
cd packages/go && go test -tags=e2e ./pkg/es4 -run 'TestE2E_LibSQL' -count=1 -timeout 10m
```

GitHub Actions: run workflow **e2e** with `layer=libsql` (no RustFS). `layer=all` also includes these tests via `./...`.

When authoring new libSQL cases, follow the existing harness: temp-dir isolation + start/`t.Cleanup` clear, `OpenWith` + `SkipAsyncRestore: true` for deterministic restore in tests, and **test-only** `SnapshotNow` after writes when proving Save→re-Open. Do not rely on interval waits. Remote `libsql://` is explicitly out of E2E scope.

## Docs map

| Path | Role |
|------|------|
| [`docs/README.md`](./docs/README.md) | Product pillar (purpose, architecture diagrams, usage modes) |
| [`docs/index.md`](./docs/index.md) | OKF index (`okf_version`) |
| [`docs/roadmap.md`](./docs/roadmap.md) | Phase / provisional SemVer hub (may lag tags slightly) |
| [`docs/plans/`](./docs/plans/) | Upcoming / completed plan notes |
| [`docs/specs/`](./docs/specs/) | Normative specs: options · state · tx · snapshot · recovery · es4-server |
| [`docs/tests/`](./docs/tests/) | Test specs, including [`docs/tests/e2e/`](./docs/tests/e2e/) |
| [`docs/charter/`](./docs/charter/) | Engineering charter (OKF, TDD, versioning, git) |
| [`.github/CI.md`](./.github/CI.md) | CI / E2E / release policy |

Prefer **code + specs** over older roadmap wording when they disagree.

## License

[MIT](./LICENSE) © Bicycle for Mind LLC.
