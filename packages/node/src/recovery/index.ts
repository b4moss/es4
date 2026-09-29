import type { Options } from "../options/types.js";
import {
  RecoveryBackendFile,
  RecoveryBackendLibSQL,
  RecoveryBackendObject,
} from "../options/types.js";
import { resolvedRecoveryBackend } from "../options/effective.js";
import { newFileTTL } from "./file.js";
import { openLibSQL } from "./libsql.js";
import { openObject } from "./object.js";
import type { RecoveryStore } from "./types.js";

export type { RecoveryStore } from "./types.js";
export { ErrRecoveryNotFound, genID, parseGenID, keepGeneration } from "./types.js";
export { FileRecovery, newFileTTL } from "./file.js";
export { LibSQLRecovery, openLibSQL } from "./libsql.js";
export {
  ObjectRecovery,
  openObject,
  newObject,
  type ObjectAPI,
  type ObjectConfig,
} from "./object.js";

/** Build Recovery from Effective Options. Returns null when none configured. */
export async function openFromOptions(
  opts: Options,
  signal?: AbortSignal,
): Promise<RecoveryStore | null> {
  const backend = resolvedRecoveryBackend(opts);
  if (backend === "") return null;
  switch (backend) {
    case RecoveryBackendFile:
      return newFileTTL(opts.recovery_path, opts.recovery_ttl);
    case RecoveryBackendLibSQL:
      return openLibSQL(
        opts.recovery_libsql_url,
        opts.recovery_libsql_auth_token,
        opts.recovery_ttl,
      );
    case RecoveryBackendObject:
      return openObject({
        bucket: opts.recovery_s3_bucket,
        prefix: opts.recovery_s3_prefix,
        region: opts.recovery_s3_region,
        endpoint: opts.recovery_s3_endpoint,
        ttlMs: opts.recovery_ttl,
      });
    default:
      throw new Error(`recovery: unknown backend ${JSON.stringify(backend)}`);
  }
}
