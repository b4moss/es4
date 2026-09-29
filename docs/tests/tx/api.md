# Tx — テスト仕様

公開 Tx API（`BeginTx` + Tx 上の CRUD + Commit/Rollback）のテスト仕様。  
正本の振る舞い: [`docs/specs/tx/`](../../specs/tx/)。

### Begin / Commit / Rollback

- Memory と SQLite の両方で同じ契約。

#### テスト：正常系
- Begin → SET → Commit 後、State GET で見える
- Begin → SET → Rollback 後、State GET は欠落
- Tx 上の GET / EXISTS / DELETE / CLEAR が Tx 視界で動く
- 未 Commit の Close → Rollback（再 Open で欠落）

#### テスト: 異常系
- Commit 後の Tx 再利用 → エラー（`ErrTxDone`）
- ネスト Begin → エラー（`ErrNestedTx`）
- Tx 上の欠落 GET/DELETE → `ErrNotFound`、EXISTS → false
- `ErrInvalidKey` / `ErrInvalidValue` / context キャンセル

### 並行スモーク

#### テスト：正常系
- 複数ゴルーチンからの Set / BeginTx でも破壊しない（ネスト競合はエラー可）

----

以上
