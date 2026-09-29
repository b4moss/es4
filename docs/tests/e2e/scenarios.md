# E2E — テスト仕様（Object Recovery × RustFS）

Object Recovery を Docker 上の RustFS（S3 互換）に対して実行する E2E の**テスト仕様**。  
振る舞い・構成・隔離の正本: [`e2e-spec.md`](./e2e-spec.md)。  
ドメイン正本: [`docs/specs/recovery/`](../../specs/recovery/) · [`docs/specs/snapshot/`](../../specs/snapshot/) · [`docs/specs/options/`](../../specs/options/)。

実装: `packages/go/pkg/es4/object_recovery_e2e_test.go`（`//go:build e2e`）· `packages/go/internal/e2e/` · `docker/e2e/`。  
必須検証: RustFS 起動後 `cd packages/go && go test -tags=e2e ./... -count=1`。  
CI: `.github/workflows/e2e-object-recovery.yml`（**`workflow_dispatch` のみ**）。

本ファイルの対象シナリオは **5 本**（仕様 §5.1–§5.5）。TTL／世代の任意シナリオ（§5.6）は含めない。

## 前提（全ケース共通）

- RustFS が健全（`GET {endpoint}/health` 成功）
- S3 クライアントは E2E ヘルパが組み立て、`ObjectConfig.Client` + `OpenWith` で注入する（方式 A）
- 各ケース: **新規バケット**＋**一意プレフィックス**。開始時にプレフィックスをクリア。終了時（成功／失敗問わず）`t.Cleanup` で再クリア
- State は空 `state_path`（Memory）。再起動は **Close → 再 Open**。復元の証明は Object Recovery → Restore に依存する
- Snapshot は **`SnapshotNow` で明示 Save**（間隔任せにしない）
- Options: `recovery_backend=object`、`restore_on_startup=true`、`memory_only=false`
- **検証範囲:** Recovery に Save 済みの世代のみが再起動後に戻る。未 Save の State 変更は戻らない

----

## 1. 保存 → 再起動 → リカバリ

対応: 仕様 §5.1 · `TestE2E_ObjectRecovery_SaveRestartRestore`

### 1.1 正常系

#### テスト：正常系
- Open → `Set`（例: キー `users/1`、任意 JSON）→ `SnapshotNow` 成功
- Save 直後、対象プレフィックス配下を List すると **オブジェクトが 1 つ以上**存在する
- Close → 同じ bucket／prefix／endpoint で再 Open
- `Get` が投入時と **同一 JSON**、`Exists` が `true`

### 1.2 異常系

#### テスト：異常系
- （本ケースでは Save 成功後の復元を主眼とする。未 Save の欠落は §3 で扱う）
- 再 Open 後に当該キーが欠落、または JSON 不一致 → **FAIL**（正常系の否定）

----

## 2. 複数キー／階層・Tx Commit

対応: 仕様 §5.2 · `TestE2E_ObjectRecovery_MultiKeyHierarchy`

### 2.1 正常系

#### テスト：正常系
- 単一セグメントと階層キー（例: `plain`、`a/b/c`、`x/y`、`solo`）に異なる JSON（文字列／オブジェクト／配列／数値）を `Set`
- `BeginTx` → Tx 上で `Set` → `Commit`（例: `tx/key`）
- `SnapshotNow` → Close → 再 Open
- 直接 `Set` した全キーおよび Tx Commit 済みキーが、それぞれ **同一 JSON** で `Get` できる

### 2.2 異常系

#### テスト：異常系
- Tx を **Commit せず** Rollback／Close した場合、その Tx 内キーは外側 State に残らず、Save しても（外側に無いため）復元対象にならない（既存 Tx 契約。本 E2E 5 本では専用ケースなし。§3 の未 Save と混同しない）
- Save 後の再 Open でいずれかのキー欠落または値不一致 → **FAIL**

----

## 3. 未保存は戻らない

対応: 仕様 §5.3 · `TestE2E_ObjectRecovery_UnsavedNotRestored`  
（境界・異常系シナリオ。期待は「戻らないこと」）

### 3.1 正常系（仕様どおり戻らない＝成功）

#### テスト：正常系
- Open → `Set`（例: `ephemeral`）するが **`SnapshotNow` を呼ばない**
- Close 後、対象プレフィックスの List が **空**
- 再 Open → 当該キーの `Get` が **`ErrNotFound`**（`IsNotFound`）

### 3.2 異常系（仕様違反＝テスト FAIL 条件）

#### テスト：異常系
- 未 Save なのにプレフィックスにオブジェクトが残る → **FAIL**
- 未 Save なのに再 Open 後にキーが読める → **FAIL**（誤って永続化された／別経路で復元された）

----

## 4. 空プレフィックスからの起動

対応: 仕様 §5.4 · `TestE2E_ObjectRecovery_EmptyBucketStartup`

### 4.1 正常系

#### テスト：正常系
- プレフィックスが空のまま Open（`restore_on_startup=true`）が **成功**する
- State は空: 任意キーの `Exists` が `false`、`Get` が **`ErrNotFound`**
- （既存契約どおり）Recovery 欠落は致命エラーにせず空 State で続行する

### 4.2 異常系

#### テスト：異常系
- 空プレフィックスで Open 自体が失敗する → **FAIL**（欠落は続行が正）
- 空のはずなのに存在しないキーの `Exists` が `true`、または `Get` が成功する → **FAIL**

----

## 5. クリーンアップの冪等

対応: 仕様 §5.5 · `TestE2E_ObjectRecovery_CleanupIdempotent`

### 5.1 正常系

#### テスト：正常系
- Open → `Set` → `SnapshotNow` → Close により、プレフィックス配下にオブジェクトが 1 つ以上ある
- `ClearPrefix` を **1 回目**実行 → エラーなし
- 同じプレフィックスで `ClearPrefix` を **2 回目**実行 → エラーなし（冪等）
- List 結果が **空**

### 5.2 異常系

#### テスト：異常系
- 1 回目または 2 回目の Clear がエラーを返す → **FAIL**
- Clear 後もプレフィックス配下にオブジェクトが残る → **FAIL**（次ランのゼロスタートを妨げうる）

----

## 非対象（本テスト仕様に含めない）

- §5.6 TTL／世代剪定の E2E
- File／libSQL Recovery E2E、Es4 Server HTTP E2E
- 本番 AWS S3／GCS への到達
- Redis／Valkey State Backend
- push／pull_request で自動実行される CI（E2E は手動 `workflow_dispatch` のみ）

----

## 成功条件

- 上記 §1–§5 の正常系がすべて PASS。各節の異常系に該当する観測が無いこと
- 失敗したランのあとでも、次ランの開始時クリアでゼロからやり直せる（fixture + §5）
- 正本 [`e2e-spec.md`](./e2e-spec.md) および実装テスト名と対応が取れていること

----

以上
