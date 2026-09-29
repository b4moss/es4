import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { randomUUID } from "node:crypto";
import {
  ErrClosed,
  ErrInvalidKey,
  ErrInvalidValue,
  ErrNestedTx,
  ErrNotFound,
  ErrTxDone,
  isEs4Error,
} from "../errors.js";
import type { DB } from "../db.js";
import type { Tx } from "../state/types.js";

const MAX_BODY = 1 << 20; // 1 MiB

export const CodeNotFound = "not_found";
export const CodeInvalidKey = "invalid_key";
export const CodeInvalidValue = "invalid_value";
export const CodeConflict = "conflict";
export const CodeNotReady = "not_ready";
export const CodeInternal = "internal";

type TxEntry = { tx: Tx; done: boolean };

export class Es4Server {
  private readonly db: DB;
  private readonly txs = new Map<string, TxEntry>();

  constructor(db: DB) {
    this.db = db;
  }

  handler(): (
    req: IncomingMessage,
    res: ServerResponse,
  ) => Promise<void> {
    return (req, res) => this.handle(req, res);
  }

  listen(port: number, host = "0.0.0.0"): ReturnType<typeof createServer> {
    const server = createServer((req, res) => {
      void this.handle(req, res);
    });
    server.listen(port, host);
    return server;
  }

  close(): void {
    for (const [, e] of this.txs) {
      if (!e.done) void e.tx.rollback().catch(() => undefined);
      e.done = true;
    }
    this.txs.clear();
  }

  private async handle(
    req: IncomingMessage,
    res: ServerResponse,
  ): Promise<void> {
    try {
      const url = new URL(req.url ?? "/", "http://localhost");
      const path = url.pathname;
      const method = req.method ?? "GET";

      if (method === "GET" && path === "/livez") {
        res.writeHead(200);
        res.end();
        return;
      }
      if (method === "GET" && path === "/readyz") {
        res.writeHead(this.db.Ready ? 200 : 503);
        res.end();
        return;
      }

      // Reject trailing slash (no redirect) — Go mux parity for registered paths.
      if (path.length > 1 && path.endsWith("/")) {
        res.writeHead(404);
        res.end();
        return;
      }

      if (method === "POST" && path === "/v1/clear") {
        if (!this.requireReady(res)) return;
        await this.db.clear();
        res.writeHead(200);
        res.end();
        return;
      }

      if (method === "POST" && path === "/v1/tx") {
        if (!this.requireReady(res)) return;
        try {
          const tx = await this.db.beginTx();
          const id = randomUUID();
          this.txs.set(id, { tx, done: false });
          writeJSON(res, 200, { id });
        } catch (e) {
          mapError(res, e);
        }
        return;
      }

      const txCommit = path.match(/^\/v1\/tx\/([^/]+)\/commit$/);
      if (method === "POST" && txCommit) {
        if (!this.requireReady(res)) return;
        await this.finishTx(res, txCommit[1]!, "commit");
        return;
      }
      const txRollback = path.match(/^\/v1\/tx\/([^/]+)\/rollback$/);
      if (method === "POST" && txRollback) {
        if (!this.requireReady(res)) return;
        await this.finishTx(res, txRollback[1]!, "rollback");
        return;
      }
      const txClear = path.match(/^\/v1\/tx\/([^/]+)\/clear$/);
      if (method === "POST" && txClear) {
        if (!this.requireReady(res)) return;
        const entry = this.txs.get(txClear[1]!);
        if (!entry) {
          writeError(res, 404, CodeNotFound, "tx not found");
          return;
        }
        if (entry.done) {
          writeError(res, 409, CodeConflict, "tx finished");
          return;
        }
        try {
          await entry.tx.clear();
          res.writeHead(200);
          res.end();
        } catch (e) {
          mapError(res, e);
        }
        return;
      }

      const txKeysEmpty = path.match(/^\/v1\/tx\/([^/]+)\/keys$/);
      if (txKeysEmpty && ["PUT", "GET", "DELETE", "HEAD"].includes(method)) {
        if (!this.requireReady(res)) return;
        writeError(res, 400, CodeInvalidKey, "empty key");
        return;
      }

      const txKey = path.match(/^\/v1\/tx\/([^/]+)\/keys\/(.+)$/);
      if (txKey && ["PUT", "GET", "DELETE", "HEAD"].includes(method)) {
        if (!this.requireReady(res)) return;
        await this.handleTxKey(res, req, txKey[1]!, decodeURIComponent(txKey[2]!), method);
        return;
      }

      if (path === "/v1/keys" && ["PUT", "GET", "DELETE", "HEAD"].includes(method)) {
        if (!this.requireReady(res)) return;
        writeError(res, 400, CodeInvalidKey, "empty key");
        return;
      }

      const stateKey = path.match(/^\/v1\/keys\/(.+)$/);
      if (stateKey && ["PUT", "GET", "DELETE", "HEAD"].includes(method)) {
        if (!this.requireReady(res)) return;
        await this.handleStateKey(
          res,
          req,
          decodeURIComponent(stateKey[1]!),
          method,
        );
        return;
      }

      res.writeHead(404);
      res.end();
    } catch (e) {
      mapError(res, e);
    }
  }

  private requireReady(res: ServerResponse): boolean {
    if (this.db.Ready) return true;
    writeError(res, 503, CodeNotReady, "restore not ready");
    return false;
  }

  private async finishTx(
    res: ServerResponse,
    id: string,
    op: "commit" | "rollback",
  ): Promise<void> {
    const entry = this.txs.get(id);
    if (!entry) {
      writeError(res, 404, CodeNotFound, "tx not found");
      return;
    }
    if (entry.done) {
      writeError(res, 409, CodeConflict, "tx finished");
      return;
    }
    try {
      if (op === "commit") await entry.tx.commit();
      else await entry.tx.rollback();
      entry.done = true;
      res.writeHead(200);
      res.end();
    } catch (e) {
      mapError(res, e);
    }
  }

  private async handleStateKey(
    res: ServerResponse,
    req: IncomingMessage,
    key: string,
    method: string,
  ): Promise<void> {
    if (key === "") {
      writeError(res, 400, CodeInvalidKey, "empty key");
      return;
    }
    try {
      switch (method) {
        case "PUT": {
          const body = await readBody(req);
          await this.db.set(key, body);
          res.writeHead(200);
          res.end();
          break;
        }
        case "GET": {
          const val = await this.db.get(key);
          res.writeHead(200, { "Content-Type": "application/json" });
          res.end(val);
          break;
        }
        case "DELETE": {
          await this.db.delete(key);
          res.writeHead(200);
          res.end();
          break;
        }
        case "HEAD": {
          const ok = await this.db.exists(key);
          res.writeHead(ok ? 200 : 404);
          res.end();
          break;
        }
      }
    } catch (e) {
      mapError(res, e);
    }
  }

  private async handleTxKey(
    res: ServerResponse,
    req: IncomingMessage,
    id: string,
    key: string,
    method: string,
  ): Promise<void> {
    const entry = this.txs.get(id);
    if (!entry) {
      writeError(res, 404, CodeNotFound, "tx not found");
      return;
    }
    if (entry.done) {
      writeError(res, 409, CodeConflict, "tx finished");
      return;
    }
    if (key === "") {
      writeError(res, 400, CodeInvalidKey, "empty key");
      return;
    }
    try {
      switch (method) {
        case "PUT": {
          const body = await readBody(req);
          await entry.tx.set(key, body);
          res.writeHead(200);
          res.end();
          break;
        }
        case "GET": {
          const val = await entry.tx.get(key);
          res.writeHead(200, { "Content-Type": "application/json" });
          res.end(val);
          break;
        }
        case "DELETE": {
          await entry.tx.delete(key);
          res.writeHead(200);
          res.end();
          break;
        }
        case "HEAD": {
          const ok = await entry.tx.exists(key);
          res.writeHead(ok ? 200 : 404);
          res.end();
          break;
        }
      }
    } catch (e) {
      mapError(res, e);
    }
  }
}

function writeJSON(res: ServerResponse, status: number, body: unknown): void {
  const data = JSON.stringify(body);
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(data);
}

function writeError(
  res: ServerResponse,
  status: number,
  code: string,
  message: string,
): void {
  writeJSON(res, status, { error: { code, message } });
}

function mapError(res: ServerResponse, err: unknown): void {
  if (isEs4Error(err, ErrNotFound) || err === ErrNotFound) {
    writeError(res, 404, CodeNotFound, "key not found");
    return;
  }
  if (isEs4Error(err, ErrInvalidKey) || err === ErrInvalidKey) {
    writeError(res, 400, CodeInvalidKey, (err as Error).message);
    return;
  }
  if (isEs4Error(err, ErrInvalidValue) || err === ErrInvalidValue) {
    writeError(res, 400, CodeInvalidValue, (err as Error).message);
    return;
  }
  if (
    isEs4Error(err, ErrNestedTx) ||
    err === ErrNestedTx ||
    isEs4Error(err, ErrTxDone) ||
    err === ErrTxDone
  ) {
    writeError(res, 409, CodeConflict, (err as Error).message);
    return;
  }
  if (isEs4Error(err, ErrClosed) || err === ErrClosed) {
    writeError(res, 500, CodeInternal, "closed");
    return;
  }
  writeError(
    res,
    500,
    CodeInternal,
    err instanceof Error ? err.message : String(err),
  );
}

async function readBody(req: IncomingMessage): Promise<string> {
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of req) {
    const buf = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    size += buf.length;
    if (size > MAX_BODY) throw new Error("body too large");
    chunks.push(buf);
  }
  const raw = Buffer.concat(chunks).toString("utf8");
  if (raw === "") throw new Error("empty body");
  try {
    JSON.parse(raw);
  } catch {
    throw Object.assign(new Error("not valid JSON"), { cause: ErrInvalidValue });
  }
  return raw;
}

/** Resolve listen address: PORT (digits → :PORT) > ES4_LISTEN_ADDR > :8080. */
export function resolveListenAddr(
  getenv: (k: string) => string | undefined = (k) => process.env[k],
): { host: string; port: number } {
  const portEnv = getenv("PORT");
  if (portEnv && /^\d+$/.test(portEnv)) {
    return { host: "0.0.0.0", port: Number(portEnv) };
  }
  const addr = getenv("ES4_LISTEN_ADDR") || ":8080";
  if (addr.startsWith(":")) {
    return { host: "0.0.0.0", port: Number(addr.slice(1)) || 8080 };
  }
  const idx = addr.lastIndexOf(":");
  if (idx > 0) {
    return {
      host: addr.slice(0, idx),
      port: Number(addr.slice(idx + 1)) || 8080,
    };
  }
  return { host: "0.0.0.0", port: 8080 };
}

export function newServer(db: DB): Es4Server {
  return new Es4Server(db);
}
