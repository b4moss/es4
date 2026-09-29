---
状態: 方針確定
マイルストーン: v0.6.0（暫定 / Phase 5）
---

# Phase 5 — State Backend の拡張

## 目的

コアアーキテクチャを特定の State Backend に依存させず、柔軟性を拡張する。

## ざっくり範囲

- その他のインメモリ実装 … **Unscheduled**（意図は Redis / Valkey 寄り。下記決定事項）
- その他の組み込みデータベース … **Unscheduled**（同上）
- Redis / Valkey Adapter … **Unscheduled**
- State Backendごとの最適化（共通 API の範囲内）

## 決定事項

PO Q&A で確定した Phase 5（`v0.6.0` 暫定）の事実。実装詳細は `仕様詳細` へ上げるときに詰める。

### 「その他」Backend と Redis / Valkey

- **その他 Backend のコア追加:** Unscheduled
- **PO 注:** 「その他 Backend」は意図として実質 **Redis / Valkey** を指す
- **Redis / Valkey Adapter:** Unscheduled（上記と同じ扱い。発火条件の別枠は設けない）

### Backend 最適化の境界

- **範囲:** 共有 API（State API・Tx API など公開面）の契約を壊さない最適化のみ
- Backend 固有 API や、共通 API を破る最適化は行わない

## メモ

- SemVer `v0.6.0` は暫定割当。Redis / Valkey および「その他」コア追加は Unscheduled。
- 詳細仕様は後日詰める。実装完了後は本ファイルを `docs/specs/` へ**移動**する。
- 関連: [roadmap](../../roadmap.md) · [Phase 1 決定事項](../../specs/state/) · [Phase 2 決定事項](../v0.3.0/sqlite-state.md#決定事項) · [open-questions](../open-questions.md)（残件なし）

----

以上
