# es4 キーデリミタ `/` → `:` 計画

| 項目 | 値 |
|------|-----|
| 状態 | **決定事項反映済み**（一問一答 2026-10-11）。実装未着手 |
| マイルストーン | [v0.10.0](https://github.com/b4moss/es4/milestone/1)（次 minor・現行リリース v0.9.0 の次） |
| Tracking Issue | [#37](https://github.com/b4moss/es4/issues/37) |
| Plans PR | [#36](https://github.com/b4moss/es4/pull/36)（base: `main`・merged） |
| 対象リポジトリ | [b4moss/es4](https://github.com/b4moss/es4) |
| 開発方針の正本 | [b4moss/charter](https://github.com/b4moss/charter)（リポ `docs/charter/` 同梱） |
| override | `docs/override-charter.md` は **現状設定なし** → 憲章どおり |
| 関連計画 | ss-cachian: [es4 Layer Driver](https://github.com/b4moss/ss-cachian/blob/develop/docs/plans/v0.11.0/es4-layer-driver.md)（Milestone [v0.11.0](https://github.com/b4moss/ss-cachian/milestone/2)・Issue [#68](https://github.com/b4moss/ss-cachian/issues/68)・PR [#67](https://github.com/b4moss/ss-cachian/pull/67)） |
| 転記元 | Project Context 計画ドラフト（2026-10-10 転記・2026-10-11 決定追記） |

## 決定事項（2026-10-11）

こうきさん一問一答の確定。Issue [#37](https://github.com/b4moss/es4/issues/37)・Milestone **v0.10.0**。

| # | 項目 | 決定 |
|---|------|------|
| 1 | HTTP キー写像 | **A** — `/v1/keys/a:b:c`（パス1段に `:` キー） |
| 2 | `/` 文字 | **通常文字として許可**（階層意味なし。拒否は `:` 規則のみ） |
| 3 | Scan | **v0.10.0 に含める**（デリミタの後・同 Milestone） |
| 4 | Scan API | **B** — 一括 `[]string`、ページングなし、Tx 内可 |
| 5 | 移行文書 | **A** — 短い「捨てて再投入」 |
| 6 | Recovery 旧キー | **D** — 該当レコード削除＋ユーザー向けに強く注意喚起 |

**実装順:** デリミタ変更 → Scan →（その後）ss-cachian Driver。

**本ファイルの位置づけ:** es4 リポジトリ内 `docs/plans/` の正本（v0.10.0）。実装コードは別途。charter どおり実装 Agent は本ファイルを編集しない。

**方針更新（確定）:** 以前の「ss-cachian とキー規則を揃えない／opaque」は **撤回**。es4 側を Redis/Valkey スタイル（階層区切り **`:`**）に合わせる。

----

## 1. 目的・範囲

### 目的

論理キーの階層区切りを **`/` から `:` へ変更**し、ss-cachian 既定キー（`app:cache:tenant:query`）や Redis/Valkey 慣例と語彙を揃える。v0.x のため **破壊的変更を許容**する（自動移行は第一版の必須にしない）。

### 入れる

| 項目 | 内容 |
|------|------|
| `ValidateKey` | 区切りを `:` に変更。拒否: 空・先頭 `:`・末尾 `:`・連続 `::`（現行の `/` 規則の写し） |
| 文書・例 | README / specs / tests / E2E / Server 写像説明の `a/b` → `a:b` 系 |
| Firestore 写像 | 論理区切りが `:` になることに追随（現行は `/` → `%2F` 前提の説明・Node 分割実装） |
| HTTP Es4 Server | パス上のキー写像を新規則に合わせる（**決定 A:** `/v1/keys/a:b:c`） |
| `Scan(prefix)` | **v0.10.0 に含める**。API **B:** 一括 `[]string`、ページングなし、Tx 内可 |
| Go / Node パリティ | 両ポート同時に契約を揃える |

### 入れない（第一版）

- 既存永続データの自動マイグレーションツール（短い「捨てて再投入」文書のみ）
- キーデリミタの実行時設定（`/` と `:` の両立スイッチ）
- ss-cachian Driver 本体（[別計画](https://github.com/b4moss/ss-cachian/blob/develop/docs/plans/v0.11.0/es4-layer-driver.md)）

### Scan との関係（範囲の切り方）

- ss-cachian PurgePrefix が `Scan` にハード依存する（別計画）。
- **決定（2026-10-11）:** Scan を **v0.10.0 に含める**。順序はデリミタ → Scan（§5）。API 形は **B**（一括 `[]string`・ページングなし・Tx 内可）。Issue はデリミタと Scan を分けるのを推奨（依存は本文リンク）。

----

## 2. リポジトリ現状（根拠）

転記時点の main（タグ `v0.9.0` 付近）。

| 事実 | 根拠 |
|------|------|
| キーは `/` 階層。拒否: 空・先頭/末尾 `/`・連続 `//` | `docs/specs/state/state.md`・`ValidateKey` in `packages/go/internal/state/store.go` |
| Node も同規則（`validateKey`） | `packages/node/src/state/types.ts` 系 |
| **scan / LIST なし** | state specs・es4-server specs「LIST／scan … 範囲外」 |
| Firestore: Doc ID = path-percent-encode。コメント・specs・テストは **`/` → `%2F`** | `EncodeFirestoreDocID`・Node `firestore.ts` |
| HTTP: `/v1/keys/{key…}`。**キーはパス階層そのもの** | `docs/specs/es4-server/es4-server.md` |
| テスト仕様: 複数階層例 `a/b/c`、異常系に先頭/末尾/連続 `/` | `docs/tests/state/api.md` |
| SemVer: `v0.n.0` は破壊的変更許容 | charter `versioning-rule.md`・roadmap 注記 |

補足（現状の挙動）: 現行 `ValidateKey` は `:` を拒否しない。そのため `app:cache:…` 形式のキーは **今日でも Set 可能**だが、階層の正本区切りは文書・Server・firestore 説明とも `/` になっている。

----

## 3. 影響範囲（`/` → `:`）

### 3.1 コア検証

| 箇所 | 変更 |
|------|------|
| Go `ValidateKey` | `/` 検査 → `:` 検査。エラー文言（`leading slash` 等）も更新 |
| Node `validateKey` | 同上 |
| 契約テスト / miniredis / sqlite 等 | フィクスチャキーを `:` へ |

### 3.2 Firestore `%2F` 写像

| 現状 | 変更後の含意 |
|------|----------------|
| 論理 `/` を Doc ID 上 `%2F` と説明・テスト | 論理区切りは `:`。Go `PathEscape` は `:` を **`%3A`** にエンコードする |
| Node は `split("/")` + `join("%2F")` | **`:` 分割＋`%3A` join**、または Go と同値の PathEscape 互換実装へ置換（パリティテスト必須） |
| 既存 Emulator/本番コレクション内の Doc ID（`%2F` 系） | **旧論理キーは新クライアントから無効キー／別キー扱い**になり得る → 破壊的。移行は §6 |

「`%2F` をやめ `%3A` に切り替え」は、区切り文字変更の結果としてドキュメントとテストを更新する、という意味。エンコード関数を「区切りだけ置換」に薄くするか、引き続き汎用 PathEscape にするかは実装詳細（受け入れは **論理キー ↔ Doc ID のラウンドトリップと Go/Node 一致**）。

### 3.3 HTTP Es4 Server

現状は URL パスの `/` が論理キーの `/` に直結。論理キーが `:` になると写像が必要。

| 案 | 概要 | 利点 | 欠点 |
|----|------|------|------|
| **A（採用）** | `/v1/keys/a:b:c` のように **パス1段に `:` キー**を載せる | 論理キーと一致しやすい | 旧 `/v1/keys/a/b` クライアントは破壊 |
| B | URL では従来どおり階層 `/` を使い、サーバが **`/` → `:` に正規化** | URL 見た目を維持 | 論理と URL の二重語彙。`:` を含むセグメントの扱いが要定義 |
| C | パーセントエンコード必須（例: キー全体を1セグメントで渡す） | 予約文字を避けやすい | クライアント負担・specs 大幅更新 |

**決定（2026-10-11）: 案 A。** 第一版の埋め込み利用（ss-cachian）は Server 非依存だが、**公開契約として Server を同時に直す**。

### 3.4 specs / tests / E2E / README

更新対象（網羅チェックリスト）:

- `docs/specs/state/state.md`（キー節）
- `docs/tests/state/api.md`（正常・異常例）
- Firestore 行列（`%2F` → 新写像の例）
- `docs/specs/es4-server/`・`docs/tests/es4-server/`
- E2E シナリオ・カタログ内のサンプルキー（`docs/tests/e2e/`、Go/Node e2e）
- ルート README / README_ja / packages 配下 README の `users/1` 等
- Redis/Valkey・契約テストのリテラル（`users/1` 等）
- Node `firestore.encode` テスト

### 3.5 既存キー・永続データ

| Backend | 影響 |
|---------|------|
| Memory | プロセス寿命のみ。再起動で消える |
| SQLite / Recovery スナップショット | 旧キー文字列がディスクに残る。新 `ValidateKey` で読めない／Export に旧キーが混在し得る |
| Redis/Valkey HASH field | フィールド名が論理キーそのもの → 旧 `/` キーは残存 |
| Firestore | Doc ID が旧エンコードのまま残る |

第一版は **破壊的・移行なし（データ捨て／再投入）を既定**（決定: 移行文書 A）。自動変換は入れない。

### 3.6 `/` 文字の扱い（決定済み）

Redis はキーに `/` を普通に持てる。変更後:

- **決定（2026-10-11）:** `/` は **通常文字として許可**（階層意味は持たない）。拒否するのは `:` 規則のみ。

----

## 4. charter 準拠フェーズと Issue / PR タイミング

| # | フェーズ | 誰 | 成果物 | 完了の目安 |
|---|----------|-----|--------|------------|
| **A** | 方針合意 | PO + 設計 | 本ファイル承認。§9 決定済み（2026-10-11） | 完了 |
| **B** | リポジトリ plans / roadmap | **main / PO** | 本ファイル転記（PR #36）。決定追記フォロー | 版名確定（v0.10.0） |
| **C** | テスト仕様更新 | 設計 → 実装 Agent | `docs/tests/state` 等を `:` 規則へ | FIX |
| **D** | TDD 実装（Go + Node） | 実装 Agent | ValidateKey・firestore・Server・全テスト緑 | 受け入れ条件 |
| **E** | （同版・必須）Scan | 実装 Agent | `Scan(prefix) []string`（ページングなし・Tx 内可）+ テスト | ss-cachian が依存可 |
| **F** | マージ後 specs 昇格 | **main / PO** | specs/README を現行に合わせる | specs 正本 |

### Issue / PR

- Issue は「キーデリミタ破壊的変更」と「Scan」を **分ける**のを推奨（依存は本文リンク）。本転記ではデリミタを Milestone `v0.10.0` の主 Issue とする。
- 実装 PR はテスト仕様 FIX 後。`plans`/`roadmap`/`specs` は実装 Agent が触らない。
- npm / Go モジュール利用者が壊れるため、Release ノートに **BREAKING** を明示（PO）。

----

## 5. Scan との順序（決定済み）

ss-cachian PurgePrefix は `Scan` 必須。`Scan` のアルゴリズムは「文字列 prefix」でありデリミタ非依存だが、**テスト・文書・E2E のキー語彙**はデリミタに依存する。

| 順序 | 評価 |
|------|------|
| **1) デリミタ変更 → 2) Scan**（採用） | フィクスチャを最終語彙で一度だけ書ける。ss-cachian は両方の後に着手 |
| 同一マイルストーンでデリミタ確定後に Scan を続ける | 採用と実質同じ。PR を分けるかは運用好み |
| Scan 先行（`/` のまま）→ 後からデリミタ | Scan テスト・例を二度書きするコスト。非採用 |
| ss-cachian Driver を Scan なしで暫定 | 承認済み方針（Scan + Delete）に反する。不可 |

**決定:** es4 で **先に `/` → `:`** をマージ（または同一版の先頭コミット列）し、続けて **薄い `Scan(prefix)`**。その後 ss-cachian Driver。

**Scan API（決定 B・2026-10-11）:** 一括列挙 `[]string`、**ページングなし**、**Tx 内可**。要件は「prefix 前方一致の論理キー列挙が PurgePrefix に足りる」こと。

----

## 6. 移行 / 破壊的扱い

| 方針 | 内容 |
|------|------|
| SemVer | v0.x の破壊的変更として **v0.10.0** に載せる |
| 互換レイヤ | 設けない（`/` と `:` の両解釈は第一版に入れない） |
| データ | 既定は **再作成**（移行文書 **A:** 短い「捨てて再投入」）。自動マイグレーションは入れない |
| Recovery 旧キー | **D:** 該当レコードは **削除**。ユーザー向けドキュメントで破壊的変更と同時に **強く注意喚起** |
| コミュニケーション | README・Release・短い Migration 節に「旧 `/` キーは無効／非サポート・捨てて再投入」と明記 |

----

## 7. 受け入れ条件

1. Go / Node で `ValidateKey` が `:` 規則（空・先頭・末尾・連続）を拒否し、合法な `a:b:c` を受け入れる。
2. state / firestore encode / server / e2e の specs・tests・コード例が新規則に更新され、CI（単体＋既存で回る範囲の E2E）が緑。
3. Firestore 論理キー ↔ Doc ID が Go/Node で一致し、文書が現行エンコードを正しく説明する。
4. 旧 `/` 階層を前提にした公開説明が残っていない（アーカイブ以外）。
5. `Scan(prefix) []string`（ページングなし・Tx 内可）が論理キーを返し、Delete と組み合わせて prefix 削除がテストできる。
6. charter 順を踏んでいる。

----

## 8. リスク

| リスク | 影響 | 緩和 |
|--------|------|------|
| HTTP 写像の実装ずれ | Server とライブラリで語彙が割れる | 決定 A に固定（`/v1/keys/a:b:c`） |
| Firestore 旧 Doc 残留 | 運用データ消失扱い | BREAKING 明示・短い再投入手順 |
| Node と Go の Escape 不一致 | 稀な文字で Doc ID 不一致 | 共通テストベクトル |
| roadmap 未記載のまま実装 | charter 違反 | フェーズ B をゲートに（本転記） |
| Scan 仕様の膨張 | デリミタ PR が巨大化 | Scan は薄い一括列挙に限定（別 Issue 可） |

----

## 9. 決定済み（2026-10-11）

先頭の **決定事項（2026-10-11）** と同内容。記録用の詳細:

1. **HTTP キー写像** — **案 A**（`/v1/keys/a:b:c`）。
2. **`/` を通常文字として許すか** — **許可**（階層意味なし。拒否は `:` 規則のみ）。
3. **Scan** — **v0.10.0 に含める**。**API 形 B**（一括 `[]string`・ページングなし・Tx 内可）。
4. **移行文書の厚さ** — **A**（短い「捨てて再投入」）。
5. **Recovery スナップショット内の旧キー** — **D**（該当レコード削除＋ユーザー向けに強く注意喚起）。

----

## 10. 関連リンク

- State キー規則（現行 `/`）: `docs/specs/state/state.md`
- Es4 Server キー写像: `docs/specs/es4-server/es4-server.md`
- ss-cachian アダプター計画: [ss-cachian `docs/plans/v0.11.0/es4-layer-driver.md`](https://github.com/b4moss/ss-cachian/blob/develop/docs/plans/v0.11.0/es4-layer-driver.md)

----

以上
