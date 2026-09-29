# plans 索引

これからやる内容の索引。版フォルダは [`roadmap.md`](../roadmap.md) の暫定 SemVer に合わせる。  
実装完了後は該当ファイルを `docs/specs/` へ**移動**し、本索引を更新する。

プロダクト親論点は各 Phase plan の決定事項／現行 specs へ落とした。[open-questions.md](./open-questions.md) は**残件なし（クローズ）**。Phase 1・Phase 2 は実装完了し specs へ移動済み（[state](../specs/state/) · [tx](../specs/tx/) · [snapshot](../specs/snapshot/) · [recovery](../specs/recovery/) · [options](../specs/options/)）。Phase 3: [v0.4.0/external-recovery.md](./v0.4.0/external-recovery.md#決定事項)、Phase 4: [v0.5.0/es4-server.md](./v0.5.0/es4-server.md#決定事項)、Phase 5: [v0.6.0/state-backend-extension.md](./v0.6.0/state-backend-extension.md#決定事項)。wishlist ではなく本索引配下のバックログとする。

| 暫定マイルストーン | Phase | 状態 | ファイル |
|--------------------|-------|------|----------|
| `v0.1.0`（暫定） | Phase 0 — スキャフォールド | スキャフォールド完了 | [scaffold.md](./v0.1.0/scaffold.md) |
| `v0.2.0`（暫定） | Phase 1 — 最小構成 | **実装完了 → specs** | [state](../specs/state/) · [snapshot](../specs/snapshot/) · [recovery](../specs/recovery/) · [options](../specs/options/) |
| `v0.3.0`（暫定） | Phase 2 — SQLite State | **実装完了 → specs** | [state](../specs/state/) · [tx](../specs/tx/) · [snapshot](../specs/snapshot/) · [options](../specs/options/) |
| `v0.4.0`（暫定） | Phase 3 — 外部 Recovery Storage | 方針確定 | [external-recovery.md](./v0.4.0/external-recovery.md) |
| `v0.5.0`（暫定） | Phase 4 — Es4 Server | 方針確定 | [es4-server.md](./v0.5.0/es4-server.md) |
| `v0.6.0`（暫定） | Phase 5 — State Backend の拡張 | 方針確定 | [state-backend-extension.md](./v0.6.0/state-backend-extension.md) |
| （横断） | 未決論点 | 残件なし | [open-questions.md](./open-questions.md) |

SemVer が正式決定されるまで版号は provisional。フォルダ名・roadmap 表と同時に改名する。`v0.3.0` の Git タグは本索引の対象外（マージ後にコーディネータが打つ）。

----

以上
