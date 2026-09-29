# Phase 8 — v0.9.0 Node.js／TypeScript ポート（Plan）

状態: **PO 承認済み**（実装 draft PR）  
マイルストーン: v0.9.0（暫定）  
正本: Go `v0.8.0`（tag）の契約・語彙・E2E カタログ。言語固有の例外は本 Plan で明示したノブ以外認めない。

実装先: `packages/node`  
詳細テスト仕様: [test-spec-node.md](./test-spec-node.md)

----

## 1. 目的

- `packages/node` に、Go `packages/go`（**v0.8.0 相当**）の **公開契約・語彙・振る舞いを同型**で移植する
- 単体／契約テストに加え、**E2E も同カタログ・同 layer 方針**まで揃える
- 仕様の正本は引き続き `docs/specs/*`・`docs/tests/*`（言語非依存）。Node は実装面。Go を壊さない

## 2. スコープ（in）

Go v0.8.0 で実装済みの次を **すべて** TypeScript で満たす。

| 領域 | 内容 |
|------|------|
| Options | Defaults → YAML → `ES4_*`。キー名・型規則・Validate／Effective／memory_only 無視規則 |
| State | Memory · SQLite（`state_path`）· Redis／Valkey · Firestore |
| Tx | BeginTx → Set／Get／Delete → Commit／Rollback。ネスト `ErrNestedTx` |
| Snapshot／Recovery（内部） | 周期 Snapshot・起動 Restore・File／libSQL／Object Recovery・TTL。公開 API に Snapshot／Recovery を出さない |
| Open／Close | Open 優先表（注入 → memory_only → redis\|valkey → firestore → state_path → Memory） |
| Es4 Server | HTTP Server（パス・Health・env 写像は specs/es4-server と同契約） |
| E2E | object／file／memory／libsql／redis／valkey／firestore。redis／valkey／firestore は **`layer=all` に入れない** |
| Docs | roadmap・plans/v0.9.0・README EN／JA の Node 節 |

## 3. 非対象（v0.9.0）

- Go 契約の変更・拡張
- 語彙の独自改名（camelCase Options）
- Turso リモート必須 E2E・本番 GCP 必須 Firestore E2E
- 公開 `SnapshotNow`（テストヘルパのみ）
- npm publish（リポジトリ内パッケージ完成まで）

## 4. 語彙・契約

- Options／YAML: **snake_case**。TS プロパティも同じ（`state_path` 等）
- env: `ES4_` + SCREAMING_SNAKE
- duration: Go duration 文字列。bool: 小文字 `true`/`false` のみ
- 空 env: スキップ
- Sentinel: `ErrNotFound`・`ErrInvalidKey`・`ErrInvalidValue`・`ErrClosed`・`ErrNestedTx` 等
- Firestore Clear: **設定 collection のみ**

## 5. 技術ノブ（承認）

| # | 項目 | 選択 |
|---|------|------|
| N1 | ランタイム | Node 20+ LTS＋TypeScript strict |
| N2 | テスト | Vitest |
| N3 | SQLite | `better-sqlite3` |
| N4 | Redis | `ioredis` |
| N5 | Firestore | `@google-cloud/firestore`＋Emulator |
| N6 | Object／S3 | AWS SDK v3 |
| N7 | YAML | `yaml` |
| N8 | PR | 1 本 draft（library＋unit＋E2E＋Server） |
| N9 | npm 公開 | 本マイルストーンではしない |
| N10 | Options 名 | snake_case 固定 |

## 6. 成功条件

- Node 公開 API で Go README Quick start／Recovery／Redis／Firestore 相当が動く
- `docs/tests/*` 必須ケースが Node でも PASS（言語固有 skip 以外）
- Go CI／E2E を壊さない
- roadmap に v0.9.0 行。README「Node stub」を更新
- draft→merge→（指示後）tag `v0.9.0`

----

以上
