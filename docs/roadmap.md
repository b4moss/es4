# Roadmap

SemVer・マイルストーン一覧のハブ。詳細な作業内容の正本は [`plans/`](./plans/) 側。  
ここに書く Phase / 版号は **暫定**（PO が SemVer を正式決定するまでの仮割当）。

| 暫定版 | Phase | 状態 | 短い目標 | 詳細 |
|--------|-------|------|----------|------|
| `v0.1.0`（暫定） | Phase 0 — スキャフォールド | スキャフォールド完了 | リポジトリ骨格（空ディレクトリ + `.gitkeep`）のみを置く | [plans/v0.1.0](./plans/v0.1.0/) |
| `v0.2.0`（暫定） | Phase 1 — 最小構成 | **実装完了**（specs へ移動済み。Git タグ `v0.2.0` 済み） | State → Snapshot → Recovery の基本ライフサイクル | [specs/state](./specs/state/) · [snapshot](./specs/snapshot/) · [recovery](./specs/recovery/) · [options](./specs/options/) |
| `v0.3.0`（暫定） | Phase 2 — SQLite State | **実装完了**（specs へ移動済み。Git タグ `v0.3.0` 済み） | オンディスク SQLite State・Tx API・論理 entries Snapshot | [specs/state](./specs/state/) · [tx](./specs/tx/) · [snapshot](./specs/snapshot/) · [options](./specs/options/) |
| `v0.4.0`（暫定） | Phase 3 — 外部 Recovery Storage | **実装完了**（specs へ移動済み。Git タグ `v0.4.0` 済み） | libSQL / S3互換 Object・世代 TTL | [specs/recovery](./specs/recovery/) · [options](./specs/options/) |
| `v0.5.0`（暫定） | Phase 4 — Es4 Server | **実装完了**（specs へ移動済み。Git タグ `v0.5.0` 済み） | HTTP Es4 Server・Docker・Health（末尾 `/` なし） | [specs/es4-server](./specs/es4-server/) |
| `v0.6.0`（暫定） | Phase 5 — State Backend の拡張 | **実装完了**（Adapter 境界は specs/state へ反映。Git タグ `v0.6.0` 済み） | コアを特定 Backend に依存させず Adapter 境界を正本化 | [specs/state](./specs/state/) · [plans/v0.6.0](./plans/v0.6.0/) |
| `v0.7.0`（暫定） | Phase 6 — E2E Object Recovery × RustFS | **実装完了**（E2E 仕様・Compose・`workflow_dispatch`。Git タグは未打） | RustFS 上で Snapshot Save → 再起動 Restore を自動化 | [tests/e2e](./tests/e2e/) · [plans/v0.7.0](./plans/v0.7.0/) |
| `v0.7.1`（暫定） | Phase 6 追補 — E2E 三層 | **実装完了**（File SQLite／In-memory・`e2e.yml` layer。Git タグは未打） | 共通カタログに沿い File／Memory 層を追加（S3 §5 は維持） | [tests/e2e](./tests/e2e/) · [plans/v0.7.1](./plans/v0.7.1/) |
| `v0.7.2`（暫定） | Phase 6 追補 — GitHub Actions スイート | **実装完了**（CI／CodeQL／Scorecard／release／publish-go／docker。Git タグは未打） | B4MOSS 兄弟に揃えた標準 Actions を追加（E2E 手動は維持） | [plans/v0.7.2](./plans/v0.7.2/) · [.github/CI.md](../.github/CI.md) |
| `v0.7.3`（暫定） | Phase 6 追補 — E2E libSQL Recovery | **実装完了**（File／InMemory・docs・`e2e.yml` layer=libsql。Git タグは未打） | 共通カタログに沿い libSQL File（C1–C5）／Memory（C2／C4／C5）を追加 | [tests/e2e](./tests/e2e/) · [plans/v0.7.3](./plans/v0.7.3/) |
| `v0.8.0`（暫定） | Phase 7 — 外部サービスの充実（State Backend） | **実装中**（Redis／Valkey #29 マージ済み＋Firestore State + E2E。両 Backend 共存） | Redis／Valkey（同一アダプタ）と Firestore を State Backend として追加 | [plans/v0.8.0](./plans/v0.8.0/) · [specs/state](./specs/state/) · [tests/e2e](./tests/e2e/) |

版号の付け方は [versioning-rule](./charter/versioning-rule.md) に従う。`v0.n.0` は正式リリース前のため破壊的変更を許容する。正式な版名が決まったら本表と `plans/` フォルダ名を揃えて更新する。

プロダクト親論点は各 Phase の決定事項／現行 specs へ落とした。[plans/open-questions.md](./plans/open-questions.md) は**残件なし**。Phase 0: [scaffold](./plans/v0.1.0/scaffold.md#決定事項)、Phase 1: [state](./specs/state/) · [snapshot](./specs/snapshot/) · [recovery](./specs/recovery/) · [options](./specs/options/)、Phase 2: [state](./specs/state/) · [tx](./specs/tx/) · [options](./specs/options/)、Phase 3: [recovery](./specs/recovery/) · [options](./specs/options/)、Phase 4: [es4-server](./specs/es4-server/)、Phase 5: [state](./specs/state/)（[完了注記](./plans/v0.6.0/state-backend-extension.md)）、Phase 6: [e2e](./tests/e2e/)（[v0.7.0](./plans/v0.7.0/e2e-object-recovery.md) · [v0.7.1](./plans/v0.7.1/e2e-three-layer.md) · [v0.7.3](./plans/v0.7.3/e2e-libsql.md)）· Actions（[v0.7.2](./plans/v0.7.2/github-actions.md)）、Phase 7: [v0.8.0](./plans/v0.8.0/)（[redis-valkey-state](./plans/v0.8.0/redis-valkey-state.md) · [firestore-state](./plans/v0.8.0/firestore-state.md)。両 Backend 共存）。

関連: [pillar](./README.md) · [plans 索引](./plans/README.md) · [open-questions](./plans/open-questions.md)

----

以上
