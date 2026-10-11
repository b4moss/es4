# v0.10.0 — キーデリミタ `:` 化 ＋ Scan

- **状態:** 決定事項反映済み（2026-10-11）・実装未着手
- **マイルストーン:** [v0.10.0](https://github.com/b4moss/es4/milestone/1)（現行リリース v0.9.0 の次 minor）
- **詳細:** [key-delimiter.md](./key-delimiter.md)

## 目的（要約）

1. 論理キー階層区切りを `/` → `:`（破壊的・Redis/Valkey スタイル）
2. 薄い `Scan(prefix) []string`（ページングなし・Tx 内可）— ss-cachian PurgePrefix の前提

## 決定（2026-10-11・要約）

- HTTP: `/v1/keys/a:b:c`（案 A）
- `/` は通常文字として許可
- Scan を同 Milestone に含める（API B）
- 移行: 短い「捨てて再投入」／Recovery 旧キーは削除＋強い注意喚起

## 依存する下流

- [ss-cachian es4 Layer Driver（v0.11.0）](https://github.com/b4moss/ss-cachian/blob/develop/docs/plans/v0.11.0/es4-layer-driver.md)
