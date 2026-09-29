# State 仕様

現行バージョンに存在する公開 State API の正本（Phase 1 / SemVer `v0.2.0`、Phase 2 / SemVer `v0.3.0` で SQLite Backend 追加、Phase 5 / SemVer `v0.6.0` で Adapter 境界を正本化）。

## 概要

JSON ドキュメントストア。公開面は State API と **別面の Tx API**（Snapshot / Recovery は内部）。  
実装: `packages/go/pkg/es4`（入口）と `packages/go/internal/state`（Memory / SQLite / Redis・Valkey アダプタ）。

## 操作

| 操作 | 欠落キー | 備考 |
|------|----------|------|
| `SET` | （作成／上書き） | 値は任意の JSON ドキュメント |
| `GET` | **エラー** | |
| `DELETE` | **エラー** | |
| `EXISTS` | `false`（エラーなし） | |
| `CLEAR` | — | 全消去。空でも成功 |

呼び出しは `context.Context` 付きの同期 API（非同期志向。同期前提のブロッキング設計にはしない）。

## キー

- 区切り文字 `/` による階層文字列
- **拒否:** 空、先頭 `/`、末尾 `/`、連続 `//`
- Phase 2 でも scan / `LIST` は持たない

## 値

- 任意の JSON ドキュメント（object / array / スカラー含む。`null`／配列／数値／文字列も可）
- 不正 JSON（空・`nil`・構文不正）は拒否（`ErrInvalidValue`）

## 並行

- 同一プロセス内の呼び出しは内部ロックで直列化する

## Adapter（`Store`）境界

State Adapter は差し替え可能。全 Backend が共有する内部契約は `internal/state.Store`（公開 `pkg/es4` はこれを配線する。Backend 固有の公開 API は追加しない）。

| 操作 | 契約 |
|------|------|
| `Set` / `Get` / `Delete` / `Exists` / `Clear` | 公開 State と同じキー／値規則 |
| `Export` | **deep copy**。返却 map や `json.RawMessage` を呼び出し側が改変しても Store に漏れない。Store 側の後続変更も、以前返した Export を自動では変えない |
| `Replace` | **原子的**。成功後は渡したエントリ集合のみが観測される（空 map は全消去＝Clear 相当）。並行読取はロック待ちしてよいが、旧＋新の混在を公開しない |
| `BeginTx` | Tx 面を開始。ネストは `ErrNestedTx` |
| `Close` | 以降の Store 操作は `ErrClosed`（またはそれを wrap）。Memory／SQLite／Redis の二重 Close は冪等（エラーなし） |

### 実装済み Backend

- **Phase 1:** インメモリ（`Memory`）
- **Phase 2:** オンディスク SQLite ファイル（`SQLite`、ドライバ `modernc.org/sqlite`・CGO なし）。スキーマは単純 KV（`key TEXT PRIMARY KEY`、value に JSON）
- **Phase 7 / v0.8.0:** Redis プロトコル State（`Redis`、クライアント `github.com/redis/go-redis/v9`・CGO なし）。**Valkey は同一アダプタ**（`state_backend=valkey` はエイリアス）。単一 HASH（`state_redis_key_prefix` または既定 `es4:state`）の field = 論理キー、value = JSON bytes
- Snapshot / Restore 用に Export / Replace を内部で持つ（論理 `{ "entries": { ... } }`。DB バイナリコピーはしない）

### 最適化の境界（Phase 5）

- Memory／SQLite／Redis の **内部**最適化のみ許可（不要コピー削減・ロック粒度・クエリ形など）
- 共有公開 API（State／Tx／Server HTTP）の契約・シグネチャを壊さないこと
- Backend 固有の公開 API や、共通契約を破る最適化は行わない

### Unscheduled Backend

- Firestore Adapter … **Unscheduled**（v0.8.0 の次 PR 予定。本ツリーでは Options キー・依存なし）
- その他のインメモリ／組み込み DB … **Unscheduled**
- 将来追加する場合も、本節の `Store` 境界と Export／Replace 契約を満たす Adapter として載せる（公開面の分岐を増やさない）

## Backend 選択（`Open`）

`Options.Effective()` を消費する。優先順位:

| 優先 | Effective 条件 | Backend |
|------|----------------|---------|
| 1 | `OpenWith` で State 注入 | 注入優先 |
| 2 | `memory_only=true` | Memory（`state_path` / `state_backend` / redis 系無視） |
| 3 | `state_backend=redis` または `valkey` | Redis アダプタ（`state_redis_url` 必須） |
| 4 | `state_backend=memory` | Memory（`state_path` 無視可） |
| 5 | `state_path` 非空（または `state_backend=sqlite`） | SQLite（欠落ファイルは新規空で成功。開けない path は Open エラー） |
| 6 | それ以外（空 `state_path`） | Memory（互換） |

## ライフサイクル入口

- `es4.Open` / `Close` が State と内部 Snapshot／Recovery を配線する
- `memory_only` 時は Recovery 関連を無視（設定エラーにしない）

## 関連

- Tx API: [`docs/specs/tx/`](../tx/)
- テスト仕様: [`docs/tests/state/api.md`](../../tests/state/api.md)
- Snapshot: [`docs/specs/snapshot/`](../snapshot/)
- Recovery: [`docs/specs/recovery/`](../recovery/)
- Options: [`docs/specs/options/`](../options/)
- Phase 5 plan（完了注記）: [`docs/plans/v0.6.0/state-backend-extension.md`](../../plans/v0.6.0/state-backend-extension.md)

----

以上
