# Options（設定）仕様

現行バージョンに存在する振る舞いの正本（Phase 1–3）。  
State / Snapshot / Recovery / Tx の正本は各ドメイン specs を参照。

## 概要

ライブラリへの設定の正本は **Options**。任意で YAML 設定ファイルを読め、環境変数で同じ概念を重ねられる。Server（Phase 4）の `ES4_` 環境変数写像と同じ名前・既定を共有する。

## キーと既定

| キー (`snake_case`) | 型 | 既定 | env（Phase 4 と同じ写像） |
|---------------------|----|------|---------------------------|
| `snapshot_interval` | Go duration 文字列のみ | `30s` | `ES4_SNAPSHOT_INTERVAL` |
| `restore_on_startup` | bool（小文字 `true`/`false` のみ） | `true` | `ES4_RESTORE_ON_STARTUP` |
| `memory_only` | bool（小文字 `true`/`false` のみ） | `false` | `ES4_MEMORY_ONLY` |
| `recovery_path` | string（ファイル／世代ディレクトリ） | `""` | `ES4_RECOVERY_PATH` |
| `recovery_backend` | string（`file` \| `libsql` \| `object`） | `""` | `ES4_RECOVERY_BACKEND` |
| `recovery_ttl` | Go duration 文字列のみ | `0`（最新のみ） | `ES4_RECOVERY_TTL` |
| `recovery_libsql_url` | string | `""` | `ES4_RECOVERY_LIBSQL_URL` |
| `recovery_libsql_auth_token` | string | `""` | `ES4_RECOVERY_LIBSQL_AUTH_TOKEN` |
| `recovery_s3_bucket` | string | `""` | `ES4_RECOVERY_S3_BUCKET` |
| `recovery_s3_prefix` | string | `""` | `ES4_RECOVERY_S3_PREFIX` |
| `recovery_s3_region` | string | `""` | `ES4_RECOVERY_S3_REGION` |
| `recovery_s3_endpoint` | string | `""` | `ES4_RECOVERY_S3_ENDPOINT` |
| `state_path` | string（オンディスク SQLite パス） | `""` | `ES4_STATE_PATH` |

## 振る舞い

### 組み立て順

- 前提: Options が正本。設定ファイルは Options を補う入力。
- 手順: `Defaults` →（任意）YAML ファイル →（任意）env overlay
- 空の env 値は未設定としてスキップする（下位レイヤの値をクリアしない）
- 例外: ファイル欠落（パス指定時）／不正 YAML／不正 duration・bool／不正 `recovery_backend`／負の `recovery_ttl` はエラー
- `Load` / `FromFile` は組み立て後に `Validate()` する

### 設定ファイル

- 形式: **YAML のみ**（JSON は対象外）。キーは上表の `snake_case`
- duration は **Go duration 文字列のみ**（例: `"30s"` / `"24h"`）。数値秒（`30` / `"30"`）は拒否
- bool は **小文字の `true` / `false` のみ**。`True` / `TRUE` / `on` / `off` / `1` / `0` などは拒否

### 環境変数

- 接頭辞 `ES4_` + SCREAMING_SNAKE（例: `snapshot_interval` → `ES4_SNAPSHOT_INTERVAL`）
- duration / bool の受理規則は設定ファイルと同じ
- 空文字は未設定扱い（スキップ）。下位の値を消さない（`recovery_libsql_auth_token` も同様）

### memory_only と Recovery / state_path

- 前提: `memory_only` が `true`
- 手順: Recovery 関連設定（`recovery_*`・`snapshot_interval`・`restore_on_startup`）および `state_path` は**無視して続行**する。設定エラーにはしない（必須キー検査もスキップ）
- 下流の Snapshot / Recovery / Open は `Effective()` の値を消費する（`Effective()` は上記をクリアする）

### recovery_backend

- 値: `file` \| `libsql` \| `object`。それ以外はエラー
- 空かつ `recovery_path` 非空 → `file` とみなす
- 空かつ `recovery_path` 空 → Recovery Adapter なし
- `file` → `recovery_path` 必須
- `libsql` → `recovery_libsql_url` 必須
- `object` → `recovery_s3_bucket` 必須
- Object 認証は Options 外の標準 AWS 系 env（SDK 既定）

### recovery_ttl

- 既定 `0` = 最新世代のみ。正 = Save 後に古い世代を剪定。負はエラー

### state_path

- オンディスク SQLite State のパス。非空かつ `memory_only` false のとき Open は SQLite Backend を選ぶ
- 空なら Memory（互換）。`memory_only` true なら無視

## 関連

- テスト仕様: [`docs/tests/options/load.md`](../../tests/options/load.md)
- State: [`docs/specs/state/`](../state/)
- Snapshot: [`docs/specs/snapshot/`](../snapshot/)
- Recovery: [`docs/specs/recovery/`](../recovery/)
- Tx: [`docs/specs/tx/`](../tx/)
- Server env: [`docs/plans/v0.5.0/es4-server.md`](../../plans/v0.5.0/es4-server.md#決定事項)

----

以上
