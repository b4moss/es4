# Tx 仕様

State API とは**別面**の公開トランザクション API の正本（Phase 2 / SemVer `v0.3.0`）。

## 概要

`BeginTx` で開始し、Tx 上で `SET` / `GET` / `DELETE` / `EXISTS` / `CLEAR` / `SCAN` を行い、`Commit` / `Rollback` で終了する。  
実装: `packages/go/pkg/es4`（公開）と `packages/go/internal/state`（Memory / SQLite / Redis・Valkey / Firestore）。

## 操作

| 操作 | 備考 |
|------|------|
| `BeginTx(ctx)` | ネスト不可（既に Tx 中ならエラー） |
| Tx `SET` / `GET` / `DELETE` / `EXISTS` / `CLEAR` / `SCAN` | キー／値規則は State API と同じ（v0.10.0: `:` 区切り・`Scan`。**v0.11.0:** Set は `WithExpiresAt` / `{ expiresAt }` を受け付け、Commit で Backend に値＋期限を反映）。欠落 GET/DELETE → エラー、EXISTS → false。期限切れは不在 |
| `Commit` | 変更を確定。以降の Tx 再利用はエラー |
| `Rollback` | 変更を破棄。以降の Tx 再利用はエラー |

## 規則

- State API 面への追加ではない（別面）
- ネスト Tx は不可
- 未 Commit のまま `Close` → Rollback
- Memory / SQLite 双方で同じ公開契約
- 同一プロセス内は内部ロックで直列化（Phase 1 と同様）

## 関連

- State: [`docs/specs/state/`](../state/)
- テスト仕様: [`docs/tests/tx/api.md`](../../tests/tx/api.md)

----

以上
