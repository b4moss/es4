import type { RecoveryStore } from "../recovery/types.js";
import type { JsonRaw, Store } from "../state/types.js";

export const CURRENT_VERSION = 1;

export interface Envelope {
  version: number;
  created_at: string; // RFC3339 UTC
  payload: MemoryPayload | string; // object or raw JSON string
}

export interface MemoryPayload {
  entries: Record<string, JsonRaw>;
}

export interface SnapshotConfig {
  state: Store;
  recovery: RecoveryStore | null;
  intervalMs: number;
  clock?: () => Date;
}

export class Manager {
  private readonly state: Store;
  private readonly recovery: RecoveryStore | null;
  private readonly intervalMs: number;
  private readonly clock: () => Date;
  private timer: ReturnType<typeof setInterval> | null = null;
  private running = false;

  constructor(cfg: SnapshotConfig) {
    this.state = cfg.state;
    this.recovery = cfg.recovery;
    this.intervalMs = cfg.intervalMs;
    this.clock = cfg.clock ?? (() => new Date());
  }

  start(): void {
    if (this.running || this.intervalMs <= 0) return;
    this.running = true;
    this.timer = setInterval(() => {
      void this.take();
    }, this.intervalMs);
    // Unref so periodic snapshots don't keep process alive alone.
    if (typeof this.timer === "object" && "unref" in this.timer) {
      this.timer.unref();
    }
  }

  stop(): void {
    if (!this.running) return;
    this.running = false;
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  async take(signal?: AbortSignal): Promise<void> {
    const entries = await this.state.export(signal);
    const payload: MemoryPayload = { entries: Object.fromEntries(entries) };
    const env = {
      version: CURRENT_VERSION,
      created_at: this.clock().toISOString(),
      payload,
    };
    const data = Buffer.from(JSON.stringify(env), "utf8");
    if (this.recovery) {
      await this.recovery.save(new Uint8Array(data), signal);
    }
    this.resetTicker();
  }

  private resetTicker(): void {
    if (!this.running || this.intervalMs <= 0) return;
    if (this.timer) clearInterval(this.timer);
    this.timer = setInterval(() => {
      void this.take();
    }, this.intervalMs);
    if (typeof this.timer === "object" && "unref" in this.timer) {
      this.timer.unref();
    }
  }
}

export function decodeEnvelope(data: Uint8Array): {
  version: number;
  created_at: string;
  payload: MemoryPayload;
} {
  const raw = JSON.parse(Buffer.from(data).toString("utf8")) as {
    version: number;
    created_at: string;
    payload: MemoryPayload | string;
  };
  let payload: MemoryPayload;
  if (typeof raw.payload === "string") {
    payload = JSON.parse(raw.payload) as MemoryPayload;
  } else {
    payload = raw.payload;
  }
  if (!payload.entries) payload.entries = {};
  return {
    version: raw.version,
    created_at: raw.created_at,
    payload,
  };
}

export async function restoreInto(
  state: Store,
  data: Uint8Array,
  signal?: AbortSignal,
): Promise<void> {
  const env = decodeEnvelope(data);
  await state.replace(env.payload.entries, signal);
}
