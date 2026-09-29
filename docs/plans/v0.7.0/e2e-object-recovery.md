---
状態: 実装完了（E2E ハーネス／Compose／workflow_dispatch）
マイルストーン: v0.7.0（暫定 / Phase 6）
---

# Phase 6 — E2E Object Recovery × RustFS

## 目的

Docker 上の **RustFS**（S3 互換）を Object Recovery バックエンドとして使い、
Snapshot Save → プロセス再起動（Close → 再 Open）→ Restore を自動化 E2E で担保する。

## 範囲

- `//go:build e2e` テスト（§5.1–§5.5）: [`docs/tests/e2e/e2e-spec.md`](../../tests/e2e/e2e-spec.md)
- Compose: [`docker/e2e/docker-compose.yml`](../../../docker/e2e/docker-compose.yml)
- 共有ヘルパ: `packages/go/internal/e2e`（S3 クライアント注入・バケット／プレフィックス隔離）
- GitHub Actions: `.github/workflows/e2e-object-recovery.yml`（**`workflow_dispatch` のみ**）

## 非対象

- デフォルト PR/push CI への E2E 組み込み
- Redis／Valkey・libSQL／File Recovery の E2E
- Es4 Server HTTP E2E・本番クラウド S3
- SemVer タグ／GitHub Release（本マイルストーンの PR では打たない）

## 決定事項

- S3 クライアントは **方式 A**: `recovery.ObjectConfig.Client`（`ObjectAPI`）＋ `es4.OpenWith` で注入
- State は空 `state_path`（Memory）＋ Object Recovery。再起動証明は Recovery → Restore に依存
- 明示フラッシュはテスト用 `SnapshotNow`（間隔待ちに依存しない）
- テストごと新規バケット＋一意プレフィックス。開始時クリアと `t.Cleanup`（成功／失敗とも）

## 完了注記

- 行動仕様の正本は [`docs/tests/e2e/e2e-spec.md`](../../tests/e2e/e2e-spec.md)（ドメイン specs は既存 recovery／snapshot／options／state）。
- 検証: RustFS 起動下で `cd packages/go && go test -tags=e2e ./... -count=1`
- SemVer タグ `v0.7.0` はマージ後にコーディネータが打つ（本 PR では打たない・Release しない）。
- 関連: [roadmap](../../roadmap.md) · [plans 索引](../README.md) · [docker/e2e](../../../docker/e2e/README.md)

----

以上
