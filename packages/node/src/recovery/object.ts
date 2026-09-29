import {
  CreateBucketCommand,
  DeleteObjectCommand,
  GetObjectCommand,
  HeadBucketCommand,
  ListObjectsV2Command,
  PutObjectCommand,
  S3Client,
} from "@aws-sdk/client-s3";
import {
  assertNotAborted,
  GEN_FILE_PREFIX,
  GEN_FILE_SUFFIX,
  genID,
  keepGeneration,
  notFound,
  parseGenID,
  type RecoveryStore,
} from "./types.js";

export interface ObjectAPI {
  putObject(bucket: string, key: string, body: Uint8Array): Promise<void>;
  getObject(bucket: string, key: string): Promise<Uint8Array>;
  listObjectKeys(bucket: string, prefix: string): Promise<string[]>;
  deleteObject(bucket: string, key: string): Promise<void>;
  ensureBucket?(bucket: string): Promise<void>;
}

export interface ObjectConfig {
  bucket: string;
  prefix?: string;
  region?: string;
  endpoint?: string;
  ttlMs?: number;
  client?: ObjectAPI;
  clock?: () => Date;
}

function normalizePrefix(p: string): string {
  p = p.replace(/^\/+|\/+$/g, "");
  if (p === "") return "";
  return p + "/";
}

export class ObjectRecovery implements RecoveryStore {
  private readonly bucket: string;
  private readonly prefix: string;
  private readonly ttlMs: number;
  private readonly client: ObjectAPI;
  private readonly clock: () => Date;

  constructor(cfg: ObjectConfig) {
    if (!cfg.bucket) throw new Error("recovery: empty s3 bucket");
    if (!cfg.client) throw new Error("recovery: nil object client");
    this.bucket = cfg.bucket;
    this.prefix = normalizePrefix(cfg.prefix ?? "");
    this.ttlMs = cfg.ttlMs ?? 0;
    this.client = cfg.client;
    this.clock = cfg.clock ?? (() => new Date());
  }

  private objectKey(id: string): string {
    return this.prefix + GEN_FILE_PREFIX + id + GEN_FILE_SUFFIX;
  }

  private parseKey(key: string): string | null {
    let k = key;
    if (this.prefix !== "") {
      if (!k.startsWith(this.prefix)) return null;
      k = k.slice(this.prefix.length);
    }
    if (!k.startsWith(GEN_FILE_PREFIX) || !k.endsWith(GEN_FILE_SUFFIX)) {
      return null;
    }
    const id = k.slice(
      GEN_FILE_PREFIX.length,
      k.length - GEN_FILE_SUFFIX.length,
    );
    try {
      parseGenID(id);
      return id;
    } catch {
      return null;
    }
  }

  async save(data: Uint8Array, signal?: AbortSignal): Promise<void> {
    assertNotAborted(signal);
    const id = genID(this.clock());
    await this.client.putObject(this.bucket, this.objectKey(id), data);
    await this.prune();
  }

  private async listGenKeys(): Promise<string[]> {
    const all = await this.client.listObjectKeys(
      this.bucket,
      this.prefix + GEN_FILE_PREFIX,
    );
    const keys = all.filter((k) => this.parseKey(k) !== null);
    keys.sort();
    return keys;
  }

  private async prune(): Promise<void> {
    const keys = await this.listGenKeys();
    if (keys.length === 0) return;
    const now = this.clock();
    const newest = keys[keys.length - 1]!;
    for (const key of keys) {
      if (key === newest) continue;
      const id = this.parseKey(key);
      if (!id) continue;
      const created = parseGenID(id);
      if (keepGeneration(created, now, this.ttlMs)) continue;
      await this.client.deleteObject(this.bucket, key);
    }
  }

  async load(signal?: AbortSignal): Promise<Uint8Array> {
    assertNotAborted(signal);
    const keys = await this.listGenKeys();
    if (keys.length === 0) notFound();
    const newest = keys[keys.length - 1]!;
    return this.client.getObject(this.bucket, newest);
  }
}

class AWSS3Client implements ObjectAPI {
  constructor(private readonly s3: S3Client) {}

  async putObject(
    bucket: string,
    key: string,
    body: Uint8Array,
  ): Promise<void> {
    await this.s3.send(
      new PutObjectCommand({ Bucket: bucket, Key: key, Body: body }),
    );
  }

  async getObject(bucket: string, key: string): Promise<Uint8Array> {
    const out = await this.s3.send(
      new GetObjectCommand({ Bucket: bucket, Key: key }),
    );
    const bytes = await out.Body?.transformToByteArray();
    if (!bytes) throw new Error("recovery: empty object body");
    return bytes;
  }

  async listObjectKeys(bucket: string, prefix: string): Promise<string[]> {
    const keys: string[] = [];
    let token: string | undefined;
    do {
      const out = await this.s3.send(
        new ListObjectsV2Command({
          Bucket: bucket,
          Prefix: prefix,
          ContinuationToken: token,
        }),
      );
      for (const obj of out.Contents ?? []) {
        if (obj.Key) keys.push(obj.Key);
      }
      token = out.IsTruncated ? out.NextContinuationToken : undefined;
    } while (token);
    return keys;
  }

  async deleteObject(bucket: string, key: string): Promise<void> {
    await this.s3.send(new DeleteObjectCommand({ Bucket: bucket, Key: key }));
  }

  async ensureBucket(bucket: string): Promise<void> {
    try {
      await this.s3.send(new HeadBucketCommand({ Bucket: bucket }));
    } catch {
      try {
        await this.s3.send(new CreateBucketCommand({ Bucket: bucket }));
      } catch {
        /* may already exist / race */
      }
    }
  }
}

export async function openObject(cfg: ObjectConfig): Promise<ObjectRecovery> {
  if (cfg.client) return new ObjectRecovery(cfg);
  const region = cfg.region || process.env.AWS_REGION || "us-east-1";
  const s3 = new S3Client({
    region,
    endpoint: cfg.endpoint || undefined,
    forcePathStyle: Boolean(cfg.endpoint),
    credentials:
      process.env.AWS_ACCESS_KEY_ID && process.env.AWS_SECRET_ACCESS_KEY
        ? {
            accessKeyId: process.env.AWS_ACCESS_KEY_ID,
            secretAccessKey: process.env.AWS_SECRET_ACCESS_KEY,
          }
        : undefined,
  });
  const api = new AWSS3Client(s3);
  if (cfg.bucket) {
    try {
      await api.ensureBucket(cfg.bucket);
    } catch {
      /* ignore — Load/Save will surface errors */
    }
  }
  return new ObjectRecovery({ ...cfg, client: api });
}

export function newObject(cfg: ObjectConfig): ObjectRecovery {
  return new ObjectRecovery(cfg);
}
