# State — テスト仕様

公開 State API（`SET` / `GET` / `DELETE` / `EXISTS` / `CLEAR`）のテスト仕様。  
正本の振る舞い: [`docs/specs/state/`](../../specs/state/)。

### Set / Get / Exists / Delete / Clear

- 階層キー（`/`）と任意 JSON 値に対する CRUD。同一プロセス内は内部ロックで直列化。
- 呼び出しは `context.Context` 付きの同期 API（非同期志向）。
- Memory / SQLite で同じ公開契約。

#### テスト：正常系
- `SET` したキーを `GET` で同じ JSON として読める
- `EXISTS` は存在するキーで `true`、欠落キーで `false`（エラーなし）
- `CLEAR` は全消去する。空の State に対する `CLEAR` も成功する
- 単一セグメント（`a`）と複数階層（`a/b/c`）のキーを扱える

#### テスト: 異常系
- `GET` 欠落キー → エラー（`ErrNotFound`）
- `DELETE` 欠落キー → エラー（`ErrNotFound`）
- 空キー / 先頭 `/` / 末尾 `/` / 連続 `//` → `ErrInvalidKey`
- 不正 JSON 値 → `ErrInvalidValue`
- キャンセル済み `context` → `context` エラー

### Memory / SQLite Export / Replace

- Snapshot / Restore 用のアダプタ内部 API。論理 `{entries}` 形式。

#### テスト：正常系
- `Export` の結果を別 Memory / SQLite へ `Replace` すると同じ内容になる（相互に同じ entries）

### SQLite 永続

#### テスト：正常系
- Close → 再 Open で内容が残る
- 別 path は非干渉
- 欠落ファイルは新規空で成功

#### テスト: 異常系
- 開けない path → Open エラー

### Open 配線

#### テスト：正常系
- `memory_only` → Memory
- `state_path` 非空 → SQLite
- `state_path` 空 → Memory
- `OpenWith` 注入が優先
- SQLite + `recovery_path` + `restore_on_startup` で Restore 可（欠落は空続行）
- `memory_only` 時 Recovery R/W なし

----

以上
