# Snapshot 仕様

内部 Snapshot の正本（公開 API ではない）。Phase 1–3 / SemVer `v0.2.0`–`v0.4.0`。論理 entries 封筒を維持（DB／オブジェクト生コピーなし）。

## 概要

Current State から Recovery Point へ渡す中間表現。実装は `packages/go/internal/snapshot`。

## 封筒

```text
{ "version": <int>, "created_at": <RFC3339>, "payload": <backend-specific JSON> }
```

- **version:** Phase 1 では `1`
- **後方互換:** `v0` 期間は形式の後方互換を約束しない
- **payload:** 論理 entries 形式 `{ "entries": { "<key>": <json>, ... } }`（Memory / SQLite 共通）。SQLite の DB ファイルを Recovery にコピーしない（Export で entries を載せる）

## ライフサイクル

- **定期:** `Open`（Manager.Start）で開始、`Close`（Stop）で停止
- **間隔:** `Options.Effective().SnapshotInterval`。`<= 0` または `memory_only` なら定期なし
- **明示（内部）:** 取得すると定期 Snapshot の間隔タイマーをリセットする。公開 API にはしない
- **memory_only（Effective）:** 定期 Snapshot も Recovery 書き込みもしない（エラーにしない）

## クラッシュ一貫性

- 直近の**成功した** Snapshot／Recovery Point まで復旧できる
- それより新しい Current State の損失は許容する

## 関連

- テスト仕様: [`docs/tests/snapshot/lifecycle.md`](../../tests/snapshot/lifecycle.md)
- State: [`docs/specs/state/`](../state/)
- Recovery: [`docs/specs/recovery/`](../recovery/)

----

以上
