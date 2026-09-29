# Snapshot — テスト仕様

内部 Snapshot（封筒・定期／明示・タイマー）のテスト仕様。公開 API ではない。  
正本の振る舞い: [`docs/specs/snapshot/`](../../specs/snapshot/)。

### Envelope

- 版付き共通封筒（`version` + `created_at` + `payload`）。payload は Backend 固有。

#### テスト：正常系
- Take → Recovery 保存 → Decode で version / created_at / memory payload が復元できる
- Recovery が nil（書き込み無効）でも Take はエラーにしない

### Periodic

- Open/Start で間隔タイマー開始、Close/Stop で停止。間隔は Options.Effective の `snapshot_interval`。

#### テスト：正常系
- Interval > 0 なら一定時間内に Recovery へ書き込まれる
- Stop 後は追加書き込みが止まる（Close 経由で検証可）

### Explicit（内部）

- 明示 Snapshot は間隔タイマーをリセットする

#### テスト：正常系
- Take 直後にファイルを消すと、次の定期書き込みは概ね 1 間隔後になる（即時再発火しない）

### RestoreInto

#### テスト：正常系
- 封筒バイト列を State へ Restore するとエントリが戻る

----

以上
