---
状態: 意図スタブ
マイルストーン: v0.4.0（暫定 / Phase 3）
---

# Phase 3 — 外部 Recovery Storage

## 目的

Recovery Storage をローカルファイルシステムから切り離し、外部ストレージへ拡張する。

## ざっくり範囲

- libSQL Adapter（**先に**「使える」にする）
- オブジェクトストレージ Adapter（S3 互換）— libSQL の直後に ASAP、**サポート対象**
- スナップショットの世代管理
- スナップショットの保持ポリシー

## やらぬこと（本マイルストーン）

- Litestream 連携 / Adapter … **Unscheduled**（Phase 3 の範囲外・時期未定）

## 決定事項

PO Q&A batch-3 で確定した Phase 3 Adapter 順。世代管理・保持ポリシーのノブなどは未決（[open-questions](../open-questions.md)）。

### Recovery Adapter の実装順

1. **libSQL** … 最初に実装し「使える」状態にする
2. **S3 互換オブジェクトストレージ** … libSQL の直後に ASAP。Phase 3 のサポート対象 Adapter とする
3. **Litestream** … Unscheduled（本 Phase では計画しない）

## メモ

- SemVer `v0.4.0` は暫定割当。詳細仕様は後日詰める。
- Phase 1 のファイル Recovery は単一ファイル上書き・パスは Options（[決定事項](../v0.2.0/minimal-core.md#決定事項)）。本 Phase で世代管理・外部 Adapter へ広げる。
- 実装完了後は本ファイルを `docs/specs/` へ**移動**する。
- 関連: [roadmap](../../roadmap.md) · [Phase 1 決定事項](../v0.2.0/minimal-core.md#決定事項) · [open-questions](../open-questions.md)

----

以上
