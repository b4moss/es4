# v0.9.0 — Node.js／TypeScript ポート 詳細テスト仕様（ドラフト）

状態: **PO 承認済み**  
正本: Go tag `v0.8.0` ＋ `docs/specs/*` ＋ `docs/tests/*`  
Plan: [plan.md](./plan.md)

実装先: `packages/node`  
成功の定義: **同じ ID・同じ期待**を Node で満たす（言語ランタイム由来の差は本仕様で明示したものだけ）。

----

## 0. 共通規則

### 0.1 語彙
- Options／YAML キー: **snake_case**（TS プロパティも同じ）
- env: `ES4_` + SCREAMING_SNAKE
- 公開 API 名・sentinel: Go と同名（`open`／`Open` のケーシングは言語慣例で `open`/`close` 可。**Options キーと Err* 名は一致必須**）
- duration: Go duration 文字列のみ。bool: 小文字 `true`/`false` のみ
- 空 env: スキップ（下位を消さない）

### 0.2 Open 優先（Effective 後）
1. State 注入（OpenWith 相当）  
2. `memory_only` → Memory  
3. `state_backend=redis|valkey` → Redis アダプタ  
4. `state_backend=firestore` → Firestore  
5. `state_path` 非空 → SQLite  
6. それ以外 → Memory  

### 0.3 E2E 共通
- Compose: 既存 `docker/e2e` 共有
- `layer=redis|valkey|firestore` は **`all` に入れない**
- ローカル: 必要 env 欠落 → **skip**。CI 当該 layer → **fail-on-missing**
- `snapshot_interval=0`＋テスト専用明示フラッシュ（Go の SnapshotNow 相当）で Save 証明
- Firestore Clear／空 Replace: **設定 collection のみ**

### 0.4 ゲート（マージ最低限）
Options 必須欠落、State contract（Memory＋少なくとも1永続 Backend）、Tx、Close、Open 配線、E2E 各 layer の C 系必須、Firestore S-N16 相当。

----

## 1. Options（`docs/tests/options/load.md` 写本）

### 1.1 正常系
| ID | 内容 | 期待 |
|----|------|------|
| O-N1 | Defaults | Go と同既定（interval 30s、restore true、memory_only false、空 path／backend 等） |
| O-N2 | YAML 上書き | 指定キー反映 |
| O-N3 | 部分 YAML | 未指定は既定 |
| O-N4 | env が YAML を上書き | env 最終 |
| O-N5 | 空 env スキップ | 下位残存 |
| O-N6 | `memory_only=true`＋recovery／state／redis／firestore 設定 | Validate 成功。Effective は Ignore |
| O-N7 | `recovery_backend` 空＋`recovery_path` 非空 | Resolved `file` |
| O-N8 | Firestore `database_id` 省略 | Effective `(default)` |
| O-N9 | Redis／Valkey URL＋prefix | Load 成功 |
| O-N10 | `state_backend=memory` | Validate 成功 |

### 1.2 異常系
| ID | 内容 | 期待 |
|----|------|------|
| O-E1 | 欠落ファイル（パス指定） | エラー |
| O-E2 | 不正 YAML | エラー |
| O-E3 | duration 非 Go 文字列（数値秒含む） | エラー |
| O-E4 | bool が true/false 以外 | エラー |
| O-E5 | 不正 `recovery_backend` | エラー |
| O-E6 | 負 `recovery_ttl` | エラー |
| O-E7 | file で path 欠落／libsql で url 欠落／object で bucket 欠落 | Validate エラー（memory_only 時スキップ） |
| O-E8 | `state_backend=redis|valkey` で URL 空 | Validate エラー |
| O-E9 | `state_backend=firestore` で project または collection 空 | Validate エラー |
| O-E10 | collection に `/` | Validate エラー |
| O-E11 | `state_backend=sqlite` で path 空 | Validate エラー |
| O-E12 | 未知 `state_backend`／大文字混在 | Validate エラー |

----

## 2. State API（`docs/tests/state/api.md` 写本）

全 Backend（Memory／SQLite／Redis／Firestore）で contract 行列。

### 2.1 CRUD 正常系
| ID | 内容 | 期待 |
|----|------|------|
| S-N1 | Set→Get 同一 JSON | 成功 |
| S-N2 | Exists true／false | エラーなし |
| S-N3 | Delete 後 NotFound | 成功 |
| S-N4 | Clear（空も成功）。Firestore は **自 collection のみ** | 成功 |
| S-N5 | 単一／階層キー | 成功 |
| S-N6 | null／array／object／number／string | Set 可 |

### 2.2 CRUD 異常系
| ID | 内容 | 期待 |
|----|------|------|
| S-E1 | Get 欠落 | ErrNotFound |
| S-E2 | Delete 欠落 | ErrNotFound |
| S-E3 | 無効キー | ErrInvalidKey |
| S-E4 | 不正 JSON | ErrInvalidValue |
| S-E5 | AbortSignal／cancelled context 相当 | 中断エラー |

### 2.3 Export／Replace／Tx／Close 正常系
| ID | 内容 | 期待 |
|----|------|------|
| S-N7 | Export deep copy | 独立 |
| S-N8 | Replace 原子的・旧消失 | 成功 |
| S-N9 | 空 Replace＝Clear | 全欠落（Firestore は自 collection） |
| S-N10 | Tx Commit 可視 | 成功 |
| S-N11 | Tx Rollback 非可視 | 成功 |
| S-N12 | 二重 Close 冪等 | エラーなし |
| S-N13 | Close→再 Open 永続（SQLite／Redis／Firestore） | 内容残る |
| S-N14 | 隔離（別 path／prefix／collection） | 非干渉 |
| S-N15 | Export→他 Store Replace | 往復一致 |
| S-N16 | Firestore: 他 collection 残存で Clear | **他は消えない** |
| S-N17 | Firestore: 空 Replace も他 collection 非破壊 | 同上 |

### 2.4 Export／Tx／Close 異常系
| ID | 内容 | 期待 |
|----|------|------|
| S-E6 | ネスト BeginTx | ErrNestedTx |
| S-E7 | Close 後操作 | ErrClosed |
| S-E8 | 使用済み Tx 再使用 | 既存 Go と同じ失敗 |
| S-E9 | Replace 中の並行読取 | 旧新混在なし |

### 2.5 Open 配線
| ID | 内容 | 期待 |
|----|------|------|
| W-N1..N5 | firestore／redis／memory_only／注入／path | Go と同じ選択 |
| W-E1..E3 | 不完全 Options／未起動／到達不能 | エラーまたは方針どおり skip |

### 2.6 キー写像（Redis／Firestore）
| ID | 内容 | 期待 |
|----|------|------|
| K-N1..N5 | Redis HASH／Firestore `%2F` 往復・Export は論理キー | Go と同じ |
| K-E1..E3 | 無効キー／ID 上限／不正 JSON | 同上 |

----

## 3. Snapshot／Recovery（内部・`docs/tests` 写本）

公開 API に出さない。Open ライフサイクルと E2E で証明。

### 3.1 正常系
| ID | 内容 | 期待 |
|----|------|------|
| R-N1 | File Recovery Save→再 Open Restore | 一致 |
| R-N2 | libSQL File C1–C5 相当 | Go §L と同じ |
| R-N3 | libSQL Memory C2／C4／C5 | 同じ |
| R-N4 | Object（RustFS）§5.1–§5.5 | **改変しない**・同結果 |
| R-N5 | TTL 剪定（設定時） | Go と同じ |
| R-N6 | memory_only 時 Recovery R/W なし | 成功 |

### 3.2 異常系
| ID | 内容 | 期待 |
|----|------|------|
| R-E1 | 必須 recovery 設定欠落 | Validate／Open エラー |
| R-E2 | 到達不能 Object endpoint（CI 方針どおり） | 失敗または skip 規則 |

----

## 4. Tx 公開面（`docs/specs/tx`）

| ID | 内容 | 期待 |
|----|------|------|
| T-N1 | Begin→Set→Commit | 外側可視 |
| T-N2 | Begin→Set→Rollback | 非可視 |
| T-E1 | ネスト | ErrNestedTx |
| T-E2 | Close 後 Begin | ErrClosed |

----

## 5. Es4 Server（`docs/specs/es4-server`・`docs/tests` 相当）

| ID | 内容 | 期待 |
|----|------|------|
| H-N1 | Health liveness 常時 OK | 200 |
| H-N2 | readiness: Restore Ready 後のみ | Go と同じ |
| H-N3 | REST 風 State（キー path） | specs 同契約 |
| H-E1 | 欠落 GET | エラー応答（Go と同じステータス／形） |
| H-E2 | 無効キー | エラー応答 |

（Server を後続 PR に分ける場合は H-* をその PR のゲートに移す。Plan N8）

----

## 6. E2E（`docs/tests/e2e` 写本）

プレフィックス例: `e2e.node.*` または `TestE2E_*` 相当の describe 名。**カタログ ID は Go と同一**。

| Layer | 必須 | 備考 |
|-------|------|------|
| object | §5.1–§5.5 | RustFS |
| file | C 系（File SQLite） | |
| memory | Memory 層 | |
| libsql | File C1–C5・Memory C2/C4/C5 | |
| redis | C1–C5 | all 外 |
| valkey | C1–C5 | all 外 |
| firestore | C1–C5（＋N6/N7・E3/E4 推奨） | all 外・Clear 隔離 |

| ID | 内容 | 期待 |
|----|------|------|
| F-E1 | ローカル env 欠落 | skip |
| F-E2 | CI 当該 layer 欠落 | fail |

----

## 7. CI

| ID | 内容 | 期待 |
|----|------|------|
| C-N1 | packages/node の unit／contract を ci に載せる | PASS |
| C-N2 | e2e.yml に Node 実行経路（job 分割または runtime 入力） | Go 非破壊 |
| C-N3 | layer 追加であれ既存 Go layer を壊さない | 回帰なし |

----

## 8. 成功条件／非対象

成功: §1–§6 必須 ID が Node で PASS、docs/plans/v0.9.0・README 更新、Go 非破壊、draft→merge。tag は別指示。

非対象: 契約変更、Turso／本番 GCP 必須、公開 SnapshotNow、npm publish 必須、camel Options。

----

## 付録 — チェックリスト

Options O-N1..N10 / O-E1..E12  
State S-N1..N17 / S-E1..E9 / W-* / K-*  
Recovery R-N* / R-E*  
Tx T-*  
Server H-*（分割可）  
E2E 全 layer C 系  

----

以上
