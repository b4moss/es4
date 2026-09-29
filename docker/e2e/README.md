# E2E Docker — RustFS（S3 互換）

Object Recovery E2E（`//go:build e2e`）用の RustFS Compose。

## 起動

```bash
# リポジトリルートから
docker compose -f docker/e2e/docker-compose.yml up -d
curl -sf http://127.0.0.1:9000/health
```

| 項目 | 値 |
|------|-----|
| S3 API | `http://127.0.0.1:9000`（path-style） |
| Console | `http://127.0.0.1:9001` |
| Access key | `es4e2eaccess` |
| Secret key | `es4e2esecretkey` |
| Region | `us-east-1`（ダミー可） |

## テスト

```bash
# 三層（Object は RustFS 必須。未起動時は Object のみ skip）
export AWS_ACCESS_KEY_ID=es4e2eaccess
export AWS_SECRET_ACCESS_KEY=es4e2esecretkey
export ES4_E2E_S3_ENDPOINT=http://127.0.0.1:9000
export ES4_E2E_S3_REGION=us-east-1

cd packages/go && go test -tags=e2e ./... -count=1

# 層ごと（RustFS 不要な例）
go test -tags=e2e ./pkg/es4 -run 'TestE2E_FileSQLite_' -count=1
go test -tags=e2e ./pkg/es4 -run 'TestE2E_Memory_' -count=1
```

## 停止

```bash
docker compose -f docker/e2e/docker-compose.yml down -v
```

行動仕様の正本: [`docs/tests/e2e/e2e-spec.md`](../../docs/tests/e2e/e2e-spec.md)。
GitHub Actions: `.github/workflows/e2e.yml`（`workflow_dispatch` のみ・`layer` 入力）。レガシー別名: `e2e-object-recovery.yml`。
