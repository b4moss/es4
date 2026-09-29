---
状態: 実装完了（specs へ反映済み）
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
- Adapter（`Store`）境界の docs／契約テスト正本化

## 決定事項

PO Q&A で確定した Phase 5（`v0.6.0` 暫定）の事実。実装詳細の正本は [`docs/specs/state/`](../../specs/state/) へ反映済み。

### 「その他」Backend と Redis / Valkey

- **その他 Backend のコア追加:** Unscheduled
- **PO 注:** 「その他 Backend」は意図として実質 **Redis / Valkey** を指す
- **Redis / Valkey Adapter:** Unscheduled（上記と同じ扱い。発火条件の別枠は設けない）

### Backend 最適化の境界

- **範囲:** 共有 API（State API・Tx API など公開面）の契約を壊さない最適化のみ
- Backend 固有 API や、共通 API を破る最適化は行わない

### Adapter 境界（実装完了で specs へ）

- `Store` の Set／Get／Delete／Exists／Clear／Export／Replace／BeginTx／Close を共有契約とする
- Export は deep copy、Replace は原子的（旧＋新の混在を公開しない）
- Memory／SQLite の契約テストで両 Backend が同一セマンティクスを満たすことを担保

## 完了注記

- **実装完了。** Adapter 境界・Export／Replace・最適化境界・Unscheduled の記述は [`docs/specs/state/state.md`](../../specs/state/state.md) が正本。
- テスト仕様: [`docs/tests/state/api.md`](../../tests/state/api.md)
- 本 plan ファイルは履歴・決定事項の参照用に残す（specs へ内容を反映済み。別ドメイン specs へのファイル移動は不要）。
- SemVer タグ `v0.6.0` はマージ後にコーディネータが打つ（本マイルストーンの PR では打たない）。
- 関連: [roadmap](../../roadmap.md) · [Phase 1–2 決定事項](../../specs/state/) · [Tx](../../specs/tx/) · [open-questions](../open-questions.md)（残件なし）

----

以上
