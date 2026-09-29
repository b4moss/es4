import {
  DEFAULT_FIRESTORE_DATABASE_ID,
  RecoveryBackendFile,
  RecoveryBackendLibSQL,
  RecoveryBackendObject,
  StateBackendFirestore,
  StateBackendMemory,
  StateBackendRedis,
  StateBackendSQLite,
  StateBackendValkey,
  type Options,
} from "./types.js";

export function effective(o: Options): Options {
  if (o.memory_only) {
    return {
      snapshot_interval: 0,
      restore_on_startup: false,
      memory_only: true,
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
  const out = { ...o };
  if (out.state_firestore_database_id === "") {
    out.state_firestore_database_id = DEFAULT_FIRESTORE_DATABASE_ID;
  }
  return out;
}

export function usesRecovery(o: Options): boolean {
  return !o.memory_only;
}

export function resolvedRecoveryBackend(o: Options): string {
  if (o.recovery_backend !== "") return o.recovery_backend;
  if (o.recovery_path !== "") return RecoveryBackendFile;
  return "";
}

export function isRedisStateBackend(o: Options): boolean {
  return (
    o.state_backend === StateBackendRedis ||
    o.state_backend === StateBackendValkey
  );
}

export function isFirestoreStateBackend(o: Options): boolean {
  return o.state_backend === StateBackendFirestore;
}

export function validateFirestoreCollection(id: string): void {
  if (id === "") {
    throw new Error(
      "options: state_firestore_collection: required when state_backend=firestore",
    );
  }
  if (id.includes("/")) {
    throw new Error(
      "options: state_firestore_collection: must not contain '/'",
    );
  }
  if (id === "." || id === "..") {
    throw new Error(
      `options: state_firestore_collection: invalid collection id ${JSON.stringify(id)}`,
    );
  }
}

export function validate(o: Options): void {
  if (o.recovery_ttl < 0) {
    throw new Error("options: recovery_ttl: must be >= 0");
  }
  switch (o.recovery_backend) {
    case "":
    case RecoveryBackendFile:
    case RecoveryBackendLibSQL:
    case RecoveryBackendObject:
      break;
    default:
      throw new Error(
        `options: recovery_backend: want file|libsql|object, got ${JSON.stringify(o.recovery_backend)}`,
      );
  }
  switch (o.state_backend) {
    case "":
    case StateBackendMemory:
    case StateBackendSQLite:
    case StateBackendRedis:
    case StateBackendValkey:
    case StateBackendFirestore:
      break;
    default:
      throw new Error(
        `options: state_backend: want memory|sqlite|redis|valkey|firestore, got ${JSON.stringify(o.state_backend)}`,
      );
  }
  if (o.memory_only) return;

  switch (resolvedRecoveryBackend(o)) {
    case RecoveryBackendFile:
      if (o.recovery_path === "") {
        throw new Error(
          "options: recovery_path: required when recovery_backend=file",
        );
      }
      break;
    case RecoveryBackendLibSQL:
      if (o.recovery_libsql_url === "") {
        throw new Error(
          "options: recovery_libsql_url: required when recovery_backend=libsql",
        );
      }
      break;
    case RecoveryBackendObject:
      if (o.recovery_s3_bucket === "") {
        throw new Error(
          "options: recovery_s3_bucket: required when recovery_backend=object",
        );
      }
      break;
  }

  if (isRedisStateBackend(o) && o.state_redis_url === "") {
    throw new Error(
      `options: state_redis_url: required when state_backend=${o.state_backend}`,
    );
  }
  if (o.state_backend === StateBackendSQLite && o.state_path === "") {
    throw new Error(
      "options: state_path: required when state_backend=sqlite",
    );
  }
  if (isFirestoreStateBackend(o)) {
    if (o.state_firestore_project_id === "") {
      throw new Error(
        "options: state_firestore_project_id: required when state_backend=firestore",
      );
    }
    validateFirestoreCollection(o.state_firestore_collection);
  }
}
