import {
  ErrClosed,
  ErrNestedTx,
  ErrNotFound,
  ErrTxDone,
  throwIfAborted,
} from "../errors.js";
import {
  deepCopyRaw,
  isExpired,
  keysWithPrefix,
  metaExpiresAt,
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

type MemEntry = { value: JsonRaw; expiresAt: Date | null };

export class MemoryStore implements Store {
  private entries = new Map<string, MemEntry>();
  private closed = false;
  private active: MemoryTx | null = null;

  async set(
    key: string,
    value: JsonRaw,
    meta?: SetMeta,
    signal?: AbortSignal,
  ): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    validateValue(value);
    if (this.closed) throw ErrClosed;
    this.entries.set(key, {
      value: deepCopyRaw(value),
      expiresAt: metaExpiresAt(meta),
    });
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    if (this.closed) throw ErrClosed;
    const e = this.entries.get(key);
    if (e === undefined) throw ErrNotFound;
    if (isExpired(e.expiresAt)) {
      this.entries.delete(key);
      throw ErrNotFound;
    }
    return deepCopyRaw(e.value);
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    if (this.closed) throw ErrClosed;
    const e = this.entries.get(key);
    if (e === undefined) throw ErrNotFound;
    if (isExpired(e.expiresAt)) {
      this.entries.delete(key);
      throw ErrNotFound;
    }
    this.entries.delete(key);
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    if (this.closed) throw ErrClosed;
    const e = this.entries.get(key);
    if (e === undefined) return false;
    if (isExpired(e.expiresAt)) {
      this.entries.delete(key);
      return false;
    }
    return true;
  }

  async clear(signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    if (this.closed) throw ErrClosed;
    this.entries = new Map();
  }

  async scan(prefix: string, signal?: AbortSignal): Promise<string[]> {
    throwIfAborted(signal);
    validateScanPrefix(prefix);
    if (this.closed) throw ErrClosed;
    const live: string[] = [];
    for (const [k, e] of this.entries) {
      if (prefix !== "" && !k.startsWith(prefix)) continue;
      if (isExpired(e.expiresAt)) continue;
      live.push(k);
    }
    return live;
  }

  async export(signal?: AbortSignal): Promise<Map<string, JsonRaw>> {
    throwIfAborted(signal);
    if (this.closed) throw ErrClosed;
    const out = new Map<string, JsonRaw>();
    for (const [k, e] of this.entries) {
      if (isExpired(e.expiresAt)) continue;
      out.set(k, deepCopyRaw(e.value));
    }
    return out;
  }

  async replace(
    entries: Map<string, JsonRaw> | Record<string, JsonRaw>,
    signal?: AbortSignal,
  ): Promise<void> {
    throwIfAborted(signal);
    const next = new Map<string, MemEntry>();
    for (const [k, v] of toEntriesMap(entries)) {
      validateKey(k);
      validateValue(v);
      next.set(k, { value: deepCopyRaw(v), expiresAt: null });
    }
    if (this.closed) throw ErrClosed;
    this.entries = next;
  }

  async beginTx(signal?: AbortSignal): Promise<Tx> {
    throwIfAborted(signal);
    if (this.closed) throw ErrClosed;
    if (this.active) throw ErrNestedTx;
    const tx = new MemoryTx(this);
    this.active = tx;
    return tx;
  }

  async close(): Promise<void> {
    if (this.active) {
      this.active.markDone();
      this.active = null;
    }
    this.closed = true;
    this.entries = new Map();
  }

  /** @internal */
  _entries(): Map<string, MemEntry> {
    return this.entries;
  }
  /** @internal */
  _setEntries(m: Map<string, MemEntry>): void {
    this.entries = m;
  }
  /** @internal */
  _clearActive(tx: MemoryTx): void {
    if (this.active === tx) this.active = null;
  }
  /** @internal */
  _isClosed(): boolean {
    return this.closed;
  }
  /** @internal */
  _isActive(tx: MemoryTx): boolean {
    return this.active === tx;
  }
}

class MemoryTx implements Tx {
  private overlay = new Map<string, OverlayEntry>();
  private cleared = false;
  private done = false;

  constructor(private readonly m: MemoryStore) {}

  markDone(): void {
    this.done = true;
  }

  private guard(): void {
    if (this.done || !this.m._isActive(this)) throw ErrTxDone;
    if (this.m._isClosed()) throw ErrClosed;
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
    this.guard();
    this.overlay.set(key, {
      value: deepCopyRaw(value),
      expiresAt: metaExpiresAt(meta),
    });
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    this.guard();
    const e = this.overlay.get(key);
    if (e) {
      if (e.deleted || isExpired(e.expiresAt)) throw ErrNotFound;
      return deepCopyRaw(e.value);
    }
    if (this.cleared) throw ErrNotFound;
    const base = this.m._entries().get(key);
    if (base === undefined || isExpired(base.expiresAt)) throw ErrNotFound;
    return deepCopyRaw(base.value);
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    this.guard();
    const e = this.overlay.get(key);
    if (e) {
      if (e.deleted || isExpired(e.expiresAt)) throw ErrNotFound;
      this.overlay.set(key, { deleted: true });
      return;
    }
    if (this.cleared) throw ErrNotFound;
    const base = this.m._entries().get(key);
    if (base === undefined || isExpired(base.expiresAt)) throw ErrNotFound;
    this.overlay.set(key, { deleted: true });
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    this.guard();
    const e = this.overlay.get(key);
    if (e) return !e.deleted && !isExpired(e.expiresAt);
    if (this.cleared) return false;
    const base = this.m._entries().get(key);
    return base !== undefined && !isExpired(base.expiresAt);
  }

  async clear(signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    this.guard();
    this.cleared = true;
    this.overlay = new Map();
  }

  async scan(prefix: string, signal?: AbortSignal): Promise<string[]> {
    throwIfAborted(signal);
    validateScanPrefix(prefix);
    this.guard();
    const seen = new Set<string>();
    if (!this.cleared) {
      for (const [k, base] of this.m._entries()) {
        if (prefix !== "" && !k.startsWith(prefix)) continue;
        const e = this.overlay.get(k);
        if (e) {
          if (e.deleted || isExpired(e.expiresAt)) continue;
          seen.add(k);
          continue;
        }
        if (isExpired(base.expiresAt)) continue;
        seen.add(k);
      }
    }
    for (const [k, e] of this.overlay) {
      if (e.deleted || isExpired(e.expiresAt)) continue;
      if (prefix !== "" && !k.startsWith(prefix)) continue;
      seen.add(k);
    }
    return [...seen];
  }

  async commit(): Promise<void> {
    this.guard();
    if (this.cleared) this.m._setEntries(new Map());
    const entries = this.m._entries();
    for (const [k, e] of this.overlay) {
      if (e.deleted) entries.delete(k);
      else {
        entries.set(k, {
          value: deepCopyRaw(e.value),
          expiresAt: e.expiresAt ?? null,
        });
      }
    }
    this.done = true;
    this.m._clearActive(this);
  }

  async rollback(): Promise<void> {
    if (this.done || !this.m._isActive(this)) throw ErrTxDone;
    this.done = true;
    this.m._clearActive(this);
  }
}

export function newMemory(): MemoryStore {
  return new MemoryStore();
}
