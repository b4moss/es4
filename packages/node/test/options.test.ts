import { describe, expect, it } from "vitest";
import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  applyEnv,
  defaults,
  effective,
  fromFile,
  load,
  parseGoDuration,
  parseStrictBool,
  validate,
  DEFAULT_SNAPSHOT_INTERVAL_MS,
  DEFAULT_FIRESTORE_DATABASE_ID,
} from "../src/options/index.js";

describe("Options O-N*", () => {
  it("O-N1 Defaults", () => {
    const d = defaults();
    expect(d.snapshot_interval).toBe(DEFAULT_SNAPSHOT_INTERVAL_MS);
    expect(d.restore_on_startup).toBe(true);
    expect(d.memory_only).toBe(false);
    expect(d.state_path).toBe("");
    expect(d.state_backend).toBe("");
    expect(d.recovery_path).toBe("");
    expect(d.recovery_backend).toBe("");
  });

  it("O-N2 YAML override", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-opts-"));
    const path = join(dir, "cfg.yaml");
    await writeFile(
      path,
      `snapshot_interval: "10s"\nmemory_only: true\nstate_path: /tmp/x.db\n`,
    );
    const opts = await fromFile(path);
    expect(opts.snapshot_interval).toBe(10_000);
    expect(opts.memory_only).toBe(true);
    expect(opts.state_path).toBe("/tmp/x.db");
  });

  it("O-N3 partial YAML keeps defaults", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-opts-"));
    const path = join(dir, "cfg.yaml");
    await writeFile(path, `state_path: /tmp/only.db\n`);
    const opts = await fromFile(path);
    expect(opts.state_path).toBe("/tmp/only.db");
    expect(opts.snapshot_interval).toBe(DEFAULT_SNAPSHOT_INTERVAL_MS);
    expect(opts.restore_on_startup).toBe(true);
  });

  it("O-N4 env overrides YAML", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-opts-"));
    const path = join(dir, "cfg.yaml");
    await writeFile(path, `snapshot_interval: "10s"\n`);
    const opts = await load(path, (k) =>
      k === "ES4_SNAPSHOT_INTERVAL" ? "5s" : undefined,
    );
    expect(opts.snapshot_interval).toBe(5_000);
  });

  it("O-N5 empty env skipped", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-opts-"));
    const path = join(dir, "cfg.yaml");
    await writeFile(path, `state_path: /tmp/keep.db\n`);
    const opts = await load(path, (k) =>
      k === "ES4_STATE_PATH" ? "" : undefined,
    );
    expect(opts.state_path).toBe("/tmp/keep.db");
  });

  it("O-N6 memory_only + recovery settings Validate + Effective Ignore", () => {
    const o = {
      ...defaults(),
      memory_only: true,
      recovery_path: "/tmp/r",
      recovery_backend: "file",
      state_path: "/tmp/s.db",
      state_backend: "redis",
      state_redis_url: "redis://x",
      state_firestore_project_id: "p",
      state_firestore_collection: "c",
    };
    expect(() => validate(o)).not.toThrow();
    const eff = effective(o);
    expect(eff.memory_only).toBe(true);
    expect(eff.snapshot_interval).toBe(0);
    expect(eff.restore_on_startup).toBe(false);
    expect(eff.recovery_path).toBe("");
    expect(eff.state_path).toBe("");
    expect(eff.state_backend).toBe("");
    expect(eff.state_redis_url).toBe("");
  });

  it("O-N7 empty recovery_backend + recovery_path → file", () => {
    const o = { ...defaults(), recovery_path: "/tmp/r.snap" };
    expect(() => validate(o)).not.toThrow();
  });

  it("O-N8 Firestore database_id default", () => {
    const o = {
      ...defaults(),
      state_backend: "firestore",
      state_firestore_project_id: "demo",
      state_firestore_collection: "col",
      state_firestore_database_id: "",
    };
    const eff = effective(o);
    expect(eff.state_firestore_database_id).toBe(DEFAULT_FIRESTORE_DATABASE_ID);
  });

  it("O-N9 Redis URL + prefix load", () => {
    const o = applyEnv(defaults(), (k) => {
      if (k === "ES4_STATE_BACKEND") return "redis";
      if (k === "ES4_STATE_REDIS_URL") return "redis://127.0.0.1:6379/0";
      if (k === "ES4_STATE_REDIS_KEY_PREFIX") return "es4:test";
      return undefined;
    });
    expect(o.state_backend).toBe("redis");
    expect(o.state_redis_url).toBe("redis://127.0.0.1:6379/0");
    expect(o.state_redis_key_prefix).toBe("es4:test");
    expect(() => validate(o)).not.toThrow();
  });

  it("O-N10 state_backend=memory", () => {
    const o = { ...defaults(), state_backend: "memory" };
    expect(() => validate(o)).not.toThrow();
  });
});

describe("Options O-E*", () => {
  it("O-E1 missing file", async () => {
    await expect(fromFile("/no/such/es4-config.yaml")).rejects.toThrow(/read config/);
  });

  it("O-E2 invalid YAML", async () => {
    const dir = await mkdtemp(join(tmpdir(), "es4-opts-"));
    const path = join(dir, "bad.yaml");
    await writeFile(path, `{{{{not: valid: yaml`);
    await expect(fromFile(path)).rejects.toThrow(/parse config/);
  });

  it("O-E3 duration non-Go string", () => {
    expect(() => parseGoDuration("30")).toThrow(/numeric seconds/);
    expect(() => parseGoDuration("abc")).toThrow(/invalid duration/);
  });

  it("O-E4 bool not true/false", () => {
    expect(() => parseStrictBool("True")).toThrow();
    expect(() => parseStrictBool("1")).toThrow();
    expect(() => parseStrictBool("yes")).toThrow();
  });

  it("O-E5 bad recovery_backend", () => {
    expect(() =>
      validate({ ...defaults(), recovery_backend: "s3" }),
    ).toThrow(/recovery_backend/);
  });

  it("O-E6 negative recovery_ttl", () => {
    expect(() =>
      validate({ ...defaults(), recovery_ttl: -1 }),
    ).toThrow(/recovery_ttl/);
  });

  it("O-E7 required keys for backends", () => {
    expect(() =>
      validate({ ...defaults(), recovery_backend: "file" }),
    ).toThrow(/recovery_path/);
    expect(() =>
      validate({ ...defaults(), recovery_backend: "libsql" }),
    ).toThrow(/recovery_libsql_url/);
    expect(() =>
      validate({ ...defaults(), recovery_backend: "object" }),
    ).toThrow(/recovery_s3_bucket/);
  });

  it("O-E8 redis without URL", () => {
    expect(() =>
      validate({ ...defaults(), state_backend: "redis" }),
    ).toThrow(/state_redis_url/);
    expect(() =>
      validate({ ...defaults(), state_backend: "valkey" }),
    ).toThrow(/state_redis_url/);
  });

  it("O-E9 firestore missing project/collection", () => {
    expect(() =>
      validate({
        ...defaults(),
        state_backend: "firestore",
        state_firestore_collection: "c",
      }),
    ).toThrow(/project_id/);
    expect(() =>
      validate({
        ...defaults(),
        state_backend: "firestore",
        state_firestore_project_id: "p",
      }),
    ).toThrow(/collection/);
  });

  it("O-E10 collection with slash", () => {
    expect(() =>
      validate({
        ...defaults(),
        state_backend: "firestore",
        state_firestore_project_id: "p",
        state_firestore_collection: "a/b",
      }),
    ).toThrow(/\//);
  });

  it("O-E11 sqlite without path", () => {
    expect(() =>
      validate({ ...defaults(), state_backend: "sqlite" }),
    ).toThrow(/state_path/);
  });

  it("O-E12 unknown / mixed-case state_backend", () => {
    expect(() =>
      validate({ ...defaults(), state_backend: "Redis" }),
    ).toThrow(/state_backend/);
    expect(() =>
      validate({ ...defaults(), state_backend: "mongo" }),
    ).toThrow(/state_backend/);
  });
});
