import { Firestore } from "@google-cloud/firestore";
import {
  ErrClosed,
  ErrFirestoreDocIDTooLong,
  ErrNestedTx,
  ErrNotFound,
  ErrTxDone,
  throwIfAborted,
  wrapError,
} from "../errors.js";
import { DEFAULT_FIRESTORE_DATABASE_ID } from "../options/types.js";
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

const FIRESTORE_MAX_DOC_ID_BYTES = 1500;

/**
 * Path-percent-encode matching Go `url.PathEscape` for typical keys.
 * Especially: "/" → "%2F".
 */
export function encodeFirestoreDocID(logicalKey: string): string {
  // Go url.PathEscape encodes each path segment; "/" between segments → %2F.
  const id = logicalKey
    .split("/")
    .map((seg) => encodeURIComponent(seg).replace(/[!'()*]/g, (c) => {
      // encodeURIComponent leaves ! ' ( ) * unescaped; PathEscape encodes them.
      return "%" + c.charCodeAt(0).toString(16).toUpperCase().padStart(2, "0");
    }))
    .join("%2F");
  const bytes = Buffer.byteLength(id, "utf8");
  if (bytes > FIRESTORE_MAX_DOC_ID_BYTES) {
    throw wrapError(
      ErrFirestoreDocIDTooLong,
      `${bytes} bytes (max ${FIRESTORE_MAX_DOC_ID_BYTES})`,
    );
  }
  return id;
}

export function decodeFirestoreDocID(docID: string): string {
  try {
    return docID
      .split("%2F")
      .map((seg) => decodeURIComponent(seg))
      .join("/");
  } catch (e) {
    throw new Error(`state: decode firestore doc id: ${(e as Error).message}`);
  }
}

function validateCollectionID(id: string): void {
  if (id === "") throw new Error("state: empty firestore collection");
  if (id.includes("/")) {
    throw new Error("state: firestore collection must not contain '/'");
  }
  if (id === "." || id === "..") {
    throw new Error(`state: invalid firestore collection ${JSON.stringify(id)}`);
  }
}

export interface FirestoreConfig {
  projectId: string;
  databaseId?: string;
  collection: string;
  client?: Firestore;
  ownsClient?: boolean;
}

export class FirestoreStore implements Store {
  private readonly mu = new AsyncMutex();
  private closed = false;
  private active: FirestoreOverlayTx | null = null;
  private readonly client: Firestore;
  private readonly owns: boolean;
  readonly projectId: string;
  readonly databaseId: string;
  readonly collection: string;

  private constructor(
    client: Firestore,
    projectId: string,
    databaseId: string,
    collection: string,
    owns: boolean,
  ) {
    this.client = client;
    this.projectId = projectId;
    this.databaseId = databaseId;
    this.collection = collection;
    this.owns = owns;
  }

  static async open(
    projectId: string,
    databaseId: string,
    collection: string,
    signal?: AbortSignal,
  ): Promise<FirestoreStore> {
    return FirestoreStore.openConfig(
      { projectId, databaseId, collection },
      signal,
    );
  }

  static async openConfig(
    cfg: FirestoreConfig,
    signal?: AbortSignal,
  ): Promise<FirestoreStore> {
    throwIfAborted(signal);
    if (!cfg.projectId) throw new Error("state: empty firestore project id");
    validateCollectionID(cfg.collection);
    const dbID =
      cfg.databaseId && cfg.databaseId !== ""
        ? cfg.databaseId
        : DEFAULT_FIRESTORE_DATABASE_ID;
    let client = cfg.client;
    let owns = cfg.ownsClient ?? false;
    if (!client) {
      client = new Firestore({
        projectId: cfg.projectId,
        databaseId: dbID,
        ignoreUndefinedProperties: true,
      });
      owns = true;
    }
    return new FirestoreStore(
      client,
      cfg.projectId,
      dbID,
      cfg.collection,
      owns,
    );
  }

  private coll() {
    return this.client.collection(this.collection);
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

  /** Clear only this configured collection (S-N16/N17). */
  private async clearCollection(): Promise<void> {
    const snap = await this.coll().get();
    if (snap.empty) return;
    const batchSize = 400;
    let batch = this.client.batch();
    let n = 0;
    for (const doc of snap.docs) {
      batch.delete(doc.ref);
      n++;
      if (n >= batchSize) {
        await batch.commit();
        batch = this.client.batch();
        n = 0;
      }
    }
    if (n > 0) await batch.commit();
  }

  async set(key: string, value: JsonRaw, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    validateValue(value);
    const docID = encodeFirestoreDocID(key);
    await this.withNoTx(async () => {
      await this.coll().doc(docID).set({ value });
    });
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    const docID = encodeFirestoreDocID(key);
    return await this.withNoTx(async () => {
      const snap = await this.coll().doc(docID).get();
      if (!snap.exists) throw ErrNotFound;
      const data = snap.data() as { value?: string } | undefined;
      if (data?.value === undefined) throw ErrNotFound;
      return deepCopyRaw(data.value);
    });
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    const docID = encodeFirestoreDocID(key);
    await this.withNoTx(async () => {
      const ref = this.coll().doc(docID);
      const snap = await ref.get();
      if (!snap.exists) throw ErrNotFound;
      await ref.delete();
    });
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    const docID = encodeFirestoreDocID(key);
    return await this.withNoTx(async () => {
      const snap = await this.coll().doc(docID).get();
      return snap.exists;
    });
  }

  async clear(signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    await this.withNoTx(async () => {
      await this.clearCollection();
    });
  }

  async export(signal?: AbortSignal): Promise<Map<string, JsonRaw>> {
    throwIfAborted(signal);
    return await this.withNoTx(async () => {
      const snap = await this.coll().get();
      const out = new Map<string, JsonRaw>();
      for (const doc of snap.docs) {
        const logical = decodeFirestoreDocID(doc.id);
        const data = doc.data() as { value?: string };
        if (data.value !== undefined) out.set(logical, deepCopyRaw(data.value));
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
    const encoded: Array<{ id: string; value: JsonRaw }> = [];
    for (const [k, v] of map) {
      validateKey(k);
      validateValue(v);
      encoded.push({ id: encodeFirestoreDocID(k), value: deepCopyRaw(v) });
    }
    await this.withNoTx(async () => {
      await this.clearCollection();
      const batchSize = 400;
      let batch = this.client.batch();
      let n = 0;
      for (const e of encoded) {
        batch.set(this.coll().doc(e.id), { value: e.value });
        n++;
        if (n >= batchSize) {
          await batch.commit();
          batch = this.client.batch();
          n = 0;
        }
      }
      if (n > 0) await batch.commit();
    });
  }

  async beginTx(signal?: AbortSignal): Promise<Tx> {
    throwIfAborted(signal);
    return await this.mu.withLock(() => {
      if (this.closed) throw ErrClosed;
      if (this.active) throw ErrNestedTx;
      const tx = new FirestoreOverlayTx(this);
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
        if (this.owns) await this.client.terminate();
      }
    });
  }

  /** @internal */
  _client(): Firestore {
    return this.client;
  }
  /** @internal */
  _coll() {
    return this.coll();
  }
  /** @internal */
  async _clearCollection(): Promise<void> {
    await this.clearCollection();
  }
  /** @internal */
  _clearActive(tx: FirestoreOverlayTx): void {
    if (this.active === tx) this.active = null;
  }
  /** @internal */
  _isClosed(): boolean {
    return this.closed;
  }
  /** @internal */
  _isActive(tx: FirestoreOverlayTx): boolean {
    return this.active === tx;
  }
  /** @internal */
  _mu(): AsyncMutex {
    return this.mu;
  }
}

class FirestoreOverlayTx implements Tx {
  private overlay = new Map<string, OverlayEntry>();
  private cleared = false;
  private done = false;

  constructor(private readonly f: FirestoreStore) {}

  markDone(): void {
    this.done = true;
  }

  private guard(): void {
    if (this.done || !this.f._isActive(this)) throw ErrTxDone;
    if (this.f._isClosed()) throw ErrClosed;
  }

  async set(key: string, value: JsonRaw, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    validateValue(value);
    await this.f._mu().withLock(() => {
      this.guard();
      this.overlay.set(key, { value: deepCopyRaw(value) });
    });
  }

  async get(key: string, signal?: AbortSignal): Promise<JsonRaw> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.f._mu().withLock(async () => {
      this.guard();
      const e = this.overlay.get(key);
      if (e) {
        if (e.deleted) throw ErrNotFound;
        return deepCopyRaw(e.value);
      }
      if (this.cleared) throw ErrNotFound;
      const snap = await this.f._coll().doc(encodeFirestoreDocID(key)).get();
      if (!snap.exists) throw ErrNotFound;
      const data = snap.data() as { value?: string };
      if (data.value === undefined) throw ErrNotFound;
      return deepCopyRaw(data.value);
    });
  }

  async delete(key: string, signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    validateKey(key);
    await this.f._mu().withLock(async () => {
      this.guard();
      const e = this.overlay.get(key);
      if (e) {
        if (e.deleted) throw ErrNotFound;
        this.overlay.set(key, { deleted: true });
        return;
      }
      if (this.cleared) throw ErrNotFound;
      const snap = await this.f._coll().doc(encodeFirestoreDocID(key)).get();
      if (!snap.exists) throw ErrNotFound;
      this.overlay.set(key, { deleted: true });
    });
  }

  async exists(key: string, signal?: AbortSignal): Promise<boolean> {
    throwIfAborted(signal);
    validateKey(key);
    return await this.f._mu().withLock(async () => {
      this.guard();
      const e = this.overlay.get(key);
      if (e) return !e.deleted;
      if (this.cleared) return false;
      const snap = await this.f._coll().doc(encodeFirestoreDocID(key)).get();
      return snap.exists;
    });
  }

  async clear(signal?: AbortSignal): Promise<void> {
    throwIfAborted(signal);
    await this.f._mu().withLock(() => {
      this.guard();
      this.cleared = true;
      this.overlay = new Map();
    });
  }

  async commit(): Promise<void> {
    await this.f._mu().withLock(async () => {
      this.guard();
      if (this.cleared) await this.f._clearCollection();
      const batchSize = 400;
      let batch = this.f._client().batch();
      let n = 0;
      const flush = async () => {
        if (n === 0) return;
        await batch.commit();
        batch = this.f._client().batch();
        n = 0;
      };
      for (const [k, e] of this.overlay) {
        const ref = this.f._coll().doc(encodeFirestoreDocID(k));
        if (e.deleted) batch.delete(ref);
        else batch.set(ref, { value: e.value });
        n++;
        if (n >= batchSize) await flush();
      }
      await flush();
      this.done = true;
      this.f._clearActive(this);
    });
  }

  async rollback(): Promise<void> {
    await this.f._mu().withLock(() => {
      if (this.done || !this.f._isActive(this)) throw ErrTxDone;
      this.done = true;
      this.f._clearActive(this);
    });
  }
}

export async function openFirestore(
  projectId: string,
  databaseId: string,
  collection: string,
  signal?: AbortSignal,
): Promise<FirestoreStore> {
  return FirestoreStore.open(projectId, databaseId, collection, signal);
}
