---
状態: 実装完了（GitHub Actions スイート追加）
マイルストーン: v0.7.2（暫定 / Phase 6 追補 — Actions）
---

# Phase 6 追補 — GitHub Actions スイート

## 目的

B4MOSS 兄弟リポジトリ（median / crudian / d4run）に揃えた **標準 Actions 一式** を追加する。  
既存の E2E（`workflow_dispatch` のみ）は変更しない。

## 範囲

| ファイル | 役割 |
|----------|------|
| `.github/workflows/ci.yml` | Go vet + unit test（`-tags=e2e` なし）。main push 時は coverage → Codecov |
| `.github/workflows/codeql.yml` | PR → main。languages: actions + go。`upload: never`（private） |
| `.github/workflows/scorecard.yml` | weekly + main push + `branch_protection_rule` |
| `.github/workflows/release-on-tag.yml` | `v[0-9]*` / `packages/go/v[0-9]*` で Release 作成（既存なら skip） |
| `.github/workflows/publish-go.yml` | Go module 公開（tag / `release` ブランチ） |
| `.github/workflows/docker-image.yml` | PR で `docker/Dockerfile` を build（push しない） |
| `.github/scripts/should-publish-go.sh` | publish-go の skip / tag 判定 |
| `packages/go/VERSION` | `0.7.1`（最終リリース済み SemVer に一致。本マイルストーンではタグを切らない） |
| `.github/CI.md` | 単位 CI / E2E 手動の方針メモ |
| `.github/dependabot.yml` | github-actions + gomod のみ（週次・major 除外・group） |
| `codecov.yml` | informational coverage；E2E パスを ignore |

## 非対象

- `e2e.yml` / `e2e-object-recovery.yml` のトリガ・挙動変更
- npm / PHP / Bun publish、goreleaser / Homebrew
- SemVer タグ／GitHub Release の作成（本 PR では打たない）

## 参照マッピング

| New workflow / file | Purpose | Primary reference |
|---------------------|---------|-------------------|
| `ci.yml` | Go vet + unit tests; Codecov on main push | b4moss/median `ci.yml`（Go job）; setup-go は既存 `e2e.yml` |
| `codeql.yml` | PR → main; actions + go; `upload: never` | b4moss/median `codeql.yml` + b4moss/crudian matrix |
| `scorecard.yml` | weekly + main + branch_protection_rule | b4moss/median / crudian / d4run `scorecard.yml` |
| `release-on-tag.yml` | Release on `v*` / `packages/go/v*` | b4moss/d4run + median title logic |
| `publish-go.yml` | Publish Go module + proxy ping | b4moss/median / crudian `publish-go.yml` |
| `should-publish-go.sh` | Decide skip/tag/version | b4moss/median / crudian script（module path → es4） |
| `docker-image.yml` | PR build es4-server image (no push) | b4moss/crudian `docker-image.yml`（簡略・repo-root context） |
| `packages/go/VERSION` | `0.7.1` for publish-go gate | median/crudian VERSION 慣習 |
| `dependabot.yml` | Weekly Actions + Go module updates (no npm/composer) | b4moss/median `dependabot.yml`（Go-only subset） |
| `codecov.yml` | Informational coverage; ignore e2e paths | b4moss/d4run `codecov.yml` |
| `CI.md` | Unit CI vs manual E2E policy | b4moss/crudian / median CI.md 方針 |

## 決定事項

- CodeQL は private のため `upload: never`（GHAS 導入まで）
- 単位 CI に E2E を混ぜない（`-tags=e2e` 禁止）
- `packages/go/VERSION` は `0.7.1`（Actions-only マイルストーンのため `0.7.2` にしない）
- Docker は `docker build -f docker/Dockerfile .` と同じ context／file

## 完了注記

- SemVer タグ `v0.7.2` は本 PR では打たない（マージ後にコーディネータが打つ）
- 関連: [roadmap](../../roadmap.md) · [plans 索引](../README.md) · [CI.md](../../../.github/CI.md) · [v0.7.1](../v0.7.1/e2e-three-layer.md)

----

以上
