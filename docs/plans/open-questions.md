---
状態: 未決バックログ
マイルストーン: 横断（Phase 1 中心 / 後続 Phase 含む）
---

# Open Questions（未決プロダクト論点）

PO ビジョン（[`docs/README.md`](../README.md) · [`roadmap.md`](../roadmap.md) · [`plans/v0.1.0`](./v0.1.0/)–[`v0.6.0`](./v0.6.0/)）から読み取れるが、**まだ決めていない**プロダクト論点の一覧。  
確定した事実はここに書かない。選択肢は既存 docs から読み取れる範囲に限る。  
なお Phase 1（最小構成）のプロダクト作業は暫定 SemVer **`v0.2.0`** 配下（`v0.1.0` はスキャフォールド専用）。

Phase 1 の State API・公開面・Adapter 差し替え・設定の渡し方・Snapshot 封筒・ファイル Recovery の上書き／欠落時挙動は [v0.2.0/minimal-core.md の決定事項](./v0.2.0/minimal-core.md#決定事項) を正とする（旧 P1／P1a／P1b・同期非同期、P2／P2a／P2b、P3 の渡し方、P4／P4a、P5a／P5b）。

## 並べ方

優先度は「興味深さ」ではなく、**答えると他の論点が不要になる／強く拘束される**順。  
親論点の下に従属フォローをぶら下げてよいが、**トップレベル番号は全体の優先順**とする。欠番（P1・P2 など）は解決済みのため空けている。

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

**渡し方（決定済み）:** Options が正本。任意で設定ファイルも読める。詳細は [minimal-core 決定事項](./v0.2.0/minimal-core.md#決定事項)。

**アンロック:** Phase 1 実装の受け入れ境界が固まる。Phase 4「環境変数による設定」は、ここで決めた設定面の写像になる。

**選択肢（docs 上・設定候補）:**
- スナップショット間隔
- 明示的スナップショット取得の有無（API として存在することは計画済み。設定か操作かは API 側）
- 起動時復元の有無／必須度
- Memory-only（永続化なし）モード
- ファイルベース Recovery のパス類（パス自体は P5）

**Phase:** 1

従属:
- **P3a.** Memory-only 時に「間隔設定」や Recovery 関連設定を無視するか、設定エラーにするか
- **P3b.** ライブラリ設定と、後の Server 環境変数で**同じ概念名・同じ既定**を共有するか（共有するなら Phase 4 の命名が拘束される）

---

### P4. Snapshot 形式の後方互換範囲

**問い:** 形式の後方互換を `v0.n` 期間にどこまで約束するか（versioning-rule 上、`v0` は破壊的変更可）。

**形式方針（決定済み）:** 版付きの共通封筒＋ Backend 固有ペイロード。詳細は [minimal-core 決定事項](./v0.2.0/minimal-core.md#決定事項)。

**アンロック:** Phase 2「SQLite を利用したスナップショット」、Phase 3 の世代管理・保持ポリシーの互換戦略。互換約束の幅が Recovery の移行コストを決める。

**Phase:** 1 で初版の約束幅／後続で Backend 差分を載せるときの拘束

（旧 **P4a** は決定済みのため削除）

---

### P5. ファイル Recovery Storage の配置・命名

**問い:** Phase 1 のファイル Recovery は、どこに・何という名で置けば「復旧可能な Recovery Point」になるか。

**決定済み（従属）:** 単一ファイル上書き（旧 P5a）。起動時に Recovery が無い／読めない場合は空 State で続行（旧 P5b）。正本は [minimal-core 決定事項](./v0.2.0/minimal-core.md#決定事項)。

**アンロック:** Phase 1 の Restore 受け入れ条件のうち配置契約。Phase 3 外部 Adapter は「同じ Recovery 契約の別実装」になるため、配置・命名の骨格が先。

**選択肢（docs 上）:**
- Phase 1: ファイルベース
- ライフサイクル: periodic / explicit → Snapshot → Recovery Point → restart/failure で State へ戻す
- Phase 3: Object（S3 互換等）・libSQL・Litestream 等へ拡張

**Phase:** 1

---

### P6. Snapshot ライフサイクルの意味論

**問い:** 「間隔」「明示取得」「起動時復元」「Memory-only」が同時に存在するとき、どれが必須で、どれが互いに打ち消すか。クラッシュ一貫性にどこまでコミットするか。

**アンロック:** Phase 1 の受け入れ条件とテスト観点。P3（設定）・P5（Recovery）の解釈を閉じる。

**選択肢（docs 上）:**
- 計画範囲に、間隔設定・明示取得・起動時復元・Memory-only が並記
- Current State は「揮発してよい」、Recovery Point は「復旧のために残す」
- 起動時欠落は空 State 続行（決定済み）を前提に、Memory-only や「起動時復元オフ」との組み合わせを詰める

**Phase:** 1

従属:
- **P6a.** 明示 Snapshot と定期 Snapshot の優先・抑止関係
- **P6b.** Restore 完了前に State API を受け付けるか（Ready の定義）

---

### P7. Phase 1 の並行・プロセス前提

**問い:** Phase 1 組み込みは、同一プロセス内の複数呼び出しに対してどの安全性を約束するか（単一利用者前提か、ロックするか、文書化だけか）。

**アンロック:** Phase 2「並行アクセスへの対応」の差分範囲。Phase 4 の複数アプリ共有とは層が違うが、「単一プロセス内」の約束がないと Server 実装も迷う。

**選択肢（docs 上）:**
- Phase 1: 最小ライフサイクル（並行は明記なし）
- Phase 2: 並行アクセス対応を範囲に含む
- Phase 4: 複数アプリ／インスタンスから共有（Server）
- 呼び出しは非同期志向（決定済み）だが、並行安全性の約束とは別論点

**Phase:** 1 で最小約束／本格は Phase 2・4

---

### P8. Go モジュール／パッケージ分割（初期実装）

**問い:** 初期 Go 実装の module path・パッケージ境界（API / State / Snapshot / Recovery / 設定）をどう切るか。

**アンロック:** Adapter 境界（公開は State API のみ・State／Recovery は差し替え可、決定済み）のコード上の置き場、テスト配置、後の Node.js 版への写し方の参照点。プロダクト行為そのものより実装骨格だが、Phase 1 着手前に決める価値が高い。

**選択肢（docs 上）:**
- 言語: Go が初期実装、Node.js（TypeScript）は Go 版の後
- 公開面は State API のみ（Snapshot / Recovery は内部）

**Phase:** 1

---

### P9. 暫定 SemVer（`v0.1.0`–`v0.6.0`）の正式化方針

**問い:** roadmap / plans の版号をいつ・何を条件に正式号へ固定するか。最初の Git タグ（versioning-rule）をどの成果物で打つか。

**アンロック:** フォルダ名・roadmap 表・外部向け版名の一致。機能 API より文書・配布の横断論点。機能実装を直接はブロックしないためトップレベルでは下位。

**選択肢（docs 上）:**
- 現状はすべて「暫定」
- `v0.n.0` は正式リリース前で破壊的変更可
- `v1.0.0` は PO 判断
- 版を上げたら main で Git タグ

**Phase:** 横断（Phase 1 完了前後が候補になりやすい）

---

## Phase 2 以降（セクション内は高 → 低）

トップレベル残件（P3–P9）および [Phase 1 決定事項](./v0.2.0/minimal-core.md#決定事項) が親。ここでは後続 Phase 固有の未決だけを Phase 内順位で列挙する。

### Phase 2 — SQLite State（`v0.3.0` 暫定）

1. **「インメモリ SQLite」の具体と Snapshot 連携**  
   State の持ち方と「SQLite を利用したスナップショット」の関係（共通封筒＋ Backend 固有ペイロード方針に従属）。  
   **アンロック:** Phase 2 実装の中心。ファイル Recovery（Phase 1 契約）との接続。

2. **トランザクション API の載せ方**  
   Phase 1 決定の State API（`SET` / `GET` / `DELETE` / `EXISTS` / `CLEAR`、階層キー、任意 JSON、非同期志向）をどう拡張するか（別 API か、同一面に追加か）。scan / `LIST` をいつ持つかも含む。  
   **アンロック:** 受け入れ条件と、HTTP 化（Phase 4）時の写像。

3. **並行アクセスモデル**  
   P7 の Phase 1 約束を前提に、どこまで強めるか。  
   **アンロック:** SQLite State のロック／接続の設計。

### Phase 3 — 外部 Recovery Storage（`v0.4.0` 暫定）

1. **外部 Adapter の実装順**  
   S3 互換オブジェクトストレージ / libSQL / Litestream 連携のどれを先に「使える」にするか。  
   **アンロック:** Phase 3 の計画分割と、世代管理の最初の実証先。

2. **世代管理・保持ポリシーのノブ**  
   何を設定可能にし、既定をどうするか（P3 の設定面の拡張。Phase 1 は単一ファイル上書き）。  
   **アンロック:** 運用向け受け入れと、Object / libSQL 実装の共通要件。

3. **GCS 等の位置づけ**  
   pillar 図は Object Storage に S3 / GCS を例示。Phase 3 範囲の「S3 互換」との関係（同一 Adapter か別か）は未決。

### Phase 4 — Es4 Server（`v0.5.0` 暫定）

1. **HTTP API の形（ライブラリ State API への写像）**  
   Phase 1 決定の State API が親。パス／メソッド／エラー表現はライブラリ契約に揃える前提で決める。  
   **アンロック:** Docker イメージ・クライアント・Cloud Run 手順のすべて。

2. **環境変数スキーマ**  
   P3 の設定面の Server 写像（渡し方の正本は Options）。名前・必須／任意・既定。  
   **アンロック:** 運用ドキュメントとイメージのエントリポイント。

3. **Health / Readiness の意味**  
   Restore 中・Recovery 到達不能時にどれを fail とするか（P6b と接続。起動時欠落は空 State 続行が前提）。  
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
- [minimal-core 決定事項](./v0.2.0/minimal-core.md#決定事項) に落とした事実（State API 操作・キー／値・非同期志向、公開面、Adapter 差し替え、Options／設定ファイル、Snapshot 封筒、単一ファイル Recovery、起動時欠落時の空 State）
- 詳細仕様の「後日詰め」作業そのもの（それは各 `plans/vX.Y.Z` を `仕様詳細` へ上げる話であり、本ファイルの親論点決定後に行う）
- wishlist 向けの雑多メモ（現時点で wishlist に残件なし）

## 関連

- [pillar](../README.md) · [roadmap](../roadmap.md) · [plans 索引](./README.md)
- Phase plans: [v0.1.0](./v0.1.0/scaffold.md) · [v0.2.0](./v0.2.0/minimal-core.md) · [v0.3.0](./v0.3.0/sqlite-state.md) · [v0.4.0](./v0.4.0/external-recovery.md) · [v0.5.0](./v0.5.0/es4-server.md) · [v0.6.0](./v0.6.0/state-backend-extension.md)

----

以上
