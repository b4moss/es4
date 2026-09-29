# Options（設定）仕様

現行バージョンに存在する振る舞いの正本（Phase 1 スライス: Options のみ）。  
未実装の State API / Snapshot / Recovery はここに置かず [plans/v0.2.0/minimal-core.md](../../plans/v0.2.0/minimal-core.md) を参照。

## 概要

ライブラリへの設定の正本は **Options**。任意で設定ファイルを読め、環境変数で同じ概念を重ねられる。Server（Phase 4）の `ES4_` 環境変数写像と同じ名前・既定を共有する。

## キーと既定

| キー (`snake_case`) | 型 | 既定 | env（Phase 4 と同じ写像） |
|---------------------|----|------|---------------------------|
| `snapshot_interval` | duration | `30s` | `ES4_SNAPSHOT_INTERVAL` |
| `restore_on_startup` | bool（on/off） | on | `ES4_RESTORE_ON_STARTUP` |
| `memory_only` | bool（on/off） | off | `ES4_MEMORY_ONLY` |
| `recovery_path` | string（ファイルパス） | `""` | `ES4_RECOVERY_PATH` |

## 振る舞い

### 組み立て順

- 前提: Options が正本。設定ファイルは Options を補う入力。
- 手順: `Defaults` →（任意）設定ファイル →（任意）env overlay
- 例外: ファイル欠落（パス指定時）／不正 JSON／不正 duration・bool はエラー

### 設定ファイル

- 形式: JSON。キーは上表の `snake_case`
- `snapshot_interval` は Go duration 文字列（例: `"30s"`）
- bool は JSON bool、または `"on"` / `"off"` / `"true"` / `"false"` / `"1"` / `"0"`

### memory_only と Recovery

- 前提: `memory_only` が on
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
