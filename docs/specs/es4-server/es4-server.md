# Es4 Server 仕様

現行バージョンの HTTP Es4 Server 正本（Phase 4 / SemVer `v0.5.0`）。

実装: `packages/go/cmd/es4-server`（入口）と `packages/go/internal/server`（HTTP 写像）。  
Docker: `docker/Dockerfile`（マルチステージ静的バイナリ）。

## 概要

単一プロセスの HTTP サーバ。複数クライアントが HTTP 経由で同一 State を共有する。  
**1 プロセス＝1 Store**（マルチレプリカ間の State 共有は範囲外）。認証は範囲外。

Cloud Run 向け: **公開パスに末尾 `/` を付けない**。末尾 `/` 付きへの 301／308 リダイレクトは行わない。末尾 `/` 付きは **404**。

## 待受

優先順位: `PORT`（数字のみ → `:{PORT}`）> `ES4_LISTEN_ADDR` > 既定 `:8080`。

## 設定

ライブラリ Options と同じ組み立て順:

1. Defaults
2. 任意 `ES4_CONFIG_PATH` の YAML
3. `ES4_*` env overlay（空はスキップ）

サーバ固有の追加は待受アドレスのみ。`memory_only=true` でも起動・State HTTP は動作する。

## Health（末尾 `/` なし）

| パス | 挙動 |
|------|------|
| `GET /livez` | 常に **200** |
| `GET /readyz` | Restore Ready 後 **200**、それ以前 **503** |

`/livez/`・`/readyz/` は正規とせず **404**。

ライブラリの `DB.Ready()` が Restore 完了（またはスキップ）を示す。Server はこのフラグで Readiness と State／Tx ゲートを行う。

## API 接頭辞

`/v1`（`/v1/` は正規としない → **404**）。

## State 写像（Ready 後）

| メソッド | パス | 写像 | 成功 |
|----------|------|------|------|
| `PUT` | `/v1/keys/{key…}` | SET（body＝JSON） | **200** |
| `GET` | `/v1/keys/{key…}` | GET | **200**＋JSON |
| `DELETE` | `/v1/keys/{key…}` | DELETE | **200** |
| `HEAD` | `/v1/keys/{key…}` | EXISTS | あり **200**／なし **404**（本文なし） |
| `POST` | `/v1/clear` | CLEAR | **200** |

- キーはパス階層そのもの（例: `/v1/keys/a/b` → `a/b`）
- 空キー相当 `/v1/keys`・`/v1/keys/` → **400** `invalid_key`
- 欠落 GET／DELETE → **404** `not_found`
- 不正 JSON PUT → **400** `invalid_value`
- `/v1/clear/` → **404**

## Tx 写像（Ready 後・別面）

| メソッド | パス | 写像 | 成功 |
|----------|------|------|------|
| `POST` | `/v1/tx` | BeginTx | **200** `{"id":"…"}` |
| `PUT`/`GET`/`DELETE`/`HEAD` | `/v1/tx/{id}/keys/{key…}` | Tx 上の State | State と同様 |
| `POST` | `/v1/tx/{id}/clear` | Tx CLEAR | **200** |
| `POST` | `/v1/tx/{id}/commit` | Commit | **200** |
| `POST` | `/v1/tx/{id}/rollback` | Rollback | **200** |

- ネスト Begin・終了後の同一 id 再利用 → **409** `conflict`
- 未知 id → **404**
- `/v1/tx/` および `…/commit/`・`…/rollback/`・`…/clear/` → **404**

## Ready 前

State／Tx のいずれも **503** `not_ready`。`GET /livez` は **200**、`GET /readyz` は **503**。

## エラー JSON

```json
{"error":{"code":"…","message":"…"}}
```

code: `not_found` / `invalid_key` / `invalid_value` / `conflict` / `not_ready`

## Docker / Cloud Run

```bash
docker build -f docker/Dockerfile .
```

- `EXPOSE 8080`
- Cloud Run は `PORT` を設定する（サーバが尊重）
- ヘルス例: `GET /livez`・`GET /readyz`（末尾 `/` なし）
- 推奨 Recovery: 外部（libSQL／Object）。ファイル Recovery も許容

## 範囲外

認証・認可、gRPC／WebSocket、LIST／scan、公開 Snapshot API、マルチレプリカ共有 State。Redis／Valkey 等の追加 State Backend は Phase 5 でも **Unscheduled**（[`docs/specs/state/`](../state/)）。

## 関連

- テスト仕様: [`docs/tests/es4-server/`](../../tests/es4-server/)
- State / Tx / Options / Recovery: [`docs/specs/state/`](../state/) · [`tx`](../tx/) · [`options`](../options/) · [`recovery`](../recovery/)

----

以上
