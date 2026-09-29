---
状態: 実装完了（libSQL Recovery E2E・docs・workflow_dispatch layer）
マイルストーン: v0.7.3（暫定 / Phase 6 追補 — libSQL E2E）
---

# Phase 6 追補 — E2E libSQL Recovery（File / InMemory）

## 目的

共通カタログ C1–C6 に沿い、**libSQL Recovery** の E2E を追加する。

- **File-backed**（ローカル `file:` DSN）: C1–C5（再起動 Restore）
- **InMemory**（`:memory:`）: C2・C4・C5（再起動復元なし）

S3 互換 §5.1–§5.5 の手順・判定・既存 `TestE2E_ObjectRecovery_*` は変更しない。リモート Turso は非対象。

## 範囲

- File: `TestE2E_LibSQLFile_*`（空 `state_path` + `recovery_backend=libsql` + `file:<temp>/recovery.db`、明示 `SnapshotNow`、temp 隔離）
- InMemory: `TestE2E_LibSQLMemory_*`（`:memory:`、プロセス内 SnapshotNow、再起動なし）
- docs: [`e2e-spec.md`](../../tests/e2e/e2e-spec.md) §L · [`scenarios.md`](../../tests/e2e/scenarios.md)
- CI: [`.github/workflows/e2e.yml`](../../../.github/workflows/e2e.yml) に `layer=libsql`（RustFS 不要。`all` でも libSQL は外部サービス非依存）

## 非対象

- S3 §5.1–§5.5 の改変・既存 Object／File／Memory アサーションの破壊
- リモート `libsql://`／Turso クラウド
- push／PR 自動 E2E、SemVer タグ／Release、PR ready 化

## 決定事項

- File 層の State は空 `state_path`（Memory）。再起動証明は libSQL Recovery → Restore のみ（Object 層と同趣旨）
- InMemory は Options の `:memory:` DSNで Open（注入不要であることをエージェント VM で確認）
- `layer=libsql` は `TestE2E_LibSQL` プレフィックス。レガシー `e2e-object-recovery.yml` は object 専用のまま

## 完了注記

- 検証: `go test -tags=e2e ./pkg/es4 -run 'TestE2E_LibSQL' -count=1`（RustFS 不要）
- SemVer タグ `v0.7.3` は本 PR では打たない
- 関連: [roadmap](../../roadmap.md) · [plans 索引](../README.md) · [v0.7.1](../v0.7.1/e2e-three-layer.md) · [v0.7.2](../v0.7.2/github-actions.md)

----

以上
