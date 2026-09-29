---
状態: 未決バックログ
マイルストーン: 横断（Phase 1 中心 / 後続 Phase 含む）
---

# Open Questions（未決プロダクト論点）

PO ビジョン（[`docs/README.md`](../README.md) · [`roadmap.md`](../roadmap.md) · [`plans/v0.1.0`](./v0.1.0/)–[`v0.6.0`](./v0.6.0/)）から読み取れるが、**まだ決めていない**プロダクト論点の一覧。  
確定した事実はここに書かない。選択肢は既存 docs から読み取れる範囲に限る。  
なお Phase 1（最小構成）のプロダクト作業は暫定 SemVer **`v0.2.0`** 配下（`v0.1.0` はスキャフォールド専用。スキャフォールド完了・最初の Git タグ `v0.1.0` の方針は [scaffold 決定事項](./v0.1.0/scaffold.md#決定事項)）。

Phase 1 の State API・公開面・Adapter 差し替え・設定の渡し方／Memory-only／概念名共有・Snapshot 封筒／`v0` 非互換・ファイル Recovery・Ready／明示 Snapshot タイマー・同一プロセス直列化・キー区切りは [v0.2.0/minimal-core.md の決定事項](./v0.2.0/minimal-core.md#決定事項) を正とする（旧 P1／P1a／P1b・同期非同期、P2／P2a／P2b、P3 の渡し方／P3a／P3b、P4／P4a、P5／P5a／P5b、P6a／P6b、P7、キー区切り `/`）。Go module path・最初の Git タグ方針は [scaffold 決定事項](./v0.1.0/scaffold.md#決定事項)（旧 P8／P9）。

## 並べ方

優先度は「興味深さ」ではなく、**答えると他の論点が不要になる／強く拘束される**順。  
親論点の下に従属フォローをぶら下げてよいが、**トップレベル番号は全体の優先順**とする。欠番（P1・P2・P4・P5・P7・P8・P9 など）は解決済みのため空けている。

| 記号 | 意味 |
|------|------|
| **アンロック** | これを決めると何が進む／何が拘束されるか |
| **選択肢** | 既存 docs から読み取れる候補のみ（無い場合は省略） |
| **Phase** | `1` = Phase 1（`v0.2.0` 暫定）で深く決める必要 / `後続` = Phase 2 以降 |

wishlist（PO メモ）とは別物。決まったら該当 `plans/` を更新し、本一覧から落とす。

---

## トップレベル（優先度高 → 低）

### P3. Phase 1 設定値の一覧・既定

**問い:** Phase 1 でユーザーが触る設定は何か。キー名と既定値をどうするか。

**決定済み（従属）:** 渡し方は Options が正本（任意で設定ファイル）。Memory-only 時は Recovery 関連設定を無視して続行（旧 P3a）。ライブラリ Options と Server 環境変数は同じ概念名・同じ既定（旧 P3b）。Recovery ファイルパスは Options で指定。詳細は [minimal-core 決定事項](./v0.2.0/minimal-core.md#決定事項)。

**残る問い:** キー名の列挙と各既定値の具体（間隔の単位・既定秒数、起動時復元の既定オン／オフ、Memory-only の既定など）。

**アンロック:** Phase 1 実装の受け入れ境界が固まる。Phase 4「環境変数による設定」は、ここで決めた設定面の写像になる。

**選択肢（docs 上・設定候補）:**
- スナップショット間隔
- 明示的スナップショット取得の有無（操作としての存在は計画済み。公開 API にするかは別論点・未決）
- 起動時復元の有無／必須度
- Memory-only（永続化なし）モード
- ファイルベース Recovery のパス（パス指定手段は Options・決定済み）

**Phase:** 1

---

### P6. Snapshot ライフサイクルの意味論（残余）

**問い:** 「間隔」「明示取得」「起動時復元」「Memory-only」が同時に存在するとき、どれが必須で、どれが互いに打ち消すか。クラッシュ一貫性にどこまでコミットするか。

**決定済み（従属）:** 明示 Snapshot は定期間隔タイマーをリセット（旧 P6a）。Restore 完了前でも State API を受け付ける（旧 P6b）。Memory-only 時は Recovery 関連設定を無視（旧 P3a）。正本は [minimal-core 決定事項](./v0.2.0/minimal-core.md#決定事項)。

**残る問い:**
- 起動時復元オフと Memory-only 以外の組み合わせで、どれを必須／任意とするか
- クラッシュ一貫性（どこまでのコミットか）
- **明示 Snapshot の公開 API:** 以前スキップされた論点。公開面は State API のみ（決定済み）だが、明示取得を利用者がどう起動するかは**未決のまま**とする（ここで決めない）

**アンロック:** Phase 1 の受け入れ条件とテスト観点。P3（設定キー一覧）の解釈を閉じる。

**Phase:** 1

---

## Phase 2 以降（セクション内は高 → 低）

トップレベル残件（P3・P6）および [Phase 1 決定事項](./v0.2.0/minimal-core.md#決定事項) が親。ここでは後続 Phase 固有の未決だけを Phase 内順位で列挙する。

### Phase 2 — SQLite State（`v0.3.0` 暫定）

1. **「インメモリ SQLite」の具体と Snapshot 連携**  
   State の持ち方と「SQLite を利用したスナップショット」の関係（共通封筒＋ Backend 固有ペイロード方針に従属。`v0` 期間は形式後方互換なし）。  
   **アンロック:** Phase 2 実装の中心。ファイル Recovery（Phase 1 契約）との接続。

2. **トランザクション API の載せ方**  
   Phase 1 決定の State API（`SET` / `GET` / `DELETE` / `EXISTS` / `CLEAR`、階層キー区切り `/`、任意 JSON、非同期志向）をどう拡張するか（別 API か、同一面に追加か）。scan / `LIST` をいつ持つかも含む。  
   **アンロック:** 受け入れ条件と、HTTP 化（Phase 4）時の写像。

3. **並行アクセスモデル**  
   Phase 1 の同一プロセス直列化（内部ロック）を前提に、どこまで強めるか。  
   **アンロック:** SQLite State のロック／接続の設計。

### Phase 3 — 外部 Recovery Storage（`v0.4.0` 暫定）

1. **外部 Adapter の実装順**  
   S3 互換オブジェクトストレージ / libSQL / Litestream 連携のどれを先に「使える」にするか。  
   **アンロック:** Phase 3 の計画分割と、世代管理の最初の実証先。

2. **世代管理・保持ポリシーのノブ**  
   何を設定可能にし、既定をどうするか（P3 の設定面の拡張。Phase 1 は単一ファイル上書き・パスは Options）。  
   **アンロック:** 運用向け受け入れと、Object / libSQL 実装の共通要件。

3. **GCS 等の位置づけ**  
   pillar 図は Object Storage に S3 / GCS を例示。Phase 3 範囲の「S3 互換」との関係（同一 Adapter か別か）は未決。

### Phase 4 — Es4 Server（`v0.5.0` 暫定）

1. **HTTP API の形（ライブラリ State API への写像）**  
   Phase 1 決定の State API が親。パス／メソッド／エラー表現はライブラリ契約に揃える前提で決める。  
   **アンロック:** Docker イメージ・クライアント・Cloud Run 手順のすべて。

2. **環境変数スキーマ**  
   P3 の設定面の Server 写像（渡し方の正本は Options。概念名・既定はライブラリと共有）。名前・必須／任意・既定の具体。  
   **アンロック:** 運用ドキュメントとイメージのエントリポイント。

3. **Health / Readiness の意味**  
   Restore 中・Recovery 到達不能時にどれを fail とするか（Restore 完了前の State API 受付は決定済み。起動時欠落は空 State 続行が前提）。  
   **アンロック:** Cloud Run／オーケストレータ向けの受け入れ。

4. **Cloud Run 固有の前提**  
   インスタンス寿命・ファイルシステム・外部 Recovery 必須かどうかなど、Phase 3 成果物との組み合わせ条件。

### Phase 5 — State Backend の拡張（`v0.6.0` 暫定）

1. **「その他」Backend の採択基準**  
   何が揃えばコアに載せるか（State／Recovery Adapter 契約を満たすこと以外の製品判断）。

2. **Redis / Valkey Adapter の発火条件**  
   plans どおり「具体的なユースケースが生じた場合」のみ。ユースケースの判定者・最小要件は未決。

3. **Backend ごとの最適化の範囲**  
   共通 API を壊さない最適化と、Backend 固有 API の許容境界（Phase 1 決定の公開面・API に従属）。

---

## 意図的にここに書かないもの

- すでに pillar / roadmap / 各 Phase plans に**方針として書いてある範囲**そのもの（例: Go 先行、Adapter 化、Phase 1 で Memory JSON + ファイル Recovery、など）
- [minimal-core 決定事項](./v0.2.0/minimal-core.md#決定事項) に落とした事実（State API 操作・キー区切り `/`／値・非同期志向、公開面、Adapter 差し替え、Options／設定ファイル／概念名共有／Memory-only、Snapshot 封筒／`v0` 非互換、単一ファイル Recovery／パス、起動時欠落時の空 State、Ready、明示 Snapshot のタイマー、同一プロセス直列化）
- [scaffold 決定事項](./v0.1.0/scaffold.md#決定事項) に落とした事実（レイアウト、Go module path、最初の Git タグ `v0.1.0` 方針）
- 詳細仕様の「後日詰め」作業そのもの（それは各 `plans/vX.Y.Z` を `仕様詳細` へ上げる話であり、本ファイルの親論点決定後に行う）
- wishlist 向けの雑多メモ（現時点で wishlist に残件なし）

## 関連

- [pillar](../README.md) · [roadmap](../roadmap.md) · [plans 索引](./README.md)
- Phase plans: [v0.1.0](./v0.1.0/scaffold.md) · [v0.2.0](./v0.2.0/minimal-core.md) · [v0.3.0](./v0.3.0/sqlite-state.md) · [v0.4.0](./v0.4.0/external-recovery.md) · [v0.5.0](./v0.5.0/es4-server.md) · [v0.6.0](./v0.6.0/state-backend-extension.md)

----

以上
