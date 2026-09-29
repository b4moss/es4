import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["test/**/*.test.ts"],
    exclude: ["e2e/**", "dist/**", "node_modules/**"],
    testTimeout: 30_000,
    hookTimeout: 30_000,
  },
});
