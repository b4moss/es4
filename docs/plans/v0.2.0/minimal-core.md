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

## 決定事項

PO Q&A で確定した Phase 1（`v0.2.0` 暫定）の事実。実装詳細（受け入れ条件・エラー表現・テスト方針など）は `仕様詳細` へ上げるときに詰める。

### State API

- **操作:** `SET` / `GET` / `DELETE` / `EXISTS` / `CLEAR`
- **キー:** 区切り文字 `/` による階層文字列。Phase 1 では scan / `LIST` は持たない
- **値:** 任意の JSON ドキュメント
- **呼び出し:** Phase 1 から非同期志向（channels / Future 風）。同期前提にはしない

### 公開面と Adapter

- **公開面:** State API のみ。Snapshot と Recovery は内部に閉じる
- **差し替え:** Phase 1 時点で State Adapter・Recovery Adapter の両方を差し替え可能にする

### 設定

- **正本:** Options（ライブラリへの渡し方のソース・オブ・トゥルース）
- **任意:** 設定ファイルも読める（Options を補う入力。正本は Options）
- **概念名と既定:** ライブラリ Options と、後の Server 環境変数は**同じ概念名・同じ既定**を共有する（Phase 4 の命名を拘束する）
- **Recovery ファイルパス:** Options で指定する
- **Memory-only:** Recovery 関連設定（パス・間隔など）は無視して続行する（設定エラーにはしない）

### Snapshot / Recovery

- **Snapshot 形式:** 版付きの共通封筒＋ Backend 固有ペイロード
- **後方互換:** `v0` 期間は形式の後方互換を約束しない
- **ファイル Recovery（Phase 1）:** 単一ファイルの上書き
- **起動時:** Recovery が無い／読めない場合は空 State で続行する
- **Ready:** Restore 完了前でも State API を受け付ける
- **明示 Snapshot:** 取得すると定期 Snapshot の間隔タイマーをリセットする

### 並行

- **Phase 1:** 同一プロセス内の呼び出しは内部ロックで直列化する

## メモ

- SemVer `v0.2.0` は暫定割当。正式版名が決まり次第フォルダと roadmap を更新する。
- 詳細仕様（受け入れ条件・API・テスト方針）は実装直前に `仕様詳細` へ上げて詰める。残る未決は [open-questions](../open-questions.md)（設定キー一覧・既定の細部、明示 Snapshot の公開 API 扱い、クラッシュ一貫性の幅、Phase 2 以降など）。
- 実装完了後は本ファイルを `docs/specs/` の対応ドメインへ**移動**する（plans に残さない）。
- 関連: [roadmap](../../roadmap.md) · [pillar](../../README.md) · [open-questions](../open-questions.md)

----

以上
