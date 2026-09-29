---
状態: 意図スタブ
マイルストーン: v0.5.0（暫定 / Phase 5）
---

# Phase 5 — State Backend の拡張

## 目的

コアアーキテクチャを特定の State Backend に依存させず、柔軟性を拡張する。

## ざっくり範囲

- その他のインメモリ実装
- その他の組み込みデータベース
- Redis / Valkey Adapter（具体的なユースケースが生じた場合）
- State Backendごとの最適化

## メモ

- SemVer `v0.5.0` は暫定割当。Redis / Valkey Adapter は条件付き（ユースケース発生時）。
- 詳細仕様は後日詰める。実装完了後は本ファイルを `docs/specs/` へ**移動**する。
- 関連: [roadmap](../../roadmap.md)

----

以上
