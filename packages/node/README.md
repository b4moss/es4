# @b4moss/es4 (Node.js / TypeScript)

TypeScript port of Es4 Go **v0.8.0** contracts. Options keys stay **snake_case** (`state_path`, `ES4_*` env). Snapshot／Recovery remain internal.

## Requirements

- Node.js **20+**
- TypeScript strict (built with `tsc`)

## Install (workspace)

```bash
cd packages/node
npm install
```

Package is **private** (not published to npm in v0.9.0).

## Quick start

```ts
import { open, defaults } from "@b4moss/es4";

const db = await open({ ...defaults(), memory_only: true });
await db.set("hello", JSON.stringify({ n: 1 }));
console.log(await db.get("hello"));
await db.close();
```

## Scripts

| Script | Purpose |
|--------|---------|
| `npm test` | Unit / contract (Vitest) |
| `npm run lint` | `tsc --noEmit` |
| `npm run build` | Emit `dist/` |
| `npm run test:e2e:memory` | E2E memory layer |
| `npm run test:e2e:file` | E2E file SQLite + File Recovery |
| `npm run test:e2e:libsql` | E2E libSQL Recovery |
| `npm run test:e2e:object` | E2E Object Recovery (needs RustFS) |
| `npm run test:e2e:redis` | E2E Redis State (`ES4_E2E_REDIS_URL`) |
| `npm run test:e2e:valkey` | E2E Valkey State (`ES4_E2E_VALKEY_URL`) |
| `npm run test:e2e:firestore` | E2E Firestore (`FIRESTORE_EMULATOR_HOST`) |

Compose services: `docker compose -f docker/e2e/docker-compose.yml up -d …` (shared with Go).

## Open priority

1. Injected State (`openWith`)  
2. `memory_only` → Memory  
3. `state_backend=redis|valkey` → Redis adapter  
4. `state_backend=firestore` → Firestore  
5. `state_backend=memory` → Memory  
6. `state_path` non-empty or `state_backend=sqlite` → SQLite  
7. else → Memory  

## Docs

- Plan: [`docs/plans/v0.9.0/`](../../docs/plans/v0.9.0/)
- Specs (language-neutral): [`docs/specs/`](../../docs/specs/)
