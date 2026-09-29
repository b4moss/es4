# Options — 読み込み・既定・memory_only

Options（設定）の単体テスト仕様。正本の振る舞い: [`docs/specs/options/`](../../specs/options/)。決定事項: [minimal-core](../../plans/v0.2.0/minimal-core.md#決定事項)。

### Defaults

- Phase 1 の既定値を返す

#### テスト：正常系
- `snapshot_interval` は 30 秒
- `restore_on_startup` は on（true）
- `memory_only` は off（false）
- `recovery_path` は空文字

### Load（defaults → 任意ファイル → env）

- 既定の上に任意の設定ファイル、さらに環境変数を重ねて Options を組み立てる
- キーは `snake_case`。環境変数は `ES4_` + SCREAMING_SNAKE（同じ概念名・同じ既定）

#### テスト：正常系
- ファイルなし・env なしなら Defaults と同じ
- JSON ファイルの値が Defaults を上書きする
- 同名の env がファイル値をさらに上書きする
- 未設定のキーは既定（または下位レイヤの値）のまま残る

#### テスト: 異常系
- 設定ファイルが存在しない（パス指定あり）→ エラー
- 設定ファイルが不正な JSON → エラー
- `snapshot_interval` が不正な duration → エラー
- bool 系キーが解釈不能な文字列 → エラー

### ApplyEnv

- `ES4_SNAPSHOT_INTERVAL` / `ES4_RESTORE_ON_STARTUP` / `ES4_MEMORY_ONLY` / `ES4_RECOVERY_PATH` を Options に写像する

#### テスト：正常系
- 設定された env のみ上書きする
- bool は `on`/`off`/`true`/`false`/`1`/`0` を受け付ける（大小無視可）

#### テスト: 異常系
- 不正な duration / bool 文字列 → エラー

### Effective（memory_only）

- `memory_only` が on のとき Recovery 関連設定は無視する（設定エラーにはしない）

#### テスト：正常系
- `memory_only` off なら Effective は入力と同じ
- `memory_only` on なら `recovery_path`・`snapshot_interval`・`restore_on_startup` は無視相当（空／ゼロ／off）になる
- 生の Options に Recovery 値が残っていてもエラーにしない

----

以上
