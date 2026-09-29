---
状態: 方針確定
マイルストーン: v0.5.0（暫定 / Phase 4）
---

# Phase 4 — Es4 Server

## 目的

複数のアプリケーション・インスタンスから共有できる State Store として利用可能にする。

## ざっくり範囲

- HTTP API（REST 風・キーをパスに置く）
- Dockerイメージ
- 環境変数による設定（`ES4_` + SCREAMING_SNAKE）
- Health / Readiness Endpoint
- 外部 Recovery Storage
- Cloud Runへの対応

## 決定事項

PO Q&A で確定した Phase 4（`v0.5.0` 暫定）の事実。実装詳細は `仕様詳細` へ上げるときに詰める。

### HTTP API

- **形:** REST 風。キーはパスに置く
- ライブラリ State API（Phase 1）および Tx API（Phase 2・別面）への写像。メソッド／エラー表現の詳細は `仕様詳細` で詰める

### 環境変数

- **接頭辞:** `ES4_`
- **形:** SCREAMING_SNAKE（例: Options の `snapshot_interval` → `ES4_SNAPSHOT_INTERVAL`）
- 概念名・既定はライブラリ Options と共有（[Phase 1 決定事項](../../specs/state/)）

### Health / Readiness

- **Liveness:** 常に OK
- **Readiness:** Restore Ready になってから OK（それ以前は fail）
- ライブラリ組み込みの「Restore 完了前でも State API を受け付ける」（Phase 1）とは層が違う。Server のオーケストレータ向け Ready はこの決定に従う

### Cloud Run

- **推奨:** 外部 Recovery（Phase 3 成果物）
- **許容:** ファイル Recovery も引き続き使える（必須ではない）

## メモ

- SemVer `v0.5.0` は暫定割当。詳細仕様は後日詰める。
- 実装完了後は本ファイルを `docs/specs/` へ**移動**する。
- 関連: [roadmap](../../roadmap.md) · [Phase 1 決定事項](../../specs/state/) · [Phase 3 Recovery](../../specs/recovery/) · [open-questions](../open-questions.md)（残件なし）

----

以上
