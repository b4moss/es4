---
状態: 方針確定
マイルストーン: v0.1.0（暫定 / Phase 0）
---

# Phase 0 — スキャフォールド

## 目的

リポジトリの骨格だけを置く。実装コード・仕様・テスト仕様は書かない。

## ざっくり範囲

- 必要なディレクトリツリーを **`.gitkeep` のみ**で追加する
- 正確なパッケージ分割は [open-questions P8](../open-questions.md) 未決のため、以下は **暫定プレースホルダ**とする（公開 API を固定しない）

暫定トップレベル（空ディレクトリ）:

- `internal/state/`
- `internal/snapshot/`
- `internal/recovery/`
- `internal/config/`

既存の `docs/specs/`・`docs/tests/` はそのまま（空プレースホルダでよい）。

## 範囲外

- State / Snapshot / Recovery の振る舞い実装
- `go.mod` や Go / TypeScript のソース追加
- `docs/specs/`・`docs/tests/` への仕様記入

## メモ

- SemVer `v0.1.0` は暫定割当。正式版名が決まり次第フォルダと roadmap を更新する。
- ツリー名は P8 確定後に改名してよい。本マイルストーンは「空ディレクトリが存在する」ことだけを成果とする。
- 関連: [roadmap](../../roadmap.md) · [pillar](../../README.md) · [open-questions](../open-questions.md)（P8）

----

以上
