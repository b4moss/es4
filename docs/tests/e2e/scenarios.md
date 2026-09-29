# E2E — テスト仕様（Object Recovery × RustFS）

Object Recovery を Docker 上の RustFS（S3 互換）に対して実行する E2E の**テスト仕様**（シナリオ 5 本）。  
**期待値（正常系／異常系）の出し方:** [`e2e-spec.md`](./e2e-spec.md) の §5 および実装 `packages/go/pkg/es4/object_recovery_e2e_test.go`（PR #18）が検証している基準そのものを反映する。本ファイルで新規の判定基準は設けない。

振る舞い・構成・隔離の正本: [`e2e-spec.md`](./e2e-spec.md)。  
ドメイン正本: [`docs/specs/recovery/`](../../specs/recovery/) · [`docs/specs/snapshot/`](../../specs/snapshot/) · [`docs/specs/options/`](../../specs/options/)。

実装・実行:
- `packages/go/pkg/es4/object_recovery_e2e_test.go`（`//go:build e2e`）· `packages/go/internal/e2e/` · `docker/e2e/`
- RustFS 起動後: `cd packages/go && go test -tags=e2e ./... -count=1`
- CI: `.github/workflows/e2e-object-recovery.yml`（`workflow_dispatch` のみ）

対象は仕様 §5.1–§5.5 のみ（§5.6 TTL は E2E 必須外のため含めない）。

## 前提（検証で共通に使っているもの）

実装の `TestMain`／`newFixture`／`open` および `e2e-spec.md` §3–§4 より:

- RustFS 健全（ヘルパが endpoint の health を待つ）
- S3 クライアントをヘルパで組み立て、`ObjectConfig.Client` + `OpenWith` で注入（方式 A）
- テストごと新規バケット＋一意プレフィックス。開始時 `ClearPrefix`。終了時（成功／失敗問わず）Cleanup で再クリア
- State は空 `state_path`（Memory）。再起動は Close → 再 Open。`restore_on_startup=true`、`recovery_backend=object`
- Save は `SnapshotNow` で明示（間隔任せにしない）
- 検証範囲（`e2e-spec.md` §2）: **Recovery に Save 済み**の内容のみ再起動後に戻る

----

## 1. 保存 → 再起動 → リカバリ（正常系）

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

## 2. 複数キー／階層（正常系）

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

## 3. 未保存は戻らない（異常系／境界）

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

## 4. バケット空（プレフィックス空）からの起動（異常系）

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

## 5. クリーンアップ・冪等

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

## 非対象

`e2e-spec.md` §6 および §5.6 のとおり（本 5 本に含めない）:

- TTL／世代剪定の E2E（任意）
- File／libSQL Recovery E2E、Es4 Server HTTP E2E
- 本番 AWS S3／GCS、Redis／Valkey
- push／pull_request 自動 CI（E2E は手動 `workflow_dispatch` のみ）

----

## 成功条件

`e2e-spec.md` §7 および実装の PASS 基準どおり:

- §1–§5（仕様 §5.1–§5.5）が自動化テストとして RustFS 起動下で PASS
- 失敗ラン後も開始時クリアで次ランをゼロからやり直せる
- 期待は Save 済みのみが再起動後に戻る、という仕様 §2 と一致している

----

以上
