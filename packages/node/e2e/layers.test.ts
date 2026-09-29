import { describe, expect, it } from "vitest";
import { randomUUID } from "node:crypto";
import { mkdirSync, rmSync } from "node:fs";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { openWith } from "../src/db.js";
import { defaults } from "../src/options/index.js";
import { ErrNotFound, isEs4Error } from "../src/errors.js";

const layer = process.env.ES4_E2E_LAYER ?? "all";

function want(l: string): boolean {
  return layer === "all" || layer === l;
}

function requireOrSkip(
  envName: string,
  requireFlag: string,
): string | undefined {
  const v = process.env[envName];
  if (v && v !== "") return v;
  if (process.env[requireFlag] === "1") {
    throw new Error(`${envName} required when ${requireFlag}=1`);
  }
  return undefined;
}

describe.skipIf(!want("memory"))("e2e.node.memory — C2/C4/C5", () => {
  it("C2 MultiKeyOps", async () => {
    const db = await openWith({
      options: { ...defaults(), memory_only: true },
      disableRestore: true,
    });
    await db.set("plain", `"p"`);
    await db.set("a/b/c", `{"h":1}`);
    await db.set("x/y", `42`);
    const tx = await db.beginTx();
    await tx.set("txk", `"committed"`);
    await tx.commit();
    expect(await db.get("plain")).toBe(`"p"`);
    expect(await db.get("a/b/c")).toBe(`{"h":1}`);
    expect(await db.get("x/y")).toBe(`42`);
    expect(await db.get("txk")).toBe(`"committed"`);
    await db.close();
  });

  it("C4 EmptyStartup", async () => {
    const db = await openWith({
      options: { ...defaults(), memory_only: true },
      disableRestore: true,
    });
    expect(await db.exists("any")).toBe(false);
    await db.set("after", `1`);
    expect(await db.get("after")).toBe(`1`);
    await db.close();
  });

  it("C5 CleanupIdempotent", async () => {
    const db = await openWith({
      options: { ...defaults(), memory_only: true },
      disableRestore: true,
    });
    await db.clear();
    await db.clear();
    await db.close();
  });
});

describe.skipIf(!want("file"))("e2e.node.file — File SQLite C1–C5", () => {
  async function fixture() {
    const dir = await mkdtemp(join(tmpdir(), "es4-e2e-file-"));
    const statePath = join(dir, "state.db");
    const recoveryPath = join(dir, "recovery", "rp.snap");
    mkdirSync(join(dir, "recovery"), { recursive: true });
    const opts = {
      ...defaults(),
      state_path: statePath,
      recovery_backend: "file",
      recovery_path: recoveryPath,
      restore_on_startup: true,
      snapshot_interval: 0,
    };
    const clear = () => {
      for (const p of [
        statePath,
        statePath + "-wal",
        statePath + "-shm",
        recoveryPath,
      ]) {
        rmSync(p, { force: true });
      }
      rmSync(join(dir, "recovery"), { recursive: true, force: true });
      mkdirSync(join(dir, "recovery"), { recursive: true });
    };
    clear();
    return { dir, statePath, recoveryPath, opts, clear };
  }

  it("C1 SaveRestartRestore", async () => {
    const f = await fixture();
    const db1 = await openWith({ options: f.opts, skipAsyncRestore: true });
    await db1.set("hello", `{"n":1}`);
    await db1.snapshotNow();
    await db1.close();
    rmSync(f.statePath, { force: true });
    rmSync(f.statePath + "-wal", { force: true });
    rmSync(f.statePath + "-shm", { force: true });
    const db2 = await openWith({ options: f.opts, skipAsyncRestore: true });
    expect(await db2.get("hello")).toBe(`{"n":1}`);
    await db2.close();
    f.clear();
    rmSync(f.dir, { recursive: true, force: true });
  });

  it("C2 MultiKeyHierarchy", async () => {
    const f = await fixture();
    const db1 = await openWith({ options: f.opts, skipAsyncRestore: true });
    await db1.set("plain", `"p"`);
    await db1.set("a/b/c", `{"h":1}`);
    await db1.snapshotNow();
    await db1.close();
    rmSync(f.statePath, { force: true });
    const db2 = await openWith({ options: f.opts, skipAsyncRestore: true });
    expect(await db2.get("plain")).toBe(`"p"`);
    expect(await db2.get("a/b/c")).toBe(`{"h":1}`);
    await db2.close();
    f.clear();
    rmSync(f.dir, { recursive: true, force: true });
  });

  it("C3 UnsavedNotRestored", async () => {
    const f = await fixture();
    const db1 = await openWith({ options: f.opts, skipAsyncRestore: true });
    await db1.set("saved", `1`);
    await db1.snapshotNow();
    await db1.set("unsaved", `2`);
    await db1.close();
    rmSync(f.statePath, { force: true });
    const db2 = await openWith({ options: f.opts, skipAsyncRestore: true });
    expect(await db2.get("saved")).toBe(`1`);
    await expect(db2.get("unsaved")).rejects.toSatisfy((e) =>
      isEs4Error(e, ErrNotFound),
    );
    await db2.close();
    f.clear();
    rmSync(f.dir, { recursive: true, force: true });
  });

  it("C4 EmptyStartup", async () => {
    const f = await fixture();
    const db = await openWith({ options: f.opts, skipAsyncRestore: true });
    expect(await db.exists("x")).toBe(false);
    await db.set("x", `1`);
    expect(await db.get("x")).toBe(`1`);
    await db.close();
    f.clear();
    rmSync(f.dir, { recursive: true, force: true });
  });

  it("C5 CleanupIdempotent", async () => {
    const f = await fixture();
    f.clear();
    f.clear();
    rmSync(f.dir, { recursive: true, force: true });
  });
});

describe.skipIf(!want("libsql"))("e2e.node.libsql — File C1–C5 + Memory C2/C4/C5", () => {
  it("LibSQLFile C1 SaveRestartRestore", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-e2e-libsql-"));
    const rec = join(dir, "recovery.db");
    const opts = {
      ...defaults(),
      recovery_backend: "libsql",
      recovery_libsql_url: `file:${rec}`,
      restore_on_startup: true,
      snapshot_interval: 0,
      state_backend: "memory",
    };
    const db1 = await openWith({ options: opts, skipAsyncRestore: true });
    await db1.set("k", `{"libsql":true}`);
    await db1.snapshotNow();
    await db1.close();
    const db2 = await openWith({ options: opts, skipAsyncRestore: true });
    expect(await db2.get("k")).toBe(`{"libsql":true}`);
    await db2.close();
    rmSync(dir, { recursive: true, force: true });
  });

  it("LibSQLFile C2 MultiKey", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-e2e-libsql-"));
    const rec = join(dir, "recovery.db");
    const opts = {
      ...defaults(),
      recovery_backend: "libsql",
      recovery_libsql_url: `file:${rec}`,
      restore_on_startup: true,
      snapshot_interval: 0,
    };
    const db1 = await openWith({ options: opts, skipAsyncRestore: true });
    await db1.set("a/b", `1`);
    await db1.set("c", `"x"`);
    await db1.snapshotNow();
    await db1.close();
    const db2 = await openWith({ options: opts, skipAsyncRestore: true });
    expect(await db2.get("a/b")).toBe(`1`);
    expect(await db2.get("c")).toBe(`"x"`);
    await db2.close();
    rmSync(dir, { recursive: true, force: true });
  });

  it("LibSQLFile C3 UnsavedNotRestored", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-e2e-libsql-"));
    const rec = join(dir, "recovery.db");
    const opts = {
      ...defaults(),
      recovery_backend: "libsql",
      recovery_libsql_url: `file:${rec}`,
      restore_on_startup: true,
      snapshot_interval: 0,
    };
    const db1 = await openWith({ options: opts, skipAsyncRestore: true });
    await db1.set("saved", `1`);
    await db1.snapshotNow();
    await db1.set("unsaved", `2`);
    await db1.close();
    const db2 = await openWith({ options: opts, skipAsyncRestore: true });
    expect(await db2.get("saved")).toBe(`1`);
    await expect(db2.get("unsaved")).rejects.toSatisfy((e) =>
      isEs4Error(e, ErrNotFound),
    );
    await db2.close();
    rmSync(dir, { recursive: true, force: true });
  });

  it("LibSQLFile C4 EmptyStartup", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-e2e-libsql-"));
    const opts = {
      ...defaults(),
      recovery_backend: "libsql",
      recovery_libsql_url: `file:${join(dir, "empty.db")}`,
      restore_on_startup: true,
      snapshot_interval: 0,
    };
    const db = await openWith({ options: opts, skipAsyncRestore: true });
    expect(await db.exists("x")).toBe(false);
    await db.close();
    rmSync(dir, { recursive: true, force: true });
  });

  it("LibSQLFile C5 Cleanup", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-e2e-libsql-"));
    rmSync(dir, { recursive: true, force: true });
    rmSync(dir, { recursive: true, force: true });
  });

  it("LibSQLMemory C2/C4/C5", async () => {
    const opts = {
      ...defaults(),
      recovery_backend: "libsql",
      recovery_libsql_url: ":memory:",
      restore_on_startup: false,
      snapshot_interval: 0,
    };
    const db = await openWith({
      options: opts,
      disableRestore: true,
      skipAsyncRestore: true,
    });
    await db.set("a/b", `1`);
    await db.snapshotNow();
    expect(await db.get("a/b")).toBe(`1`);
    await db.clear();
    await db.clear();
    await db.close();
  });
});

describe.skipIf(!want("object"))("e2e.node.object — Object Recovery §5.1–§5.5", () => {
  const endpoint =
    process.env.ES4_E2E_S3_ENDPOINT || "http://127.0.0.1:9000";

  async function probe(): Promise<boolean> {
    try {
      const r = await fetch(`${endpoint}/health`, {
        signal: AbortSignal.timeout(2000),
      });
      return r.ok;
    } catch {
      return false;
    }
  }

  it("C1–C5 Object Recovery", async () => {
    const ok = await probe();
    if (!ok) {
      if (process.env.ES4_E2E_REQUIRE_RUSTFS === "1") {
        throw new Error(`RustFS not reachable at ${endpoint}`);
      }
      return; // skip locally
    }
    process.env.AWS_ACCESS_KEY_ID ||=
      process.env.ES4_E2E_S3_ACCESS_KEY || "es4e2eaccess";
    process.env.AWS_SECRET_ACCESS_KEY ||=
      process.env.ES4_E2E_S3_SECRET_KEY || "es4e2esecretkey";
    const bucket = `es4-e2e-${randomUUID().slice(0, 8)}`;
    const prefix = `e2e/${randomUUID()}/`;
    const opts = {
      ...defaults(),
      recovery_backend: "object",
      recovery_s3_bucket: bucket,
      recovery_s3_prefix: prefix,
      recovery_s3_endpoint: endpoint,
      recovery_s3_region: process.env.ES4_E2E_S3_REGION || "us-east-1",
      restore_on_startup: true,
      snapshot_interval: 0,
    };

    // C1
    const db1 = await openWith({ options: opts, skipAsyncRestore: true });
    await db1.set("hello", `{"obj":1}`);
    await db1.snapshotNow();
    await db1.close();
    const db2 = await openWith({ options: opts, skipAsyncRestore: true });
    expect(await db2.get("hello")).toBe(`{"obj":1}`);

    // C2
    await db2.set("a/b/c", `{"h":1}`);
    await db2.snapshotNow();
    await db2.close();
    const db3 = await openWith({ options: opts, skipAsyncRestore: true });
    expect(await db3.get("a/b/c")).toBe(`{"h":1}`);

    // C3
    await db3.set("unsaved", `9`);
    await db3.close();
    const db4 = await openWith({ options: opts, skipAsyncRestore: true });
    await expect(db4.get("unsaved")).rejects.toSatisfy((e) =>
      isEs4Error(e, ErrNotFound),
    );
    await db4.close();

    // C4 empty bucket (new prefix)
    const optsEmpty = {
      ...opts,
      recovery_s3_prefix: `e2e-empty/${randomUUID()}/`,
    };
    const dbEmpty = await openWith({
      options: optsEmpty,
      skipAsyncRestore: true,
    });
    expect(await dbEmpty.exists("x")).toBe(false);
    await dbEmpty.close();

    // C5 cleanup idempotent — open/close twice
    await openWith({ options: opts, skipAsyncRestore: true }).then((d) =>
      d.close(),
    );
    await openWith({ options: opts, skipAsyncRestore: true }).then((d) =>
      d.close(),
    );
  });
});

describe.skipIf(!want("redis"))("e2e.node.redis — Redis State C1–C5", () => {
  it("C1–C5", async () => {
    const url = requireOrSkip("ES4_E2E_REDIS_URL", "ES4_E2E_REQUIRE_REDIS");
    if (!url) return;
    const prefix = `e2e/${Date.now()}/${randomUUID()}/`;
    const opts = {
      ...defaults(),
      state_backend: "redis",
      state_redis_url: url,
      state_redis_key_prefix: prefix,
      snapshot_interval: 0,
      restore_on_startup: false,
    };
    const open = () =>
      openWith({ options: opts, disableRestore: true, skipAsyncRestore: true });

    // C1
    const db1 = await open();
    await db1.set("hello", `{"redis":1}`);
    await db1.close();
    const db2 = await open();
    expect(await db2.get("hello")).toBe(`{"redis":1}`);

    // C2
    await db2.set("a/b/c", `{"h":1}`);
    await db2.set("x/y", `42`);
    const tx = await db2.beginTx();
    await tx.set("txk", `"ok"`);
    await tx.commit();
    await db2.close();
    const db3 = await open();
    expect(await db3.get("a/b/c")).toBe(`{"h":1}`);
    expect(await db3.get("txk")).toBe(`"ok"`);

    // C3 rollback not restored
    const tx2 = await db3.beginTx();
    await tx2.set("rolled", `1`);
    await tx2.rollback();
    await db3.close();
    const db4 = await open();
    await expect(db4.get("rolled")).rejects.toSatisfy((e) =>
      isEs4Error(e, ErrNotFound),
    );

    // C4
    await db4.clear();
    expect(await db4.exists("hello")).toBe(false);
    await db4.set("again", `1`);
    expect(await db4.get("again")).toBe(`1`);

    // C5
    await db4.clear();
    await db4.clear();
    await db4.close();
  });
});

describe.skipIf(!want("valkey"))("e2e.node.valkey — Valkey State C1–C5", () => {
  it("C1–C5", async () => {
    const url = requireOrSkip("ES4_E2E_VALKEY_URL", "ES4_E2E_REQUIRE_VALKEY");
    if (!url) return;
    const prefix = `e2e/${Date.now()}/${randomUUID()}/`;
    const opts = {
      ...defaults(),
      state_backend: "valkey",
      state_redis_url: url,
      state_redis_key_prefix: prefix,
      snapshot_interval: 0,
      restore_on_startup: false,
    };
    const open = () =>
      openWith({ options: opts, disableRestore: true, skipAsyncRestore: true });
    const db1 = await open();
    await db1.set("hello", `{"valkey":1}`);
    await db1.close();
    const db2 = await open();
    expect(await db2.get("hello")).toBe(`{"valkey":1}`);
    await db2.set("a/b/c", `1`);
    await db2.close();
    const db3 = await open();
    expect(await db3.get("a/b/c")).toBe(`1`);
    const tx = await db3.beginTx();
    await tx.set("r", `1`);
    await tx.rollback();
    await db3.close();
    const db4 = await open();
    await expect(db4.get("r")).rejects.toSatisfy((e) =>
      isEs4Error(e, ErrNotFound),
    );
    await db4.clear();
    await db4.clear();
    await db4.close();
  });
});

describe.skipIf(!want("firestore"))("e2e.node.firestore — Firestore State C1–C5 + N16/N17", () => {
  it("C1–C5 + collection isolation", async () => {
    const host = process.env.FIRESTORE_EMULATOR_HOST;
    if (!host) {
      if (process.env.ES4_E2E_REQUIRE_FIRESTORE === "1") {
        throw new Error("FIRESTORE_EMULATOR_HOST required");
      }
      return;
    }
    const project =
      process.env.ES4_E2E_FIRESTORE_PROJECT_ID || "demo-es4";
    const collection = `e2e_${randomUUID().replace(/-/g, "").slice(0, 16)}`;
    const other = `other_${randomUUID().replace(/-/g, "").slice(0, 16)}`;
    const opts = {
      ...defaults(),
      state_backend: "firestore",
      state_firestore_project_id: project,
      state_firestore_collection: collection,
      snapshot_interval: 0,
      restore_on_startup: false,
    };
    const open = () =>
      openWith({ options: opts, disableRestore: true, skipAsyncRestore: true });

    const db1 = await open();
    await db1.set("hello", `{"fs":1}`);
    await db1.close();
    const db2 = await open();
    expect(await db2.get("hello")).toBe(`{"fs":1}`);
    await db2.set("x/y/z", `{"h":1}`);
    expect(await db2.get("x/y/z")).toBe(`{"h":1}`);
    await db2.close();

    // sibling collection survives Clear (S-N16)
    const otherDb = await openWith({
      options: { ...opts, state_firestore_collection: other },
      disableRestore: true,
    });
    await otherDb.set("keep", `"yes"`);
    await otherDb.close();

    const db3 = await open();
    await db3.clear();
    expect(await db3.exists("hello")).toBe(false);
    await db3.close();

    const otherDb2 = await openWith({
      options: { ...opts, state_firestore_collection: other },
      disableRestore: true,
    });
    expect(await otherDb2.get("keep")).toBe(`"yes"`);

    // empty Replace (S-N17)
    await otherDb2.replace(new Map());
    expect(await otherDb2.exists("keep")).toBe(false);
    // recreate keep, clear primary via empty replace — other untouched was cleared itself;
    // re-seed other and prove primary empty replace doesn't touch it.
    await otherDb2.set("keep2", `"y"`);
    await otherDb2.close();

    const db4 = await open();
    await db4.set("tmp", `1`);
    await db4.replace(new Map());
    expect(await db4.exists("tmp")).toBe(false);
    await db4.close();

    const otherDb3 = await openWith({
      options: { ...opts, state_firestore_collection: other },
      disableRestore: true,
    });
    expect(await otherDb3.get("keep2")).toBe(`"y"`);
    await otherDb3.clear();
    await otherDb3.clear();
    await otherDb3.close();
  });
});
