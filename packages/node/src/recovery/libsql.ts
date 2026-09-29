import Database from "better-sqlite3";
import { mkdirSync } from "node:fs";
import { dirname } from "node:path";
import {
  assertNotAborted,
  genID,
  keepGeneration,
  notFound,
  type RecoveryStore,
} from "./types.js";

const SCHEMA = `
CREATE TABLE IF NOT EXISTS es4_recovery (
  gen_id TEXT PRIMARY KEY NOT NULL,
  created_at INTEGER NOT NULL,
  payload BLOB NOT NULL
);
`;

export class LibSQLRecovery implements RecoveryStore {
  private constructor(
    private readonly db: Database.Database,
    readonly ttlMs: number,
    private readonly owns: boolean,
    private readonly clock: () => Date = () => new Date(),
  ) {}

  static open(dsn: string, _authToken = "", ttlMs = 0): LibSQLRecovery {
    if (dsn === "") throw new Error("recovery: empty libsql url");
    if (
      dsn.startsWith("libsql://") ||
      dsn.startsWith("https://") ||
      dsn.startsWith("http://")
    ) {
      throw new Error(
        "recovery: remote libsql URL not supported in Node v0.9.0 local driver; use file: or path",
      );
    }
    let path = dsn;
    if (dsn.startsWith("file:")) {
      path = dsn.slice("file:".length);
      const q = path.indexOf("?");
      if (q >= 0) path = path.slice(0, q);
    }
    if (path !== ":memory:") {
      const dir = dirname(path);
      if (dir && dir !== ".") mkdirSync(dir, { recursive: true });
    }
    const db = new Database(path);
    db.exec(SCHEMA);
    return new LibSQLRecovery(db, ttlMs, true);
  }

  async save(data: Uint8Array, signal?: AbortSignal): Promise<void> {
    assertNotAborted(signal);
    const now = this.clock();
    const id = genID(now);
    const nano = BigInt(now.getTime()) * 1_000_000n;
    this.db
      .prepare(
        `INSERT INTO es4_recovery (gen_id, created_at, payload) VALUES (?, ?, ?)`,
      )
      .run(id, nano.toString(), Buffer.from(data));
    this.prune(now);
  }

  private prune(now: Date): void {
    const rows = this.db
      .prepare(`SELECT gen_id, created_at FROM es4_recovery ORDER BY gen_id ASC`)
      .all() as Array<{ gen_id: string; created_at: string | number | bigint }>;
    if (rows.length === 0) return;
    const newest = rows[rows.length - 1]!.gen_id;
    for (const r of rows) {
      if (r.gen_id === newest) continue;
      const createdAt = BigInt(r.created_at);
      const created = new Date(Number(createdAt / 1_000_000n));
      if (keepGeneration(created, now, this.ttlMs)) continue;
      this.db
        .prepare(`DELETE FROM es4_recovery WHERE gen_id = ?`)
        .run(r.gen_id);
    }
  }

  async load(signal?: AbortSignal): Promise<Uint8Array> {
    assertNotAborted(signal);
    const row = this.db
      .prepare(
        `SELECT payload FROM es4_recovery ORDER BY gen_id DESC LIMIT 1`,
      )
      .get() as { payload: Buffer } | undefined;
    if (!row) notFound();
    return new Uint8Array(row.payload);
  }

  async close(): Promise<void> {
    if (this.owns) this.db.close();
  }
}

export function openLibSQL(
  dsn: string,
  authToken = "",
  ttlMs = 0,
): LibSQLRecovery {
  return LibSQLRecovery.open(dsn, authToken, ttlMs);
}
