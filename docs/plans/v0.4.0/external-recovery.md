---
状態: 意図スタブ
マイルストーン: v0.4.0（暫定 / Phase 3）
---

# Phase 3 — 外部 Recovery Storage

## 目的

Recovery Storage をローカルファイルシステムから切り離し、外部ストレージへ拡張する。

## ざっくり範囲

- オブジェクトストレージ Adapter
  - S3互換ストレージ
- libSQL Adapter
- Litestream連携 / Adapter
- スナップショットの世代管理
- スナップショットの保持ポリシー

## メモ

- SemVer `v0.4.0` は暫定割当。詳細仕様は後日詰める。
- Phase 1 のファイル Recovery は単一ファイル上書き（[決定事項](../v0.2.0/minimal-core.md#決定事項)）。本 Phase で世代管理・外部 Adapter へ広げる。
- 実装完了後は本ファイルを `docs/specs/` へ**移動**する。
- 関連: [roadmap](../../roadmap.md) · [Phase 1 決定事項](../v0.2.0/minimal-core.md#決定事項)

----

以上
