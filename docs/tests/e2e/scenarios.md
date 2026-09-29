# E2E — テスト仕様（共通カタログ C1–C6）

各層 E2E の**テスト仕様**。期待値は [`e2e-spec.md`](./e2e-spec.md) の各層節および実装アサーションに整合する。本ファイルで新規の FAIL 基準は設けない。

振る舞い・構成・隔離の正本: [`e2e-spec.md`](./e2e-spec.md)。  
ドメイン正本: [`docs/specs/recovery/`](../../specs/recovery/) · [`docs/specs/snapshot/`](../../specs/snapshot/) · [`docs/specs/options/`](../../specs/options/)。

実装・実行:
- S3: `packages/go/pkg/es4/object_recovery_e2e_test.go` · `packages/go/internal/e2e/` · `docker/e2e/`
- ファイル SQLite: `packages/go/pkg/es4/file_sqlite_e2e_test.go`
- インメモリ: `packages/go/pkg/es4/memory_e2e_test.go`
- libSQL: `packages/go/pkg/es4/libsql_file_e2e_test.go` · `libsql_memory_e2e_test.go`
- `cd packages/go && go test -tags=e2e ./... -count=1`（S3 は RustFS 起動下。未起動時は Object のみ skip。libSQL は RustFS 不要）
- CI: `.github/workflows/e2e.yml`（`workflow_dispatch` のみ・`layer` 入力: all|object|file|memory|libsql）

採用マトリクス: [`e2e-spec.md`](./e2e-spec.md) §0.2。

----

# A. S3 互換層（Object Recovery × RustFS）

対象は仕様 §5.1–§5.5 のみ（§5.6 TTL は E2E 必須外のため含めない）。**手順・期待・判定は e2e-spec §5 および既存 `TestE2E_ObjectRecovery_*` を維持する。**

## 前提（検証で共通に使っているもの）

実装の `TestMain`／`newFixture`／`open` および `e2e-spec.md` §3–§4 より:

- RustFS 健全（ヘルパが endpoint の health を待つ）。未起動時は Object ケースが skip（CI object／all は `ES4_E2E_REQUIRE_RUSTFS=1` で fail）
- S3 クライアントをヘルパで組み立て、`ObjectConfig.Client` + `OpenWith` で注入（方式 A）
- テストごと新規バケット＋一意プレフィックス。開始時 `ClearPrefix`。終了時（成功／失敗問わず）Cleanup で再クリア
- State は空 `state_path`（Memory）。再起動は Close → 再 Open。`restore_on_startup=true`、`recovery_backend=object`
- Save は `SnapshotNow` で明示（間隔任せにしない）
- 検証範囲（`e2e-spec.md` §2）: **Recovery に Save 済み**の内容のみ再起動後に戻る

----

## A.1 保存 → 再起動 → リカバリ（正常系）— C1

対応: `e2e-spec.md` §5.1 · `TestE2E_ObjectRecovery_SaveRestartRestore`  
分類: 仕様上の **正常系**

### 手順（検証どおり）

1. Open（Object Recovery、`restore_on_startup=true`）
2. State にキーを書く（実装例: `users/1` に JSON）
3. `SnapshotNow` で Object へ Save 完了
4. プレフィックス配下を List → オブジェクトが 1 つ以上（Save の実体確認）
5. Close → 同じ bucket／prefix／endpoint で再 Open
6. 手順 2 のキーを `Get`／`Exists`

### 期待（正常系）— `e2e-spec.md` §5.1「期待」および実装アサーション

- 再 Open 後に、Save 済みエントリが復元されている
- `Get` の JSON が投入時と同一、`Exists` が true
- Save 後・再 Open 前に List でオブジェクトが存在する（RustFS へ実際に保存されたことの確認）

----

## A.2 複数キー／階層（正常系）— C2

対応: `e2e-spec.md` §5.2 · `TestE2E_ObjectRecovery_MultiKeyHierarchy`  
分類: 仕様上の **正常系**

### 手順（検証どおり）

1. 複数キーを `Set`（仕様: 単一セグメントと `a/b/c`。実装はこれに加え `x/y`・`solo` および Tx `Begin`→`Set`→`Commit` の `tx/key` も Save 対象に含めて検証）
2. `SnapshotNow` → Close → 再 Open
3. Save 対象とした全キーを `Get`

### 期待（正常系）— `e2e-spec.md` §5.2 および実装アサーション

- Save 後の再起動で、書き込んだ複数キーがすべて戻る
- 各キーの `Get` 結果が投入時の JSON と同一（実装が Tx Commit 済みキーを含めた場合はそれも含む）

----

## A.3 未保存は戻らない（異常系／境界）— C3

対応: `e2e-spec.md` §5.3 · `TestE2E_ObjectRecovery_UnsavedNotRestored`  
分類: 仕様上の **異常系／境界**（「コミット済み・Save 済みのみ」の裏返し）

### 手順（検証どおり）

1. Open → `Set`（必要なら Commit）するが、**Snapshot Save を完了させない**（実装: `SnapshotNow` せず Close）
2. 再 Open
3. 当該キーを `Get`

### 期待（異常系／境界）— `e2e-spec.md` §5.3「期待」および実装アサーション

- 未 Save の内容は復元されない
- 実装の確認: Close 直後のプレフィックス List が空、再 Open 後の `Get` が `ErrNotFound`（`IsNotFound`）
- （仕様文言）当該キーは欠落（`ErrNotFound`）。直前に成功していた別世代がある構成ではその内容のみ、という一般則があるが、本ケースの fixture は空プレフィックスから始まるため **空／NotFound** が検証結果

----

## A.4 バケット空（プレフィックス空）からの起動（異常系）— C4

対応: `e2e-spec.md` §5.4 · `TestE2E_ObjectRecovery_EmptyBucketStartup`  
分類: 仕様上の **異常系**（Recovery 欠落経路）

### 手順（検証どおり）

1. プレフィックスを空にした状態で Open（`restore_on_startup=true`）
2. State の空を確認（実装: `Exists`／`Get`）

### 期待（異常系）— `e2e-spec.md` §5.4 および実装アサーション

- 起動は成功する
- State は空（既存契約: 欠落は空 State で続行）
- 実装の確認: 任意キーの `Exists` が false、`Get` が `ErrNotFound`

----

## A.5 クリーンアップ・冪等 — C5

対応: `e2e-spec.md` §5.5 · `TestE2E_ObjectRecovery_CleanupIdempotent`  
分類: 仕様 §5.5（クリーンアップ・冪等）

### 手順（検証どおり）

1. Save 済みオブジェクトがある状態を作る（実装: Set → `SnapshotNow` → Close）
2. プレフィックスを Clear（1 回目）
3. 同じプレフィックスで Clear を再実行（2 回目）
4. List

### 期待 — `e2e-spec.md` §5.5 および実装アサーション

- Cleanup／Clear がプレフィックスを空にする
- 同じプレフィックスへの再 Clear がエラーにならない（冪等）
- 実装の確認: 2 回目 Clear 後の List が空

----

# B. ファイル SQLite 層

対応: `e2e-spec.md` §F · `TestE2E_FileSQLite_*`。C1–C5。RustFS 不要。

## 前提

- `state_path` SQLite + `recovery_backend=file` / `recovery_path`
- テストごと独立 temp。開始時／`t.Cleanup` で clear
- Save は `SnapshotNow`
- **再起動の Recovery 証明:** Close 後に SQLite state（WAL／SHM 含む）を削除して再 Open（`e2e-spec.md` §F.0）。期待は File Recovery に Save 済みのみが戻ること

----

## B.1 保存 → 再起動 → リカバリ（正常系）— C1

対応: §F.1 · `TestE2E_FileSQLite_SaveRestartRestore`

### 手順

1. Open → Set → `SnapshotNow` → Recovery ファイル存在を確認
2. Close → state 削除 → 再 Open → `Get`／`Exists`

### 期待 — §F.1 および実装アサーション

- Save 済みエントリが復元される（JSON 同一、Exists true）
- Snapshot 後に Recovery ファイルが存在する

----

## B.2 複数キー／階層（正常系）— C2

対応: §F.2 · `TestE2E_FileSQLite_MultiKeyHierarchy`

### 手順

1. 複数キー Set（＋ Tx Commit キー）→ `SnapshotNow` → Close → state 削除 → 再 Open
2. 全キー `Get`

### 期待 — §F.2 および実装アサーション

- Save 対象キーがすべて同一 JSON で戻る

----

## B.3 未保存は戻らない（異常系／境界）— C3

対応: §F.3 · `TestE2E_FileSQLite_UnsavedNotRestored`

### 手順

1. Set するが `SnapshotNow` しない → Close → state 削除 → 再 Open → `Get`

### 期待 — §F.3 および実装アサーション

- Recovery ファイルは無い
- `Get` が `ErrNotFound`（`IsNotFound`）

----

## B.4 空状態からの起動 — C4

対応: §F.4 · `TestE2E_FileSQLite_EmptyStartup`

### 手順

1. 空 temp で Open → 空 State を確認

### 期待 — §F.4 および実装アサーション

- 起動成功、Exists false、`Get` が `ErrNotFound`

----

## B.5 クリーンアップ・冪等 — C5

対応: §F.5 · `TestE2E_FileSQLite_CleanupIdempotent`

### 手順

1. Save 済みを作る → clear 2 回

### 期待 — §F.5 および実装アサーション

- 再 clear がエラーにならない
- Recovery／state が残らない

----

# C. インメモリ層

対応: `e2e-spec.md` §M · `TestE2E_Memory_*`。C2・C4・C5 のみ。再起動復元なし。RustFS 不要。

## 前提

- `memory_only=true`（Recovery／`state_path` は Effective で無視）
- Open → 基本操作。Close→再 Open の復元は検証しない

----

## C.2 複数キー／階層（再起動なし）— C2

対応: §M.2 · `TestE2E_Memory_MultiKeyOps`

### 手順

1. Open → 複数キー Set（＋ Tx Commit）→ 同一プロセスで `Get`

### 期待 — §M.2 および実装アサーション

- 全キーの JSON が投入時と同一

----

## C.4 空状態からの起動＋基本操作 — C4

対応: §M.4 · `TestE2E_Memory_EmptyStartup`

### 手順

1. Open → 空確認 → Set／Get

### 期待 — §M.4 および実装アサーション

- 起動直後は空（Exists false / NotFound）
- 基本 Set／Get が成功する

----

## C.5 クリーンアップ・冪等（最小）— C5

対応: §M.5 · `TestE2E_Memory_CleanupIdempotent`

### 手順

1. Set → Clear 2 回 → Close 2 回

### 期待 — §M.5 および実装アサーション

- Clear／Close の再実行がエラーにならない
- Clear 後はキーが存在しない

----

# D. libSQL Recovery 層

対応: `e2e-spec.md` §L · `TestE2E_LibSQLFile_*` / `TestE2E_LibSQLMemory_*`。RustFS 不要。リモート Turso 非対象。

## D.F File-backed（C1–C5）

### 前提

- `recovery_backend=libsql`、`recovery_libsql_url=file:<temp>/recovery.db`
- 空 `state_path`（Memory State）。再起動は Close → 再 Open
- Save は `SnapshotNow`。Save 済み判定は Recovery テーブル行の有無

----

### D.F.1 保存 → 再起動 → リカバリ — C1

対応: §L.F.1 · `TestE2E_LibSQLFile_SaveRestartRestore`

#### 手順

1. Open → Set → `SnapshotNow` → Recovery 行ありを確認
2. Close → 再 Open → `Get`／`Exists`

#### 期待 — §L.F.1 および実装アサーション

- Save 済みエントリが復元される（JSON 同一、Exists true）

----

### D.F.2 複数キー／階層 — C2

対応: §L.F.2 · `TestE2E_LibSQLFile_MultiKeyHierarchy`

#### 手順

1. 複数キー Set（＋ Tx Commit）→ `SnapshotNow` → Close → 再 Open → 全キー `Get`

#### 期待 — §L.F.2 および実装アサーション

- Save 対象キーがすべて同一 JSON で戻る

----

### D.F.3 未保存は戻らない — C3

対応: §L.F.3 · `TestE2E_LibSQLFile_UnsavedNotRestored`

#### 手順

1. Set するが `SnapshotNow` しない → Close → 再 Open → `Get`

#### 期待 — §L.F.3 および実装アサーション

- Recovery 行は無い
- `Get` が `ErrNotFound`（`IsNotFound`）

----

### D.F.4 空状態からの起動 — C4

対応: §L.F.4 · `TestE2E_LibSQLFile_EmptyStartup`

#### 手順

1. 空 temp で Open → 空 State を確認

#### 期待 — §L.F.4 および実装アサーション

- 起動成功、Exists false、`Get` が `ErrNotFound`

----

### D.F.5 クリーンアップ・冪等 — C5

対応: §L.F.5 · `TestE2E_LibSQLFile_CleanupIdempotent`

#### 手順

1. Save 済みを作る → clear 2 回

#### 期待 — §L.F.5 および実装アサーション

- 再 clear がエラーにならない
- Recovery DB が残らない

----

## D.M InMemory（C2・C4・C5）

### 前提

- `recovery_libsql_url=:memory:`、空 `state_path`
- Open → 基本操作＋プロセス内 `SnapshotNow`。Close→再 Open の復元は検証しない

----

### D.M.2 複数キー／階層（再起動なし）— C2

対応: §L.M.2 · `TestE2E_LibSQLMemory_MultiKeyOps`

#### 手順

1. Open → 複数キー Set（＋ Tx Commit）→ `SnapshotNow` → 同一プロセスで `Get`

#### 期待 — §L.M.2 および実装アサーション

- 全キーの JSON が投入時と同一

----

### D.M.4 空状態からの起動＋基本操作 — C4

対応: §L.M.4 · `TestE2E_LibSQLMemory_EmptyStartup`

#### 手順

1. Open → 空確認 → Set／Get

#### 期待 — §L.M.4 および実装アサーション

- 起動直後は空（Exists false / NotFound）
- 基本 Set／Get が成功する

----

### D.M.5 クリーンアップ・冪等（最小）— C5

対応: §L.M.5 · `TestE2E_LibSQLMemory_CleanupIdempotent`

#### 手順

1. Set → `SnapshotNow` → Clear 2 回 → Close 2 回

#### 期待 — §L.M.5 および実装アサーション

- Clear／Close の再実行がエラーにならない
- Clear 後はキーが存在しない

----

## 非対象

`e2e-spec.md` §6 および §5.6／C6 のとおり:

- TTL／世代剪定の E2E（任意）
- リモート Turso／`libsql://` クラウド、Es4 Server HTTP E2E
- 本番 AWS S3／GCS、Redis／Valkey
- push／pull_request 自動 CI（E2E は手動 `workflow_dispatch` のみ）
- インメモリ／libSQL Memory の C1／C3（再起動復元はユースケース外）

----

## 成功条件

`e2e-spec.md` §7 および実装の PASS 基準どおり:

- S3: §A.1–§A.5（仕様 §5.1–§5.5）が RustFS 起動下で PASS
- ファイル SQLite: §B.1–§B.5（C1–C5）が PASS（RustFS 不要）
- インメモリ: §C.2・§C.4・§C.5 が PASS（RustFS 不要）
- libSQL: §D.F.1–§D.F.5 および §D.M.2・§D.M.4・§D.M.5 が PASS（RustFS 不要）
- 期待は各層の e2e-spec 節と一致（新規 FAIL 基準を設けない）

----

以上
