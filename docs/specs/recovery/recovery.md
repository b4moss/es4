# Recovery 仕様

Recovery Storage と起動時復元の正本。Phase 1–3 / SemVer `v0.2.0`–`v0.4.0`。

## 概要

Snapshot 封筒を永続化し、再起動時に State へ戻す。公開 API ではない。  
実装: `packages/go/internal/recovery`（File / libSQL / Object）と `packages/go/pkg/es4`（配線）。

封筒は **論理 entries**（SQLite DB / オブジェクト生コピーではない）。Litestream は対象外（Unscheduled）。

## 世代管理

- **Save:** 新しい世代を書き込み、その後に剪定する
- **Load:** 最新世代を返す。世代が無ければ `ErrNotFound`
- **`recovery_ttl`:** Go duration。既定 `0` = 最新のみ保持。正の値 = Save 後に `now - ttl` より古い世代を剪定（最新は常に残す）

## Adapter

| `recovery_backend` | 説明 |
|--------------------|------|
| `file`（または空 + `recovery_path`） | ローカルファイル |
| `libsql` | libSQL / SQLite 互換 DB（`recovery_libsql_url` 必須） |
| `object` | S3 互換オブジェクト（GCS 含む・別 Adapter にしない） |

### File

- **`recovery_ttl=0`:** `recovery_path` の単一ファイルを temp + rename で原子置換（Phase 1 互換）
- **`recovery_ttl>0`:** `recovery_path` を世代ディレクトリとし、`gen-<unixnano>.snap` を追加・剪定

### libSQL

- 表 `es4_recovery` に世代（`gen_id` / `created_at` / `payload`）を保存
- ローカル `file:`（またはパス）は pure-Go SQLite。リモート `libsql://` / `http(s)://` は libSQL クライアント
- 認証トークン: `recovery_libsql_auth_token`（空 env はスキップ）

### Object（S3 互換 / GCS）

- `recovery_s3_bucket` 必須。任意: `recovery_s3_prefix` / `recovery_s3_region` / `recovery_s3_endpoint`
- 認証は標準 AWS 系環境変数（SDK 既定チェーン）
- GCS は同一 Object Adapter + S3 互換 endpoint

## 起動時復元

| Effective 条件 | 挙動 |
|----------------|------|
| `restore_on_startup=true` かつ Recovery 利用可 | 最新世代があれば読んで State へ Restore |
| 欠落／読めない／不正封筒 | **空 State で続行**（致命エラーにしない） |
| `restore_on_startup=false` | Recovery を読まない |
| `memory_only=true` | Recovery の読み書きをしない（設定エラーにしない） |
| backend 未設定（空 backend かつ空 `recovery_path`） | Recovery Adapter を付けない |

## Open 配線

- Options の Effective 値から backend を選択する
- `OpenWith` で Recovery を注入した場合は **注入が優先**
- `memory_only` 時は Recovery 関連設定を無視（クリア相当）し Adapter を付けない

## Ready

- Restore 完了前でも State API を受け付ける（起動をブロックして Ready を待たない）

## 関連

- テスト仕様: [`docs/tests/recovery/`](../../tests/recovery/)
- Snapshot: [`docs/specs/snapshot/`](../snapshot/)
- State: [`docs/specs/state/`](../state/)
- Options: [`docs/specs/options/`](../options/)

----

以上
