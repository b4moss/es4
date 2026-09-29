# State 仕様

現行バージョンに存在する公開 State API の正本（Phase 1 / SemVer `v0.2.0`、Phase 2 / SemVer `v0.3.0` で SQLite Backend 追加）。

## 概要

JSON ドキュメントストア。公開面は State API と **別面の Tx API**（Snapshot / Recovery は内部）。  
実装: `packages/go/pkg/es4`（入口）と `packages/go/internal/state`（Memory / SQLite アダプタ）。

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

- 任意の JSON ドキュメント（object / array / スカラー含む）
- 不正 JSON は拒否

## 並行

- 同一プロセス内の呼び出しは内部ロックで直列化する

## Adapter

- State Adapter は差し替え可能
- **Phase 1:** インメモリ（`Memory`）
- **Phase 2:** オンディスク SQLite ファイル（`SQLite`、ドライバ `modernc.org/sqlite`・CGO なし）。スキーマは単純 KV（`key TEXT PRIMARY KEY`、value に JSON）
- Snapshot / Restore 用に Export / Replace を内部で持つ（論理 `{ "entries": { ... } }`。DB バイナリコピーはしない）

## Backend 選択（`Open`）

`Options.Effective()` を消費する。

| Effective 条件 | Backend |
|----------------|---------|
| `memory_only=true` | Memory（`state_path` 無視） |
| `state_path` 非空 | SQLite（欠落ファイルは新規空で成功。開けない path は Open エラー） |
| `state_path` 空 | Memory（互換） |
| `OpenWith` で State 注入 | 注入優先 |

## ライフサイクル入口

- `es4.Open` / `Close` が State と内部 Snapshot／Recovery を配線する
- `memory_only` 時は Recovery 関連を無視（設定エラーにしない）

## 関連

- Tx API: [`docs/specs/tx/`](../tx/)
- テスト仕様: [`docs/tests/state/api.md`](../../tests/state/api.md)
- Snapshot: [`docs/specs/snapshot/`](../snapshot/)
- Recovery: [`docs/specs/recovery/`](../recovery/)
- Options: [`docs/specs/options/`](../options/)

----

以上
