import {
  ErrInvalidKey,
  ErrInvalidValue,
  wrapError,
} from "../errors.js";

/** JSON text bytes as UTF-8 string (Go json.RawMessage parity). */
export type JsonRaw = string;

export interface Tx {
  set(key: string, value: JsonRaw, signal?: AbortSignal): Promise<void>;
  get(key: string, signal?: AbortSignal): Promise<JsonRaw>;
  delete(key: string, signal?: AbortSignal): Promise<void>;
  exists(key: string, signal?: AbortSignal): Promise<boolean>;
  clear(signal?: AbortSignal): Promise<void>;
  commit(): Promise<void>;
  rollback(): Promise<void>;
}

export interface Store {
  set(key: string, value: JsonRaw, signal?: AbortSignal): Promise<void>;
  get(key: string, signal?: AbortSignal): Promise<JsonRaw>;
  delete(key: string, signal?: AbortSignal): Promise<void>;
  exists(key: string, signal?: AbortSignal): Promise<boolean>;
  clear(signal?: AbortSignal): Promise<void>;
  export(signal?: AbortSignal): Promise<Map<string, JsonRaw>>;
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
  if (key.startsWith("/")) {
    throw wrapError(ErrInvalidKey, "leading slash");
  }
  if (key.endsWith("/")) {
    throw wrapError(ErrInvalidKey, "trailing slash");
  }
  if (key.includes("//")) {
    throw wrapError(ErrInvalidKey, "consecutive slashes");
  }
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

export type OverlayEntry = { deleted: true } | { deleted?: false; value: JsonRaw };

export function deepCopyRaw(v: JsonRaw): JsonRaw {
  return v;
}
