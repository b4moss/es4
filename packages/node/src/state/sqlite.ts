import Database from "better-sqlite3";
import { mkdirSync } from "node:fs";
import { dirname } from "node:path";
import {
  ErrClosed,
  ErrNestedTx,
  ErrNotFound,
  ErrTxDone,
  throwIfAborted,
} from "../errors.js";
import { AsyncMutex } from "./mutex.js";
import {
  deepCopyRaw,
  type JsonRaw,
  type Store,
  type Tx,
  toEntriesMap,
  validateKey,
  validateValue,
} from "./types.js";

const SCHEMA = `
CREATE TABLE IF NOT EXISTS kv (
  key TEXT PRIMARY KEY NOT NULL,
  value TEXT NOT NULL
);
`;

export class SQLiteStore implements Store {
  private readonly mu = new AsyncMutex();
  private closed = false;
  private active: SQLiteOverlayTx | null = null;
  private readonly db: Database.Database;

  private constructor(db: Database.Database, readonly path: string) {
    this.db = db;
  }

  static open(path: string): SQLiteStore {
    if (path === "") throw new Error("state: empty sqlite path");
    mkdirSync(dirname(path), { recursive: true });
    const db = new Database(path);
    db.exec(SCHEMA);
    return new SQLiteStore(db, path);
  }

  private async withNoTx<T>(fn: () => T): Promise<T> {
    let release = await this.mu.acquire();
    try {
      if (this.closed) throw ErrClosed;
      while (this.active !== null) {
        release();
        await new Promise((r) => setTimeout(r, 1));
        release = await this.mu.acquire();
        if (this.closed) throw ErrClosed;
      }
      return fn();
    } finally {
      release();
    }
  }

  async set(key: string, value: JsonRaw, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    validateValue(value);
    await this.withNoTx(() => {
      this.db
        .prepare(
          `INSERT INTO kv(key, value) VALUES(?, ?)
           ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
        )
        .run(key, value);
    });
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.withNoTx(() => {
      const row = this.db
        .prepare(`SELECT value FROM kv WHERE key = ?`)
        .get(key) as { value: string } | undefined;
      if (!row) throw ErrNotFound;
      return deepCopyRaw(row.value);
    });
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    await this.withNoTx(() => {
      const info = this.db.prepare(`DELETE FROM kv WHERE key = ?`).run(key);
      if (info.changes === 0) throw ErrNotFound;
    });
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.withNoTx(() => {
      const row = this.db
        .prepare(`SELECT 1 AS ok FROM kv WHERE key = ?`)
        .get(key);
      return row !== undefined;
    });
  }

  async clear(signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    await this.withNoTx(() => {
      this.db.prepare(`DELETE FROM kv`).run();
    });
  }

  async export(signal?: AbortSignal): Promise<Map<string, JsonRaw>> {
    throwIfAborted(signal);
    return await this.withNoTx(() => {
      const rows = this.db
        .prepare(`SELECT key, value FROM kv`)
        .all() as Array<{ key: string; value: string }>;
      const out = new Map<string, JsonRaw>();
      for (const r of rows) out.set(r.key, deepCopyRaw(r.value));
      return out;
    });
  }

  async replace(
    entries: Map<string, JsonRaw> | Record<string, JsonRaw>,
    signal?: AbortSignal,
  ): Promise<void> {
    throwIfAborted(signal);
    const map = toEntriesMap(entries);
    for (const [k, v] of map) {
      validateKey(k);
      validateValue(v);
    }
    await this.withNoTx(() => {
      const tx = this.db.transaction(() => {
        this.db.prepare(`DELETE FROM kv`).run();
        const ins = this.db.prepare(
          `INSERT INTO kv(key, value) VALUES(?, ?)`,
        );
        for (const [k, v] of map) ins.run(k, v);
      });
      tx();
    });
  }

  async beginTx(signal?: AbortSignal): Promise<Tx> {
    throwIfAborted(signal);
    return await this.mu.withLock(() => {
      if (this.closed) throw ErrClosed;
      if (this.active) throw ErrNestedTx;
      // better-sqlite3 sync transaction — use overlay like Memory for async API,
      // then apply in one SQL transaction on commit (Go uses real sql.Tx).
      const tx = new SQLiteOverlayTx(this);
      this.active = tx;
      return tx;
    });
  }

  async close(): Promise<void> {
    await this.mu.withLock(() => {
      if (this.active) {
        this.active.markDone();
        this.active = null;
      }
      if (!this.closed) {
        this.closed = true;
        this.db.close();
      }
    });
  }

  /** @internal */
  _db(): Database.Database {
    return this.db;
  }
  /** @internal */
  _clearActive(tx: SQLiteOverlayTx): void {
    if (this.active === tx) this.active = null;
  }
  /** @internal */
  _isClosed(): boolean {
    return this.closed;
  }
  /** @internal */
  _isActive(tx: SQLiteOverlayTx): boolean {
    return this.active === tx;
  }
  /** @internal */
  _mu(): AsyncMutex {
    return this.mu;
  }
}

type OverlayEntry =
  | { deleted: true }
  | { deleted?: false; value: JsonRaw };

class SQLiteOverlayTx implements Tx {
  private overlay = new Map<string, OverlayEntry>();
  private cleared = false;
  private done = false;

  constructor(private readonly s: SQLiteStore) {}

  markDone(): void {
    this.done = true;
  }

  private guard(): void {
    if (this.done || !this.s._isActive(this)) throw ErrTxDone;
    if (this.s._isClosed()) throw ErrClosed;
  }

  async set(key: string, value: JsonRaw, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    validateValue(value);
    await this.s._mu().withLock(() => {
      this.guard();
      this.overlay.set(key, { value: deepCopyRaw(value) });
    });
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.s._mu().withLock(() => {
      this.guard();
      const e = this.overlay.get(key);
      if (e) {
        if (e.deleted) throw ErrNotFound;
        return deepCopyRaw(e.value);
      }
      if (this.cleared) throw ErrNotFound;
      const row = this.s
        ._db()
        .prepare(`SELECT value FROM kv WHERE key = ?`)
        .get(key) as { value: string } | undefined;
      if (!row) throw ErrNotFound;
      return deepCopyRaw(row.value);
    });
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    await this.s._mu().withLock(() => {
      this.guard();
      const e = this.overlay.get(key);
      if (e) {
        if (e.deleted) throw ErrNotFound;
        this.overlay.set(key, { deleted: true });
        return;
      }
      if (this.cleared) throw ErrNotFound;
      const row = this.s
        ._db()
        .prepare(`SELECT 1 AS ok FROM kv WHERE key = ?`)
        .get(key);
      if (!row) throw ErrNotFound;
      this.overlay.set(key, { deleted: true });
    });
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.s._mu().withLock(() => {
      this.guard();
      const e = this.overlay.get(key);
      if (e) return !e.deleted;
      if (this.cleared) return false;
      const row = this.s
        ._db()
        .prepare(`SELECT 1 AS ok FROM kv WHERE key = ?`)
        .get(key);
      return row !== undefined;
    });
  }

  async clear(signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    await this.s._mu().withLock(() => {
      this.guard();
      this.cleared = true;
      this.overlay = new Map();
    });
  }

  async commit(): Promise<void> {
    await this.s._mu().withLock(() => {
      this.guard();
      const db = this.s._db();
      const run = db.transaction(() => {
        if (this.cleared) db.prepare(`DELETE FROM kv`).run();
        for (const [k, e] of this.overlay) {
          if (e.deleted) {
            db.prepare(`DELETE FROM kv WHERE key = ?`).run(k);
          } else {
            db.prepare(
              `INSERT INTO kv(key, value) VALUES(?, ?)
               ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
            ).run(k, e.value);
          }
        }
      });
      run();
      this.done = true;
      this.s._clearActive(this);
    });
  }

  async rollback(): Promise<void> {
    await this.s._mu().withLock(() => {
      if (this.done || !this.s._isActive(this)) throw ErrTxDone;
      this.done = true;
      this.s._clearActive(this);
    });
  }
}

export function openSQLite(path: string): SQLiteStore {
  return SQLiteStore.open(path);
}
