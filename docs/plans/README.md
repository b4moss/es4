# plans 索引

これからやる内容の索引。版フォルダは [`roadmap.md`](../roadmap.md) の暫定 SemVer に合わせる。  
実装完了後は該当ファイルを `docs/specs/` へ**移動**し、本索引を更新する。

プロダクト親論点は各 Phase plan の決定事項／現行 specs へ落とした。[open-questions.md](./open-questions.md) は**残件なし（クローズ）**。Phase 1–5 は実装完了（[state](../specs/state/) · [tx](../specs/tx/) · [snapshot](../specs/snapshot/) · [recovery](../specs/recovery/) · [options](../specs/options/) · [es4-server](../specs/es4-server/)）。Phase 5 の Adapter 境界は specs/state へ反映済み（[完了注記](./v0.6.0/state-backend-extension.md)）。Phase 6 は E2E（[tests/e2e](../tests/e2e/) · [v0.7.0](./v0.7.0/e2e-object-recovery.md) · [v0.7.1](./v0.7.1/e2e-three-layer.md)）と Actions（[v0.7.2](./v0.7.2/github-actions.md)）。wishlist ではなく本索引配下のバックログとする。

| 暫定マイルストーン | Phase | 状態 | ファイル |
|--------------------|-------|------|----------|
| `v0.1.0`（暫定） | Phase 0 — スキャフォールド | スキャフォールド完了 | [scaffold.md](./v0.1.0/scaffold.md) |
| `v0.2.0`（暫定） | Phase 1 — 最小構成 | **実装完了 → specs** | [state](../specs/state/) · [snapshot](../specs/snapshot/) · [recovery](../specs/recovery/) · [options](../specs/options/) |
| `v0.3.0`（暫定） | Phase 2 — SQLite State | **実装完了 → specs** | [state](../specs/state/) · [tx](../specs/tx/) · [snapshot](../specs/snapshot/) · [options](../specs/options/) |
| `v0.4.0`（暫定） | Phase 3 — 外部 Recovery Storage | **実装完了 → specs**（Git タグ `v0.4.0` 済み） | [recovery](../specs/recovery/) · [options](../specs/options/) |
| `v0.5.0`（暫定） | Phase 4 — Es4 Server | **実装完了 → specs**（Git タグ `v0.5.0` 済み） | [es4-server](../specs/es4-server/) |
| `v0.6.0`（暫定） | Phase 5 — State Backend の拡張 | **実装完了 → specs/state**（Git タグ `v0.6.0` 済み） | [state](../specs/state/) · [完了注記](./v0.6.0/state-backend-extension.md) |
| `v0.7.0`（暫定） | Phase 6 — E2E Object Recovery × RustFS | **実装完了**（E2E 仕様・Compose・`workflow_dispatch`。Git タグは未打） | [tests/e2e](../tests/e2e/) · [完了注記](./v0.7.0/e2e-object-recovery.md) |
| `v0.7.1`（暫定） | Phase 6 追補 — E2E 三層 | **実装完了**（File SQLite／In-memory・docs・`e2e.yml` layer 入力。Git タグは未打） | [tests/e2e](../tests/e2e/) · [完了注記](./v0.7.1/e2e-three-layer.md) |
| `v0.7.2`（暫定） | Phase 6 追補 — GitHub Actions スイート | **実装完了**（CI／CodeQL／Scorecard／release／publish-go／docker。Git タグは未打） | [完了注記](./v0.7.2/github-actions.md) · [CI.md](../../.github/CI.md) |
| （横断） | 未決論点 | 残件なし | [open-questions.md](./open-questions.md) |

SemVer が正式決定されるまで版号は provisional。フォルダ名・roadmap 表と同時に改名する。`v0.7.0`／`v0.7.1`／`v0.7.2` の Git タグは本索引の対象外（マージ後にコーディネータが打つ）。

----

以上
