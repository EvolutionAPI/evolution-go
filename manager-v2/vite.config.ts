import { defineConfig } from "vite";

export default defineConfig({
  base: "/manager-v2/",
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: true,
  },
});
