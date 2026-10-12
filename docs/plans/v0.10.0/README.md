# v0.10.0 — キーデリミタ `:` 化 ＋ Scan

- **状態:** 完了（タグ `v0.10.0` + `packages/go/v0.10.0` + npm `@b4moss/es4@0.10.0`。Issue [#37](https://github.com/b4moss/es4/issues/37) クローズ）
- **マイルストーン:** [v0.10.0](https://github.com/b4moss/es4/milestone/1)
- **詳細（完了注記）:** [key-delimiter.md](./key-delimiter.md)
- **意味論正本:** [specs/state](../../specs/state/) · [specs/es4-server](../../specs/es4-server/) · [specs/tx](../../specs/tx/) · [specs/recovery](../../specs/recovery/)
- **テスト仕様:** [tests/state](../../tests/state/) · [tests/es4-server](../../tests/es4-server/)

## 目的（要約）

1. 論理キー階層区切りを `/` → `:`（破壊的・Redis/Valkey スタイル）
2. 薄い `Scan(prefix) []string`（ページングなし・Tx 内可）— ss-cachian PurgePrefix の前提

## 決定（2026-10-11・要約）

- HTTP: `/v1/keys/a:b:c`（案 A）
- `/` は通常文字として許可
- Scan を同 Milestone に含める（API B）
- 移行: 短い「捨てて再投入」／Recovery 旧キーは削除＋強い注意喚起

## 依存する下流

- [ss-cachian es4 Layer Driver（v0.11.0）](https://github.com/b4moss/ss-cachian/blob/main/docs/plans/v0.11.0/es4-layer-driver.md) — リリース済み
