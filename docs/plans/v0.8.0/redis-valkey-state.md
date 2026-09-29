---
状態: 実装中（PR #1 — Redis／Valkey State Backend + E2E。Firestore は次 PR）
マイルストーン: v0.8.0（暫定 / Phase 7 — 外部サービスの充実）
---

# Phase 7 — v0.8.0 外部サービスの充実（State Backend）

## 目的

1. **Redis／Valkey** を State Backend アダプタとして追加する（本 PR）
2. その次に **Firestore** を State Backend アダプタとして追加する（別 PR）
3. E2E は **Redis・Valkey**（本 PR）および Firestore Emulator（次 PR）で実施する
4. **v0.8.0 の範囲はここまで**。タグはマージ後に PO 指示

## 前提（現行）

- State Adapter 境界は v0.6.0 で正本化済み（Memory / SQLite）
- 公開 API は State + Tx。Snapshot／Recovery は内部
- 既存 E2E（object / file / memory / libsql）は維持。S3 §5 は改変しない

## 本 PR（#1）の範囲 — Redis／Valkey State Backend

- **同一アダプタ**（Redis プロトコル互換クライアント）。E2E だけプロセスを Redis と Valkey の2種で証明
- Options: `state_backend`（`""`|`memory`|`sqlite`|`redis`|`valkey`）、`state_redis_url`、`state_redis_key_prefix` ＋ `ES4_*`
- Open 選択優先: 注入 → `memory_only` → `redis|valkey` → `state_path` → Memory
- `memory_only` は redis 設定を無視（エラーにしない）
- Tx: overlay → MULTI/EXEC Commit（契約満たせばよい）
- 単一 HASH（field = 論理キー）。Export／Replace は既存封筒契約
- E2E: `layer=redis` / `layer=valkey` のみ。**`layer=all` には入れない**
- State 永続の証明（Close→再 Open）。Recovery／SnapshotNow は必須としない

## 次 PR（#2）— Firestore（本 PR 非対象）

- `state_backend=firestore` ＋プロジェクト／DB／認証（エミュレータ向け env）
- E2E `layer=firestore`

## 決定事項（PO ロック）

1. Redis／Valkey は **同一アダプタ**
2. `layer=all` に新層を **含めない**
3. PR 分割: Redis/Valkey → Firestore（2本連続、同じ v0.8.0）
4. タグ: マージ後に PO 指示（本 PR では打たない）
5. Recovery アダプタ化は非対象（State only）

## 成果物（本 PR）

- `docs/plans/v0.8.0/` · roadmap · specs/state · specs/options · tests/state · tests/e2e §R§V
- Go: `internal/state` Redis adapter + Open 配線 + Options
- Compose／workflow: redis / valkey サービスと layer
- README EN／JA の Scope／E2E 表更新
- draft PR（SemVer タグなし）

## 非対象（v0.8.0 全体）

- Redis／Valkey／Firestore を **Recovery** アダプタにすること
- 本番クラウド必須の E2E（エミュレータ／Compose のみ）
- Turso リモート E2E、Node 移植、Litestream、公開 SnapshotNow

## 検証（本 PR）

```text
cd packages/go && go test ./pkg/options ./internal/state ./pkg/es4 -count=1

# Redis E2E
docker compose -f docker/e2e/docker-compose.yml up -d --wait redis
export ES4_E2E_REDIS_URL=redis://127.0.0.1:6379/0
cd packages/go && go test -tags=e2e ./pkg/es4 -run 'TestE2E_RedisState_' -count=1

# Valkey E2E
docker compose -f docker/e2e/docker-compose.yml up -d --wait valkey
export ES4_E2E_VALKEY_URL=redis://127.0.0.1:6380/0
cd packages/go && go test -tags=e2e ./pkg/es4 -run 'TestE2E_ValkeyState_' -count=1
```

----

以上
