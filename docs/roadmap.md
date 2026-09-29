# Roadmap

SemVer・マイルストーン一覧のハブ。詳細な作業内容の正本は [`plans/`](./plans/) 側。  
ここに書く Phase / 版号は **暫定**（PO が SemVer を正式決定するまでの仮割当）。

`v0.1.0`（スキャフォールド）は完了し、最初の Git タグ `v0.1.0` はコーディネータが main 上で打つ。Phase 1（`v0.2.0`）以降はこれからやる計画であり、現行仕様（`specs/`）ではない。

| 暫定版 | Phase | 状態 | 短い目標 | 詳細 |
|--------|-------|------|----------|------|
| `v0.1.0`（暫定） | Phase 0 — スキャフォールド | スキャフォールド完了／タグ付け待ち | リポジトリ骨格（空ディレクトリ + `.gitkeep`）のみを置く | [plans/v0.1.0](./plans/v0.1.0/) |
| `v0.2.0`（暫定） | Phase 1 — 最小構成 | 計画中 | State → Snapshot → Recovery の基本ライフサイクルを確立する | [plans/v0.2.0](./plans/v0.2.0/) |
| `v0.3.0`（暫定） | Phase 2 — SQLite State | 計画中 | 構造化された State と、より高頻度な State 操作に対応する | [plans/v0.3.0](./plans/v0.3.0/) |
| `v0.4.0`（暫定） | Phase 3 — 外部 Recovery Storage | 計画中 | Recovery Storage をローカル FS から切り離し、外部ストレージへ拡張する | [plans/v0.4.0](./plans/v0.4.0/) |
| `v0.5.0`（暫定） | Phase 4 — Es4 Server | 計画中 | 複数アプリ／インスタンスから共有できる State Store として利用可能にする | [plans/v0.5.0](./plans/v0.5.0/) |
| `v0.6.0`（暫定） | Phase 5 — State Backend の拡張 | 計画中 | コアを特定 Backend に依存させず、柔軟性を拡張する | [plans/v0.6.0](./plans/v0.6.0/) |

版号の付け方は [versioning-rule](./charter/versioning-rule.md) に従う。`v0.n.0` は正式リリース前のため破壊的変更を許容する。正式な版名が決まったら本表と `plans/` フォルダ名を揃えて更新する。最初の Git タグはスキャフォールド完了を条件に `v0.1.0`（[scaffold 決定事項](./plans/v0.1.0/scaffold.md#決定事項)）。

未決のプロダクト論点は優先度順に [plans/open-questions.md](./plans/open-questions.md) へ集約する（現状は Phase 3 以降の固有項目）。Phase 1 の事実は [plans/v0.2.0/minimal-core.md](./plans/v0.2.0/minimal-core.md#決定事項)、Phase 2 は [plans/v0.3.0/sqlite-state.md](./plans/v0.3.0/sqlite-state.md#決定事項)、Phase 3 Adapter 順は [plans/v0.4.0/external-recovery.md](./plans/v0.4.0/external-recovery.md#決定事項)。スキャフォールド／module path／最初のタグ方針は [plans/v0.1.0/scaffold.md](./plans/v0.1.0/scaffold.md#決定事項)。

関連: [pillar](./README.md) · [plans 索引](./plans/README.md) · [Phase 0 決定事項](./plans/v0.1.0/scaffold.md#決定事項) · [Phase 1 決定事項](./plans/v0.2.0/minimal-core.md#決定事項) · [Phase 2 決定事項](./plans/v0.3.0/sqlite-state.md#決定事項) · [open-questions](./plans/open-questions.md)

----

以上
