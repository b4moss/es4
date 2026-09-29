---
状態: 方針確定
マイルストーン: v0.4.0（暫定 / Phase 3）
---

# Phase 3 — 外部 Recovery Storage

## 目的

Recovery Storage をローカルファイルシステムから切り離し、外部ストレージへ拡張する。

## ざっくり範囲

- libSQL Adapter（**先に**「使える」にする）
- オブジェクトストレージ Adapter（S3 互換。GCS 含む）— libSQL の直後に ASAP、**サポート対象**
- スナップショットの世代管理（設定可能な TTL）
- スナップショットの保持ポリシー

## やらぬこと（本マイルストーン）

- Litestream 連携 / Adapter … **Unscheduled**（Phase 3 の範囲外・時期未定）

## 決定事項

PO Q&A で確定した Phase 3（`v0.4.0` 暫定）の事実。実装詳細は `仕様詳細` へ上げるときに詰める。

### Recovery Adapter の実装順

1. **libSQL** … 最初に実装し「使える」状態にする
2. **S3 互換オブジェクトストレージ** … libSQL の直後に ASAP。Phase 3 のサポート対象 Adapter とする
3. **Litestream** … Unscheduled（本 Phase では計画しない）

### オブジェクトストレージと GCS

- **GCS:** S3 互換 Object Adapter の対象に**含める**（別 Adapter にはしない）

### 世代管理・保持ポリシー

- **保持:** 設定可能な TTL（世代保持のノブは TTL）
- 設定キーは Phase 1 決定の `snake_case` に従う（具体キー名・既定は `仕様詳細` で詰める）

## メモ

- SemVer `v0.4.0` は暫定割当。詳細仕様は後日詰める。
- Phase 1 のファイル Recovery は単一ファイル上書き・パスは Options（[決定事項](../v0.2.0/minimal-core.md#決定事項)）。本 Phase で世代管理・外部 Adapter へ広げる。
- 実装完了後は本ファイルを `docs/specs/` へ**移動**する。
- 関連: [roadmap](../../roadmap.md) · [Phase 1 決定事項](../v0.2.0/minimal-core.md#決定事項) · [open-questions](../open-questions.md)（残件なし）

----

以上
