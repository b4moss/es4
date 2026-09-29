# Phase 7 — v0.8.0 外部サービスの充実（State Backend）

状態: 実装中（本 PR — Firestore State Backend + E2E。PR #29 Redis／Valkey は **マージ済み**。両 Backend 共存）
マイルストーン: v0.8.0（暫定 / Phase 7）

## 目的

1. **Firestore** を State Backend アダプタとして追加する（本 PR）
2. Redis／Valkey は **実装済み**（#29 マージ済み。本ツリーでも同一 Open／Options 契約で共存）
3. E2E は **Firestore Emulator**（`layer=firestore`）。**`layer=all` に入れない**（redis／valkey も同様に all 非対象）
4. タグはマージ後に PO 指示（本 PR では打たない）

## 前提（現行 main）

- State Adapter 境界は v0.6.0 で正本化済み（Memory / SQLite）
- Redis／Valkey State は #29 で main にマージ済み
- 公開 API は State + Tx。Snapshot／Recovery は内部
- 既存 E2E（object / file / memory / libsql / redis / valkey）は維持

## 本 PR の範囲 — Firestore State Backend

- Options: `state_backend`（`""`|`memory`|`sqlite`|`redis`|`valkey`|`firestore`）、`state_firestore_project_id`、`state_firestore_database_id`（Effective 既定 `(default)`）、`state_firestore_collection`（スラッシュ不可）＋ `ES4_*`
- Open 選択優先: 注入 → `memory_only` → `redis|valkey` → `firestore` → `memory` → `state_path`／sqlite → Memory
- `memory_only` は firestore／redis 設定を無視（エラーにしない）
- キー写像: 単一 collection・1 論理キー = 1 ドキュメント。Doc ID = パス・パーセントエンコード（`/` → `%2F`）。フィールド `value`
- **Clear／空 Replace は設定 collection 内のみ**（他 collection／プロジェクト全体を消さない）
- Tx: overlay → Commit（Firestore BulkWriter）。ネストは `ErrNestedTx`
- E2E: `layer=firestore` のみ。State 永続（Close→再 Open）。Recovery／SnapshotNow 必須としない
- 認証・エンドポイントは Options 外（`FIRESTORE_EMULATOR_HOST`）

## 決定事項（PO ロック・2026-09-29）

1. Firestore は **State Backend**（Recovery ではない）
2. Clear 範囲 = `state_firestore_collection` のみ
3. `layer=all` に firestore を **含めない**
4. Redis／Valkey（#29）と Firestore は **共存**（同一 v0.8.0）
5. タグなし（draft PR）

## 成果物

- `docs/plans/v0.8.0/` · roadmap · specs/state · specs/options · tests/state · tests/options · tests/e2e §5F／§Fs
- Go: `internal/state` Firestore adapter + Open 配線 + Options
- Compose／workflow: firestore emulator と `layer=firestore`
- README EN／JA の Scope／E2E 表更新（Redis／Valkey／Firestore）
- draft PR（SemVer タグなし）

## 非対象

- Firestore Recovery
- 本番 GCP 必須 E2E
- `layer=all` 混入
- サブコレクション階層写像

## 検証

```text
cd packages/go && go test ./pkg/options ./internal/state ./pkg/es4 -count=1

# Firestore Emulator E2E
docker compose -f docker/e2e/docker-compose.yml up -d --wait firestore
export FIRESTORE_EMULATOR_HOST=127.0.0.1:8080
export ES4_E2E_FIRESTORE_PROJECT_ID=demo-es4
cd packages/go && go test -tags=e2e ./pkg/es4 -run 'TestE2E_FirestoreState_' -count=1
```

詳細テスト仕様: 添付 PO 承認済み spec（O-＊／K-＊／S-＊／W-＊／F-＊）。

----

以上
