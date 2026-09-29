# CI policy (es4)

- **Unit CI** (`.github/workflows/ci.yml`): `go vet` + `go test ./...` on PRs into `main` / `develop` / `dev-v*`. Coverage + Codecov only on push to `main`. No `-tags=e2e`.
- **Product E2E** (`.github/workflows/e2e.yml`, `e2e-object-recovery.yml`): `workflow_dispatch` only. Do not add push / pull_request / schedule triggers.
- **CodeQL**: PR to `main` only; `upload: never` while the repo is private without GHAS.
- **Docker**: PR build of `docker/Dockerfile` (context repo root); does not push.
- **Publish / release**: tag-driven (`release-on-tag`, `publish-go`). Do not cut tags from CI PRs.
- **Dependabot**: `.github/dependabot.yml` — `github-actions` + `gomod` only (weekly Monday; ignore semver-major; grouped minor/patch).
- **Codecov**: root `codecov.yml` — informational project/patch; E2E paths ignored.
