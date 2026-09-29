# Options — 読み込み・既定・memory_only

Options（設定）の単体テスト仕様。正本の振る舞い: [`docs/specs/options/`](../../specs/options/)。

### Defaults

- Phase 1–3 の既定値を返す

#### テスト：正常系
- `snapshot_interval` は 30 秒（`30s`）
- `restore_on_startup` は `true`
- `memory_only` は `false`
- `recovery_path` / `recovery_backend` / libsql・s3 キーは空
- `recovery_ttl` は `0`
- `state_path` は空文字

### Load（defaults → 任意 YAML → env）

- 既定の上に任意の YAML 設定ファイル、さらに環境変数を重ねて Options を組み立てる
- キーは `snake_case`。環境変数は `ES4_` + SCREAMING_SNAKE（同じ概念名・同じ既定）
- 空の env 値は未設定としてスキップする（下位レイヤの値を消さない）

#### テスト：正常系
- ファイルなし・env なしなら Defaults と同じ（Load defaults-only）
- YAML ファイルの値が Defaults を上書きする（YAML overrides、`state_path` / recovery 拡張キー含む）
- 部分 YAML では未指定キーは既定のまま残る（partial YAML）
- 同名の env が YAML 値をさらに上書きする（env overrides YAML）
- 空／未設定の env はスキップされ、下位の値を消さない（`ES4_RECOVERY_LIBSQL_AUTH_TOKEN` 含む）
- 空 `recovery_backend` + `recovery_path` → Resolved は `file`

#### テスト: 異常系
- 設定ファイルが存在しない（パス指定あり）→ エラー（missing file）
- 設定ファイルが不正な YAML → エラー（bad YAML）
- `snapshot_interval` / `recovery_ttl` が Go duration 文字列以外（数値秒を含む）→ エラー
- bool 系キーが小文字の `true` / `false` 以外 → エラー
- 不正 `recovery_backend` → エラー
- 負の `recovery_ttl` → エラー
- backend 必須キー欠落（file/path・libsql/url・object/bucket）→ Validate エラー（`memory_only` 時はスキップ）

### Effective（memory_only）

- `memory_only` が `true` のとき Recovery 関連設定と `state_path` は無視する（設定エラーにはしない）

#### テスト：正常系
- `memory_only` `false` なら Effective は入力と同じ（memory_only off）
- `memory_only` `true` なら `recovery_*`・`snapshot_interval`・`restore_on_startup`・`state_path` は無視相当（空／ゼロ／false）になる（memory_only on）
- 生の Options に Recovery / state_path 値が残っていてもエラーにしない

----

以上
