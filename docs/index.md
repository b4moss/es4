---
okf_version: "0.1"
---

# Docs Hub（OKF v0.1）

本 `docs/` は **OKF v0.1**（Org Knowledge Framework）のプロダクト知識バンドルである。版は本ファイルのフロントマター `okf_version` で示す。

本ファイル（`index.md`）は **OKF 索引のみ**。プロダクトの目的・スコープ等の pillar 正本は [README.md](README.md)。

* [README.md](README.md) - プロダクト目的・スコープ・技術方針の pillar（正）
* [OKF v0.1 とは](charter/okf/) - 定義・最小ツリー・硬いルール・執筆サンプル
* [憲章（charter）](charter/) - 開発方針の最上位取り決めと個別ルール
* [このプロジェクト独自のルール](override-charter.md) - 憲章をオーバーライドする範囲（現状は設定なし）

# Project Docs

* [roadmap.md](roadmap.md) - マイルストーン一覧（Phase 0–7 / 暫定 SemVer `v0.1.0`–`v0.8.0`）
* [wishlist.md](wishlist.md) - PO メモ（未整理）
* [plans](plans/) - これからやる内容
* [plans/v0.1.0/scaffold.md](plans/v0.1.0/scaffold.md) - Phase 0 スキャフォールド（完了・決定事項含む）
* [plans/v0.6.0/state-backend-extension.md](plans/v0.6.0/state-backend-extension.md) - Phase 5 State Backend 拡張（実装完了・決定事項／完了注記・Git タグ済み）
* [plans/v0.7.0/e2e-object-recovery.md](plans/v0.7.0/e2e-object-recovery.md) - Phase 6 E2E Object Recovery × RustFS（実装完了・完了注記）
* [plans/v0.7.1/e2e-three-layer.md](plans/v0.7.1/e2e-three-layer.md) - Phase 6 追補 E2E 三層（File／Memory・完了注記）
* [plans/v0.7.2/github-actions.md](plans/v0.7.2/github-actions.md) - Phase 6 追補 GitHub Actions スイート（実装完了・完了注記）
* [plans/v0.7.3/e2e-libsql.md](plans/v0.7.3/e2e-libsql.md) - Phase 6 追補 E2E libSQL Recovery（File／InMemory・完了注記）
* [plans/v0.8.0/redis-valkey-state.md](plans/v0.8.0/redis-valkey-state.md) - Phase 7 Redis／Valkey State Backend（実装中・#29）
* [plans/v0.8.0/firestore-state.md](plans/v0.8.0/firestore-state.md) - Phase 7 Firestore State Backend（実装中・#29 と共存）
* [plans/open-questions.md](plans/open-questions.md) - 未決プロダクト論点（残件なし）
* [specs](specs/) - 現行機能の仕様正本
* [specs/options](specs/options/) - Options（設定）
* [specs/state](specs/state/) - State API（Adapter 境界・Export／Replace・Phase 5）
* [specs/tx](specs/tx/) - Tx API
* [specs/snapshot](specs/snapshot/) - Snapshot（内部）
* [specs/recovery](specs/recovery/) - Recovery（File / libSQL / Object・世代 TTL）
* [specs/es4-server](specs/es4-server/) - Es4 Server（HTTP・Docker・Health）
* [tests](tests/) - テスト仕様
* [tests/options](tests/options/) · [tests/state](tests/state/) · [tests/tx](tests/tx/) · [tests/snapshot](tests/snapshot/) · [tests/recovery](tests/recovery/) · [tests/es4-server](tests/es4-server/) · [tests/e2e](tests/e2e/)
