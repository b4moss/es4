---
状態: 未決バックログ
マイルストーン: 横断（後続 Phase）
---

# Open Questions（未決プロダクト論点）

PO ビジョン（[`docs/README.md`](../README.md) · [`roadmap.md`](../roadmap.md) · [`plans/v0.1.0`](./v0.1.0/)–[`v0.6.0`](./v0.6.0/)）から読み取れるが、**まだ決めていない**プロダクト論点の一覧。  
確定した事実はここに書かない。選択肢は既存 docs から読み取れる範囲に限る。  
なお Phase 1（最小構成）のプロダクト作業は暫定 SemVer **`v0.2.0`** 配下（`v0.1.0` はスキャフォールド専用。スキャフォールド完了・最初の Git タグ `v0.1.0` の方針は [scaffold 決定事項](./v0.1.0/scaffold.md#決定事項)）。

Phase 1 の論点（State API・公開面・Adapter・設定キー／既定／`snake_case`・Snapshot／Recovery・Ready・明示 Snapshot 非公開・クラッシュ一貫性・同一プロセス直列化など）は [v0.2.0/minimal-core.md の決定事項](./v0.2.0/minimal-core.md#決定事項) を正とする。  
Phase 2（オンディスク SQLite・別 Tx API・同一プロセス直列化）は [v0.3.0/sqlite-state.md の決定事項](./v0.3.0/sqlite-state.md#決定事項)。  
Phase 3 の Adapter 順は [v0.4.0/external-recovery.md の決定事項](./v0.4.0/external-recovery.md#決定事項)。  
Go module path・最初の Git タグ方針は [scaffold 決定事項](./v0.1.0/scaffold.md#決定事項)。

## 並べ方

優先度は「興味深さ」ではなく、**答えると他の論点が不要になる／強く拘束される**順。  
親論点の下に従属フォローをぶら下げてよい。欠番およびトップレベル（旧 P1–P9）は Phase 1／2 で解決済みのため空けている。

| 記号 | 意味 |
|------|------|
| **アンロック** | これを決めると何が進む／何が拘束されるか |
| **選択肢** | 既存 docs から読み取れる候補のみ（無い場合は省略） |
| **Phase** | `後続` = Phase 3 以降で決める |

wishlist（PO メモ）とは別物。決まったら該当 `plans/` を更新し、本一覧から落とす。

---

## Phase 3 以降（セクション内は高 → 低）

[Phase 1 決定事項](./v0.2.0/minimal-core.md#決定事項)・[Phase 2 決定事項](./v0.3.0/sqlite-state.md#決定事項)・[Phase 3 Adapter 順](./v0.4.0/external-recovery.md#決定事項) が親。ここでは後続 Phase 固有の未決だけを Phase 内順位で列挙する。

### Phase 3 — 外部 Recovery Storage（`v0.4.0` 暫定）

（Adapter 実装順は決定済み: libSQL → S3 互換 ASAP・サポート対象。Litestream は Unscheduled。）

1. **世代管理・保持ポリシーのノブ**  
   何を設定可能にし、既定をどうするか（設定面の拡張。Phase 1 は単一ファイル上書き・パスは Options。キーは `snake_case`）。  
   **アンロック:** 運用向け受け入れと、Object / libSQL 実装の共通要件。

2. **GCS 等の位置づけ**  
   pillar 図は Object Storage に S3 / GCS を例示。Phase 3 範囲の「S3 互換」との関係（同一 Adapter か別か）は未決。

### Phase 4 — Es4 Server（`v0.5.0` 暫定）

1. **HTTP API の形（ライブラリ State API への写像）**  
   Phase 1 決定の State API が親。パス／メソッド／エラー表現はライブラリ契約に揃える前提で決める。Tx API（Phase 2・別面）の HTTP 写像も含む。  
   **アンロック:** Docker イメージ・クライアント・Cloud Run 手順のすべて。

2. **環境変数スキーマ**  
   設定面の Server 写像（渡し方の正本は Options。概念名・既定・`snake_case` はライブラリと共有）。名前・必須／任意・既定の具体。  
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
   共通 API を壊さない最適化と、Backend 固有 API の許容境界（Phase 1 決定の公開面・API、Phase 2 の Tx API 分離に従属）。

---

## 意図的にここに書かないもの

- すでに pillar / roadmap / 各 Phase plans に**方針として書いてある範囲**そのもの（例: Go 先行、Adapter 化、Phase 1 で Memory JSON + ファイル Recovery、など）
- [minimal-core 決定事項](./v0.2.0/minimal-core.md#決定事項) に落とした事実（State API・設定キー／既定／`snake_case`、明示 Snapshot 非公開、クラッシュ一貫性、同一プロセス直列化 など）
- [sqlite-state 決定事項](./v0.3.0/sqlite-state.md#決定事項) に落とした事実（オンディスク SQLite、別 Tx API、同一プロセス直列化）
- [external-recovery 決定事項](./v0.4.0/external-recovery.md#決定事項) に落とした事実（libSQL 先行、S3 互換 ASAP・サポート、Litestream Unscheduled）
- [scaffold 決定事項](./v0.1.0/scaffold.md#決定事項) に落とした事実（レイアウト、Go module path、最初の Git タグ `v0.1.0` 方針）
- 詳細仕様の「後日詰め」作業そのもの（それは各 `plans/vX.Y.Z` を `仕様詳細` へ上げる話であり、本ファイルの親論点決定後に行う）
- wishlist 向けの雑多メモ（現時点で wishlist に残件なし）

## 関連

- [pillar](../README.md) · [roadmap](../roadmap.md) · [plans 索引](./README.md)
- Phase plans: [v0.1.0](./v0.1.0/scaffold.md) · [v0.2.0](./v0.2.0/minimal-core.md) · [v0.3.0](./v0.3.0/sqlite-state.md) · [v0.4.0](./v0.4.0/external-recovery.md) · [v0.5.0](./v0.5.0/es4-server.md) · [v0.6.0](./v0.6.0/state-backend-extension.md)

----

以上
