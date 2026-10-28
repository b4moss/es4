import { describe, expect, it, afterEach } from "vitest";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer } from "node:http";
import { ErrNotFound, isEs4Error } from "../src/errors.js";
import { DB } from "../src/db.js";
import { defaults } from "../src/options/index.js";
import {
  newMemory,
  openSQLite,
  openRedis,
  setNowFunc,
} from "../src/state/index.js";
import type { Store } from "../src/state/index.js";
import { newServer, CodeInvalidExpiry } from "../src/server/server.js";

afterEach(() => {
  setNowFunc(null);
});

async function withStores(
  fn: (name: string, open: () => Promise<Store>) => Promise<void>,
) {
  await fn("Memory", async () => newMemory());
  await fn("SQLite", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-exp-"));
    return openSQLite(join(dir, "t.db"));
  });
  // Redis via miniredis-like: skip if no REDIS; use ioredis-mock? Project uses real redis in e2e.
  // Contract for redis is covered when REDIS_URL set; otherwise Memory/SQLite only here.
  const url = process.env.ES4_TEST_REDIS_URL;
  if (url) {
    await fn("Redis", async () =>
      openRedis(url, `exp-test/${Date.now()}/`),
    );
  }
}

describe("expiresAt contract", () => {
  it("future get / past absent / clear expiry / export values-only", async () => {
    await withStores(async (name, open) => {
      const st = await open();
      try {
        const fixed = new Date("2030-06-15T12:00:00.000Z");
        setNowFunc(() => new Date(fixed));
        const future = new Date(fixed.getTime() + 3600_000);
        const past = new Date(fixed.getTime() - 3600_000);

        await st.set("ttl:future", `{"v":1}`, { expiresAt: future });
        expect(await st.get("ttl:future")).toBe(`{"v":1}`);
        expect(await st.exists("ttl:future")).toBe(true);

        await st.set("ttl:past", `{"v":1}`, { expiresAt: past });
        await expect(st.get("ttl:past")).rejects.toSatisfy((e) =>
          isEs4Error(e, ErrNotFound),
        );
        expect(await st.exists("ttl:past")).toBe(false);
        const scanned = await st.scan("ttl");
        expect(scanned).toContain("ttl:future");
        expect(scanned).not.toContain("ttl:past");

        await st.set("ttl:future", `{"v":2}`);
        setNowFunc(() => new Date(future.getTime() + 3600_000));
        expect(await st.get("ttl:future")).toBe(`{"v":2}`);
        setNowFunc(() => new Date(fixed));

        await st.clear();
        await st.set("exp:a", `1`, { expiresAt: future });
        await st.set("exp:b", `2`, { expiresAt: past });
        const exp = await st.export();
        expect(exp.has("exp:a")).toBe(true);
        expect(exp.has("exp:b")).toBe(false);
      } finally {
        await st.close();
      }
    });
  });

  it("Tx commit/rollback with expiresAt", async () => {
    const st = newMemory();
    try {
      const fixed = new Date("2031-01-01T00:00:00.000Z");
      setNowFunc(() => new Date(fixed));
      const future = new Date(fixed.getTime() + 3600_000);
      const past = new Date(fixed.getTime() - 60_000);

      const tx = await st.beginTx();
      await tx.set("tx:ttl", `{"tx":1}`, { expiresAt: future });
      expect(await tx.get("tx:ttl")).toBe(`{"tx":1}`);
      await tx.commit();
      expect(await st.get("tx:ttl")).toBe(`{"tx":1}`);

      const tx2 = await st.beginTx();
      await tx2.set("tx:roll", `1`, { expiresAt: future });
      await tx2.rollback();
      expect(await st.exists("tx:roll")).toBe(false);

      const tx3 = await st.beginTx();
      await tx3.set("tx:past", `1`, { expiresAt: past });
      await expect(tx3.get("tx:past")).rejects.toSatisfy((e) =>
        isEs4Error(e, ErrNotFound),
      );
      await tx3.rollback();
    } finally {
      await st.close();
    }
  });
});

describe("DB.set { expiresAt }", () => {
  it("public options and AbortSignal compat", async () => {
    const db = await DB.open({ ...defaults(), memory_only: true });
    try {
      const fixed = new Date("2032-03-01T10:00:00.000Z");
      setNowFunc(() => new Date(fixed));
      const future = new Date(fixed.getTime() + 1800_000);
      await db.set("a", `{"ok":true}`, { expiresAt: future });
      expect(await db.get("a")).toBe(`{"ok":true}`);

      const ac = new AbortController();
      ac.abort();
      await expect(db.set("x", `1`, ac.signal)).rejects.toThrow();
    } finally {
      await db.close();
    }
  });
});

describe("HTTP expires_at", () => {
  it("query sets expiry; invalid → 400 invalid_expiry; past → 404", async () => {
    const db = await DB.open({ ...defaults(), memory_only: true });
    const srv = newServer(db);
    const httpSrv = createServer(srv.handler());
    await new Promise<void>((resolve) => httpSrv.listen(0, "127.0.0.1", resolve));
    const addr = httpSrv.address();
    if (!addr || typeof addr === "string") throw new Error("no addr");
    const base = `http://127.0.0.1:${addr.port}`;

    try {
      const fixed = new Date("2033-04-01T00:00:00.000Z");
      setNowFunc(() => new Date(fixed));
      const future = new Date(fixed.getTime() + 3600_000).toISOString();
      const past = new Date(fixed.getTime() - 3600_000).toISOString();

      let res = await fetch(
        `${base}/v1/keys/exp?expires_at=${encodeURIComponent(future)}`,
        { method: "PUT", body: `{"e":1}`, headers: { "Content-Type": "application/json" } },
      );
      expect(res.status).toBe(200);
      res = await fetch(`${base}/v1/keys/exp`);
      expect(res.status).toBe(200);
      expect(await res.text()).toBe(`{"e":1}`);

      res = await fetch(
        `${base}/v1/keys/past?expires_at=${encodeURIComponent(past)}`,
        { method: "PUT", body: `{"e":2}`, headers: { "Content-Type": "application/json" } },
      );
      expect(res.status).toBe(200);
      res = await fetch(`${base}/v1/keys/past`);
      expect(res.status).toBe(404);

      res = await fetch(`${base}/v1/keys/bad?expires_at=not-a-date`, {
        method: "PUT",
        body: `1`,
        headers: { "Content-Type": "application/json" },
      });
      expect(res.status).toBe(400);
      const err = (await res.json()) as { error: { code: string } };
      expect(err.error.code).toBe(CodeInvalidExpiry);

      res = await fetch(`${base}/v1/keys/plain`, {
        method: "PUT",
        body: `{"p":1}`,
        headers: { "Content-Type": "application/json" },
      });
      expect(res.status).toBe(200);
    } finally {
      srv.close();
      await new Promise<void>((resolve, reject) =>
        httpSrv.close((e) => (e ? reject(e) : resolve())),
      );
      await db.close();
    }
  });
});
