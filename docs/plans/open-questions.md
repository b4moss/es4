---
状態: 残件なし（クローズ）
マイルストーン: 横断
---

# Open Questions（未決プロダクト論点）

**現状: 残件なし。** PO Q&A で Phase 0–5 の親論点はすべて各 Phase plan の決定事項へ落とした。

新規の未決が出たら本ファイルに優先度順で戻す。確定事実はここに書かず、該当 `plans/` の決定事項を正とする。

## 正本（決定事項）

| Phase | 暫定 SemVer | 決定事項 |
|-------|-------------|----------|
| Phase 0 — スキャフォールド | `v0.1.0` | [scaffold.md](./v0.1.0/scaffold.md#決定事項) |
| Phase 1 — 最小構成 | `v0.2.0` | [state](../specs/state/) · [snapshot](../specs/snapshot/) · [recovery](../specs/recovery/) · [options](../specs/options/) |
| Phase 2 — SQLite State | `v0.3.0` | [sqlite-state.md](./v0.3.0/sqlite-state.md#決定事項) |
| Phase 3 — 外部 Recovery | `v0.4.0` | [external-recovery.md](./v0.4.0/external-recovery.md#決定事項) |
| Phase 4 — Es4 Server | `v0.5.0` | [es4-server.md](./v0.5.0/es4-server.md#決定事項) |
| Phase 5 — State Backend 拡張 | `v0.6.0` | [state-backend-extension.md](./v0.6.0/state-backend-extension.md#決定事項) |

直近で閉じた Phase 3–5 の論点（世代 TTL、GCS＝S3 互換 Adapter、HTTP REST・パスキー、`ES4_`＋SCREAMING_SNAKE、Liveness／Readiness、Cloud Run、その他／Redis・Valkey Unscheduled、最適化境界）の詳細は上表の各決定事項を参照。

## 意図的にここに書かないもの

- すでに pillar / roadmap / 各 Phase plans に**方針または決定事項として書いてある範囲**
- 詳細仕様の「後日詰め」（各 `plans/vX.Y.Z` を `仕様詳細` へ上げる作業）
- wishlist 向けの雑多メモ（現時点で wishlist に残件なし）

## 関連

- [pillar](../README.md) · [roadmap](../roadmap.md) · [plans 索引](./README.md)
- Phase plans / specs: [v0.1.0](./v0.1.0/scaffold.md) · [v0.2.0 specs](../specs/state/) · [v0.3.0](./v0.3.0/sqlite-state.md) · [v0.4.0](./v0.4.0/external-recovery.md) · [v0.5.0](./v0.5.0/es4-server.md) · [v0.6.0](./v0.6.0/state-backend-extension.md)

----

以上
