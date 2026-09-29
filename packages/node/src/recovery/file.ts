import {
  mkdirSync,
  readdirSync,
  readFileSync,
  renameSync,
  rmSync,
  writeFileSync,
  existsSync,
  openSync,
  closeSync,
  fsyncSync,
  unlinkSync,
} from "node:fs";
import { dirname, join } from "node:path";
import {
  assertNotAborted,
  GEN_FILE_PREFIX,
  GEN_FILE_SUFFIX,
  genID,
  keepGeneration,
  notFound,
  parseGenID,
  type RecoveryStore,
} from "./types.js";

export class FileRecovery implements RecoveryStore {
  constructor(
    readonly path: string,
    readonly ttlMs: number = 0,
    private readonly clock: () => Date = () => new Date(),
  ) {}

  async save(data: Uint8Array, signal?: AbortSignal): Promise<void> {
    assertNotAborted(signal);
    if (this.path === "") throw new Error("recovery: empty path");
    if (this.ttlMs <= 0) {
      this.saveSingle(data);
      return;
    }
    this.saveGeneration(data);
    this.pruneGenerations();
  }

  private saveSingle(data: Uint8Array): void {
    const dir = dirname(this.path);
    mkdirSync(dir, { recursive: true });
    const tmp = join(dir, `.es4-recovery-${process.pid}-${Date.now()}.tmp`);
    try {
      const fd = openSync(tmp, "w");
      try {
        writeFileSync(fd, data);
        fsyncSync(fd);
      } finally {
        closeSync(fd);
      }
      renameSync(tmp, this.path);
    } catch (e) {
      try {
        unlinkSync(tmp);
      } catch {
        /* ignore */
      }
      throw e;
    }
  }

  private saveGeneration(data: Uint8Array): void {
    mkdirSync(this.path, { recursive: true });
    const id = genID(this.clock());
    const dest = join(this.path, GEN_FILE_PREFIX + id + GEN_FILE_SUFFIX);
    const tmp = join(
      this.path,
      `.es4-recovery-${process.pid}-${Date.now()}.tmp`,
    );
    try {
      const fd = openSync(tmp, "w");
      try {
        writeFileSync(fd, data);
        fsyncSync(fd);
      } finally {
        closeSync(fd);
      }
      renameSync(tmp, dest);
    } catch (e) {
      try {
        unlinkSync(tmp);
      } catch {
        /* ignore */
      }
      throw e;
    }
  }

  private listGenerations(): string[] {
    if (!existsSync(this.path)) return [];
    const names = readdirSync(this.path);
    const ids: string[] = [];
    for (const name of names) {
      if (!name.startsWith(GEN_FILE_PREFIX) || !name.endsWith(GEN_FILE_SUFFIX)) {
        continue;
      }
      const id = name.slice(
        GEN_FILE_PREFIX.length,
        name.length - GEN_FILE_SUFFIX.length,
      );
      try {
        parseGenID(id);
        ids.push(id);
      } catch {
        /* skip */
      }
    }
    ids.sort();
    return ids;
  }

  private pruneGenerations(): void {
    const gens = this.listGenerations();
    if (gens.length === 0) return;
    const now = this.clock();
    const newest = gens[gens.length - 1]!;
    for (const id of gens) {
      if (id === newest) continue;
      const created = parseGenID(id);
      if (keepGeneration(created, now, this.ttlMs)) continue;
      rmSync(join(this.path, GEN_FILE_PREFIX + id + GEN_FILE_SUFFIX), {
        force: true,
      });
    }
  }

  async load(signal?: AbortSignal): Promise<Uint8Array> {
    assertNotAborted(signal);
    if (this.path === "") notFound();
    if (this.ttlMs <= 0) {
      if (!existsSync(this.path)) notFound();
      return new Uint8Array(readFileSync(this.path));
    }
    const gens = this.listGenerations();
    if (gens.length === 0) notFound();
    const newest = gens[gens.length - 1]!;
    const file = join(this.path, GEN_FILE_PREFIX + newest + GEN_FILE_SUFFIX);
    if (!existsSync(file)) notFound();
    return new Uint8Array(readFileSync(file));
  }
}

export function newFileTTL(path: string, ttlMs = 0): FileRecovery {
  return new FileRecovery(path, ttlMs);
}
