# Options（設定）仕様

現行バージョンに存在する振る舞いの正本（Phase 1 スライス: Options のみ）。  
未実装の State API / Snapshot / Recovery はここに置かず [plans/v0.2.0/minimal-core.md](../../plans/v0.2.0/minimal-core.md) を参照。

## 概要

ライブラリへの設定の正本は **Options**。任意で YAML 設定ファイルを読め、環境変数で同じ概念を重ねられる。Server（Phase 4）の `ES4_` 環境変数写像と同じ名前・既定を共有する。

## キーと既定

| キー (`snake_case`) | 型 | 既定 | env（Phase 4 と同じ写像） |
|---------------------|----|------|---------------------------|
| `snapshot_interval` | Go duration 文字列のみ | `30s` | `ES4_SNAPSHOT_INTERVAL` |
| `restore_on_startup` | bool（小文字 `true`/`false` のみ） | `true` | `ES4_RESTORE_ON_STARTUP` |
| `memory_only` | bool（小文字 `true`/`false` のみ） | `false` | `ES4_MEMORY_ONLY` |
| `recovery_path` | string（ファイルパス） | `""` | `ES4_RECOVERY_PATH` |

## 振る舞い

### 組み立て順

- 前提: Options が正本。設定ファイルは Options を補う入力。
- 手順: `Defaults` →（任意）YAML ファイル →（任意）env overlay
- 空の env 値は未設定としてスキップする（下位レイヤの値をクリアしない）
- 例外: ファイル欠落（パス指定時）／不正 YAML／不正 duration・bool はエラー

### 設定ファイル

- 形式: **YAML のみ**（JSON は対象外）。キーは上表の `snake_case`
- `snapshot_interval` は **Go duration 文字列のみ**（例: `"30s"`）。数値秒（`30` / `"30"`）は拒否
- bool は **小文字の `true` / `false` のみ**。`True` / `TRUE` / `on` / `off` / `1` / `0` などは拒否

### 環境変数

- 接頭辞 `ES4_` + SCREAMING_SNAKE（例: `snapshot_interval` → `ES4_SNAPSHOT_INTERVAL`）
- duration / bool の受理規則は設定ファイルと同じ（Go duration 文字列のみ、小文字 `true`/`false` のみ）
- 空文字は未設定扱い（スキップ）。下位の値を消さない

### memory_only と Recovery

- 前提: `memory_only` が `true`
- 手順: Recovery 関連設定（`recovery_path`・`snapshot_interval`・`restore_on_startup`）は**無視して続行**する。設定エラーにはしない
- 下流（未実装の Recovery / Snapshot）は `Effective()` 相当の値を消費する想定

### recovery_path

- Recovery のファイルパスは Options の `recovery_path` で渡す（Recovery 実装自体は未着手）

## 関連

- テスト仕様: [`docs/tests/options/load.md`](../../tests/options/load.md)
- 計画（未実装の State / Snapshot / Recovery）: [`docs/plans/v0.2.0/minimal-core.md`](../../plans/v0.2.0/minimal-core.md)
- Server env: [`docs/plans/v0.5.0/es4-server.md`](../../plans/v0.5.0/es4-server.md#決定事項)

----

以上
