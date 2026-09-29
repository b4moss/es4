---
状態: 実装完了（三層 E2E・docs・workflow_dispatch）
マイルストーン: v0.7.1（暫定 / Phase 6 追補）
---

# Phase 6 追補 — E2E 三層（File SQLite / In-memory）

## 目的

共通カタログ C1–C6 に沿い、**ファイル SQLite**（ローカル永続）と **インメモリ**（永続化非期待）の E2E を追加する。S3 互換 §5.1–§5.5 の手順・判定・既存 `TestE2E_ObjectRecovery_*` は変更しない。

## 範囲

- File SQLite: C1–C5（`state_path` + File Recovery、明示 `SnapshotNow`、temp 隔離、再起動）
- In-memory: C2・C4・C5（`memory_only`、再起動復元なし、`//go:build e2e`）
- docs: [`e2e-spec.md`](../../tests/e2e/e2e-spec.md) §F／§M · [`scenarios.md`](../../tests/e2e/scenarios.md)
- CI: [`.github/workflows/e2e.yml`](../../../.github/workflows/e2e.yml)（`workflow_dispatch` + `layer` 入力。RustFS は object／all のみ）

## 非対象

- S3 §5.1–§5.5 の改変・既存 Object アサーションの破壊
- push／PR 自動 CI、SemVer タグ／Release
- libSQL／Server HTTP／本番クラウド S3 の E2E

## 決定事項

- File 層の Recovery 証明は Close 後に SQLite state を削除して再 Open（S3 層の Memory State と同趣旨）
- Object の `TestMain` は RustFS 未起動時に Object を skip（`ES4_E2E_REQUIRE_RUSTFS=1` で厳格 fail）
- レガシー `e2e-object-recovery.yml` は object 専用 alias として残す

## 完了注記

- 検証: File／Memory は RustFS 不要で `go test -tags=e2e -run 'TestE2E_FileSQLite_|TestE2E_Memory_'`。全層は RustFS 起動下で `go test -tags=e2e ./...`
- SemVer タグ `v0.7.1` は本 PR では打たない
- 関連: [roadmap](../../roadmap.md) · [plans 索引](../README.md) · [v0.7.0](../v0.7.0/e2e-object-recovery.md)

----

以上
