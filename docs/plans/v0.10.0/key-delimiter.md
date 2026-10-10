# es4 キーデリミタ `/` → `:` 計画

| 項目 | 値 |
|------|-----|
| 状態 | 方針合意ドラフト（こうきさん承認済み: Redis/Valkey スタイル `:`。v0.x 破壊的変更は許容） |
| マイルストーン | [v0.10.0](https://github.com/b4moss/es4/milestone/1)（次 minor・現行リリース v0.9.0 の次） |
| Tracking Issue | [#37](https://github.com/b4moss/es4/issues/37) |
| Plans PR | [#36](https://github.com/b4moss/es4/pull/36)（base: `main`） |
| 対象リポジトリ | [b4moss/es4](https://github.com/b4moss/es4) |
| 開発方針の正本 | [b4moss/charter](https://github.com/b4moss/charter)（リポ `docs/charter/` 同梱） |
| override | `docs/override-charter.md` は **現状設定なし** → 憲章どおり |
| 関連計画 | ss-cachian: [es4 Layer Driver](https://github.com/b4moss/ss-cachian/blob/develop/docs/plans/v0.11.0/es4-layer-driver.md)（Milestone [v0.11.0](https://github.com/b4moss/ss-cachian/milestone/2)・Issue [#68](https://github.com/b4moss/ss-cachian/issues/68)・PR [#67](https://github.com/b4moss/ss-cachian/pull/67)） |
| 転記元 | Project Context 計画ドラフト（2026-10-10 転記） |

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
| HTTP Es4 Server | パス上のキー写像を新規則に合わせる（具体形は §9） |
| Go / Node パリティ | 両ポート同時に契約を揃える |

### 入れない（第一版）

- 既存永続データの自動マイグレーションツール（任意の手作業手順の文書化は可）
- キーデリミタの実行時設定（`/` と `:` の両立スイッチ）
- ss-cachian Driver 本体（[別計画](https://github.com/b4moss/ss-cachian/blob/develop/docs/plans/v0.11.0/es4-layer-driver.md)）

### Scan との関係（範囲の切り方）

- **`Scan(prefix)` 自体は本デリミタ計画の必須成果物ではない**が、ss-cachian PurgePrefix がハード依存する（別計画）。
- **同一マイルストーン（v0.10.0）に載せることを推奨**。順序は §5。Issue はデリミタと Scan を分けるのを推奨（依存は本文リンク）。

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

補足（現状の挙動）: 現行 `ValidateKey` は `:` を拒否しない。そのため `app:cache:…` 形式のキーは **今日でも Set 可能**だが、階層の正本区切りは文書・Server・Firestore 説明とも `/` になっている。

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

現状は URL パスの `/` が論理キーの `/` に直結。論理キーが `:` になると、少なくとも次のいずれかが必要（**採用案は未決・§9**）:

| 案 | 概要 | 利点 | 欠点 |
|----|------|------|------|
| A | `/v1/keys/a:b:c` のように **パス1段に `:` キー**を載せる | 論理キーと一致しやすい | 旧 `/v1/keys/a/b` クライアントは破壊 |
| B | URL では従来どおり階層 `/` を使い、サーバが **`/` → `:` に正規化** | URL 見た目を維持 | 論理と URL の二重語彙。`:` を含むセグメントの扱いが要定義 |
| C | パーセントエンコード必須（例: キー全体を1セグメントで渡す） | 予約文字を避けやすい | クライアント負担・specs 大幅更新 |

第一版の埋め込み利用（ss-cachian）は Server 非依存だが、**公開契約として Server を同時に直す**のがパリティ上望ましい。

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

第一版は **破壊的・移行なし（データ捨て／再投入）を既定**とする案を推奨。自動変換を行うなら別計画。

### 3.6 `/` 文字の扱い（未決）

Redis はキーに `/` を普通に持てる。変更後:

- **推奨案（未承認）:** `/` は **通常文字として許可**（階層意味は持たない）。拒否するのは `:` 規則のみ。
- 代替: `/` も禁止してキー文字セットを狭める。

----

## 4. charter 準拠フェーズと Issue / PR タイミング

| # | フェーズ | 誰 | 成果物 | 完了の目安 |
|---|----------|-----|--------|------------|
| **A** | 方針合意 | PO + 設計 | 本ファイル承認。§9 回答 | コード禁止解除まで |
| **B** | リポジトリ plans / roadmap | **main / PO** | 本ファイル転記（本 PR）。roadmap 行の追記はフォロー可 | 版名確定（v0.10.0） |
| **C** | テスト仕様更新 | 設計 → 実装 Agent | `docs/tests/state` 等を `:` 規則へ | FIX |
| **D** | TDD 実装（Go + Node） | 実装 Agent | ValidateKey・Firestore・Server・全テスト緑 | 受け入れ条件 |
| **E** | （推奨同一版 or 直後）Scan | 実装 Agent | 薄い `Scan(prefix)` + テスト | ss-cachian が依存可 |
| **F** | マージ後 specs 昇格 | **main / PO** | specs/README を現行に合わせる | specs 正本 |

### Issue / PR

- Issue は「キーデリミタ破壊的変更」と「Scan」を **分ける**のを推奨（依存は本文リンク）。本転記ではデリミタを Milestone `v0.10.0` の主 Issue とする。
- 実装 PR はテスト仕様 FIX 後。`plans`/`roadmap`/`specs` は実装 Agent が触らない。
- npm / Go モジュール利用者が壊れるため、Release ノートに **BREAKING** を明示（PO）。

----

## 5. Scan との順序（推奨）

ss-cachian PurgePrefix は `Scan` 必須。`Scan` のアルゴリズムは「文字列 prefix」でありデリミタ非依存だが、**テスト・文書・E2E のキー語彙**はデリミタに依存する。

| 順序 | 評価 |
|------|------|
| **1) デリミタ変更 → 2) Scan**（推奨） | フィクスチャを最終語彙で一度だけ書ける。ss-cachian は両方の後に着手 |
| 同一マイルストーンでデリミタ確定後に Scan を続ける | 推奨と実質同じ。PR を分けるかは運用好み |
| Scan 先行（`/` のまま）→ 後からデリミタ | Scan テスト・例を二度書きするコスト。非推奨 |
| ss-cachian Driver を Scan なしで暫定 | 承認済み方針（Scan + Delete）に反する。不可 |

**推奨:** es4 で **先に `/` → `:`** をマージ（または同一版の先頭コミット列）し、続けて **薄い `Scan(prefix)`**。その後 ss-cachian Driver。

`Scan` の公開形（`[]string` 一括か iterator か、Tx 面に出すか、上限・誤差）は **未決**。要件は「prefix 前方一致の論理キー列挙が PurgePrefix に足りる」ことのみ承認済み。

----

## 6. 移行 / 破壊的扱い

| 方針 | 内容 |
|------|------|
| SemVer | v0.x の破壊的変更として **v0.10.0** に載せる |
| 互換レイヤ | 設けない（`/` と `:` の両解釈は第一版に入れない） |
| データ | 既定は **再作成**。必要ならユーザーが Export（旧バイナリ）→ キー変換 → Replace を自前で行う |
| コミュニケーション | README・Release・（任意）Migration 節に「旧 `/` キーは無効／非サポート」と明記 |

----

## 7. 受け入れ条件

1. Go / Node で `ValidateKey` が `:` 規則（空・先頭・末尾・連続）を拒否し、合法な `a:b:c` を受け入れる。
2. state / firestore encode / server / e2e の specs・tests・コード例が新規則に更新され、CI（単体＋既存で回る範囲の E2E）が緑。
3. Firestore 論理キー ↔ Doc ID が Go/Node で一致し、文書が現行エンコードを正しく説明する。
4. 旧 `/` 階層を前提にした公開説明が残っていない（アーカイブ以外）。
5. （Scan を同版に含める場合）`Scan(prefix)` が論理キーを返し、Delete と組み合わせて prefix 削除がテストできる。
6. charter 順を踏んでいる。

----

## 8. リスク

| リスク | 影響 | 緩和 |
|--------|------|------|
| HTTP 写像の決め遅れ | Server とライブラリで語彙が割れる | §9 で A/B/C を先に決める |
| Firestore 旧 Doc 残留 | 運用データ消失扱い | BREAKING 明示・再作成手順 |
| Node と Go の Escape 不一致 | 稀な文字で Doc ID 不一致 | 共通テストベクトル |
| roadmap 未記載のまま実装 | charter 違反 | フェーズ B をゲートに（本転記） |
| Scan 仕様の膨張 | デリミタ PR が巨大化 | Scan は薄い列挙に限定（別 Issue） |

----

## 9. こうきさん確認が残る点（未決）

1. **HTTP キー写像** — §3.3 の案 A / B / C（または別案）。
2. **`/` を通常文字として許すか** — §3.6。
3. **Scan を同マイルストーンに含めるか** — 含める場合の API 形（戻り値・Tx・ページング有無）。推奨は含める。
4. **移行文書の厚さ** — 「捨ててよい」一行か、Backend 別手順まで書くか。
5. **Recovery スナップショット内の旧キー** — Restore 後に無効キーをどう扱うか（拒否して Open 失敗 / 読み込み時スキップ / 対象外）。

----

## 10. 関連リンク

- State キー規則（現行 `/`）: `docs/specs/state/state.md`
- Es4 Server キー写像: `docs/specs/es4-server/es4-server.md`
- ss-cachian アダプター計画: [ss-cachian `docs/plans/v0.11.0/es4-layer-driver.md`](https://github.com/b4moss/ss-cachian/blob/develop/docs/plans/v0.11.0/es4-layer-driver.md)

----

以上
