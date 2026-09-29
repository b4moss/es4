---
状態: 未決バックログ
マイルストーン: 横断（Phase 1 中心 / 後続 Phase 含む）
---

# Open Questions（未決プロダクト論点）

PO ビジョン（[`docs/README.md`](../README.md) · [`roadmap.md`](../roadmap.md) · [`plans/v0.1.0`](./v0.1.0/)–[`v0.5.0`](./v0.5.0/)）から読み取れるが、**まだ決めていない**プロダクト論点の一覧。  
確定した事実はここに書かない。選択肢は既存 docs から読み取れる範囲に限る。

## 並べ方

優先度は「興味深さ」ではなく、**答えると他の論点が不要になる／強く拘束される**順。  
親論点の下に従属フォローをぶら下げてよいが、**トップレベル番号は全体の優先順**とする。

| 記号 | 意味 |
|------|------|
| **アンロック** | これを決めると何が進む／何が拘束されるか |
| **選択肢** | 既存 docs から読み取れる候補のみ（無い場合は省略） |
| **Phase** | `1` = Phase 1（`v0.1.0` 暫定）で深く決める必要 / `後続` = Phase 2 以降 |

wishlist（PO メモ）とは別物。決まったら該当 `plans/` を更新し、本一覧から落とす。

---

## トップレベル（優先度高 → 低）

### P1. ライブラリ State API の操作面とデータモデル

**問い:** Phase 1 の組み込みライブラリがアプリに出す State API は何か。キー／値の形、操作集合、エラーの出し方、同期／非同期の前提はどうするか。

**アンロック:** Phase 2 のトランザクション API、Phase 4 の HTTP API、Snapshot が「何を写すか」がすべてここから決まる。API を後から広げるコストが最も高い論点。

**選択肢（docs 上）:**
- ライフサイクル図は `SET` / `GET` / `DELETE` を例示（網羅宣言ではない）
- Phase 1 State は「インメモリ JSON」
- Phase 2 で「構造化・高頻度操作・トランザクション」が乗る想定

**Phase:** 1（骨格）／後続で拡張前提を残すかどうかもここで方針を決める

従属:
- **P1a.** キー空間はフラット文字列か、プレフィックス／階層を持たせるか
- **P1b.** 値は任意 JSON ドキュメントか、より狭い契約か
- **P1c.** Phase 2 以降の操作追加を見据え、Phase 1 で「最小＋拡張点」をどう残すか

---

### P2. Adapter 境界と公開面（State / Snapshot / Recovery）

**問い:** pillar 図の State Store・Snapshot Manager・Recovery Storage のうち、どれを**公開インタフェース**とし、どれを内部実装に閉じるか。ライブラリ利用時の組み立て単位は何か。

**アンロック:** Phase 2–5 の Backend／Adapter 追加、組み込みと Server でコアを共有する構想の固定。ここが揺れると後続 Phase の計画がすべて差し戻る。

**選択肢（docs 上）:**
- 技術方針: State 層と Recovery 層は Adapter 化、複数ドライバを設定可能
- 図上の流れ: Application → State API → State Store → Snapshot Manager → Recovery Storage
- 利用想定: Embedded（in-process）と Server（HTTP）の両対応

**Phase:** 1 で境界の骨格／後続 Phase で Adapter を足す前提を明文化

従属:
- **P2a.** Snapshot Manager をアプリ／運用向け API にするか、State API 配下の内部にするか
- **P2b.** Phase 1 時点で「差し替え可能」にする範囲（State のみ／Recovery のみ／両方）

---

### P3. Phase 1 設定値の一覧・既定・渡し方

**問い:** Phase 1 でユーザーが触る設定は何か。キー名、既定値、ライブラリへの渡し方（Options 構造体等）をどうするか。

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

### P4. Snapshot の表現形式と版付け

**問い:** Phase 1（Memory JSON）の Snapshot を Recovery に渡すバイト列／ファイルの形は何か。形式バージョンをどう持つか。

**アンロック:** ファイル Recovery の配置（P5）、Phase 2「SQLite を利用したスナップショット」、Phase 3 の世代管理・保持ポリシーの互換戦略。形式を後から壊すと Recovery 全体がやり直す。

**選択肢（docs 上）:**
- Phase 1: インメモリ JSON State → ファイル Recovery
- Phase 2: SQLite State 用のスナップショットが別途登場
- Phase 3: 世代管理・保持ポリシーが乗る

**Phase:** 1 で初版形式／後続で Backend 差分をどう載せるかの方針

従属:
- **P4a.** Backend ごとに形式を分けるか、共通封筒＋ペイロード分岐か
- **P4b.** 形式の後方互換を `v0.n` 期間にどこまで約束するか（versioning-rule 上、`v0` は破壊的変更可）

---

### P5. ファイル Recovery Storage の契約（配置・命名・失敗時）

**問い:** Phase 1 のファイル Recovery は、どこに・何という名・どう書けば「復旧可能な Recovery Point」になるか。欠落・破損・部分書き込み時の起動挙動は何か。

**アンロック:** Phase 1 の Restore 受け入れ条件。Phase 3 外部 Adapter は「同じ Recovery 契約の別実装」になるため、契約が先。

**選択肢（docs 上）:**
- Phase 1: ファイルベース
- ライフサイクル: periodic / explicit → Snapshot → Recovery Point → restart/failure で State へ戻す
- Phase 3: Object（S3 互換等）・libSQL・Litestream 等へ拡張

**Phase:** 1

従属:
- **P5a.** 単一ファイル上書きか、世代ファイルを Phase 1 から持つか（世代管理の本格は Phase 3）
- **P5b.** 起動時に Recovery が無い／読めない場合: 空 State で続行か、起動失敗か（Memory-only との関係は P3a／P6）

---

### P6. Snapshot ライフサイクルの意味論

**問い:** 「間隔」「明示取得」「起動時復元」「Memory-only」が同時に存在するとき、どれが必須で、どれが互いに打ち消すか。クラッシュ一貫性にどこまでコミットするか。

**アンロック:** Phase 1 の受け入れ条件とテスト観点。P3（設定）・P5（Recovery）の解釈を閉じる。

**選択肢（docs 上）:**
- 計画範囲に、間隔設定・明示取得・起動時復元・Memory-only が並記
- Current State は「揮発してよい」、Recovery Point は「復旧のために残す」

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

**Phase:** 1 で最小約束／本格は Phase 2・4

---

### P8. Go モジュール／パッケージ分割（初期実装）

**問い:** 初期 Go 実装の module path・パッケージ境界（API / State / Snapshot / Recovery / 設定）をどう切るか。

**アンロック:** Adapter 境界（P2）のコード上の置き場、テスト配置、後の Node.js 版への写し方の参照点。プロダクト行為そのものより実装骨格だが、Phase 1 着手前に決める価値が高い。

**選択肢（docs 上）:**
- 言語: Go が初期実装、Node.js（TypeScript）は Go 版の後
- 公開の切り方は P2 に依存

**Phase:** 1

---

### P9. 暫定 SemVer（`v0.1.0`–`v0.5.0`）の正式化方針

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

トップレベル P1–P8 が親。ここでは後続 Phase 固有の未決だけを Phase 内順位で列挙する。

### Phase 2 — SQLite State（`v0.2.0` 暫定）

1. **「インメモリ SQLite」の具体と Snapshot 連携**  
   State の持ち方と「SQLite を利用したスナップショット」の関係（P4 の形式方針に従属）。  
   **アンロック:** Phase 2 実装の中心。ファイル Recovery（Phase 1 契約）との接続。

2. **トランザクション API の載せ方**  
   P1 の State API をどう拡張するか（別 API か、同一面に追加か）。  
   **アンロック:** 受け入れ条件と、HTTP 化（Phase 4）時の写像。

3. **並行アクセスモデル**  
   P7 の Phase 1 約束を前提に、どこまで強めるか。  
   **アンロック:** SQLite State のロック／接続の設計。

### Phase 3 — 外部 Recovery Storage（`v0.3.0` 暫定）

1. **外部 Adapter の実装順**  
   S3 互換オブジェクトストレージ / libSQL / Litestream 連携のどれを先に「使える」にするか。  
   **アンロック:** Phase 3 の計画分割と、世代管理の最初の実証先。

2. **世代管理・保持ポリシーのノブ**  
   何を設定可能にし、既定をどうするか（P3 の設定面の拡張）。  
   **アンロック:** 運用向け受け入れと、Object / libSQL 実装の共通要件。

3. **GCS 等の位置づけ**  
   pillar 図は Object Storage に S3 / GCS を例示。Phase 3 範囲の「S3 互換」との関係（同一 Adapter か別か）は未決。

### Phase 4 — Es4 Server（`v0.4.0` 暫定）

1. **HTTP API の形（ライブラリ State API への写像）**  
   P1 が親。パス／メソッド／エラー表現はライブラリ契約に揃える前提で決める。  
   **アンロック:** Docker イメージ・クライアント・Cloud Run 手順のすべて。

2. **環境変数スキーマ**  
   P3 の設定面の Server 写像。名前・必須／任意・既定。  
   **アンロック:** 運用ドキュメントとイメージのエントリポイント。

3. **Health / Readiness の意味**  
   Restore 中・Recovery 到達不能時にどれを fail とするか（P6b と接続）。  
   **アンロック:** Cloud Run／オーケストレータ向けの受け入れ。

4. **Cloud Run 固有の前提**  
   インスタンス寿命・ファイルシステム・外部 Recovery 必須かどうかなど、Phase 3 成果物との組み合わせ条件。

### Phase 5 — State Backend の拡張（`v0.5.0` 暫定）

1. **「その他」Backend の採択基準**  
   何が揃えばコアに載せるか（P2 の Adapter 契約を満たすこと以外の製品判断）。

2. **Redis / Valkey Adapter の発火条件**  
   plans どおり「具体的なユースケースが生じた場合」のみ。ユースケースの判定者・最小要件は未決。

3. **Backend ごとの最適化の範囲**  
   共通 API を壊さない最適化と、Backend 固有 API の許容境界（P1/P2 に従属）。

---

## 意図的にここに書かないもの

- すでに pillar / roadmap / 各 Phase plans に**方針として書いてある範囲**そのもの（例: Go 先行、Adapter 化、Phase 1 で Memory JSON + ファイル Recovery、など）
- 詳細仕様の「後日詰め」作業そのもの（それは各 `plans/vX.Y.Z` を `仕様詳細` へ上げる話であり、本ファイルの親論点決定後に行う）
- wishlist 向けの雑多メモ（現時点で wishlist に残件なし）

## 関連

- [pillar](../README.md) · [roadmap](../roadmap.md) · [plans 索引](./README.md)
- Phase plans: [v0.1.0](./v0.1.0/minimal-core.md) · [v0.2.0](./v0.2.0/sqlite-state.md) · [v0.3.0](./v0.3.0/external-recovery.md) · [v0.4.0](./v0.4.0/es4-server.md) · [v0.5.0](./v0.5.0/state-backend-extension.md)

----

以上
