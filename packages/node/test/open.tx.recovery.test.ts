import { describe, expect, it } from "vitest";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { open, openWith } from "../src/db.js";
import { defaults } from "../src/options/index.js";
import { newMemory } from "../src/state/index.js";
import { ErrClosed, ErrNestedTx, isEs4Error } from "../src/errors.js";

describe("Open wiring W-N*", () => {
  it("W-N memory_only", async () => {
    const db = await open({ ...defaults(), memory_only: true });
    await db.set("k", `1`);
    expect(await db.get("k")).toBe(`1`);
    expect(db.Ready).toBe(true);
    await db.close();
  });

  it("W-N state_path SQLite", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-open-"));
    const db = await open({
      ...defaults(),
      state_path: join(dir, "s.db"),
      restore_on_startup: false,
      snapshot_interval: 0,
    });
    await db.set("k", `"sqlite"`);
    await db.close();
    const db2 = await open({
      ...defaults(),
      state_path: join(dir, "s.db"),
      restore_on_startup: false,
      snapshot_interval: 0,
    });
    expect(await db2.get("k")).toBe(`"sqlite"`);
    await db2.close();
  });

  it("W-N injection wins", async () => {
    const mem = newMemory();
    await mem.set("inj", `true`);
    const db = await openWith({
      options: { ...defaults(), state_path: "/tmp/should-not-use.db" },
      state: mem,
      disableRestore: true,
    });
    expect(await db.get("inj")).toBe(`true`);
    await db.close();
  });

  it("W-N state_backend=memory", async () => {
    const db = await open({
      ...defaults(),
      state_backend: "memory",
      state_path: "/tmp/ignored-when-memory-backend.db",
      restore_on_startup: false,
      snapshot_interval: 0,
    });
    // path ignored because state_backend=memory takes priority after redis/firestore
    await db.set("m", `1`);
    expect(await db.get("m")).toBe(`1`);
    await db.close();
  });
});

describe("Tx T-*", () => {
  it("T-N1 Commit visible", async () => {
    const db = await open({ ...defaults(), memory_only: true });
    const tx = await db.beginTx();
    await tx.set("a", `1`);
    await tx.commit();
    expect(await db.get("a")).toBe(`1`);
    await db.close();
  });

  it("T-N2 Rollback not visible", async () => {
    const db = await open({ ...defaults(), memory_only: true });
    const tx = await db.beginTx();
    await tx.set("a", `1`);
    await tx.rollback();
    expect(await db.exists("a")).toBe(false);
    await db.close();
  });

  it("T-E1 nested", async () => {
    const db = await open({ ...defaults(), memory_only: true });
    await db.beginTx();
    await expect(db.beginTx()).rejects.toSatisfy((e) =>
      isEs4Error(e, ErrNestedTx),
    );
    await db.close();
  });

  it("T-E2 Begin after Close", async () => {
    const db = await open({ ...defaults(), memory_only: true });
    await db.close();
    await expect(db.beginTx()).rejects.toSatisfy((e) =>
      isEs4Error(e, ErrClosed),
    );
  });
});

describe("Recovery R-N1 File", () => {
  it("Save → re-Open Restore", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-rec-"));
    const statePath = join(dir, "state.db");
    const recoveryPath = join(dir, "recovery.snap");
    const opts = {
      ...defaults(),
      state_path: statePath,
      recovery_backend: "file",
      recovery_path: recoveryPath,
      restore_on_startup: true,
      snapshot_interval: 0,
    };
    const db1 = await openWith({
      options: opts,
      skipAsyncRestore: true,
    });
    await db1.set("hello", `{"n":1}`);
    await db1.snapshotNow();
    await db1.close();

    // Remove state so restore must come from Recovery.
    const { rmSync } = await import("node:fs");
    rmSync(statePath, { force: true });
    rmSync(statePath + "-wal", { force: true });
    rmSync(statePath + "-shm", { force: true });

    const db2 = await openWith({
      options: opts,
      skipAsyncRestore: true,
    });
    expect(await db2.get("hello")).toBe(`{"n":1}`);
    await db2.close();
  });
});

describe("Recovery R-N6 memory_only", () => {
  it("no recovery R/W", async () => {
    const db = await open({
      ...defaults(),
      memory_only: true,
      recovery_path: "/tmp/should-ignore",
      recovery_backend: "file",
    });
    expect(db.recoveryForTest()).toBeNull();
    await db.set("k", `1`);
    await db.close();
  });
});
