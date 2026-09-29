---
状態: 方針確定
マイルストーン: v0.3.0（暫定 / Phase 2）
---

# Phase 2 — SQLite State

## 目的

構造化された State と、より高頻度な State 操作に対応する。

## ざっくり範囲

- オンディスク SQLite ファイル State（インメモリ SQLite ではない）
- SQLite を利用したスナップショット
- ファイルベースの Recovery Storage
- トランザクション対応（State API とは別の Tx API）
- 並行アクセスへの対応（同一プロセス・内部ロック直列化）

## 決定事項

PO Q&A batch-3 で確定した Phase 2（`v0.3.0` 暫定）の事実。実装詳細は `仕様詳細` へ上げるときに詰める。

### State の持ち方

- **Backend:** オンディスクの SQLite **ファイル**（インメモリ SQLite ではない）
- **Snapshot:** Phase 1 の共通封筒＋ Backend 固有ペイロード方針に従い、SQLite State 向けペイロードを載せる（`v0` 期間は形式後方互換なし）

### トランザクション

- **Tx API:** State API（`SET` / `GET` / `DELETE` / `EXISTS` / `CLEAR`）とは**別面**として提供する（同一 State API 面への追加ではない）

### 並行

- **Phase 2:** Phase 1 と同様、同一プロセス内の呼び出しは内部ロックで直列化する（この Phase ではそれ以上は強めない）

## メモ

- SemVer `v0.3.0` は暫定割当。Phase 1 完了後に詳細を詰める。
- State API・Snapshot 封筒・設定キー（`snake_case`）・クラッシュ一貫性の骨格は [Phase 1 決定事項](../v0.2.0/minimal-core.md#決定事項) に従う。
- 実装完了後は本ファイルを `docs/specs/` へ**移動**する。
- 関連: [roadmap](../../roadmap.md) · [Phase 1 決定事項](../v0.2.0/minimal-core.md#決定事項) · [open-questions](../open-questions.md)（残件なし）

----

以上
