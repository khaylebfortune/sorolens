import { defineConfig, configDefaults } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: false,
    // Playwright specs live under tests/e2e and must only run through
    // `pnpm test:e2e`; Vitest's default *.spec.* glob would otherwise pick
    // them up and fail with "Playwright Test did not expect test() to be
    // called here".
    exclude: [...configDefaults.exclude, "tests/e2e/**"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "."),
      "@sorolens/ui": path.resolve(__dirname, "../../packages/ui/src/index.ts"),
      "@sorolens/xdr": path.resolve(__dirname, "../../packages/xdr/src/index.ts"),
    },
  },
});
