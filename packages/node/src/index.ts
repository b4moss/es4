export {
  ErrNotFound,
  ErrInvalidKey,
  ErrInvalidValue,
  ErrClosed,
  ErrTxDone,
  ErrNestedTx,
  ErrFirestoreDocIDTooLong,
  isNotFound,
  isEs4Error,
} from "./errors.js";

export {
  type Options,
  defaults,
  load,
  fromFile,
  applyEnv,
  effective,
  validate,
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
  parseGoDuration,
  parseStrictBool,
  resolvedRecoveryBackend,
  isRedisStateBackend,
  isFirestoreStateBackend,
} from "./options/index.js";

export type { Store, Tx, JsonRaw } from "./state/index.js";
export {
  newMemory,
  openSQLite,
  openRedis,
  openFirestore,
  validateKey,
  validateValue,
  encodeFirestoreDocID,
  decodeFirestoreDocID,
} from "./state/index.js";

export { DB, open, openWith, type OpenConfig } from "./db.js";

export { newServer, Es4Server, resolveListenAddr } from "./server/server.js";
