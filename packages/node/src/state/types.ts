import {
  ErrInvalidKey,
  ErrInvalidValue,
  wrapError,
} from "../errors.js";

/** JSON text bytes as UTF-8 string (Go json.RawMessage parity). */
export type JsonRaw = string;

/** Public Set options (DB layer). */
export type SetOptions = { expiresAt?: Date };

/**
 * Write metadata for Store.Set / Tx.Set.
 * `undefined` / `null` expiresAt means no expiry (and clears any prior expiry).
 */
export type SetMeta = { expiresAt?: Date | null };

export interface Tx {
  set(
    key: string,
    value: JsonRaw,
    meta?: SetMeta,
    signal?: AbortSignal,
  ): Promise<void>;
  get(key: string, signal?: AbortSignal): Promise<JsonRaw>;
  delete(key: string, signal?: AbortSignal): Promise<void>;
  exists(key: string, signal?: AbortSignal): Promise<boolean>;
  clear(signal?: AbortSignal): Promise<void>;
  scan(prefix: string, signal?: AbortSignal): Promise<string[]>;
  commit(): Promise<void>;
  rollback(): Promise<void>;
}

export interface Store {
  set(
    key: string,
    value: JsonRaw,
    meta?: SetMeta,
    signal?: AbortSignal,
  ): Promise<void>;
  get(key: string, signal?: AbortSignal): Promise<JsonRaw>;
  delete(key: string, signal?: AbortSignal): Promise<void>;
  exists(key: string, signal?: AbortSignal): Promise<boolean>;
  clear(signal?: AbortSignal): Promise<void>;
  /** Bulk prefix listing (no paging). Empty prefix lists all keys. Expired keys omitted. */
  scan(prefix: string, signal?: AbortSignal): Promise<string[]>;
  /** Values only; expiry metadata omitted; expired keys skipped. */
  export(signal?: AbortSignal): Promise<Map<string, JsonRaw>>;
  /** Atomic replace; restored entries have no expiry. */
  replace(
    entries: Map<string, JsonRaw> | Record<string, JsonRaw>,
    signal?: AbortSignal,
  ): Promise<void>;
  beginTx(signal?: AbortSignal): Promise<Tx>;
  close(): Promise<void>;
}

export function validateKey(key: string): void {
  if (key === "") {
    throw wrapError(ErrInvalidKey, "empty");
  }
  if (key.startsWith(":")) {
    throw wrapError(ErrInvalidKey, "leading colon");
  }
  if (key.endsWith(":")) {
    throw wrapError(ErrInvalidKey, "trailing colon");
  }
  if (key.includes("::")) {
    throw wrapError(ErrInvalidKey, "consecutive colons");
  }
}

/** Empty prefix means all keys; otherwise same rules as validateKey. */
export function validateScanPrefix(prefix: string): void {
  if (prefix === "") return;
  validateKey(prefix);
}

export function keysWithPrefix(keys: Iterable<string>, prefix: string): string[] {
  const out: string[] = [];
  for (const k of keys) {
    if (prefix === "" || k.startsWith(prefix)) out.push(k);
  }
  return out;
}

export function validateValue(value: JsonRaw): void {
  if (value === "" || value === undefined || value === null) {
    throw wrapError(ErrInvalidValue, "empty");
  }
  try {
    JSON.parse(value);
  } catch {
    throw wrapError(ErrInvalidValue, "not valid JSON");
  }
}

export function toEntriesMap(
  entries: Map<string, JsonRaw> | Record<string, JsonRaw>,
): Map<string, JsonRaw> {
  if (entries instanceof Map) return entries;
  return new Map(Object.entries(entries));
}

export type OverlayEntry =
  | { deleted: true }
  | { deleted?: false; value: JsonRaw; expiresAt?: Date | null };

export function deepCopyRaw(v: JsonRaw): JsonRaw {
  return v;
}

let nowFunc: () => Date = () => new Date();

/** Current time used for expiry checks (overridable in tests via setNowFunc). */
export function now(): Date {
  return nowFunc();
}

/** Override the clock for expiry checks. Pass null to restore Date.now. */
export function setNowFunc(f: (() => Date) | null): void {
  nowFunc = f ?? (() => new Date());
}

/**
 * True when expiresAt is set and not strictly after `at` (equal counts as expired).
 * Missing / null expiresAt is never expired.
 */
export function isExpired(
  expiresAt?: Date | null,
  at: Date = now(),
): boolean {
  if (!expiresAt) return false;
  return !(expiresAt.getTime() > at.getTime());
}

/** Clone a Date or return null when unset. */
export function cloneExpiresAt(exp?: Date | null): Date | null {
  if (exp == null) return null;
  return new Date(exp.getTime());
}

/** Normalize SetMeta expiresAt for storage (null = no expiry). */
export function metaExpiresAt(meta?: SetMeta): Date | null {
  return cloneExpiresAt(meta?.expiresAt);
}

/** RFC3339 / RFC3339Nano UTC string for SQLite / Redis meta storage. */
export function formatExpiresAt(t: Date): string {
  return t.toISOString();
}

/** Parse an RFC3339 / RFC3339Nano expiry string. Empty → null. */
export function parseExpiresAt(s: string): Date | null {
  if (s === "") return null;
  const t = Date.parse(s);
  if (Number.isNaN(t)) {
    throw new Error(`invalid expires_at: ${s}`);
  }
  return new Date(t);
}

/**
 * Detect AbortSignal so DB.set can accept (opts | signal) as the 3rd argument.
 */
export function isAbortSignal(v: unknown): v is AbortSignal {
  return (
    typeof v === "object" &&
    v !== null &&
    typeof (v as AbortSignal).aborted === "boolean" &&
    typeof (v as AbortSignal).addEventListener === "function"
  );
}
