---
type: Plan
title: 期限付き書き込み（expires_at / Firestore TTL）計画
description: es4 State に期限付き書き込みを追加し、Firestore TTL ポリシーで自動削除できるようにする。docs のみ。
tags: [plan, v0.11.0, ttl, expires_at, firestore]
timestamp: 2026-10-28
status: decided
---

# es4 期限付き書き込み計画（v0.11.0）

| 項目 | 値 |
|------|-----|
| 状態 | **方針確定**（docs のみ・コード変更なし・本 PR はマージしない。要決定は 2026-10-28 クローズ） |
| マイルストーン | **`v0.11.0`**（PO 決定。現行タグ `v0.10.0` の次 MINOR） |
| 対象リポジトリ | [b4moss/es4](https://github.com/b4moss/es4) |
| 開発方針の正本 | [b4moss/charter](https://github.com/b4moss/charter) `v1` |
| override | `docs/override-charter.md` は **現状設定なし** → 憲章どおり |
| 下流 | ss-cachian es4 Driver（埋め込み `es4.Open`）。jparse → ss-cachian → es4（`state_backend=firestore`） |
| 転記元 | 共通前提 `session-ttl-common`（2026-10-28 PO 決定） |

## 決定済み（PO・2026-10-28）

### 共通前提（同日）

| # | 項目 | 決定 |
|---|------|------|
| 1 | 削除手段 | Firestore **TTL ポリシー**（日時型フィールド）。アプリ側の掃除 Job は作らない |
| 2 | フィールド | ドキュメント直下の日時型 **`expires_at`**。期限なしは **省略**（ゼロ値・`0001-01-01` は書かない） |
| 3 | 読み取り | TTL 削除は最大約 24h 遅れ得るため、**読み取り時も期限切れは不在扱い** |
| 4 | TTL ポリシー作成 | **利用側**（例: jparse の Terraform）。ライブラリ docs に必要な設定（フィールド名・collection）を書く |
| 5 | 本 PR | **docs のみ**。コード変更なし。**マージしない** |

### 要決定の回答（こうきさん・2026-10-28 15:14〜15:18 JST）

| # | 項目 | 決定 |
|---|------|------|
| Q1 | 公開 API | **A** — `Set` ＋ options。Go `WithExpiresAt`、Node `{ expiresAt }`。**ss-cachian もこれに合わせる** |
| Q2 | 過去の `ExpiresAt` | **A** — Set は保存成功、読み取りでは不在扱い |
| Q3 | Export / Recovery | **C** — Export／Recovery は **値のみ**。期限メタを含めないことを **文書に明記**（実装は値のみ往復） |
| Q4 | HTTP Server | **A** — クエリ `expires_at`（RFC3339）で受け取る |
| Q5 | Redis / Valkey | **A** — ネイティブ EXPIRE なし。読み取り時フィルタのみ |
| Q6 | 版号 | **A** — **`v0.11.0`** |

----

## 1. 目的・範囲

### 目的

セッション・キャッシュ等を Firestore に TTL 付きで置き、期限切れ文書を Firestore 側で自動削除できるようにする。es4 の State 書き込みが日時型 `expires_at` を扱えることが前提（現状は `{value}` のみで TTL が効かない）。

### 入れる（v0.11.0）

| 項目 | 内容 |
|------|------|
| 公開書き込み API | **決定 Q1-A:** `Set` ＋ options（Go `WithExpiresAt` / Node `{ expiresAt }`） |
| 既存 `Set` | **期限なし**のまま維持（後方互換。Firestore では `expires_at` を書かない／上書き時は削除） |
| Get / Exists / Scan | 期限切れキーは **不在**（Get → `ErrNotFound`、Exists → false、Scan から除外）。過去時刻 Set 後も同様（**Q2-A**） |
| Tx | Tx 上の書き込みも同じ期限契約 |
| HTTP Server | **決定 Q4-A:** `PUT ...?expires_at=<RFC3339>` |
| Firestore | ドキュメント `{ value, expires_at? }`。`expires_at` は **Timestamp（日時型）**。文字列 RFC3339 は不可 |
| Memory / SQLite / Redis・Valkey | 同一公開契約。Redis は読取フィルタのみ（**Q5-A**） |
| Go / Node パリティ | 両パッケージ同時。下流 ss-cachian も Q1-A に合わせる |
| Export / Recovery | **決定 Q3-C:** 値のみ。期限メタは含めないと文書明記 |
| 文書 | specs／tests／README にフィールド名・利用側 TTL 設定の注意を追記（実装 PR で反映。本 PR は [specs-draft](./specs-draft.md)） |
| テスト仕様 | [test-spec.md](./test-spec.md) |

### 入れない（第一版）

- TTL ポリシー自体の作成・管理 API（gcloud / Terraform は利用側）
- アプリ側の期限切れ掃除 Job / バックグラウンド GC
- ss-cachian / jparse 本体の変更（別リポの後続計画。API 形は本決定に合わせる）
- Recovery 世代 TTL（既存 `recovery_ttl`）との統合やリネーム
- 相対 TTL のみの API を第一級にする案（絶対時刻を正とする。相対は呼び出し側で換算可）
- Export／Replace への期限メタ追加（Q3-C: 値のみ・文書明記）
- Redis 個別キー化＋ネイティブ EXPIRE（Q5-A: 第一版対象外）

----

## 2. リポジトリ現状（根拠）

現行線: Git タグ **`v0.10.0`**（Go `packages/go` + npm `@b4moss/es4@0.10.0`）。

| 事実 | 根拠 |
|------|------|
| 公開 `DB.Set(ctx, key, value)` — 期限引数なし | `packages/go/pkg/es4/db.go` · Node `db.ts` |
| Firestore ドキュメントは `{ value: string }` のみ | `firestoreDoc` in `internal/state/firestore.go` · Node `firestore.ts`（`set({ value })`） |
| State / Tx / Server に expires / TTL 語彙なし | `docs/specs/state` · `tx` · `es4-server` |
| Redis State は単一 HASH（field＝論理キー） | `docs/specs/state` · `internal/state/redis.go` — **HASH field 単位の EXPIRE は不可** |
| ss-cachian es4 Driver は期限を **value JSON 内**の文字列 `expires_at`（RFC3339）に書き、Get 時判定 | ss-cachian `driver/es4/entry.go` · `specs/driver-es4` |
| ss-cachian firestore Driver は日時型 `expires_at` を書くが、期限なしでゼロ値を書く問題あり（別リポ） | 共通前提ドキュメント |

### 下流のギャップ

```text
いま:  ss-cachian → es4.Set(key, {value, expires_at:"RFC3339"}) → Firestore { value: "<json>" }
      ※ Firestore ドキュメントに日時型がなく、TTL ポリシーが効かない

あと:  ss-cachian → es4.Set(key, valueJSON, ExpiresAt=t) → Firestore { value, expires_at: Timestamp }
      ※ 利用側 TTL ポリシー（フィールド expires_at）で自動削除
```

----

## 3. API（決定 Q1-A）とセマンティクス

### 公開面（Go / Node）— **採用: 案 A**

比較に挙げた案（履歴）:

| 案 | 形 | 結果 |
|----|-----|------|
| **A** | `Set(ctx, key, value, opts ...SetOption)` ＋ `WithExpiresAt(t)`。Node `set(key, value, { expiresAt?: Date })` | **採用（Q1）** |
| B | `SetWithExpiry` 別メソッド | 不採用 |
| C | `SetTTL(duration)` のみ | 不採用 |
| D | value エンベロープ強制 | 不採用 |

採用根拠（確定）:

1. **後方互換** — 既存 `Set(ctx, key, value)` はそのまま無期限（Firestore は `expires_at` 省略）
2. **絶対時刻が第一級** — `WithExpiresAt` / `{ expiresAt }` が正。相対 TTL は呼び出し側で計算
3. **Tx / HTTP へ同じ語彙** — Set のオプションとして一本化
4. **ss-cachian** も同じ options 形に合わせる（PO 明示）

### 内部 `Store` 境界

公開 DB がオプションを解釈し、Adapter には例えば次を渡す（実装詳細。受け入れは契約テスト）:

- `Set(ctx, key, value, meta SetMeta)` — `SetMeta.ExpiresAt *time.Time`（nil＝無期限）
- または既存 `Set` を置き換え、全 Backend が meta を解釈

**Backend 固有の公開 API は追加しない**（v0.6.0 Adapter 境界を維持）。

### セマンティクス（全 Backend 共通・確定）

| 操作 | 振る舞い |
|------|----------|
| Set（期限なし） | 値を保存。既存の期限メタがあれば **除去**（Firestore: `expires_at` delete） |
| Set（`ExpiresAt=t`） | 値＋期限を保存。**`t` が過去でも保存は成功**（Q2-A）。直後の Get は不在扱い |
| Get | 期限切れ → `ErrNotFound`（物理削除はベストエフォートで可） |
| Exists | 期限切れ → `false` |
| Scan | 期限切れキーを結果から除外 |
| Delete / Clear | 現行どおり（メタ含む削除） |
| Export / Replace | **値のみ**（Q3-C）。期限メタは含めない（文書に明記） |

### HTTP Es4 Server — **採用: クエリ（Q4-A）**

| 案 | 概要 | 結果 |
|----|------|------|
| **S1** | `PUT /v1/keys/{key}?expires_at=<RFC3339>`（UTC）。省略＝無期限 | **採用** |
| S2 | カスタムヘッダ | 不採用 |
| S3 | Server 後続版 | 不採用 |

----

## 4. Backend 別の物理表現

| Backend | 保存 | 期限なし | 期限切れの扱い |
|---------|------|----------|----------------|
| **Firestore** | ドキュメントフィールド `expires_at`（`time.Time` / Timestamp） | **フィールド省略**（`Delete` transform で除去）。ゼロ値・文字列は書かない | 読み取りフィルタ ＋ 利用側 TTL ポリシーによる削除（遅延あり） |
| **Memory** | エントリ併置の `expiresAt *time.Time` | nil | 読み取りフィルタ。任意で Get 時にエントリ削除 |
| **SQLite** | 列 `expires_at`（NULL 可、UTC テキスト or integer は実装詳細） | NULL | 読み取りフィルタ。任意で DELETE |
| **Redis / Valkey** | 単一 HASH 維持。併置メタ（別 HASH `…:meta` 等・**非公開**）＋**読み取りフィルタのみ**（**Q5-A**。ネイティブ EXPIRE なし） | メタなし | 読み取りフィルタのみ |

### Firestore 利用側設定（ライブラリ docs に書く内容）

- TTL ポリシー対象フィールド名: **`expires_at`**
- 対象: `state_firestore_collection` で使う **collection group**（利用側 TF / gcloud）
- フィールド型: **日時（Timestamp）**。文字列や数値では TTL 対象外
- 期限なし文書: フィールドを置かない（または null）。**ゼロ時刻を書かない**
- 削除遅延: 期限後も最大約 24h 参照され得る → **クライアント（es4）の読み取りフィルタが必須**

ポリシー作成例（利用側・参考。本リポは実行しない）:

```bash
# 例: gcloud（プロジェクト／DB／collection は利用側の値）
gcloud firestore fields ttls update expires_at \
  --collection-group=SESSION_COLLECTION \
  --enable-ttl
```

----

## 5. Tx・Scan・Recovery との整合

| 面 | 方針 |
|----|------|
| **Tx** | overlay に値＋期限を保持。Commit で Backend に同じ規則で書く。Rollback で破棄 |
| **Scan** | Tx 視界でも期限切れは除外。PurgePrefix（ss-cachian）が期限切れゴーストを消そうとしても害は小さいが、列挙に載せない方がよい |
| **Export** | **値のみ**（現行 `map[string]json.RawMessage`）。**期限メタは含めない**（Q3-C） |
| **Replace** | 値のみ置換。期限メタは Restore では復元されない |
| **Snapshot / Recovery** | Export 契約に追随。**期限付きエントリの期限は Recovery 非対応**と specs／README に明記（Q3-C）。セッション主経路は Firestore 直置き＋TTL |

セッションを「Firestore 層のみ・メモリ L1 に置かない」用途では、Recovery を使わない構成が主経路。期限が必要なデータは Export／Recovery に頼らない。

----

## 6. ss-cachian からの呼び方（前提例）

es4 **v0.11.0** 実装後、ss-cachian es4 Driver が埋め込み Backend を呼ぶイメージ（**決定 Q1-A**。ss-cachian も同形）。コードは本リポでは変更しない。

### Go（埋め込み）

```go
// ss-cachian/driver/es4 側の想定（後続リポ）
exp := time.Now().UTC().Add(ttl)
// Entry の payload だけを es4 value に載せる（期限は es4 メタへ）
raw, err := json.Marshal(wireValueOnly{Value: entry.Value})
if err != nil {
	return err
}
if exp.IsZero() {
	return db.Set(ctx, key, raw) // 無期限 → Firestore は expires_at 省略
}
return db.Set(ctx, key, raw, es4.WithExpiresAt(exp))
```

Get 側:

```go
raw, err := db.Get(ctx, key)
if err != nil {
	if es4.IsNotFound(err) { // 欠落 or es4 が期限切れを不在扱い
		return Entry{}, false, nil
	}
	return Entry{}, false, err
}
// 必要なら Driver 内の二重チェックは縮小可（es4 が正）
```

### Node（埋め込み）

```ts
const expiresAt = ttlMs > 0 ? new Date(Date.now() + ttlMs) : undefined;
await db.set(key, rawJson, expiresAt ? { expiresAt } : undefined);
```

### 移行メモ（ss-cachian）

| いま | あと（決定） |
|------|----------------|
| value JSON に文字列 `expires_at` | **es4 の Set オプション**（`WithExpiresAt` / `{ expiresAt }`）に絶対時刻を渡す |
| Get 時に Driver が期限判定 | es4 Get が不在を返す。Driver 判定は防御的に残してもよい |
| firestore 直 Driver のゼロ値問題 | 別計画。es4 経由ならゼロ値を書かない契約に乗れる |

HTTP Backend（`base_url`）を使う場合は、クエリ `expires_at`（Q4-A）で同等を渡す。

----

## 7. charter 準拠フェーズ

| # | フェーズ | 誰 | 成果物 |
|---|----------|-----|--------|
| **A** | 方針合意 | PO | 本ファイル。要決定は **2026-10-28 クローズ** |
| **B** | 本計画 PR（docs） | Agent | `plans/v0.11.0/` · roadmap／plans 索引更新。**マージしない** |
| **C** | テスト仕様 FIX | 実装前 | [test-spec.md](./test-spec.md) を受け入れに固定 |
| **D** | TDD 実装（Go + Node） | 実装 Agent | Adapter・公開 API・契約テスト・Emulator・HTTP クエリ |
| **E** | specs 昇格 | 実装後 | [specs-draft](./specs-draft.md) を `docs/specs/` へ反映（Q3-C の Recovery 注記含む） |
| **F** | 下流 | ss-cachian | es4 Driver が `WithExpiresAt` / `{ expiresAt }` を利用 |

### Issue / PR

- 本 PR: **計画のみ**・base `main`・**マージしない**
- 実装 Issue は本決定を前提に切ってよい
- CI: docs のみのため `ci.yml` の paths フィルタ外（不要に回さない）

----

## 8. 受け入れ条件（実装 PR 用）

1. Go / Node で期限付き Set（`WithExpiresAt` / `{ expiresAt }`）→ Firestore Emulator 上のドキュメントに **Timestamp 型** `expires_at` がある
2. 期限なし Set では `expires_at` フィールドが **存在しない**（ゼロ値なし）。期限あり→なしの上書きでフィールドが消える
3. 期限切れ直後（TTL 未削除でも）および過去時刻 Set 直後、Get は `ErrNotFound`、Exists は false、Scan に含まれない
4. Memory / SQLite / Redis（miniredis）契約テストで同じ公開セマンティクス（Redis は読取フィルタのみ）
5. Tx 内 Set（期限付き）→ Commit 後に外側で同じ期限が観測される
6. `PUT /v1/keys/{key}?expires_at=<RFC3339>` が期限付き Set 相当。省略は無期限
7. Export／Recovery 文書に「期限メタは含まない／非対応」と明記。Export は値のみ
8. 既存の期限引数なし Set / テストが回帰しない
9. README / specs に TTL ポリシー設定（フィールド名 `expires_at`）の注意がある

----

## 9. 非対象（再掲）

- 本 PR でのコード変更・タグ打ち・npm publish
- jparse / ss-cachian / Terraform の変更
- Firestore 以外の「サーバー側 TTL Job」

----

## PO 決定（要決定クローズ・2026-10-28 15:14〜15:18 JST）

こうきさん回答。選択肢の全文は履歴として §3 比較表を参照。

| # | 決定 | 内容 |
|---|------|------|
| **Q1** | **A** | `Set` ＋ options（Go `WithExpiresAt`、Node `{ expiresAt }`）。ss-cachian もこれに合わせる |
| **Q2** | **A** | 過去時刻の Set は保存成功、読み取りでは不在扱い |
| **Q3** | **C** | Export／Recovery は値だけを対象にし、期限を含めないことを文書に明記 |
| **Q4** | **A** | HTTP サーバーはクエリ `expires_at` で受け取る |
| **Q5** | **A** | Redis／Valkey は読み取り時のフィルタのみ |
| **Q6** | **A** | 版号 **`v0.11.0`** |

残りの未決プロダクト論点はなし（本マイルストーン範囲）。

----

## 関連

- [README（本版索引）](./README.md)
- [test-spec.md](./test-spec.md)
- [specs-draft.md](./specs-draft.md)
- 現行 [specs/state](../../specs/state/) · [tests/state](../../tests/state/)
- 下流 ss-cachian [driver-es4](https://github.com/b4moss/ss-cachian/blob/main/docs/specs/driver-es4/README.md)

----

以上
