import { describe, expect, it } from "vitest";
import { createServer } from "node:http";
import { open } from "../src/db.js";
import { defaults } from "../src/options/index.js";
import { newServer } from "../src/server/server.js";

async function withServer(
  fn: (base: string) => Promise<void>,
): Promise<void> {
  const db = await open({ ...defaults(), memory_only: true });
  const es4 = newServer(db);
  const server = createServer((req, res) => {
    void es4.handler()(req, res);
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const addr = server.address();
  if (!addr || typeof addr === "string") throw new Error("no addr");
  const base = `http://127.0.0.1:${addr.port}`;
  try {
    await fn(base);
  } finally {
    es4.close();
    await new Promise<void>((resolve, reject) =>
      server.close((e) => (e ? reject(e) : resolve())),
    );
    await db.close();
  }
}

describe("Server H-*", () => {
  it("H-N1 livez always OK", async () => {
    await withServer(async (base) => {
      const r = await fetch(`${base}/livez`);
      expect(r.status).toBe(200);
    });
  });

  it("H-N2 readyz when Ready", async () => {
    await withServer(async (base) => {
      const r = await fetch(`${base}/readyz`);
      expect(r.status).toBe(200);
    });
  });

  it("H-N3 REST State", async () => {
    await withServer(async (base) => {
      const put = await fetch(`${base}/v1/keys/a/b`, {
        method: "PUT",
        body: `{"x":1}`,
        headers: { "Content-Type": "application/json" },
      });
      expect(put.status).toBe(200);
      const get = await fetch(`${base}/v1/keys/a/b`);
      expect(get.status).toBe(200);
      expect(await get.text()).toBe(`{"x":1}`);
      const head = await fetch(`${base}/v1/keys/a/b`, { method: "HEAD" });
      expect(head.status).toBe(200);
      const del = await fetch(`${base}/v1/keys/a/b`, { method: "DELETE" });
      expect(del.status).toBe(200);
    });
  });

  it("H-E1 missing GET", async () => {
    await withServer(async (base) => {
      const get = await fetch(`${base}/v1/keys/missing`);
      expect(get.status).toBe(404);
      const body = (await get.json()) as { error: { code: string } };
      expect(body.error.code).toBe("not_found");
    });
  });

  it("H-E2 invalid/empty key", async () => {
    await withServer(async (base) => {
      const get = await fetch(`${base}/v1/keys`);
      expect(get.status).toBe(400);
      const body = (await get.json()) as { error: { code: string } };
      expect(body.error.code).toBe("invalid_key");
    });
  });
});
