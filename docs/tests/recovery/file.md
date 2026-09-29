# Recovery — テスト仕様

Recovery Storage（File / libSQL / Object）と起動時復元のテスト仕様。  
正本の振る舞い: [`docs/specs/recovery/`](../../specs/recovery/)。  
外部クラウド不要（local / fake / mock）。

### 世代共通

- Save は新世代を追加し剪定する。Load は最新。欠落は `ErrNotFound`

#### テスト：正常系
- 複数 Save 後の Load は最新バイト列
- `ttl=0` では最新のみ残る
- `ttl>0` では窓内の世代が残り、窓外は Save 後に剪定される

### File Save / Load

- `ttl=0`: 単一ファイル。成功した書き込みは temp + rename の原子置換（互換）
- `ttl>0`: 世代ディレクトリ（`gen-*.snap`）

#### テスト：正常系
- Save → Load で同じバイト列が読める
- 上書き Save 後は最新内容だけが残る（ttl=0）
- 親ディレクトリが無くても Save が作成して書き込める
- ttl>0 で複数世代 → 剪定後に最新 Load

#### テスト: 異常系
- ファイル／世代欠落の Load → `ErrNotFound`
- 読めないパス（例: ディレクトリを単一ファイルとして指定）→ エラー（NotFound 以外）

### libSQL

- Save / Load / TTL 剪定。ローカル file URL でクラウド不要
- Open 配線（`recovery_backend=libsql`）。URL 必須

#### テスト：正常系
- Save → Load
- TTL 剪定
- Open → Snapshot → 再 Open で復元

#### テスト: 異常系
- URL 欠落 → Open / Validate エラー
- Load 欠落 → `ErrNotFound`

### Object（S3 互換 mock）

- mock client で Save / Load / prefix / TTL。bucket 必須
- OpenWith 注入が Options の object 選択より優先

#### テスト：正常系
- Save → Load（最新）
- prefix 配下にキーが付く
- TTL 剪定
- OpenWith 注入で Snapshot 書き込み

#### テスト: 異常系
- bucket 欠落 → エラー
- Load 欠落 → `ErrNotFound`

### Startup restore（配線）

- `restore_on_startup`（Effective）が true のとき、起動時に Recovery を読む
- 欠落／読めない場合は空 State で続行（致命エラーにしない）
- Restore 完了前でも State API を受け付ける
- `memory_only` または `restore_on_startup` false では Recovery の読み書きをスキップ（設定エラーにしない）
- OpenWith の Recovery 注入が優先。backend 選択は Options Effective

#### テスト：正常系
- Snapshot 保存後に再 Open するとキーが復元される（file / libsql / file+ttl）
- Recovery 欠落でも Open 成功・空 State
- 不正 Recovery 内容でも Open 成功・空 State 相当で API 利用可
- Open 直後（async restore 中）に `SET` / `EXISTS` できる
- `memory_only` 時は明示 Snapshot しても Recovery を作らない／Adapter なし
- `restore_on_startup=false` なら既存 Recovery を読まない
- OpenWith Recovery 注入が優先

----

以上
