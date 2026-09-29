---
状態: 方針確定
マイルストーン: v0.2.0（暫定 / Phase 1）
---

# Phase 1 — 最小構成

## 目的

State → Snapshot → Recovery という Es4 の基本ライフサイクルを確立する。

## ざっくり範囲

- インメモリ JSON State
- ファイルベースの Recovery Storage
- Snapshot / Restore
- スナップショット間隔の設定
- 明示的なスナップショット取得
- 起動時の復元
- 永続化なしの Memory-only モード

## メモ

- SemVer `v0.2.0` は暫定割当。正式版名が決まり次第フォルダと roadmap を更新する。
- 詳細仕様（受け入れ条件・API・テスト方針）は実装直前に `仕様詳細` へ上げて詰める。未決の親論点は [open-questions](../open-questions.md)（とくに P1–P8）を先に潰す。
- 実装完了後は本ファイルを `docs/specs/` の対応ドメインへ**移動**する（plans に残さない）。
- 関連: [roadmap](../../roadmap.md) · [pillar](../../README.md) · [open-questions](../open-questions.md)

----

以上
