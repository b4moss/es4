# State — テスト仕様

公開 State API（`SET` / `GET` / `DELETE` / `EXISTS` / `CLEAR`）および Adapter 内部契約（Export／Replace／Close）のテスト仕様。  
正本の振る舞い: [`docs/specs/state/`](../../specs/state/)。

実装: `packages/go/internal/state`（`store_test.go` · `contract_test.go` · `firestore_*.go`）と公開面の既存スイート。

### Set / Get / Exists / Delete / Clear

- 階層キー（`/`）と任意 JSON 値に対する CRUD。同一プロセス内は内部ロックで直列化。
- 呼び出しは `context.Context` 付きの同期 API（非同期志向）。
- Memory / SQLite / Redis / Firestore で同じ公開契約。

#### テスト：正常系
- `SET` したキーを `GET` で同じ JSON として読める
- `EXISTS` は存在するキーで `true`、欠落キーで `false`（エラーなし）
- `CLEAR` は全消去する。空の State に対する `CLEAR` も成功する
- 単一セグメント（`a`）と複数階層（`a/b/c`）のキーを扱える
- 有効 JSON の `null`／配列／オブジェクト／数値／文字列は `SET` 可能

#### テスト: 異常系
- `GET` 欠落キー → エラー（`ErrNotFound`）
- `DELETE` 欠落キー → エラー（`ErrNotFound`）
- 空キー / 先頭 `/` / 末尾 `/` / 連続 `//` → `ErrInvalidKey`（Set／Get／Delete／Exists）
- 不正 JSON 値（`nil`／空／構文不正）→ `ErrInvalidValue`
- キャンセル済み `context` → `context` エラー

### Store 契約（テーブル駆動・Memory / SQLite / Redis）

同一テストテーブルを各 Backend で実行する（`contract_test.go`。Redis は miniredis）。Firestore は Emulator 必須の別スイート（`firestore_contract_test.go`）。

#### テスト：正常系
- Set → Get → Exists true → Delete → Exists false → Get は `ErrNotFound`
- Clear 後、投入キーの Exists が false。Export は空
- Replace に複数 entries → 各キー Get 一致。旧キーは消える。空 Replace は全欠落
- BeginTx → Set → Commit 後、外側 Get で見える

#### テスト: 異常系
- 両 Backend で無効キー／不正 JSON が同じ sentinel（`ErrInvalidKey`／`ErrInvalidValue`）

### Firestore 行列（Emulator）

`FIRESTORE_EMULATOR_HOST` 必須。未設定はローカル skip／CI（`ES4_TEST_REQUIRE_FIRESTORE=1`）は fail。

#### キー写像
- Doc ID = パス・パーセントエンコード（`a/b/c` → `a%2Fb%2Fc`）。Export キーは論理キー
- エンコード後 ID が 1500 バイト超過 → `ErrFirestoreDocIDTooLong`

#### Clear 隔離（必須）
- 別 collection に直接書き込み後、対象 Store で Clear／空 Replace → 対象のみ空。**他 collection は残る**

#### 永続・隔離
- Close → 再 Open（同 project／database／collection）で内容が残る
- 別 collection は非干渉
- Export → Memory へ Replace でラウンドトリップ一致

### Export 独立性（deep copy）

#### テスト：正常系
- Export 後に返却 map のキー追加／削除や `RawMessage` 改変をしても、続けての Get／再 Export は元の Store 内容のまま
- Store 側で Set し直しても、以前受け取った entries マップは自動では変わらない

### Replace 原子性

#### テスト：正常系
- 事前キー `old` のあと `Replace({new})` 成功 → Get(new) 成功、Get(old) は `ErrNotFound`、Export は新集合のみ
- 空 map で Replace → 全欠落（Clear 相当）
- Replace 実行中の並行 Get／Export は、旧＋新の混在を公開しない（ロック待ち可）。完了後は新集合のみ

### Close 後

#### テスト: 異常系
- Close 後の Set／Get／Delete／Exists／Clear／Export／Replace／BeginTx → `ErrClosed`（または wrap）
- 二重 Close は Memory／SQLite／Redis／Firestore とも冪等（エラーなし）

### Memory / SQLite / Redis Export / Replace ラウンドトリップ

- Snapshot / Restore 用のアダプタ内部 API。論理 `{entries}` 形式。

#### テスト：正常系
- `Export` の結果を別 Memory / SQLite / Redis へ `Replace` すると同じ内容になる（相互に同じ entries）

### SQLite 永続

#### テスト：正常系
- Close → 再 Open で内容が残る
- 別 path は非干渉
- 欠落ファイルは新規空で成功

#### テスト: 異常系
- 開けない path → Open エラー

### Redis 永続・隔離

#### テスト：正常系
- Close → 再 Open（同 URL／prefix）で内容が残る（miniredis 可）
- 別 `state_redis_key_prefix` は非干渉
- Tx Rollback 後は外側に見えない

#### テスト: 異常系
- `state_backend=redis|valkey` で URL 空 → Validate エラー

### Open 配線

#### テスト：正常系
- `memory_only` → Memory（redis／firestore 設定があっても書かない）
- `state_backend=redis|valkey` + URL → Redis アダプタ
- `state_backend=firestore` + 必須キー + Emulator → Firestore
- `state_backend=memory` → Memory（`state_path` 無視可）
- `state_path` 非空 → SQLite
- `state_path` 空 → Memory
- `OpenWith` 注入が優先
- SQLite + `recovery_path` + `restore_on_startup` で Restore 可（欠落は空続行）
- `memory_only` 時 Recovery R/W なし

#### テスト: 異常系
- firestore 必須キー欠落 → Validate／Open エラー
- 到達不能 `FIRESTORE_EMULATOR_HOST` → Open または初回操作でエラー

----

以上
