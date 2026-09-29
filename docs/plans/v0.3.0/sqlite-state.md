---
状態: 意図スタブ
マイルストーン: v0.3.0（暫定 / Phase 2）
---

# Phase 2 — SQLite State

## 目的

構造化された State と、より高頻度な State 操作に対応する。

## ざっくり範囲

- インメモリ SQLite State
- SQLiteを利用したスナップショット
- ファイルベースの Recovery Storage
- トランザクション対応
- 並行アクセスへの対応

## メモ

- SemVer `v0.3.0` は暫定割当。Phase 1 完了後に詳細を詰める。
- State API・Snapshot 封筒の骨格は [Phase 1 決定事項](../v0.2.0/minimal-core.md#決定事項) に従う（本 Phase はトランザクション等の拡張。キー区切りは `/`。`v0` 期間は Snapshot 形式の後方互換なし）。
- 実装完了後は本ファイルを `docs/specs/` へ**移動**する。
- 関連: [roadmap](../../roadmap.md) · [Phase 1 決定事項](../v0.2.0/minimal-core.md#決定事項)

----

以上
