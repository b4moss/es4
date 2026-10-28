---
type: Plan
title: 期限付き書き込み（expires_at / Firestore TTL）計画
description: es4 State に期限付き書き込みを追加し、Firestore TTL ポリシーで自動削除できるようにする。docs のみ。
tags: [plan, v0.11.0, ttl, expires_at, firestore]
timestamp: 2026-10-28
status: draft
---

# es4 期限付き書き込み計画（v0.11.0）

| 項目 | 値 |
|------|-----|
| 状態 | **計画中**（docs のみ・コード変更なし・本 PR はマージしない） |
| 提案マイルストーン | **`v0.11.0`**（暫定。現行タグ `v0.10.0` の次 MINOR。既存 `plans/` と衝突なし） |
| 対象リポジトリ | [b4moss/es4](https://github.com/b4moss/es4) |
| 開発方針の正本 | [b4moss/charter](https://github.com/b4moss/charter) `v1` |
| override | `docs/override-charter.md` は **現状設定なし** → 憲章どおり |
| 下流 | ss-cachian es4 Driver（埋め込み `es4.Open`）。jparse → ss-cachian → es4（`state_backend=firestore`） |
| 転記元 | 共通前提 `session-ttl-common`（2026-10-28 PO 決定） |

## 決定済み（PO・2026-10-28）

| # | 項目 | 決定 |
|---|------|------|
| 1 | 削除手段 | Firestore **TTL ポリシー**（日時型フィールド）。アプリ側の掃除 Job は作らない |
| 2 | フィールド | ドキュメント直下の日時型 **`expires_at`**。期限なしは **省略**（ゼロ値・`0001-01-01` は書かない） |
| 3 | 読み取り | TTL 削除は最大約 24h 遅れ得るため、**読み取り時も期限切れは不在扱い** |
| 4 | TTL ポリシー作成 | **利用側**（例: jparse の Terraform）。ライブラリ docs に必要な設定（フィールド名・collection）を書く |
| 5 | 本 PR | **docs のみ**。コード変更なし。**マージしない** |

----

## 1. 目的・範囲

### 目的

セッション・キャッシュ等を Firestore に TTL 付きで置き、期限切れ文書を Firestore 側で自動削除できるようにする。es4 の State 書き込みが日時型 `expires_at` を扱えることが前提（現状は `{value}` のみで TTL が効かない）。

### 入れる（v0.11.0）

| 項目 | 内容 |
|------|------|
| 公開書き込み API | 期限（絶対時刻）を渡せる形を追加（§3 で案比較。**推奨: 案 A**） |
| 既存 `Set` | **期限なし**のまま維持（後方互換。Firestore では `expires_at` を書かない／上書き時は削除） |
| Get / Exists / Scan | 期限切れキーは **不在**（Get → `ErrNotFound`、Exists → false、Scan から除外）。遅延削除の穴埋め |
| Tx | Tx 上の書き込みも同じ期限契約 |
| Firestore | ドキュメント `{ value, expires_at? }`。`expires_at` は **Timestamp（日時型）**。文字列 RFC3339 は不可 |
| Memory / SQLite / Redis・Valkey | 同一公開契約（メタデータとして期限を保持。Backend 固有の物理削除は §4） |
| Go / Node パリティ | 両パッケージ同時 |
| 文書 | specs／tests／README にフィールド名・利用側 TTL 設定の注意を追記（実装 PR で反映。本 PR は [specs-draft](./specs-draft.md)） |
| テスト仕様 | [test-spec.md](./test-spec.md) |

### 入れない（第一版）

- TTL ポリシー自体の作成・管理 API（gcloud / Terraform は利用側）
- アプリ側の期限切れ掃除 Job / バックグラウンド GC
- ss-cachian / jparse 本体の変更（別リポの後続計画）
- Recovery 世代 TTL（既存 `recovery_ttl`）との統合やリネーム
- 相対 TTL のみの API を第一級にする案（絶対時刻を正とする。相対は呼び出し側で換算可）
- Export／Replace スナップショット形式の大規模破壊（§要決定 Q3）

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

## 3. API 案の比較と推奨

### 公開面（Go / Node）

| 案 | 形（Go イメージ） | 利点 | 欠点 |
|----|-------------------|------|------|
| **A（推奨）** | `Set(ctx, key, value, opts ...SetOption)` ＋ `WithExpiresAt(t time.Time)`。opts なし＝現行どおり無期限 | 既存呼び出しは無変更。拡張点を 1 箇所に集約。Node は `set(key, value, { expiresAt?: Date })` で自然 | 内部 `Store.Set` シグネチャ変更が必要。オプション増えると発見性がやや弱い |
| B | 別メソッド `SetWithExpiry(ctx, key, value, expiresAt time.Time)`。`expiresAt.IsZero()`＝無期限 | 意図が名前で明確。既存 `Set` を触らない | 書き込み経路が二重。Tx / Server / テストの分岐が増える |
| C | `SetTTL(ctx, key, value, ttl time.Duration)` のみ（相対） | 呼び出しが短い | Firestore／ Downstream は絶対時刻が正。時計・丸めの責任がライブラリ側に寄る。無期限と「今すぐ」の境界が曖昧 |
| D | 値エンベロープ強制 `{ "payload":..., "expires_at":... }` | Backend 非依存で value だけ運べる | **破壊的**。任意 JSON 値という現行契約を壊す。ss-cachian が既に別エンベロープを持つため二重ラップ |

### 推奨: **案 A（functional options / options オブジェクト）**

根拠:

1. **後方互換** — 既存 `Set(ctx, key, value)` はそのまま無期限（Firestore は `expires_at` 省略）
2. **絶対時刻が第一級** — `WithExpiresAt` が正。相対 TTL は呼び出し側（ss-cachian は既に `now.Add(ttl)`）で計算
3. **Tx / HTTP へ同じ語彙を載せやすい** — 「Set のオプション」として一本化
4. **D を避ける** — es4 の value は引き続き任意 JSON。期限は **ドキュメント／行メタデータ**（Firestore フィールド、SQLite 列、Memory 併置）

補足（案 B との差）: 実装コストは近いが、案 A の方が「無期限 Set で期限フィールドを消す」上書き規則を 1 API にまとめやすい。PO が「明示メソッドの方がレビューしやすい」と判断すれば B でも目的は達する（要決定 Q1）。

### 内部 `Store` 境界

公開 DB がオプションを解釈し、Adapter には例えば次を渡す（実装詳細。受け入れは契約テスト）:

- `Set(ctx, key, value, meta SetMeta)` — `SetMeta.ExpiresAt *time.Time`（nil＝無期限）
- または既存 `Set` を置き換え、全 Backend が meta を解釈

**Backend 固有の公開 API は追加しない**（v0.6.0 Adapter 境界を維持）。

### セマンティクス（全 Backend 共通・推奨）

| 操作 | 振る舞い |
|------|----------|
| Set（期限なし） | 値を保存。既存の期限メタがあれば **除去**（Firestore: `expires_at` delete） |
| Set（`ExpiresAt=t`） | 値＋期限を保存。`t` が過去でも保存は成功し、直後の Get は不在扱いでよい（または `ErrInvalidExpiry` — 要決定 Q2） |
| Get | 期限切れ → `ErrNotFound`（物理削除はベストエフォートで可） |
| Exists | 期限切れ → `false` |
| Scan | 期限切れキーを結果から除外 |
| Delete / Clear | 現行どおり（メタ含む削除） |
| Export / Replace | 要決定 Q3 |

### HTTP Es4 Server

埋め込み利用（ss-cachian）は Server 非依存だが、公開契約のパリティのため第一版で扱うかを要決定 Q4。

| 案 | 概要 |
|----|------|
| **S1（推奨）** | `PUT /v1/keys/{key}` にオプションクエリ `expires_at=<RFC3339>`（UTC）。省略＝無期限 |
| S2 | カスタムヘッダ `X-Es4-Expires-At` |
| S3 | v0.11.0 では Server 非対応（埋め込みのみ）。後続版 |

----

## 4. Backend 別の物理表現

| Backend | 保存 | 期限なし | 期限切れの扱い |
|---------|------|----------|----------------|
| **Firestore** | ドキュメントフィールド `expires_at`（`time.Time` / Timestamp） | **フィールド省略**（`Delete` transform で除去）。ゼロ値・文字列は書かない | 読み取りフィルタ ＋ 利用側 TTL ポリシーによる削除（遅延あり） |
| **Memory** | エントリ併置の `expiresAt *time.Time` | nil | 読み取りフィルタ。任意で Get 時にエントリ削除 |
| **SQLite** | 列 `expires_at`（NULL 可、UTC テキスト or integer は実装詳細） | NULL | 読み取りフィルタ。任意で DELETE |
| **Redis / Valkey** | 現状は単一 HASH のため **field 単位 EXPIRE 不可**。推奨: 併置メタ（別 HASH `…:meta` または value 横の内部構造は **非公開**）＋読み取りフィルタ | メタなし | 読み取りフィルタのみ（ネイティブ EXPIRE は要決定 Q5） |

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
| **Export** | 現状は `map[string]json.RawMessage`（値のみ）。期限メタを落とすと Restore 後に TTL が消える → **要決定 Q3** |
| **Replace** | Export と対。期限を含むなら原子的にメタも置換 |
| **Snapshot / Recovery** | Export 契約に追随。第一版でメタ非対応なら「期限は Recovery 対象外」と明記し、セッション用途は Firestore 直置き＋TTL を前提にする |

セッションを「Firestore 層のみ・メモリ L1 に置かない」用途では、Recovery を使わない構成が主経路。それでも契約の一貫性のため Q3 は PO 判断が必要。

----

## 6. ss-cachian からの呼び方（前提例）

es4 **v0.11.0** 実装後、ss-cachian es4 Driver が埋め込み Backend を呼ぶイメージ（**推奨案 A**）。コードは本リポでは変更しない。

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

| いま | あと（推奨） |
|------|----------------|
| value JSON に文字列 `expires_at` | **es4 の Set オプション**に絶対時刻を渡す |
| Get 時に Driver が期限判定 | es4 Get が不在を返す。Driver 判定は防御的に残してもよい |
| firestore 直 Driver のゼロ値問題 | 別計画。es4 経由ならゼロ値を書かない契約に乗れる |

HTTP Backend（`base_url`）を使う場合は、Server が S1 を実装していればクエリで同等を渡す。

----

## 7. charter 準拠フェーズ

| # | フェーズ | 誰 | 成果物 |
|---|----------|-----|--------|
| **A** | 方針合意 | PO | 本ファイル＋要決定の回答 |
| **B** | 本計画 PR（docs） | Agent | `plans/v0.11.0/` · roadmap／plans 索引更新。**マージしない** |
| **C** | テスト仕様 FIX | 実装前 | [test-spec.md](./test-spec.md) を受け入れに固定 |
| **D** | TDD 実装（Go + Node） | 実装 Agent | Adapter・公開 API・契約テスト・Emulator |
| **E** | specs 昇格 | 実装後 | [specs-draft](./specs-draft.md) を `docs/specs/` へ反映 |
| **F** | 下流 | ss-cachian | es4 Driver が期限付き書き込みを利用 |

### Issue / PR

- 本 PR: **計画のみ**・base `main`・**マージしない**
- 実装 Issue は要決定クローズ後に切る（推奨）
- CI: docs のみのため `ci.yml` の paths フィルタ外（不要に回さない）

----

## 8. 受け入れ条件（実装 PR 用・ドラフト）

1. Go / Node で期限付き Set → Firestore Emulator 上のドキュメントに **Timestamp 型** `expires_at` がある
2. 期限なし Set では `expires_at` フィールドが **存在しない**（ゼロ値なし）。期限あり→なしの上書きでフィールドが消える
3. 期限切れ直後（TTL 未削除でも）Get は `ErrNotFound`、Exists は false、Scan に含まれない
4. Memory / SQLite / Redis（miniredis）契約テストで同じ公開セマンティクス
5. Tx 内 Set（期限付き）→ Commit 後に外側で同じ期限が観測される
6. 既存の期限引数なし Set / テストが回帰しない
7. README / specs に TTL ポリシー設定（フィールド名 `expires_at`）の注意がある

----

## 9. 非対象（再掲）

- 本 PR でのコード変更・タグ打ち・npm publish
- jparse / ss-cachian / Terraform の変更
- Firestore 以外の「サーバー側 TTL Job」

----

## 要決定

PO への一問ずつ。いずれも **推奨つき**。

### Q1. 公開 API の形

- **A（推奨）:** `Set` ＋ options（`WithExpiresAt` / `{ expiresAt }`）
- B: `SetWithExpiry` 別メソッド
- C: `SetTTL(duration)` のみ

### Q2. 過去の `ExpiresAt` を Set したとき

- **A（推奨）:** 保存は成功し、直後の読取は不在扱い（TTL 即時eligibile と整合）
- B: `ErrInvalidExpiry` で拒否

### Q3. Export / Replace / Recovery と期限メタ

- **A（推奨・第一版）:** Export は **値のみ**（現行形式維持）。期限メタは Export 対象外と明記。セッション主経路は Firestore＋TTL
- B: Export を拡張して期限も往復（スナップショット形式の後方互換方針を別途定義）
- C: 併せて Recovery を「期限付きエントリ非対応」と Soft-deprecate 文書化のみ（実装は A と同じ）

### Q4. HTTP Es4 Server

- **A（推奨）:** クエリ `expires_at`（RFC3339）を v0.11.0 に含める
- B: ヘッダ方式
- C: Server は後続版

### Q5. Redis / Valkey のネイティブ EXPIRE

- **A（推奨・第一版）:** HASH 維持＋メタ併置＋読み取りフィルタのみ（ネイティブ EXPIRE なし）
- B: キー設計を個別キーへ変更し EXPIRE を使う（破壊的・別 Milestone 推奨）

### Q6. 版号

- **A（推奨）:** **`v0.11.0`**（MINOR・後方互換の Set 拡張）
- B: 別号（理由をコメント）

----

## 関連

- [README（本版索引）](./README.md)
- [test-spec.md](./test-spec.md)
- [specs-draft.md](./specs-draft.md)
- 現行 [specs/state](../../specs/state/) · [tests/state](../../tests/state/)
- 下流 ss-cachian [driver-es4](https://github.com/b4moss/ss-cachian/blob/main/docs/specs/driver-es4/README.md)

----

以上
