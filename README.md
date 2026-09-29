# Es4: A Server-side State Storage

Redis/Valkey ほどではないが、インメモリで揮発しないサーバーサイドステートを持ちたい——そのための State Store。組み込みライブラリとしても単独サーバーとしても使えるよう、State 層と Recovery 層を Adapter 化する構想。

初期実装は Go。Node.js（TypeScript）は Go 版のあと。

プロダクト知識の正本は **[`docs/`](./docs/)**（OKF v0.1）:

- [docs/README.md](./docs/README.md) — pillar（目的・設計・アーキテクチャ / ライフサイクル図・利用想定）
- [docs/roadmap.md](./docs/roadmap.md) — Phase 0–6 ロードマップ（暫定 SemVer `v0.1.0`–`v0.7.0`）
- [docs/plans/](./docs/plans/) — これからやる内容
- [docs/index.md](./docs/index.md) — OKF 索引
