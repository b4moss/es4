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
  type JsonRaw,
  type OverlayEntry,
  type Store,
  type Tx,
  toEntriesMap,
  validateKey,
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

  async set(key: string, value: JsonRaw, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    validateValue(value);
    await this.withNoTx(async () => {
      await this.client.hset(this.hash, key, value);
    });
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.withNoTx(async () => {
      const v = await this.client.hget(this.hash, key);
      if (v === null) throw ErrNotFound;
      return deepCopyRaw(v);
    });
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    await this.withNoTx(async () => {
      const n = await this.client.hdel(this.hash, key);
      if (n === 0) throw ErrNotFound;
    });
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.withNoTx(async () => {
      const n = await this.client.hexists(this.hash, key);
      return n === 1;
    });
  }

  async clear(signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    await this.withNoTx(async () => {
      await this.client.del(this.hash);
    });
  }

  async export(signal?: AbortSignal): Promise<Map<string, JsonRaw>> {
    throwIfAborted(signal);
    return await this.withNoTx(async () => {
      const all = (await this.client.hgetall(this.hash)) as Record<string, string>;
      const out = new Map<string, JsonRaw>();
      for (const [k, v] of Object.entries(all)) out.set(k, deepCopyRaw(v));
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
      multi.del(this.hash);
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

  async set(key: string, value: JsonRaw, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    validateValue(value);
    await this.r._mu().withLock(() => {
      this.guard();
      this.overlay.set(key, { value: deepCopyRaw(value) });
    });
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.r._mu().withLock(async () => {
      this.guard();
      const e = this.overlay.get(key);
      if (e) {
        if (e.deleted) throw ErrNotFound;
        return deepCopyRaw(e.value);
      }
      if (this.cleared) throw ErrNotFound;
      const v = await this.r._client().hget(this.r.hash, key);
      if (v === null) throw ErrNotFound;
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
        if (e.deleted) throw ErrNotFound;
        this.overlay.set(key, { deleted: true });
        return;
      }
      if (this.cleared) throw ErrNotFound;
      const n = await this.r._client().hexists(this.r.hash, key);
      if (n === 0) throw ErrNotFound;
      this.overlay.set(key, { deleted: true });
    });
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.r._mu().withLock(async () => {
      this.guard();
      const e = this.overlay.get(key);
      if (e) return !e.deleted;
      if (this.cleared) return false;
      return (await this.r._client().hexists(this.r.hash, key)) === 1;
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

  async commit(): Promise<void> {
    await this.r._mu().withLock(async () => {
      this.guard();
      const multi = this.r._client().multi();
      if (this.cleared) multi.del(this.r.hash);
      for (const [k, e] of this.overlay) {
        if (e.deleted) multi.hdel(this.r.hash, k);
        else multi.hset(this.r.hash, k, e.value);
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
