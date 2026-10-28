---
type: TechDocument
title: v0.11.0 期限付き書き込み — テスト仕様
description: 実装前の TDD 入力。正本の振る舞いは実装後に docs/tests/state へ昇格する。
tags: [tests, plan, v0.11.0, ttl, expires_at]
timestamp: 2026-10-28
status: draft
role: test-spec-for-plan
---

# v0.11.0 — 期限付き書き込み テスト仕様

計画: [plan.md](./plan.md)。実装後の昇格先: [`docs/tests/state/api.md`](../../tests/state/api.md) · [`docs/tests/tx/api.md`](../../tests/tx/api.md)（および Server を含める場合は es4-server）。

前提（決定済み）: Firestore は日時型 `expires_at`、期限なしは省略、読取時期限切れ＝不在。API 形は plan **推奨案 A**（`Set` options）で書く。PO が B を選んだ場合はメソッド名だけ読み替える。

実装: `packages/go/internal/state`（契約テーブル ＋ Firestore Emulator）と公開 `packages/go/pkg/es4`。Node 同セマンティクス。

----

## 1. Set — 期限なし（回帰）

#### 正常系

- 既存どおり `Set(key, value)` で Get が一致する（Memory / SQLite / Redis / Firestore）
- Firestore: 保存ドキュメントに **`expires_at` フィールドが存在しない**

#### 異常系

- 現行の無効キー／不正 JSON／Close 後／キャンセル context は現行 sentinel のまま

----

## 2. Set — 期限付き

#### 正常系

- `Set(key, value, WithExpiresAt(future))` 後、期限前の Get が値を返す
- Firestore Emulator: ドキュメントに `expires_at` があり、型が **Timestamp（日時）**である（文字列ではない）
- 同一キーを期限なし `Set` で上書き → Get 成功、かつ Firestore から **`expires_at` が消えている**
- 同一キーを別の未来時刻で上書き → 新しい期限が有効

#### 異常系 / 境界

- 過去の `ExpiresAt`（plan Q2 推奨 A）: Set は成功、直後の Get は `ErrNotFound`、Exists は false
- （Q2 が B の場合）過去時刻 → `ErrInvalidExpiry`（名前は実装で固定）

----

## 3. Get / Exists / Scan — 読取時フィルタ

時計はテストで注入できることが望ましい（注入不可なら短い TTL＋`time.Sleep` は最終手段）。

#### 正常系

- 期限切れキー: Get → `ErrNotFound`、Exists → `false`
- 期限切れキーは `Scan(prefix)` および `Scan("")` の結果に **含まれない**
- 期限切れと有効が混在するとき、Scan は有効キーのみ（集合一致）

#### 異常系

- 現行どおり（無効 prefix、Close 後、キャンセル）

----

## 4. Tx

#### 正常系

- BeginTx → 期限付き Set →（Commit 前）Tx 上 Get で見える
- Commit 後、外側 Get で見え、期限メタが Backend に反映（Firestore なら Timestamp フィールド）
- BeginTx → 期限付き Set → Rollback → 外側 Exists false
- Tx 上で期限切れ扱いのキーは Get 不在（overlay／読取規則の一貫）

#### 異常系

- ネスト Tx → `ErrNestedTx`（現行）

----

## 5. Store 契約テーブル（Memory / SQLite / Redis）

既存 `contract_test.go` 系に行列を追加（Redis は miniredis）。

- 期限付き Set → 期限前 Get 一致
- 期限切れ後 Get 不在
- 期限なし上書きで「期限付き→無期限」に戻せる
- Clear 後は期限付きキーも消える
- Export（Q3 推奨 A）: 値は含まれる。**期限メタの有無は現行 map 形式では表に出ない**ことを固定（または「期限は Export されない」を assert）

----

## 6. Firestore 行列（Emulator）

`FIRESTORE_EMULATOR_HOST` 必須。未設定はローカル skip／`ES4_TEST_REQUIRE_FIRESTORE=1` で fail（現行方針）。

#### フィールド

- 期限付き: `expires_at` が Timestamp
- 期限なし: フィールド欠落（null や 0001-01-01 でないこと）
- 期限付き→なし上書き: フィールド削除を Emulator 上で確認

#### 隔離

- Clear／空 Replace は設定 collection 内のみ（現行必須ケース維持）

#### 永続

- Close → 再 Open で、期限前のキーは値と期限が残る（期限メタが落ちない）

----

## 7. 公開 DB / Open 配線

- `Open` した DB で options 付き Set が各 Backend で動く
- Node: `set(key, value, { expiresAt })` が Go と同セマンティクス

----

## 8. HTTP Server（Q4 が A のとき）

- `PUT /v1/keys/{key}?expires_at=<RFC3339>` → 期限付き Set 相当
- クエリ省略 → 無期限
- 不正な日時文字列 → **400**
- 期限切れ後 `GET` → **404**

（Q4 が C なら本節は後続版へ延期）

----

## 9. パリティ

- Go / Node で同一フィクスチャ（キー・時刻・JSON）の期待を共有できること
- Firestore Doc ID 写像（`:` → `%3A`）は v0.10.0 のまま回帰しないこと

----

## 10. 意図的にテストしない（第一版）

- 本番 GCP 上での TTL ポリシーによる物理削除の完了待ち（Emulator は TTL ポリシー非再現の想定 → **読取フィルタで契約を担保**）
- ss-cachian / jparse 結合（下流リポ）
- Redis ネイティブ EXPIRE（Q5 が A の場合）

----

以上
