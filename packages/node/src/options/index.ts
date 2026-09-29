import { readFile } from "node:fs/promises";
import { parse as parseYaml } from "yaml";
import { effective, validate } from "./effective.js";
import { defaults, ENV_PREFIX, type Options } from "./types.js";

export type { Options } from "./types.js";
export {
  defaults,
  envName,
  ENV_PREFIX,
  DEFAULT_SNAPSHOT_INTERVAL_MS,
  DEFAULT_FIRESTORE_DATABASE_ID,
  DEFAULT_REDIS_HASH_KEY,
  RecoveryBackendFile,
  RecoveryBackendLibSQL,
  RecoveryBackendObject,
  StateBackendMemory,
  StateBackendSQLite,
  StateBackendRedis,
  StateBackendValkey,
  StateBackendFirestore,
} from "./types.js";
export {
  effective,
  validate,
  usesRecovery,
  resolvedRecoveryBackend,
  isRedisStateBackend,
  isFirestoreStateBackend,
  validateFirestoreCollection,
} from "./effective.js";

type PartialOpts = {
  [K in keyof Options]?: Options[K];
};

/** Parse Go duration string → milliseconds. Rejects bare numbers. */
export function parseGoDuration(s: string): number {
  if (/^[+-]?(\d+(\.\d*)?|\.\d+)$/.test(s.trim())) {
    throw new Error(
      `numeric seconds rejected; use Go duration string (e.g. "30s"), got ${JSON.stringify(s)}`,
    );
  }
  const re =
    /^([+-])?(?:(\d+(?:\.\d+)?)h)?(?:(\d+(?:\.\d+)?)m)?(?:(\d+(?:\.\d+)?)s)?(?:(\d+(?:\.\d+)?)ms)?(?:(\d+(?:\.\d+)?)us|µs)?(?:(\d+(?:\.\d+)?)ns)?$/;
  const m = s.trim().match(re);
  if (!m || m.slice(2).every((x) => x === undefined)) {
    throw new Error(
      `invalid duration ${JSON.stringify(s)}: want Go duration string (e.g. "30s")`,
    );
  }
  const sign = m[1] === "-" ? -1 : 1;
  const h = parseFloat(m[2] ?? "0");
  const min = parseFloat(m[3] ?? "0");
  const sec = parseFloat(m[4] ?? "0");
  const ms = parseFloat(m[5] ?? "0");
  const us = parseFloat(m[6] ?? "0");
  const ns = parseFloat(m[7] ?? "0");
  const totalMs =
    h * 3_600_000 +
    min * 60_000 +
    sec * 1_000 +
    ms +
    us / 1_000 +
    ns / 1_000_000;
  return sign * totalMs;
}

export function parseStrictBool(s: string): boolean {
  if (s === "true") return true;
  if (s === "false") return false;
  throw new Error(
    `invalid bool ${JSON.stringify(s)}: want lowercase true or false`,
  );
}

function merge(base: Options, over: PartialOpts): Options {
  return { ...base, ...stripUndefined(over) };
}

function stripUndefined<T extends Record<string, unknown>>(o: T): Partial<T> {
  const out: Partial<T> = {};
  for (const [k, v] of Object.entries(o)) {
    if (v !== undefined) (out as Record<string, unknown>)[k] = v;
  }
  return out;
}

function parseYamlOverlay(data: string, path: string): PartialOpts {
  let doc: unknown;
  try {
    doc = parseYaml(data, { uniqueKeys: false });
  } catch (e) {
    throw new Error(
      `options: parse config ${JSON.stringify(path)}: ${(e as Error).message}`,
    );
  }
  if (doc === null || doc === undefined) return {};
  if (typeof doc !== "object" || Array.isArray(doc)) {
    throw new Error(
      `options: parse config ${JSON.stringify(path)}: root must be a mapping`,
    );
  }
  const root = doc as Record<string, unknown>;
  const out: PartialOpts = {};

  for (const [key, raw] of Object.entries(root)) {
    switch (key) {
      case "snapshot_interval":
        out.snapshot_interval = durationFromYaml(raw, "snapshot_interval");
        break;
      case "restore_on_startup":
        out.restore_on_startup = boolFromYaml(raw, "restore_on_startup");
        break;
      case "memory_only":
        out.memory_only = boolFromYaml(raw, "memory_only");
        break;
      case "recovery_path":
        out.recovery_path = stringFromYaml(raw, "recovery_path");
        break;
      case "recovery_backend":
        out.recovery_backend = stringFromYaml(raw, "recovery_backend");
        break;
      case "recovery_ttl":
        out.recovery_ttl = durationFromYaml(raw, "recovery_ttl");
        break;
      case "recovery_libsql_url":
        out.recovery_libsql_url = stringFromYaml(raw, "recovery_libsql_url");
        break;
      case "recovery_libsql_auth_token":
        out.recovery_libsql_auth_token = stringFromYaml(
          raw,
          "recovery_libsql_auth_token",
        );
        break;
      case "recovery_s3_bucket":
        out.recovery_s3_bucket = stringFromYaml(raw, "recovery_s3_bucket");
        break;
      case "recovery_s3_prefix":
        out.recovery_s3_prefix = stringFromYaml(raw, "recovery_s3_prefix");
        break;
      case "recovery_s3_region":
        out.recovery_s3_region = stringFromYaml(raw, "recovery_s3_region");
        break;
      case "recovery_s3_endpoint":
        out.recovery_s3_endpoint = stringFromYaml(raw, "recovery_s3_endpoint");
        break;
      case "state_path":
        out.state_path = stringFromYaml(raw, "state_path");
        break;
      case "state_backend":
        out.state_backend = stringFromYaml(raw, "state_backend");
        break;
      case "state_redis_url":
        out.state_redis_url = stringFromYaml(raw, "state_redis_url");
        break;
      case "state_redis_key_prefix":
        out.state_redis_key_prefix = stringFromYaml(
          raw,
          "state_redis_key_prefix",
        );
        break;
      case "state_firestore_project_id":
        out.state_firestore_project_id = stringFromYaml(
          raw,
          "state_firestore_project_id",
        );
        break;
      case "state_firestore_database_id":
        out.state_firestore_database_id = stringFromYaml(
          raw,
          "state_firestore_database_id",
        );
        break;
      case "state_firestore_collection":
        out.state_firestore_collection = stringFromYaml(
          raw,
          "state_firestore_collection",
        );
        break;
      default:
        // unknown keys silently ignored (Go parity)
        break;
    }
  }
  return out;
}

function durationFromYaml(raw: unknown, key: string): number {
  if (typeof raw === "number") {
    throw new Error(
      `options: ${key}: numeric seconds rejected; use Go duration string (e.g. "30s"), got ${raw}`,
    );
  }
  if (typeof raw !== "string") {
    throw new Error(`options: ${key}: want Go duration string`);
  }
  try {
    return parseGoDuration(raw);
  } catch (e) {
    throw new Error(`options: ${key}: ${(e as Error).message}`);
  }
}

function boolFromYaml(raw: unknown, key: string): boolean {
  if (typeof raw === "boolean") {
    // YAML true/false scalars decode as boolean — accept only those.
    return raw;
  }
  if (typeof raw === "string") {
    try {
      return parseStrictBool(raw);
    } catch (e) {
      throw new Error(`options: ${key}: ${(e as Error).message}`);
    }
  }
  throw new Error(`options: ${key}: want lowercase true or false`);
}

function stringFromYaml(raw: unknown, key: string): string {
  if (raw === null || raw === undefined) return "";
  if (typeof raw === "string") return raw;
  if (typeof raw === "number" || typeof raw === "boolean") return String(raw);
  throw new Error(`options: ${key}: want string`);
}

export async function fromFile(path: string): Promise<Options> {
  let data: string;
  try {
    data = await readFile(path, "utf8");
  } catch (e) {
    throw new Error(
      `options: read config ${JSON.stringify(path)}: ${(e as Error).message}`,
    );
  }
  const overlay = parseYamlOverlay(data, path);
  const opts = merge(defaults(), overlay);
  validate(opts);
  return opts;
}

export function applyEnv(
  opts: Options,
  getenv: (key: string) => string | undefined = (k) => process.env[k],
): Options {
  const overlay: PartialOpts = {};
  const get = (suffix: string) => {
    const v = getenv(ENV_PREFIX + suffix);
    return v && v !== "" ? v : undefined;
  };

  const si = get("SNAPSHOT_INTERVAL");
  if (si !== undefined) {
    try {
      overlay.snapshot_interval = parseGoDuration(si);
    } catch (e) {
      throw new Error(
        `options: ${ENV_PREFIX}SNAPSHOT_INTERVAL: ${(e as Error).message}`,
      );
    }
  }
  const ros = get("RESTORE_ON_STARTUP");
  if (ros !== undefined) {
    try {
      overlay.restore_on_startup = parseStrictBool(ros);
    } catch (e) {
      throw new Error(
        `options: ${ENV_PREFIX}RESTORE_ON_STARTUP: ${(e as Error).message}`,
      );
    }
  }
  const mo = get("MEMORY_ONLY");
  if (mo !== undefined) {
    try {
      overlay.memory_only = parseStrictBool(mo);
    } catch (e) {
      throw new Error(
        `options: ${ENV_PREFIX}MEMORY_ONLY: ${(e as Error).message}`,
      );
    }
  }
  const rp = get("RECOVERY_PATH");
  if (rp !== undefined) overlay.recovery_path = rp;
  const rb = get("RECOVERY_BACKEND");
  if (rb !== undefined) overlay.recovery_backend = rb;
  const rt = get("RECOVERY_TTL");
  if (rt !== undefined) {
    try {
      overlay.recovery_ttl = parseGoDuration(rt);
    } catch (e) {
      throw new Error(
        `options: ${ENV_PREFIX}RECOVERY_TTL: ${(e as Error).message}`,
      );
    }
  }
  const rlu = get("RECOVERY_LIBSQL_URL");
  if (rlu !== undefined) overlay.recovery_libsql_url = rlu;
  const rlat = get("RECOVERY_LIBSQL_AUTH_TOKEN");
  if (rlat !== undefined) overlay.recovery_libsql_auth_token = rlat;
  const rsb = get("RECOVERY_S3_BUCKET");
  if (rsb !== undefined) overlay.recovery_s3_bucket = rsb;
  const rsp = get("RECOVERY_S3_PREFIX");
  if (rsp !== undefined) overlay.recovery_s3_prefix = rsp;
  const rsr = get("RECOVERY_S3_REGION");
  if (rsr !== undefined) overlay.recovery_s3_region = rsr;
  const rse = get("RECOVERY_S3_ENDPOINT");
  if (rse !== undefined) overlay.recovery_s3_endpoint = rse;
  const sp = get("STATE_PATH");
  if (sp !== undefined) overlay.state_path = sp;
  const sb = get("STATE_BACKEND");
  if (sb !== undefined) overlay.state_backend = sb;
  const sru = get("STATE_REDIS_URL");
  if (sru !== undefined) overlay.state_redis_url = sru;
  const srkp = get("STATE_REDIS_KEY_PREFIX");
  if (srkp !== undefined) overlay.state_redis_key_prefix = srkp;
  const sfpi = get("STATE_FIRESTORE_PROJECT_ID");
  if (sfpi !== undefined) overlay.state_firestore_project_id = sfpi;
  const sfdi = get("STATE_FIRESTORE_DATABASE_ID");
  if (sfdi !== undefined) overlay.state_firestore_database_id = sfdi;
  const sfc = get("STATE_FIRESTORE_COLLECTION");
  if (sfc !== undefined) overlay.state_firestore_collection = sfc;

  return merge(opts, overlay);
}

/** Defaults → optional YAML → ES4_* env → Validate. */
export async function load(
  configPath = "",
  getenv?: (key: string) => string | undefined,
): Promise<Options> {
  let opts = defaults();
  if (configPath !== "") {
    opts = await fromFile(configPath);
    // fromFile already validated; re-apply env on top of file merge of defaults.
    // fromFile returns Defaults+file validated; we need Defaults→file→env.
    // Re-load file overlay properly:
    const data = await readFile(configPath, "utf8");
    opts = merge(defaults(), parseYamlOverlay(data, configPath));
  }
  opts = applyEnv(opts, getenv);
  validate(opts);
  return opts;
}

/** Convenience: Effective view after Load-style merge. */
export function toEffective(o: Options): Options {
  return effective(o);
}
