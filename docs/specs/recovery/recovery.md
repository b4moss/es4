# Recovery 仕様

ファイルベース Recovery Storage と起動時復元の正本。Phase 1 / SemVer `v0.2.0`（Phase 2 でも単一ファイル封筒を維持）。

## 概要

Snapshot 封筒を永続化し、再起動時に State へ戻す。公開 API ではない。  
実装: `packages/go/internal/recovery`（File Adapter）と `packages/go/pkg/es4`（配線）。

## ファイル Recovery（Phase 1 / 2）

- **配置:** Options の `recovery_path`（単一ファイル）
- **書き込み:** 成功のたびに temp ファイルへ書いて rename する原子置換
- **読み込み:** そのパスの現行ファイルが Recovery Point
- **内容:** Snapshot の論理 entries 封筒（SQLite DB バイナリではない）

## 起動時復元

| Effective 条件 | 挙動 |
|----------------|------|
| `restore_on_startup=true` かつ Recovery 利用可 | ファイルがあれば読んで State へ Restore（Memory / SQLite いずれも可） |
| ファイル欠落／読めない／不正封筒 | **空 State で続行**（致命エラーにしない） |
| `restore_on_startup=false` | Recovery を読まない |
| `memory_only=true` | Recovery の読み書きをしない（設定エラーにしない） |
| `recovery_path` 空 | Recovery Adapter を付けない（読み書きスキップ） |

## Ready

- Restore 完了前でも State API を受け付ける（起動をブロックして Ready を待たない）

## Adapter

- Recovery Adapter は差し替え可能（現行実装: 単一ファイル）。外部 Recovery は Phase 3

## 関連

- テスト仕様: [`docs/tests/recovery/file.md`](../../tests/recovery/file.md)
- Snapshot: [`docs/specs/snapshot/`](../snapshot/)
- State: [`docs/specs/state/`](../state/)
- Options: [`docs/specs/options/`](../options/)

----

以上
