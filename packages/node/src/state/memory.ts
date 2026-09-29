import {
  ErrClosed,
  ErrNestedTx,
  ErrNotFound,
  ErrTxDone,
  throwIfAborted,
} from "../errors.js";
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

export class MemoryStore implements Store {
  private entries = new Map<string, JsonRaw>();
  private closed = false;
  private active: MemoryTx | null = null;

  async set(key: string, value: JsonRaw, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    validateValue(value);
    if (this.closed) throw ErrClosed;
    this.entries.set(key, deepCopyRaw(value));
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    if (this.closed) throw ErrClosed;
    const v = this.entries.get(key);
    if (v === undefined) throw ErrNotFound;
    return deepCopyRaw(v);
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    if (this.closed) throw ErrClosed;
    if (!this.entries.has(key)) throw ErrNotFound;
    this.entries.delete(key);
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    if (this.closed) throw ErrClosed;
    return this.entries.has(key);
  }

  async clear(signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    if (this.closed) throw ErrClosed;
    this.entries = new Map();
  }

  async export(signal?: AbortSignal): Promise<Map<string, JsonRaw>> {
    throwIfAborted(signal);
    if (this.closed) throw ErrClosed;
    const out = new Map<string, JsonRaw>();
    for (const [k, v] of this.entries) out.set(k, deepCopyRaw(v));
    return out;
  }

  async replace(
    entries: Map<string, JsonRaw> | Record<string, JsonRaw>,
    signal?: AbortSignal,
  ): Promise<void> {
    throwIfAborted(signal);
    const next = new Map<string, JsonRaw>();
    for (const [k, v] of toEntriesMap(entries)) {
      validateKey(k);
      validateValue(v);
      next.set(k, deepCopyRaw(v));
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
  _entries(): Map<string, JsonRaw> {
    return this.entries;
  }
  /** @internal */
  _setEntries(m: Map<string, JsonRaw>): void {
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

  async set(key: string, value: JsonRaw, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    validateValue(value);
    this.guard();
    this.overlay.set(key, { value: deepCopyRaw(value) });
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    this.guard();
    const e = this.overlay.get(key);
    if (e) {
      if (e.deleted) throw ErrNotFound;
      return deepCopyRaw(e.value);
    }
    if (this.cleared) throw ErrNotFound;
    const v = this.m._entries().get(key);
    if (v === undefined) throw ErrNotFound;
    return deepCopyRaw(v);
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    this.guard();
    const e = this.overlay.get(key);
    if (e) {
      if (e.deleted) throw ErrNotFound;
      this.overlay.set(key, { deleted: true });
      return;
    }
    if (this.cleared) throw ErrNotFound;
    if (!this.m._entries().has(key)) throw ErrNotFound;
    this.overlay.set(key, { deleted: true });
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    this.guard();
    const e = this.overlay.get(key);
    if (e) return !e.deleted;
    if (this.cleared) return false;
    return this.m._entries().has(key);
  }

  async clear(signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    this.guard();
    this.cleared = true;
    this.overlay = new Map();
  }

  async commit(): Promise<void> {
    this.guard();
    if (this.cleared) this.m._setEntries(new Map());
    const entries = this.m._entries();
    for (const [k, e] of this.overlay) {
      if (e.deleted) entries.delete(k);
      else entries.set(k, deepCopyRaw(e.value));
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
