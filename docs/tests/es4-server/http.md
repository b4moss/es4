# Es4 Server — テスト仕様

HTTP Es4 Server（`packages/go/cmd/es4-server` / `internal/server`）のテスト仕様。  
正本の振る舞い: [`docs/specs/es4-server/`](../../specs/es4-server/)。  
必須検証: `cd packages/go && go test ./...`（外部クラウド不要）。

正規パスはすべて **末尾 `/` なし**。末尾 `/` 付きは **404**（リダイレクトなし）。

## 1. Health

### `/livez`

#### テスト：正常系
- `GET /livez` → **200**（Restore 前後どちらでも）

#### テスト：末尾スラッシュ
- `GET /livez/` → **404**（リダイレクトなし）

### `/readyz`

#### テスト：正常系
- Restore Ready **前** → **503**
- Restore Ready **後** → **200**
- `restore_on_startup=false` / `memory_only` 等で即 Ready なら起動直後 200 可

#### テスト：末尾スラッシュ
- `GET /readyz/` → **404**

## 2. State HTTP（Ready 後）

#### テスト：正常系
- `PUT /v1/keys/a/b` body=`{"x":1}` → **200**
- 直後 `GET` → **200**・同じ JSON
- `HEAD` → **200**・本文なし
- `DELETE` → **200**；直後 GET → **404** `not_found`；欠落 HEAD → **404**
- `POST /v1/clear` → **200**；複数キー後に全 GET 404
- 単一セグメント `/v1/keys/a` も同様

#### テスト：異常系
- 欠落 GET／DELETE → **404** `not_found`
- 空キー相当 `/v1/keys`・`/v1/keys/`・規則違反キー → **400** `invalid_key`
- 不正 JSON PUT → **400** `invalid_value`

#### テスト：末尾スラッシュ
- `GET /v1/` → **404**
- `POST /v1/clear/` → **404**

## 3. Tx HTTP（Ready 後）

#### テスト：正常系
- `POST /v1/tx` → **200** `{"id":"…"}`（非空）
- Begin → PUT → commit → 外側 GET で見える
- Begin → PUT → rollback → 外側 GET は 404
- Tx 上の GET／HEAD／DELETE／clear が Tx 視界で動く

#### テスト：異常系
- ネスト `POST /v1/tx` → **409** `conflict`
- commit／rollback 後の同一 id 再利用 → **409** `conflict`
- 未知 id → **404**
- Tx 上の欠落 GET／DELETE → **404** `not_found`；欠落 HEAD → **404**
- 不正キー → **400** `invalid_key`；不正 JSON → **400** `invalid_value`

#### テスト：末尾スラッシュ
- `POST /v1/tx/`・`…/commit/`・`…/rollback/`・`…/clear/` → **404**

## 4. Ready 前

遅い Recovery mock 等で Restore 完了前を再現。

- State／Tx（keys／clear／tx）→ **503** `not_ready`
- 同時に `GET /livez` **200**、`GET /readyz` **503**

## 5. 待受アドレス

- `PORT=9090` → `:9090`
- `ES4_LISTEN_ADDR=:7070`（PORT なし）→ `:7070`
- どちらも未設定 → `:8080`
- 両方あるとき **PORT が勝つ**
- 不正 `PORT`（非数字）→ エラー

## 6. 設定組み立て

- Defaults → `ES4_CONFIG_PATH` YAML → env overlay
- 空 env はスキップ
- `memory_only=true` でも Server 起動・State HTTP 動作

## 7. 末尾スラッシュ回帰表

| 正規 | 末尾 `/` 付き（404） |
|------|----------------------|
| `GET /livez` | `GET /livez/` |
| `GET /readyz` | `GET /readyz/` |
| `POST /v1/clear` | `POST /v1/clear/` |
| `POST /v1/tx` | `POST /v1/tx/` |
| `POST /v1/tx/{id}/commit` | `…/commit/` |
| `POST /v1/tx/{id}/rollback` | `…/rollback/` |
| `POST /v1/tx/{id}/clear` | `…/clear/` |

キー階層の `/`（例: `/v1/keys/a/b`）は本表の対象外。

----

以上
