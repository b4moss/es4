import { describe, expect, it } from "vitest";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  ErrClosed,
  ErrInvalidKey,
  ErrInvalidValue,
  ErrNestedTx,
  ErrNotFound,
  ErrTxDone,
  isEs4Error,
} from "../src/errors.js";
import { newMemory, openSQLite, validateKey } from "../src/state/index.js";
import type { Store } from "../src/state/index.js";

async function runContract(name: string, openStore: () => Promise<Store> | Store) {
  describe(`State contract — ${name}`, () => {
    it("S-N1 Set→Get", async () => {
      const s = await openStore();
      try {
        await s.set("k", `{"a":1}`);
        expect(await s.get("k")).toBe(`{"a":1}`);
      } finally {
        await s.close();
      }
    });

    it("S-N2 Exists", async () => {
      const s = await openStore();
      try {
        expect(await s.exists("x")).toBe(false);
        await s.set("x", `"y"`);
        expect(await s.exists("x")).toBe(true);
      } finally {
        await s.close();
      }
    });

    it("S-N3 Delete → NotFound", async () => {
      const s = await openStore();
      try {
        await s.set("d", `1`);
        await s.delete("d");
        await expect(s.get("d")).rejects.toSatisfy((e) =>
          isEs4Error(e, ErrNotFound),
        );
      } finally {
        await s.close();
      }
    });

    it("S-N4 Clear", async () => {
      const s = await openStore();
      try {
        await s.set("a", `1`);
        await s.clear();
        expect(await s.exists("a")).toBe(false);
        await s.clear(); // empty ok
      } finally {
        await s.close();
      }
    });

    it("S-N5 hierarchy keys", async () => {
      const s = await openStore();
      try {
        await s.set("a/b/c", `"ok"`);
        expect(await s.get("a/b/c")).toBe(`"ok"`);
      } finally {
        await s.close();
      }
    });

    it("S-N6 JSON types", async () => {
      const s = await openStore();
      try {
        for (const v of [`null`, `[1]`, `{"x":1}`, `42`, `"hi"`]) {
          await s.set("t", v);
          expect(await s.get("t")).toBe(v);
        }
      } finally {
        await s.close();
      }
    });

    it("S-N7 Export deep copy", async () => {
      const s = await openStore();
      try {
        await s.set("e", `{"n":1}`);
        const exp = await s.export();
        exp.set("e", `{"n":2}`);
        expect(await s.get("e")).toBe(`{"n":1}`);
      } finally {
        await s.close();
      }
    });

    it("S-N8 Replace atomic", async () => {
      const s = await openStore();
      try {
        await s.set("old", `1`);
        await s.replace(new Map([["new", `2`]]));
        expect(await s.exists("old")).toBe(false);
        expect(await s.get("new")).toBe(`2`);
      } finally {
        await s.close();
      }
    });

    it("S-N9 empty Replace = Clear", async () => {
      const s = await openStore();
      try {
        await s.set("z", `1`);
        await s.replace(new Map());
        expect(await s.exists("z")).toBe(false);
      } finally {
        await s.close();
      }
    });

    it("S-N10 Tx Commit", async () => {
      const s = await openStore();
      try {
        const tx = await s.beginTx();
        await tx.set("t", `"c"`);
        await tx.commit();
        expect(await s.get("t")).toBe(`"c"`);
      } finally {
        await s.close();
      }
    });

    it("S-N11 Tx Rollback", async () => {
      const s = await openStore();
      try {
        const tx = await s.beginTx();
        await tx.set("t", `"r"`);
        await tx.rollback();
        expect(await s.exists("t")).toBe(false);
      } finally {
        await s.close();
      }
    });

    it("S-N12 double Close", async () => {
      const s = await openStore();
      await s.close();
      await s.close();
    });

    it("S-E1 Get missing", async () => {
      const s = await openStore();
      try {
        await expect(s.get("nope")).rejects.toSatisfy((e) =>
          isEs4Error(e, ErrNotFound),
        );
      } finally {
        await s.close();
      }
    });

    it("S-E2 Delete missing", async () => {
      const s = await openStore();
      try {
        await expect(s.delete("nope")).rejects.toSatisfy((e) =>
          isEs4Error(e, ErrNotFound),
        );
      } finally {
        await s.close();
      }
    });

    it("S-E3 invalid key", async () => {
      const s = await openStore();
      try {
        await expect(s.set("", `1`)).rejects.toSatisfy((e) =>
          isEs4Error(e, ErrInvalidKey),
        );
        await expect(s.set("/a", `1`)).rejects.toSatisfy((e) =>
          isEs4Error(e, ErrInvalidKey),
        );
      } finally {
        await s.close();
      }
    });

    it("S-E4 invalid JSON", async () => {
      const s = await openStore();
      try {
        await expect(s.set("k", `{`)).rejects.toSatisfy((e) =>
          isEs4Error(e, ErrInvalidValue),
        );
      } finally {
        await s.close();
      }
    });

    it("S-E5 aborted signal", async () => {
      const s = await openStore();
      try {
        const ac = new AbortController();
        ac.abort(new Error("cancelled"));
        await expect(s.set("k", `1`, ac.signal)).rejects.toThrow(/cancelled|abort/i);
      } finally {
        await s.close();
      }
    });

    it("S-E6 nested BeginTx", async () => {
      const s = await openStore();
      try {
        await s.beginTx();
        await expect(s.beginTx()).rejects.toSatisfy((e) =>
          isEs4Error(e, ErrNestedTx),
        );
      } finally {
        await s.close();
      }
    });

    it("S-E7 ops after Close", async () => {
      const s = await openStore();
      await s.close();
      await expect(s.set("k", `1`)).rejects.toSatisfy((e) =>
        isEs4Error(e, ErrClosed),
      );
    });

    it("S-E8 reused Tx", async () => {
      const s = await openStore();
      try {
        const tx = await s.beginTx();
        await tx.commit();
        await expect(tx.set("k", `1`)).rejects.toSatisfy((e) =>
          isEs4Error(e, ErrTxDone),
        );
      } finally {
        await s.close();
      }
    });
  });
}

await runContract("Memory", () => newMemory());

await runContract("SQLite", async () => {
  const dir = await mkdtemp(join(tmpdir(), "es4-sqlite-"));
  return openSQLite(join(dir, "state.db"));
});

describe("keys", () => {
  it("validateKey rejects bad forms", () => {
    expect(() => validateKey("")).toThrow();
    expect(() => validateKey("/a")).toThrow();
    expect(() => validateKey("a/")).toThrow();
    expect(() => validateKey("a//b")).toThrow();
    expect(() => validateKey("a/b")).not.toThrow();
  });
});

describe("S-N13 SQLite persistence", () => {
  it("Close → re-Open retains", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-sqlite-"));
    const path = join(dir, "p.db");
    const s1 = openSQLite(path);
    await s1.set("persist", `"yes"`);
    await s1.close();
    const s2 = openSQLite(path);
    expect(await s2.get("persist")).toBe(`"yes"`);
    await s2.close();
  });
});

describe("S-N14 isolation", () => {
  it("separate sqlite paths", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-sqlite-"));
    const a = openSQLite(join(dir, "a.db"));
    const b = openSQLite(join(dir, "b.db"));
    await a.set("k", `"a"`);
    await b.set("k", `"b"`);
    expect(await a.get("k")).toBe(`"a"`);
    expect(await b.get("k")).toBe(`"b"`);
    await a.close();
    await b.close();
  });
});

describe("S-N15 Export→Replace roundtrip", () => {
  it("memory → sqlite", async () => {
    const m = newMemory();
    await m.set("a/b", `{"x":1}`);
    const exp = await m.export();
    const dir = await mkdtemp(join(tmpdir(), "es4-sqlite-"));
    const s = openSQLite(join(dir, "rt.db"));
    await s.replace(exp);
    expect(await s.get("a/b")).toBe(`{"x":1}`);
    await m.close();
    await s.close();
  });
});
