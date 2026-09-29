# es4

[![CI](https://github.com/b4moss/es4/actions/workflows/ci.yml/badge.svg)](https://github.com/b4moss/es4/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/codecov/c/github/b4moss/es4)](https://codecov.io/gh/b4moss/es4)
[![Go Reference](https://pkg.go.dev/badge/github.com/b4moss/es4/packages/go.svg)](https://pkg.go.dev/github.com/b4moss/es4/packages/go)
[![Release](https://img.shields.io/github/v/release/b4moss/es4)](https://github.com/b4moss/es4/releases)
[![License](https://img.shields.io/github/license/b4moss/es4)](https://github.com/b4moss/es4/blob/main/LICENSE)
[![OpenSSF Scorecard](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fapi.scorecard.dev%2Fprojects%2Fgithub.com%2Fb4moss%2Fes4&query=%24.score&label=OpenSSF%20Scorecard&suffix=%2F10)](https://scorecard.dev/viewer/?uri=github.com/b4moss/es4)

差し替え可能な Recovery を備えた、サーバーサイド向け組み込み State Store。Redis／Valkey を持ち込まずにプロセス内で耐久性のある状態を持ちたいとき向け。

- [English](./README.md)

プロダクト知識は **[`docs/`](./docs/)**（OKF）にあります。このルート README はインストールと使い方の**日本語入口**です（英語版が正本）。詳細な仕様と計画は `docs/` に置きます。

現行モノレポのライン: **Git タグ `v0.8.0`**（Go ライブラリ）。Node.js／TypeScript 移植は **`packages/node`**（npm の `@b4moss/es4` **v0.9.0**）。

## Purpose（目的）

- プロセス内に JSON のキー／値 **State** を保持する（Memory またはオンディスク SQLite）
- **Recovery** アダプタ（file / libSQL / S3 互換 object）経由でリカバリポイントを永続化する
- Open 時に Snapshot → Save → Restore を駆動し、Snapshot／Recovery を公開 API に出さない
- **組み込みライブラリ**としても、単体の **HTTP Es4 Server**（`packages/go/cmd/es4-server`）としても使える

## Scope（範囲）

**対象内（Go で `v0.7.3` 時点に実装済み）:**

| Layer | Choices |
|-------|---------|
| State | Memory · on-disk SQLite (`state_path`) · Redis／Valkey (`state_backend` + `state_redis_url`) · Firestore (`state_backend=firestore`) |
| Recovery | `file` · `libsql` (local) · `object` (S3-compatible) |
| Public API | State: `Set` / `Get` / `Delete` / `Exists` / `Clear` · Tx: `BeginTx` → Commit/Rollback |
| Server | HTTP Es4 Server + Docker image workflows |
| E2E | Object (RustFS) · File SQLite · Memory · libSQL File/Memory · Redis State · Valkey State · Firestore Emulator (`workflow_dispatch`; redis／valkey／firestore は **`layer=all` 非対象**） |

**対象内（Node・`packages/node`・マイルストーン v0.9.0）:**

| Layer | Choices |
|-------|---------|
| State | Memory · SQLite · Redis／Valkey · Firestore（Go と同 Options 語彙） |
| Recovery | `file` · `libsql`（ローカル）· `object`（S3 互換）— 内部のみ |
| Public API | `open` / `close` · State · Tx · HTTP Es4 Server |
| E2E | Go と同 layer／カタログ ID（`npm run test:e2e:<layer>`） |

**対象外／未実装:**

- プロダクト E2E におけるリモート Turso（`libsql://`）
- 本番 GCP 必須の Firestore E2E（Emulator のみ）
- 公開 `SnapshotNow` API（明示フラッシュは**テスト用ヘルパ**のみ）

## Install（インストール）

**要件:** Go **1.24+**（モジュールは `go 1.24` を宣言。toolchain は `packages/go/go.mod` の `go1.26.8`）。

モジュールパス（現行。改名が承認されるまで）:

```text
github.com/b4moss/es4/packages/go
```

ネストした Go モジュールタグ（`packages/go/vX.Y.Z`）はまだ公開していません（`packages/go/VERSION` はまだ `0.7.1`）。タグ `v0.7.3` のモノレポコミットをピンします:

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

任意の HTTP サーバーバイナリ（クローンから）:

```bash
cd packages/go
go run ./cmd/es4-server
```

サーバーの Options は `ES4_*` 環境変数から読み込みます（ライブラリ Options と同じ snake_case キー）。[`docs/specs/es4-server/`](./docs/specs/es4-server/) を参照。

### Node.js / TypeScript（`packages/node`）

**要件:** Node.js **20+**。パッケージ: [`@b4moss/es4`](https://www.npmjs.com/package/@b4moss/es4)（[`packages/node/README.md`](./packages/node/README.md)）。

```bash
npm install @b4moss/es4
```

クローンから:

```bash
cd packages/node
npm ci
npm test
npm run test:e2e:memory   # file|libsql|object|redis|valkey|firestore
```

```ts
import { open, defaults } from "@b4moss/es4";

const db = await open({ ...defaults(), memory_only: true });
await db.set("hello", JSON.stringify({ n: 1 }));
await db.close();
```

## Quick start（クイックスタート）

最小の **memory-only** ストア（Recovery なし）:

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

ローカルの **SQLite State + file Recovery**（定期 Snapshot。未設定時の既定間隔は 30s）:

```go
db, err := es4.Open(ctx, options.Options{
    StatePath:        "./data/state.db",
    RecoveryBackend:  options.RecoveryBackendFile, // or omit if RecoveryPath is set
    RecoveryPath:     "./data/recovery.snap",
    RestoreOnStartup: true,
})
```

キーは `/` 区切りの階層文字列（先頭・末尾の `/` なし、`//` なし）。値は任意の JSON ドキュメント（`json.RawMessage`）。欠落キー: `Get`／`Delete` → `es4.ErrNotFound`、`Exists` → `false, nil`。

トランザクション（別の Tx 面）:

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

Options は Defaults → 任意の YAML → `ES4_*` env を `options.Load(path, nil)` で組み立てます。

## Usage — Recovery backends & product E2E

ライブラリの Open は `options.Options.Effective()` からバックエンドを選びます（[`docs/specs/options/`](./docs/specs/options/) と `packages/go/pkg/options` を参照）。Snapshot／Recovery は**内部**です。本番コードは `snapshot_interval`（既定 `30s`）と `restore_on_startup`（既定 `true`）に依存します。E2E テストは `snapshot_interval=0` を設定し、明示フラッシュのために**テスト専用**の `SnapshotNow` ヘルパを呼び出します。

### Library wiring（最小）

**Memory（永続化なし）**

```go
es4.Open(ctx, options.Options{MemoryOnly: true})
```

**File Recovery**（`recovery_backend=file` または非空の `recovery_path`）

```go
es4.Open(ctx, options.Options{
    StatePath:       "/var/es4/state.db", // empty → Memory State
    RecoveryBackend: options.RecoveryBackendFile,
    RecoveryPath:    "/var/es4/recovery.snap",
})
```

**Object Recovery**（S3 互換。プロセス環境／AWS 既定チェーンのクレデンシャルが必要）

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

**libSQL Recovery**（ローカル file または `:memory:`。リモート Turso はプロダクト E2E の対象外）

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

任意: `recovery_ttl`（Go duration。`0` は最新世代のみ保持）、認証付き DSN 用の `recovery_libsql_auth_token`。

**Redis／Valkey State**（同一アダプタ。`valkey` はエイリアス）

```go
es4.Open(ctx, options.Options{
    StateBackend:        options.StateBackendRedis, // or StateBackendValkey
    StateRedisURL:       "redis://127.0.0.1:6379/0",
    StateRedisKeyPrefix: "myapp/prod/", // 任意。空なら HASH es4:state
    SnapshotInterval:    0,
})
```

**Firestore State**（Emulator: `FIRESTORE_EMULATOR_HOST` を設定）

```go
es4.Open(ctx, options.Options{
    StateBackend:               options.StateBackendFirestore,
    StateFirestoreProjectID:    "demo-es4",
    StateFirestoreCollection:   "es4-state",
    // StateFirestoreDatabaseID: "", // Effective 既定: (default)
    SnapshotInterval:           0,
})
```

### Product E2E — workflow 層を実行する

E2E は**手動のみ**（`.github/workflows/e2e.yml`、`workflow_dispatch`）。入力 `layer`:

| `layer` | What runs | Extra service |
|---------|-----------|---------------|
| `all` | `go test -tags=e2e ./...` (every `//go:build e2e` package) | RustFS 必須。Redis／Valkey／Firestore は env 未設定なら skip |
| `object` | `TestE2E_ObjectRecovery_*` | RustFS |
| `file` | `TestE2E_FileSQLite_*` | No |
| `memory` | `TestE2E_Memory_*` | No |
| `libsql` | `TestE2E_LibSQL*` (File C1–C5 + Memory C2/C4/C5) | No |
| `redis` | `TestE2E_RedisState_*` | Compose `redis`（**`all` 非対象**） |
| `valkey` | `TestE2E_ValkeyState_*` | Compose `valkey`（**`all` 非対象**） |
| `firestore` | `TestE2E_FirestoreState_*` | Compose `firestore`（**`all` 非対象**） |

ローカルコマンド（リポジトリルート／`packages/go` から）:

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

# Firestore Emulator（layer=all 非対象）
docker compose -f docker/e2e/docker-compose.yml up -d --wait firestore
export FIRESTORE_EMULATOR_HOST=127.0.0.1:8080
export ES4_E2E_FIRESTORE_PROJECT_ID=demo-es4
go test -tags=e2e ./pkg/es4 -run 'TestE2E_FirestoreState_' -count=1 -timeout 10m
```

Compose の既定とクリーンアップ: [`docker/e2e/README.md`](./docker/e2e/README.md)。振る舞いカタログ: [`docs/tests/e2e/e2e-spec.md`](./docs/tests/e2e/e2e-spec.md)。

ハーネスファイル（すべて `//go:build e2e`）:

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

仕様: [`docs/tests/e2e/e2e-spec.md`](./docs/tests/e2e/e2e-spec.md) §L · plan: [`docs/plans/v0.7.3/`](./docs/plans/v0.7.3/)。

| Sub-layer | Catalog | Options pattern |
|-----------|---------|-----------------|
| **File-backed** | C1–C5 | `recovery_backend=libsql`, `recovery_libsql_url=file:<temp>/recovery.db`, empty `state_path` (Memory State), `restore_on_startup=true`, `snapshot_interval=0` |
| **InMemory** | C2 / C4 / C5 only | same backend, `recovery_libsql_url=:memory:` (no restart/restore scenarios) |

実行:

```bash
cd packages/go && go test -tags=e2e ./pkg/es4 -run 'TestE2E_LibSQL' -count=1 -timeout 10m
```

GitHub Actions: ワークフロー **e2e** を `layer=libsql` で実行（RustFS 不要）。`layer=all` でも `./...` 経由でこれらのテストが含まれます。

新しい libSQL ケースを書くときは既存ハーネスに従ってください: temp ディレクトリ隔離 + 開始／`t.Cleanup` での clear、テストでの決定的 Restore のための `OpenWith` + `SkipAsyncRestore: true`、Save→再 Open を証明するときは書き込み後に**テスト専用**の `SnapshotNow`。間隔待ちに依存しないこと。リモート `libsql://` は E2E の対象外と明示されています。

## Docs map（ドキュメント案内）

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

食い違うときは、古い roadmap の文言より **コード + specs** を優先してください。

## License（ライセンス）

[MIT](./LICENSE) © Bicycle for Mind LLC.
