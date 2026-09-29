# State 仕様

現行バージョンに存在する公開 State API の正本（Phase 1 / SemVer `v0.2.0`）。

## 概要

インメモリ JSON ドキュメントストア。公開面は本 API のみ（Snapshot / Recovery は内部）。  
実装: `packages/go/pkg/es4`（入口）と `packages/go/internal/state`（アダプタ）。

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
- Phase 1 では scan / `LIST` は持たない

## 値

- 任意の JSON ドキュメント（object / array / スカラー含む）
- 不正 JSON は拒否

## 並行

- 同一プロセス内の呼び出しは内部ロックで直列化する

## Adapter

- State Adapter は差し替え可能（Phase 1 実装: インメモリ）
- Snapshot / Restore 用に Export / Replace を内部で持つ

## ライフサイクル入口

- `es4.Open` / `Close` が State と内部 Snapshot／Recovery を配線する
- `Options.Effective()` を消費する（`memory_only` 時は Recovery 関連を無視）

## 関連

- テスト仕様: [`docs/tests/state/api.md`](../../tests/state/api.md)
- Snapshot: [`docs/specs/snapshot/`](../snapshot/)
- Recovery: [`docs/specs/recovery/`](../recovery/)
- Options: [`docs/specs/options/`](../options/)

----

以上
