#!/usr/bin/env node
import { load } from "../options/index.js";
import { open } from "../db.js";
import { newServer, resolveListenAddr } from "./server.js";

async function main(): Promise<void> {
  const configPath = process.env.ES4_CONFIG_PATH ?? "";
  const opts = await load(configPath);
  const db = await open(opts);
  const server = newServer(db);
  const { host, port } = resolveListenAddr();
  const httpServer = server.listen(port, host);
  const shutdown = async () => {
    server.close();
    httpServer.close();
    await db.close();
    process.exit(0);
  };
  process.on("SIGINT", () => void shutdown());
  process.on("SIGTERM", () => void shutdown());
  console.error(`es4-server listening on ${host}:${port}`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
