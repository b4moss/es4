---
状態: 意図スタブ
マイルストーン: v0.3.0（暫定 / Phase 3）
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

- SemVer `v0.3.0` は暫定割当。詳細仕様は後日詰める。
- 実装完了後は本ファイルを `docs/specs/` へ**移動**する。
- 関連: [roadmap](../../roadmap.md)

----

以上
