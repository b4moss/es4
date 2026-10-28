import { Redis as IoRedis } from "ioredis";
import type { Redis as RedisClient } from "ioredis";
import {
  ErrClosed,
  ErrNestedTx,
  ErrNotFound,
  ErrTxDone,
  throwIfAborted,
} from "../errors.js";
import { DEFAULT_REDIS_HASH_KEY } from "../options/types.js";
import { AsyncMutex } from "./mutex.js";
import {
  deepCopyRaw,
  formatExpiresAt,
  isExpired,
  metaExpiresAt,
  parseExpiresAt,
  type JsonRaw,
  type OverlayEntry,
  type SetMeta,
  type Store,
  type Tx,
  toEntriesMap,
  validateKey,
  validateScanPrefix,
  validateValue,
} from "./types.js";

export { DEFAULT_REDIS_HASH_KEY };

export interface RedisConfig {
  url?: string;
  keyPrefix?: string;
  client?: RedisClient;
  ownsClient?: boolean;
}

export class RedisStore implements Store {
  private readonly mu = new AsyncMutex();
  private closed = false;
  private active: RedisOverlayTx | null = null;
  private readonly client: RedisClient;
  private readonly owns: boolean;
  readonly hash: string;

  private constructor(client: RedisClient, hash: string, owns: boolean) {
    this.client = client;
    this.hash = hash;
    this.owns = owns;
  }

  static async open(
    url: string,
    keyPrefix = "",
    signal?: AbortSignal,
  ): Promise<RedisStore> {
    return RedisStore.openConfig({ url, keyPrefix }, signal);
  }

  static async openConfig(
    cfg: RedisConfig,
    signal?: AbortSignal,
  ): Promise<RedisStore> {
    throwIfAborted(signal);
    const hash =
      cfg.keyPrefix && cfg.keyPrefix !== ""
        ? cfg.keyPrefix
        : DEFAULT_REDIS_HASH_KEY;
    let client = cfg.client;
    let owns = cfg.ownsClient ?? false;
    if (!client) {
      if (!cfg.url) throw new Error("state: empty redis url");
      client = new IoRedis(cfg.url, {
        maxRetriesPerRequest: 1,
        lazyConnect: true,
      });
      owns = true;
      try {
        await client.connect();
        await client.ping();
      } catch (e) {
        try {
          client.disconnect();
        } catch {
          /* ignore */
        }
        throw new Error(`state: redis ping: ${(e as Error).message}`, {
          cause: e,
        });
      }
    }
    return new RedisStore(client, hash, owns);
  }

  hashKey(): string {
    return this.hash;
  }

  /** Companion HASH for expiry metadata (read-time filter; no native EXPIRE). */
  metaHash(): string {
    return `${this.hash}:meta`;
  }

  private async withNoTx<T>(fn: () => Promise<T>): Promise<T> {
    let release = await this.mu.acquire();
    try {
      if (this.closed) throw ErrClosed;
      while (this.active !== null) {
        release();
        await new Promise((r) => setTimeout(r, 1));
        release = await this.mu.acquire();
        if (this.closed) throw ErrClosed;
      }
      return await fn();
    } finally {
      release();
    }
  }

  private async readExpires(key: string): Promise<Date | null> {
    const s = await this.client.hget(this.metaHash(), key);
    if (s === null || s === "") return null;
    return parseExpiresAt(s);
  }

  private async purgeExpired(key: string): Promise<void> {
    const multi = this.client.multi();
    multi.hdel(this.hash, key);
    multi.hdel(this.metaHash(), key);
    await multi.exec();
  }

  async set(
    key: string,
    value: JsonRaw,
    meta?: SetMeta,
    signal?: AbortSignal,
  ): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    validateValue(value);
    const exp = metaExpiresAt(meta);
    await this.withNoTx(async () => {
      const multi = this.client.multi();
      multi.hset(this.hash, key, value);
      if (exp === null) multi.hdel(this.metaHash(), key);
      else multi.hset(this.metaHash(), key, formatExpiresAt(exp));
      await multi.exec();
    });
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.withNoTx(async () => {
      const v = await this.client.hget(this.hash, key);
      if (v === null) throw ErrNotFound;
      const exp = await this.readExpires(key);
      if (isExpired(exp)) {
        await this.purgeExpired(key);
        throw ErrNotFound;
      }
      return deepCopyRaw(v);
    });
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    await this.withNoTx(async () => {
      const n = await this.client.hexists(this.hash, key);
      if (n === 0) throw ErrNotFound;
      const exp = await this.readExpires(key);
      if (isExpired(exp)) {
        await this.purgeExpired(key);
        throw ErrNotFound;
      }
      const multi = this.client.multi();
      multi.hdel(this.hash, key);
      multi.hdel(this.metaHash(), key);
      await multi.exec();
    });
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.withNoTx(async () => {
      const n = await this.client.hexists(this.hash, key);
      if (n === 0) return false;
      const exp = await this.readExpires(key);
      if (isExpired(exp)) {
        await this.purgeExpired(key);
        return false;
      }
      return true;
    });
  }

  async clear(signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    await this.withNoTx(async () => {
      await this.client.del(this.hash, this.metaHash());
    });
  }

  async scan(prefix: string, signal?: AbortSignal): Promise<string[]> {
    throwIfAborted(signal);
    validateScanPrefix(prefix);
    return await this.withNoTx(async () => {
      const fields = await this.client.hkeys(this.hash);
      const meta = (await this.client.hgetall(this.metaHash())) as Record<
        string,
        string
      >;
      const out: string[] = [];
      for (const k of fields) {
        if (prefix !== "" && !k.startsWith(prefix)) continue;
        const raw = meta[k];
        const exp = raw ? parseExpiresAt(raw) : null;
        if (isExpired(exp)) continue;
        out.push(k);
      }
      return out;
    });
  }

  async export(signal?: AbortSignal): Promise<Map<string, JsonRaw>> {
    throwIfAborted(signal);
    return await this.withNoTx(async () => {
      const all = (await this.client.hgetall(this.hash)) as Record<
        string,
        string
      >;
      const meta = (await this.client.hgetall(this.metaHash())) as Record<
        string,
        string
      >;
      const out = new Map<string, JsonRaw>();
      for (const [k, v] of Object.entries(all)) {
        const raw = meta[k];
        const exp = raw ? parseExpiresAt(raw) : null;
        if (isExpired(exp)) continue;
        out.set(k, deepCopyRaw(v));
      }
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
    await this.withNoTx(async () => {
      const multi = this.client.multi();
      multi.del(this.hash, this.metaHash());
      if (map.size > 0) {
        const flat: string[] = [];
        for (const [k, v] of map) {
          flat.push(k, v);
        }
        multi.hset(this.hash, ...flat);
      }
      await multi.exec();
    });
  }

  async beginTx(signal?: AbortSignal): Promise<Tx> {
    throwIfAborted(signal);
    return await this.mu.withLock(() => {
      if (this.closed) throw ErrClosed;
      if (this.active) throw ErrNestedTx;
      const tx = new RedisOverlayTx(this);
      this.active = tx;
      return tx;
    });
  }

  async close(): Promise<void> {
    await this.mu.withLock(async () => {
      if (this.active) {
        this.active.markDone();
        this.active = null;
      }
      if (!this.closed) {
        this.closed = true;
        if (this.owns) {
          try {
            await this.client.quit();
          } catch {
            this.client.disconnect();
          }
        }
      }
    });
  }

  /** @internal */
  _client(): RedisClient {
    return this.client;
  }
  /** @internal */
  _clearActive(tx: RedisOverlayTx): void {
    if (this.active === tx) this.active = null;
  }
  /** @internal */
  _isClosed(): boolean {
    return this.closed;
  }
  /** @internal */
  _isActive(tx: RedisOverlayTx): boolean {
    return this.active === tx;
  }
  /** @internal */
  _mu(): AsyncMutex {
    return this.mu;
  }
  /** @internal */
  async _readExpires(key: string): Promise<Date | null> {
    return this.readExpires(key);
  }
}

class RedisOverlayTx implements Tx {
  private overlay = new Map<string, OverlayEntry>();
  private cleared = false;
  private done = false;

  constructor(private readonly r: RedisStore) {}

  markDone(): void {
    this.done = true;
  }

  private guard(): void {
    if (this.done || !this.r._isActive(this)) throw ErrTxDone;
    if (this.r._isClosed()) throw ErrClosed;
  }

  async set(
    key: string,
    value: JsonRaw,
    meta?: SetMeta,
    signal?: AbortSignal,
  ): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    validateValue(value);
    await this.r._mu().withLock(() => {
      this.guard();
      this.overlay.set(key, {
        value: deepCopyRaw(value),
        expiresAt: metaExpiresAt(meta),
      });
    });
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.r._mu().withLock(async () => {
      this.guard();
      const e = this.overlay.get(key);
      if (e) {
        if (e.deleted || isExpired(e.expiresAt)) throw ErrNotFound;
        return deepCopyRaw(e.value);
      }
      if (this.cleared) throw ErrNotFound;
      const v = await this.r._client().hget(this.r.hash, key);
      if (v === null) throw ErrNotFound;
      const exp = await this.r._readExpires(key);
      if (isExpired(exp)) throw ErrNotFound;
      return deepCopyRaw(v);
    });
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    await this.r._mu().withLock(async () => {
      this.guard();
      const e = this.overlay.get(key);
      if (e) {
        if (e.deleted || isExpired(e.expiresAt)) throw ErrNotFound;
        this.overlay.set(key, { deleted: true });
        return;
      }
      if (this.cleared) throw ErrNotFound;
      const n = await this.r._client().hexists(this.r.hash, key);
      if (n === 0) throw ErrNotFound;
      const exp = await this.r._readExpires(key);
      if (isExpired(exp)) throw ErrNotFound;
      this.overlay.set(key, { deleted: true });
    });
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.r._mu().withLock(async () => {
      this.guard();
      const e = this.overlay.get(key);
      if (e) return !e.deleted && !isExpired(e.expiresAt);
      if (this.cleared) return false;
      if ((await this.r._client().hexists(this.r.hash, key)) !== 1) return false;
      const exp = await this.r._readExpires(key);
      return !isExpired(exp);
    });
  }

  async clear(signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    await this.r._mu().withLock(() => {
      this.guard();
      this.cleared = true;
      this.overlay = new Map();
    });
  }

  async scan(prefix: string, signal?: AbortSignal): Promise<string[]> {
    throwIfAborted(signal);
    validateScanPrefix(prefix);
    return await this.r._mu().withLock(async () => {
      this.guard();
      const seen = new Set<string>();
      if (!this.cleared) {
        const fields = await this.r._client().hkeys(this.r.hash);
        const meta = (await this.r
          ._client()
          .hgetall(this.r.metaHash())) as Record<string, string>;
        for (const k of fields) {
          if (prefix !== "" && !k.startsWith(prefix)) continue;
          const e = this.overlay.get(k);
          if (e) {
            if (e.deleted || isExpired(e.expiresAt)) continue;
            seen.add(k);
            continue;
          }
          const raw = meta[k];
          const exp = raw ? parseExpiresAt(raw) : null;
          if (isExpired(exp)) continue;
          seen.add(k);
        }
      }
      for (const [k, e] of this.overlay) {
        if (e.deleted || isExpired(e.expiresAt)) continue;
        if (prefix !== "" && !k.startsWith(prefix)) continue;
        seen.add(k);
      }
      return [...seen];
    });
  }

  async commit(): Promise<void> {
    await this.r._mu().withLock(async () => {
      this.guard();
      const multi = this.r._client().multi();
      if (this.cleared) multi.del(this.r.hash, this.r.metaHash());
      for (const [k, e] of this.overlay) {
        if (e.deleted) {
          multi.hdel(this.r.hash, k);
          multi.hdel(this.r.metaHash(), k);
        } else {
          multi.hset(this.r.hash, k, e.value);
          if (e.expiresAt == null) multi.hdel(this.r.metaHash(), k);
          else multi.hset(this.r.metaHash(), k, formatExpiresAt(e.expiresAt));
        }
      }
      await multi.exec();
      this.done = true;
      this.r._clearActive(this);
    });
  }

  async rollback(): Promise<void> {
    await this.r._mu().withLock(() => {
      if (this.done || !this.r._isActive(this)) throw ErrTxDone;
      this.done = true;
      this.r._clearActive(this);
    });
  }
}

export async function openRedis(
  url: string,
  keyPrefix = "",
  signal?: AbortSignal,
): Promise<RedisStore> {
  return RedisStore.open(url, keyPrefix, signal);
}
