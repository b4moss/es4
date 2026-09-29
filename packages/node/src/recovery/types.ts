import { ErrRecoveryNotFound, throwIfAborted } from "../errors.js";

export { ErrRecoveryNotFound };

export interface RecoveryStore {
  save(data: Uint8Array, signal?: AbortSignal): Promise<void>;
  load(signal?: AbortSignal): Promise<Uint8Array>;
  close?(): Promise<void>;
}

/** Zero-padded Unix-nano string (lexicographic order) — Go genID parity. */
export function genID(date = new Date()): string {
  // Approximate nanoseconds: ms * 1e6 (JS Date has ms resolution).
  const nano = BigInt(date.getTime()) * 1_000_000n;
  return nano.toString().padStart(20, "0");
}

export function parseGenID(id: string): Date {
  const n = BigInt(id);
  return new Date(Number(n / 1_000_000n));
}

/** ttlMs <= 0 → keep only newest (caller deletes others). ttlMs > 0 → age filter. */
export function keepGeneration(
  created: Date,
  now: Date,
  ttlMs: number,
): boolean {
  if (ttlMs <= 0) return false;
  return created.getTime() >= now.getTime() - ttlMs;
}

export const GEN_FILE_PREFIX = "gen-";
export const GEN_FILE_SUFFIX = ".snap";

export function assertNotAborted(signal?: AbortSignal): void {
  throwIfAborted(signal);
}

export function notFound(): never {
  throw ErrRecoveryNotFound;
}
