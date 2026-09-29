# Recovery — テスト仕様

ファイルベース Recovery Storage と起動時復元のテスト仕様。  
正本の振る舞い: [`docs/specs/recovery/`](../../specs/recovery/)。

### File Save / Load

- 単一ファイル。成功した書き込みは temp + rename の原子置換。

#### テスト：正常系
- Save → Load で同じバイト列が読める
- 上書き Save 後は最新内容だけが残る
- 親ディレクトリが無くても Save が作成して書き込める

#### テスト: 異常系
- ファイル欠落の Load → `ErrNotFound`
- 読めないパス（例: ディレクトリを指定）→ エラー（NotFound 以外）

### Startup restore（配線）

- `restore_on_startup`（Effective）が true のとき、起動時に Recovery を読む
- 欠落／読めない場合は空 State で続行（致命エラーにしない）
- Restore 完了前でも State API を受け付ける
- `memory_only` または `restore_on_startup` false では Recovery の読み書きをスキップ（設定エラーにしない）

#### テスト：正常系
- Snapshot 保存後に再 Open するとキーが復元される
- Recovery 欠落でも Open 成功・空 State
- 不正 Recovery 内容でも Open 成功・空 State 相当で API 利用可
- Open 直後（async restore 中）に `SET` / `EXISTS` できる
- `memory_only` 時は明示 Snapshot しても Recovery ファイルを作らない
- `restore_on_startup=false` なら既存 Recovery を読まない

----

以上
