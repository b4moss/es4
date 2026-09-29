import {
  ErrClosed,
  ErrNotFound,
  isNotFound,
  throwIfAborted,
} from "./errors.js";
import {
  effective,
  isFirestoreStateBackend,
  isRedisStateBackend,
  type Options,
  validate,
  StateBackendMemory,
  StateBackendSQLite,
} from "./options/index.js";
import { openFromOptions, type RecoveryStore } from "./recovery/index.js";
import { Manager, restoreInto } from "./snapshot/manager.js";
import {
  newMemory,
  openFirestore,
  openRedis,
  openSQLite,
  type JsonRaw,
  type Store,
  type Tx,
} from "./state/index.js";
import { ErrRecoveryNotFound } from "./errors.js";

export interface OpenConfig {
  options: Options;
  state?: Store;
  recovery?: RecoveryStore | null;
  /** Run startup restore synchronously (tests). */
  skipAsyncRestore?: boolean;
  /** Skip startup restore entirely. */
  disableRestore?: boolean;
}

export class DB {
  private readonly opts: Options;
  private readonly state: Store;
  private readonly recovery: RecoveryStore | null;
  private readonly snap: Manager;
  private closed = false;
  private ready = false;
  private restorePromise: Promise<void> = Promise.resolve();

  private constructor(
    opts: Options,
    state: Store,
    recovery: RecoveryStore | null,
    snap: Manager,
  ) {
    this.opts = opts;
    this.state = state;
    this.recovery = recovery;
    this.snap = snap;
  }

  static async open(options: Options, signal?: AbortSignal): Promise<DB> {
    return DB.openWith({ options }, signal);
  }

  static async openWith(cfg: OpenConfig, signal?: AbortSignal): Promise<DB> {
    throwIfAborted(signal);
    const eff = effective(cfg.options);
    validate(eff);

    let st = cfg.state;
    if (!st) {
      st = await openDefaultState(eff, signal);
    }

    let rec: RecoveryStore | null;
    if (cfg.recovery !== undefined) {
      rec = cfg.recovery;
    } else if (!eff.memory_only) {
      try {
        rec = await openFromOptions(eff, signal);
      } catch (e) {
        await st.close();
        throw e;
      }
    } else {
      rec = null;
    }

    const snapRec = !eff.memory_only && rec ? rec : null;
    let interval = eff.snapshot_interval;
    if (eff.memory_only) interval = 0;

    const mgr = new Manager({
      state: st,
      recovery: snapRec,
      intervalMs: interval,
    });

    const db = new DB(eff, st, rec, mgr);

    const doRestore =
      !cfg.disableRestore &&
      eff.restore_on_startup &&
      !eff.memory_only &&
      rec !== null;

    if (doRestore) {
      if (cfg.skipAsyncRestore) {
        await db.restoreOnce(signal);
        db.ready = true;
      } else {
        db.restorePromise = (async () => {
          await db.restoreOnce();
          db.ready = true;
        })();
      }
    } else {
      db.ready = true;
    }

    mgr.start();
    return db;
  }

  private async restoreOnce(signal?: AbortSignal): Promise<void> {
    if (!this.recovery) return;
    try {
      const data = await this.recovery.load(signal);
      if (!data || data.length === 0) return;
      await restoreInto(this.state, data, signal);
    } catch (e) {
      // Missing / unreadable / corrupt → empty State, continue (not fatal).
      if (
        e === ErrRecoveryNotFound ||
        (e instanceof Error && e.message.includes("recovery: not found"))
      ) {
        return;
      }
      return;
    }
  }

  private guard(signal?: AbortSignal): void {
    throwIfAborted(signal);
    if (this.closed) throw ErrClosed;
  }

  async set(key: string, value: JsonRaw, signal?: AbortSignal): Promise<void> {
    this.guard(signal);
    return this.state.set(key, value, signal);
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    this.guard(signal);
    return this.state.get(key, signal);
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    this.guard(signal);
    return this.state.delete(key, signal);
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    this.guard(signal);
    return this.state.exists(key, signal);
  }

  async clear(signal?: AbortSignal): Promise<void> {
    this.guard(signal);
    return this.state.clear(signal);
  }

  async export(signal?: AbortSignal): Promise<Map<string, JsonRaw>> {
    this.guard(signal);
    return this.state.export(signal);
  }

  async replace(
    entries: Map<string, JsonRaw> | Record<string, JsonRaw>,
    signal?: AbortSignal,
  ): Promise<void> {
    this.guard(signal);
    return this.state.replace(entries, signal);
  }

  async beginTx(signal?: AbortSignal): Promise<Tx> {
    this.guard(signal);
    return this.state.beginTx(signal);
  }

  isReady(): boolean {
    return this.ready;
  }

  /** Alias matching Go `Ready()`. */
  readyFlag(): boolean {
    return this.ready;
  }

  get Ready(): boolean {
    return this.ready;
  }

  async close(): Promise<void> {
    if (this.closed) return;
    this.closed = true;
    this.snap.stop();
    await this.restorePromise;
    if (this.recovery?.close) {
      try {
        await this.recovery.close();
      } catch {
        /* ignore */
      }
    }
    await this.state.close();
  }

  /** Test-only explicit Snapshot (Go SnapshotNow). */
  async snapshotNow(signal?: AbortSignal): Promise<void> {
    this.guard(signal);
    await this.snap.take(signal);
  }

  /** Test helper: wait for async restore. */
  async waitRestore(): Promise<void> {
    await this.restorePromise;
  }

  /** @internal test helper */
  stateForTest(): Store {
    return this.state;
  }

  /** @internal test helper */
  recoveryForTest(): RecoveryStore | null {
    return this.recovery;
  }
}

async function openDefaultState(
  eff: Options,
  signal?: AbortSignal,
): Promise<Store> {
  if (eff.memory_only) return newMemory();
  if (isRedisStateBackend(eff)) {
    return openRedis(eff.state_redis_url, eff.state_redis_key_prefix, signal);
  }
  if (isFirestoreStateBackend(eff)) {
    return openFirestore(
      eff.state_firestore_project_id,
      eff.state_firestore_database_id,
      eff.state_firestore_collection,
      signal,
    );
  }
  if (eff.state_backend === StateBackendMemory) return newMemory();
  if (eff.state_path !== "" || eff.state_backend === StateBackendSQLite) {
    return openSQLite(eff.state_path);
  }
  return newMemory();
}

export async function open(options: Options, signal?: AbortSignal): Promise<DB> {
  return DB.open(options, signal);
}

export async function openWith(
  cfg: OpenConfig,
  signal?: AbortSignal,
): Promise<DB> {
  return DB.openWith(cfg, signal);
}

export { ErrNotFound, isNotFound };
