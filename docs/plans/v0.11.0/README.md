---
type: Plan
title: v0.11.0 期限付き書き込み（expires_at）
description: Firestore TTL 対応のための State 期限付き書き込み計画の索引。
tags: [plan, v0.11.0, ttl, expires_at]
timestamp: 2026-10-28
---

# v0.11.0 — 期限付き書き込み（expires_at）

- **状態:** **実装中**（計画は main マージ済み。実装 PR でコード／specs 反映）
- **マイルストーン:** **`v0.11.0`**（PO 決定 Q6-A・現行線 `v0.10.0` の次 MINOR）
- **詳細計画:** [plan.md](./plan.md)
- **テスト仕様（実装前）:** [test-spec.md](./test-spec.md)
- **specs 更新案:** [specs-draft.md](./specs-draft.md)
- **意味論正本（現行・未反映）:** [specs/state](../../specs/state/) · [specs/tx](../../specs/tx/) · [specs/es4-server](../../specs/es4-server/)
- **下流:** ss-cachian es4 Layer Driver（`WithExpiresAt` / `{ expiresAt }` に合わせる）

## 目的（要約）

1. State 書き込みに **絶対時刻の期限**を載せられるようにする（Q1-A）
2. Firestore ではドキュメントに日時型 `expires_at` を書き、**期限なしはフィールド省略**（TTL ポリシーで自動削除）
3. TTL 削除遅延（最大約 24h）に備え、読み取り時も期限切れを不在扱いする（過去時刻 Set も同様・Q2-A）
4. Go / Node パリティ。Memory / SQLite / Redis・Valkey でも契約を揃える（Redis は読取フィルタ・Q5-A）

## PO 決定（2026-10-28 15:14〜15:18 JST）

| # | 決定 |
|---|------|
| Q1 | **A** — Set ＋ options（`WithExpiresAt` / `{ expiresAt }`）。ss-cachian も同形 |
| Q2 | **A** — 過去時刻 Set は保存成功、読取は不在 |
| Q3 | **C** — Export／Recovery は値のみ。期限非対応を文書明記 |
| Q4 | **A** — HTTP クエリ `expires_at` |
| Q5 | **A** — Redis／Valkey は読取フィルタのみ |
| Q6 | **A** — `v0.11.0` |

詳細は [plan.md §PO 決定](./plan.md#po-決定要決定クローズ2026-10-28-15141518-jst)。

----

以上
