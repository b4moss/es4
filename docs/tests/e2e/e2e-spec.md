# E2E テスト仕様 — Object Recovery × RustFS（S3 互換）

正本のドメイン仕様: [`docs/specs/recovery/`](../../specs/recovery/) · [`docs/specs/snapshot/`](../../specs/snapshot/) · [`docs/specs/options/`](../../specs/options/) · [`docs/specs/state/`](../../specs/state/)。  
本ファイルは **Docker 上の RustFS を S3 互換 Object Recovery として使い、プロセス再起動後の復元を検証する** E2E の手順と成功条件を定める。

実装配置（v0.7.0）:
- ハーネス: `packages/go/pkg/es4/object_recovery_e2e_test.go`（`//go:build e2e`）＋共有ヘルパ `packages/go/internal/e2e`
- Compose: [`docker/e2e/docker-compose.yml`](../../../docker/e2e/docker-compose.yml)（README: [`docker/e2e/README.md`](../../../docker/e2e/README.md)）
- CI: [`.github/workflows/e2e-object-recovery.yml`](../../../.github/workflows/e2e-object-recovery.yml)（**`workflow_dispatch` のみ**）
- 本仕様: `docs/tests/e2e/e2e-spec.md`（本ファイル）

必須検証コマンド:
```text
# RustFS 起動後（docker compose -f docker/e2e/docker-compose.yml up -d）
cd packages/go && go test -tags=e2e ./... -count=1
```

エンドポイント／認証（Compose 既定）:
- S3 API: `http://127.0.0.1:9000`（path-style）
- Access / Secret: `es4e2eaccess` / `es4e2esecretkey`
- Region: `us-east-1`（ダミー可）
- 上書き: `ES4_E2E_S3_ENDPOINT` · `ES4_E2E_S3_REGION` · `ES4_E2E_S3_ACCESS_KEY` / `AWS_ACCESS_KEY_ID` · `ES4_E2E_S3_SECRET_KEY` / `AWS_SECRET_ACCESS_KEY`

----

## 1. 目的

1. es4 の Object Recovery（`recovery_backend=object`）が、**RustFS（S3 互換）** に Snapshot 封筒を実オブジェクトとして書き込めること
2. es4 **プロセスを再起動**したあと、`restore_on_startup` により **最後に成功保存された Recovery 世代** から State が復元されること
3. バケット運用（テストごと新規／毎回クリア／失敗時もクリア）とプレフィックス隔離を再現可能な手順にすること
4. S3 クライアントは **E2E 用に差し替え／注入**し、テストコードの肥大化を抑えること

----

## 2. 用語と検証範囲（コミット済みのみ）

es4 の永続化経路は次のとおりです。

1. 公開 State／Tx でエントリを更新する（Tx は **Commit 後**に外側 State へ反映）
2. Snapshot が State を `Export` し、共通封筒を Recovery へ **Save** する
3. 再起動時、Recovery の最新世代を **Load** し State へ `Replace`（Restore）する

クラッシュ一貫性の既決どおり、復元できるのは **最後に成功した Snapshot／Recovery Point まで**です。

| 用語（本 E2E） | 意味 | 再起動後に戻ることを期待するか |
|----------------|------|--------------------------------|
| **コミット済み・保存済み（検証対象）** | Tx があれば Commit 済みであり、かつ Object Recovery への **Snapshot Save が成功**した時点の State entries | **はい** |
| 未 Commit の Tx 変更 | Rollback 相当。外側 State に未反映 | **いいえ**（仕様どおり戻らない） |
| Commit 済みだが Snapshot 未完了 | プロセス内 State にはあるが、Object 上の最新 Recovery 世代には未反映 | **いいえ**（本 E2E の成功条件に含めない。「コミット済みのみ」かつ **Recovery 保存済み**が対象） |
| Snapshot 進行中の失敗／中断 | 不完全な世代は Load 対象にしない（既存 Adapter 契約）。欠落／不正は空 State 続行 | 空または直前の成功世代 |

**明記:** 本 E2E が保証するのは「**Recovery にコミット（Save）済みの世代が、プロセス再起動後に State へ戻る**」ことだけです。Snapshot 間隔待ちに依存させず、テストは **明示的な Snapshot／フラッシュ手順**（公開 API が無い場合はテスト用フックまたは十分短い `snapshot_interval`＋同期待機ヘルパ）で Save 完了を待ってからプロセスを落とします。

----

## 3. 構成

### 3.1 RustFS（Docker）

- Docker Compose（[`docker/e2e/docker-compose.yml`](../../../docker/e2e/docker-compose.yml)）で **RustFS** を 1 サービス起動する
- es4 からは **S3 互換 API**（AWS SDK v2 系）で到達する
- エンドポイント: `http://127.0.0.1:9000`（`ES4_E2E_S3_ENDPOINT` で上書き可）
- 認証: E2E 専用キー（Compose 既定）を静的クレデンシャルプロバイダ／標準 AWS 系環境変数で渡す
- ホストの永続ボリュームに依存しすぎないこと（テストはバケット／プレフィックスのクリアで隔離）。RustFS データディレクトリは CI ではエフェメラルでよい

### 3.2 es4 側 Options（Effective）

| キー | E2E 推奨値 |
|------|------------|
| `recovery_backend` | `object` |
| `recovery_s3_bucket` | テスト用バケット名（§4） |
| `recovery_s3_prefix` | ラン／ケース隔離用プレフィックス（§4） |
| `recovery_s3_region` | `us-east-1`（ダミー可） |
| `recovery_s3_endpoint` | RustFS の HTTP エンドポイント |
| `restore_on_startup` | `true` |
| `memory_only` | `false`（Recovery を使う） |
| `state_path` | **空**（Memory State）。再起動証明は Recovery → Restore に固定 |
| `snapshot_interval` | `0`（明示 `SnapshotNow` でフラッシュ） |

**State Backend:** 空 `state_path` で Memory を選び、Close 後にプロセス内 State は消える。復元の証明は **Recovery → Restore** に依存する（本 E2E の目的）。仕様上は「再起動後の Open で Restore が走り、検証キーが読める」ことを成功とする。

### 3.3 S3 クライアントの差し替え（肥大化抑制）

既存契約: `internal/recovery.ObjectConfig.Client`（`ObjectAPI`）および `OpenWith` による Recovery 注入。

E2E では **方式 A** に固定する。

| 方式 | 内容 |
|------|------|
| **A（採用）** | `internal/e2e` ヘルパが RustFS endpoint＋静的クレデンシャルで AWS SDK S3 クライアントを組み立て、`ObjectConfig{Client: ...}` ＋ `OpenWith` で注入。テスト本体は「Open → 書き込み → SnapshotNow → Close → 再 Open → Get」に留める |
| B | 環境変数のみ（`AWS_ACCESS_KEY_ID` 等＋`recovery_s3_*`）で本番同様の `OpenObject` 経路。ヘルパは env セットアップとバケット準備に限定 |

テストコードに RustFS の管理 UI 操作や大きな SDK ラッパを埋め込まない。共通ヘルパ（バケット作成／空にする／プレフィックス削除／Compose 待機）は `internal/e2e` に集約する。

----

## 4. バケット・プレフィックス隔離とクリーンアップ

### 4.1 命名

- **バケット:** テストごとに新規作成する。名前は衝突しにくいこと（例: `es4-e2e-<unix>-<random>`）。RustFS／S3 のバケット命名規則に従う
- **プレフィックス:** ケースごとに一意（例: `run-<id>/case-<name>/`）。`recovery_s3_prefix` に設定する。同一バケットを再利用する場合でもプレフィックスで隔離する

### 4.2 開始時クリア

各テスト（または `t.Run` ケース）の **開始時**に次を行う。

1. バケットが無ければ作成する
2. 対象プレフィックス配下のオブジェクトを **すべて削除**する（List → Delete）。バケット自体を空にする方針でもよい
3. その後に es4 を Open する

### 4.3 終了時・失敗時クリア

- `t.Cleanup`（または同等）で、**成功／失敗を問わず** 対象プレフィックス（またはテスト用バケット全体）を削除する
- バケットをテストごとに新規作成した場合は、可能ならバケット削除まで行う（RustFS が空バケット削除のみ許す場合は先にオブジェクト削除）
- クリーンアップ失敗はログに残す。次ランがゼロから始められるよう、開始時クリア（§4.2）を必須とする（失敗時残骸があっても開始時に消す）

### 4.4 並列

- `-parallel` する場合は **バケットまたはプレフィックスがケースごとに一意**であること
- 共有グローバルバケットをクリアし合う設計は禁止

----

## 5. シナリオ

共通前提: RustFS が健全（ヘルスまたは単純な Put/Get プローブ成功）。ヘルパでバケット／プレフィックス準備済み。

### 5.1 正常系 — 保存 → 再起動 → リカバリ

1. es4 を Open（Object Recovery、`restore_on_startup=true`）
2. State にキーを書く（階層キー可）。Tx を使う場合は **Commit まで完了**させる
3. **Snapshot を Object へ Save 完了**まで待つ（明示フラッシュまたはヘルパ）。RustFS 上にプレフィックス配下のオブジェクトが 1 つ以上存在することを List または Head で確認してよい
4. es4 を **Close**（プロセス再起動の代替。実プロセス fork でもよいが、Close→同一バイナリ再 Open で可）
5. **同じ** bucket／prefix／endpoint（および State path 方針）で再度 Open
6. 手順 2 で書いたキーを `Get` → **同じ JSON**。Exists true

#### 期待
- 再 Open 後にコミット済み・Save 済みエントリが復元されている
- RustFS 上のオブジェクトが「実際に保存された」ことの証拠として、Save 後・再 Open 前に List でキーが存在する

### 5.2 正常系 — 複数キー／階層

- 複数キー（単一セグメントと `a/b/c`）を Save 後、再起動ですべて戻る

### 5.3 異常系／境界 — 未保存は戻らない（コミット済みのみの裏返し）

1. Open → Set（必要なら Commit）するが、**Snapshot Save を完了させない**（間隔前に Closeする、またはフラッシュしない）
2. 再 Open
3. 当該キーは **欠落**（`ErrNotFound`）または、直前に成功していた別世代の内容のみ（このケースでは空が期待なら空）

#### 期待
- 「未コミット／未 Save」は復元されないことが仕様どおりであること

### 5.4 異常系 — バケット空からの起動

1. プレフィックスを空にした状態で Open（`restore_on_startup=true`）
2. 起動は成功し、State は空（既存: 欠落は空 State 続行）

### 5.5 クリーンアップ・冪等

1. ケース実行後 Cleanup がプレフィックスを空にする
2. 同じプレフィックスで開始時クリアを再実行してもエラーにならない（冪等）

### 5.6 （任意）TTL／世代

- `recovery_ttl` を使う場合の剪定は単体／統合テスト側が厚い。E2E では必須としない。含めるなら「Save 二回 → 古い世代が剪定され、再起動後は最新のみ」を短く確認する

----

## 6. 非対象

- Redis／Valkey State Backend
- libSQL／File Recovery の E2E（別仕様）
- Es4 Server HTTP の E2E（本ファイルはライブラリ Open／Recovery 経路。Server 経由は将来拡張）
- 本番クラウド（AWS S3／GCS）への到達（RustFS ローカルで代替）
- 負荷・性能 SLO
- 認証・認可プロダクト機能

----

## 7. 成功条件

- §5.1・§5.2・§5.3・§5.4・§5.5 を自動化テストとして実装し、RustFS 起動下で PASS
- 失敗したランのあとも、次ランの開始時クリアでゼロからやり直せる
- 仕様どおり **Recovery に Save 済みの内容のみ**が再起動後に戻る旨が、本ファイルとテスト名／コメントで一致している
- S3 アクセスは共有ヘルパ＋クライアント差し替えに閉じ、ケース本体が薄いこと

----

## 8. 関連

- シナリオ別テスト仕様（正常系／異常系）: [`docs/tests/e2e/scenarios.md`](./scenarios.md)

- Recovery: [`docs/specs/recovery/recovery.md`](../../specs/recovery/recovery.md)
- Options（`recovery_s3_*`）: [`docs/specs/options/options.md`](../../specs/options/options.md)
- Snapshot: [`docs/specs/snapshot/`](../../specs/snapshot/)
- 単体に近い Object テスト: `packages/go/internal/recovery/object_test.go`（本 E2E とは別。フェイク）
- Plan: [`docs/plans/v0.7.0/e2e-object-recovery.md`](../../plans/v0.7.0/e2e-object-recovery.md)

----

以上