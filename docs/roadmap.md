# Roadmap

SemVer・マイルストーン一覧のハブ。詳細な作業内容の正本は [`plans/`](./plans/) 側。  
ここに書く Phase / 版号は **暫定**（PO が SemVer を正式決定するまでの仮割当）。

| 暫定版 | Phase | 状態 | 短い目標 | 詳細 |
|--------|-------|------|----------|------|
| `v0.1.0`（暫定） | Phase 0 — スキャフォールド | スキャフォールド完了 | リポジトリ骨格（空ディレクトリ + `.gitkeep`）のみを置く | [plans/v0.1.0](./plans/v0.1.0/) |
| `v0.2.0`（暫定） | Phase 1 — 最小構成 | **実装完了**（specs へ移動済み。Git タグ `v0.2.0` 済み） | State → Snapshot → Recovery の基本ライフサイクル | [specs/state](./specs/state/) · [snapshot](./specs/snapshot/) · [recovery](./specs/recovery/) · [options](./specs/options/) |
| `v0.3.0`（暫定） | Phase 2 — SQLite State | **実装完了**（specs へ移動済み。Git タグ `v0.3.0` 済み） | オンディスク SQLite State・Tx API・論理 entries Snapshot | [specs/state](./specs/state/) · [tx](./specs/tx/) · [snapshot](./specs/snapshot/) · [options](./specs/options/) |
| `v0.4.0`（暫定） | Phase 3 — 外部 Recovery Storage | **実装完了**（specs へ移動済み。Git タグは未打） | libSQL / S3互換 Object・世代 TTL | [specs/recovery](./specs/recovery/) · [options](./specs/options/) |
| `v0.5.0`（暫定） | Phase 4 — Es4 Server | 計画中 | 複数アプリ／インスタンスから共有できる State Store として利用可能にする | [plans/v0.5.0](./plans/v0.5.0/) |
| `v0.6.0`（暫定） | Phase 5 — State Backend の拡張 | 計画中 | コアを特定 Backend に依存させず、柔軟性を拡張する | [plans/v0.6.0](./plans/v0.6.0/) |

版号の付け方は [versioning-rule](./charter/versioning-rule.md) に従う。`v0.n.0` は正式リリース前のため破壊的変更を許容する。正式な版名が決まったら本表と `plans/` フォルダ名を揃えて更新する。

プロダクト親論点は各 Phase の決定事項／現行 specs へ落とした。[plans/open-questions.md](./plans/open-questions.md) は**残件なし**。Phase 0: [scaffold](./plans/v0.1.0/scaffold.md#決定事項)、Phase 1: [state](./specs/state/) · [snapshot](./specs/snapshot/) · [recovery](./specs/recovery/) · [options](./specs/options/)、Phase 2: [state](./specs/state/) · [tx](./specs/tx/) · [options](./specs/options/)、Phase 3: [recovery](./specs/recovery/) · [options](./specs/options/)、Phase 4: [es4-server](./plans/v0.5.0/es4-server.md#決定事項)、Phase 5: [state-backend-extension](./plans/v0.6.0/state-backend-extension.md#決定事項)。

関連: [pillar](./README.md) · [plans 索引](./plans/README.md) · [open-questions](./plans/open-questions.md)

----

以上
