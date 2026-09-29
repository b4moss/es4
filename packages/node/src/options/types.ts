/** Options keys are snake_case — same vocabulary as Go / YAML / ES4_* env. */

export const ENV_PREFIX = "ES4_";
export const DEFAULT_SNAPSHOT_INTERVAL_MS = 30_000;
export const DEFAULT_FIRESTORE_DATABASE_ID = "(default)";
export const DEFAULT_REDIS_HASH_KEY = "es4:state";

export const RecoveryBackendFile = "file";
export const RecoveryBackendLibSQL = "libsql";
export const RecoveryBackendObject = "object";

export const StateBackendMemory = "memory";
export const StateBackendSQLite = "sqlite";
export const StateBackendRedis = "redis";
export const StateBackendValkey = "valkey";
export const StateBackendFirestore = "firestore";

export type RecoveryBackend =
  | ""
  | typeof RecoveryBackendFile
  | typeof RecoveryBackendLibSQL
  | typeof RecoveryBackendObject;

export type StateBackend =
  | ""
  | typeof StateBackendMemory
  | typeof StateBackendSQLite
  | typeof StateBackendRedis
  | typeof StateBackendValkey
  | typeof StateBackendFirestore;

/**
 * Library Options. Duration fields are milliseconds after Load/parse
 * (Go `time.Duration` parity). YAML/env input uses Go duration strings.
 */
export interface Options {
  snapshot_interval: number;
  restore_on_startup: boolean;
  memory_only: boolean;
  recovery_path: string;
  recovery_backend: string;
  recovery_ttl: number;
  recovery_libsql_url: string;
  recovery_libsql_auth_token: string;
  recovery_s3_bucket: string;
  recovery_s3_prefix: string;
  recovery_s3_region: string;
  recovery_s3_endpoint: string;
  state_path: string;
  state_backend: string;
  state_redis_url: string;
  state_redis_key_prefix: string;
  state_firestore_project_id: string;
  state_firestore_database_id: string;
  state_firestore_collection: string;
}

export function defaults(): Options {
  return {
    snapshot_interval: DEFAULT_SNAPSHOT_INTERVAL_MS,
    restore_on_startup: true,
    memory_only: false,
    recovery_path: "",
    recovery_backend: "",
    recovery_ttl: 0,
    recovery_libsql_url: "",
    recovery_libsql_auth_token: "",
    recovery_s3_bucket: "",
    recovery_s3_prefix: "",
    recovery_s3_region: "",
    recovery_s3_endpoint: "",
    state_path: "",
    state_backend: "",
    state_redis_url: "",
    state_redis_key_prefix: "",
    state_firestore_project_id: "",
    state_firestore_database_id: "",
    state_firestore_collection: "",
  };
}

export function envName(snakeKey: string): string {
  return ENV_PREFIX + snakeKey.toUpperCase();
}
