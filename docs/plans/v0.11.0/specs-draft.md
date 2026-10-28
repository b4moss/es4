---
type: TechDocument
title: v0.11.0 specs 更新案
description: 実装後に docs/specs へ反映する差分案。現行 specs の正本はまだ v0.10.0 のまま。
tags: [specs, draft, v0.11.0, ttl]
timestamp: 2026-10-28
status: decided
---

# specs 更新案（v0.11.0・未反映）

実装完了後に現行ファイルへマージする。**今は案**であり `docs/specs/**` の正本は変更しない（本計画 PR ではこのファイルのみ）。

PO 決定（2026-10-28）: Q1-A / Q2-A / Q3-C / Q4-A / Q5-A / Q6-A（`v0.11.0`）。

----

## `docs/specs/state/state.md` への追記案

### 操作表

| 操作 | 欠落キー | 備考 |
|------|----------|------|
| `SET` | （作成／上書き） | 値は任意 JSON。**オプションで絶対期限 `ExpiresAt`**。省略＝無期限 |
| （他は現行） | | |

### 新節「期限（expires_at）」

- 公開 Set はオプションで期限（UTC 絶対時刻）を付けられる
- 期限なし Set は、既存の期限メタを **除去**する
- Get / Exists / Scan は、保存上残っていても **期限切れを不在扱い**する（Firestore TTL の削除遅延を吸収）
- 期限は value JSON の中身ではなく、**Backend メタデータ**として持つ

### Firestore Backend 節の更新

- フィールド: 最低限 `value`（JSON テキスト）。任意で **`expires_at`（日時型 Timestamp）**
- 期限なし: `expires_at` を **書かない／削除する**（ゼロ値禁止）
- 利用側: collection に TTL ポリシーを `expires_at` へ設定する（作成は利用側）

### Redis 節の注記

- 単一 HASH のためネイティブ per-field TTL は使わない（第一版）。読み取りフィルタで契約を満たす

### Export / Replace / Recovery（**Q3-C**）

- Export／Replace は **値のみ**（現行形式）。**期限メタは含めない**
- Recovery／Snapshot も同様。**期限付きエントリの期限は Recovery 非対応**と明記する（実装は値のみ往復。ソフトな注意喚起を specs と README に書く）
- 期限が必要なデータは Firestore 直置き＋TTL（および読取フィルタ）を前提とし、Export／Recovery に頼らない

### 過去時刻（**Q2-A**）

- 過去の `ExpiresAt` での Set は成功する
- 直後の Get／Exists／Scan は不在扱い（TTL 即時 eligible と整合）

----

## `docs/specs/tx/tx.md` への追記案

- Tx 上の `SET` も State と同じ期限オプション（`WithExpiresAt` / `{ expiresAt }`）を受け付ける
- Commit で Backend に値＋期限メタを反映

----

## `docs/specs/es4-server/es4-server.md` への追記案（**Q4-A**）

- `PUT /v1/keys/{key}` に任意クエリ `expires_at`（RFC3339、UTC）
- 省略時は無期限 Set
- 不正値は **400** `invalid_expiry`（名前は実装で固定）

----

## `docs/specs/options/options.md`

- 第一版で Options キー追加は **不要**（TTL ポリシーは利用側）
- README に運用メモへのリンクを足す程度でよい

----

## `docs/specs/recovery/recovery.md` への追記案（**Q3-C**）

- State エントリの **期限メタは Snapshot／Recovery の対象外**
- Restore 後、期限情報は復元されない（値のみ）。運用上の注意を明記

----

## README（ルート EN/JA）への運用メモ案

- Firestore 利用時: 対象 collection にフィールド `expires_at` の TTL ポリシーを設定すること
- 期限付き書き込み API の短い例（Go `WithExpiresAt` / Node `{ expiresAt }`）
- 「TTL 削除は遅延し得る。es4 は読取時に期限切れを不在扱いする」
- 「Export／Recovery は値のみ。期限メタは含まない」（Q3-C）

----

以上
