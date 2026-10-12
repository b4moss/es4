import { describe, expect, it } from "vitest";
import { newMemory } from "../src/state/index.js";
import { filterRestoreEntries, restoreInto } from "../src/snapshot/manager.js";

describe("Recovery restore decision D", () => {
  it("filterRestoreEntries drops slash-hierarchy keys", () => {
    const out = filterRestoreEntries({
      "a:b:c": `{"new":true}`,
      "a/b/c": `{"old":true}`,
      keep: `1`,
      ":bad": `0`,
    });
    expect(out).toEqual({
      "a:b:c": `{"new":true}`,
      keep: `1`,
    });
  });

  it("restoreInto applies filtered entries", async () => {
    const st = newMemory();
    const env = {
      version: 1,
      created_at: new Date().toISOString(),
      payload: {
        entries: {
          "a:b:c": `{"new":true}`,
          "a/b/c": `{"old":true}`,
        },
      },
    };
    const data = new TextEncoder().encode(JSON.stringify(env));
    await restoreInto(st, data);
    expect(await st.get("a:b:c")).toBe(`{"new":true}`);
    expect(await st.exists("a/b/c")).toBe(false);
    await st.close();
  });
});
