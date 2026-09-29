# v0.9.0 Node port — completion notes (draft)

実装: `packages/node`（`@b4moss/es4` **v0.9.0**）  
正本契約: Go tag `v0.8.0`

## Included

- Options (snake_case, Defaults→YAML→`ES4_*`, Validate/Effective/memory_only)
- State: Memory · SQLite · Redis/Valkey · Firestore
- Tx · Open priority · internal Snapshot/Recovery (file/libsql/object + TTL)
- HTTP Es4 Server (`src/server`)
- Unit/contract Vitest + E2E layers (object/file/memory/libsql/redis/valkey/firestore)
- CI job `packages/node`; `e2e.yml` `runtime: go|node|both`

## Run

```bash
cd packages/node
npm ci
npm test
npm run test:e2e:memory   # etc.
```

SemVer tag `v0.9.0` marks the Node port commit. npm publish uses `.github/workflows/publish-npm.yml` (Trusted Publisher on `release`; one-shot first create via `workflow_dispatch` + `NPM_TOKEN`).
