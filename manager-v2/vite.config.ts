import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, ".", "VITE_");
  const backendTarget = env.VITE_BACKEND_URL || "http://localhost:8080";
  const backendProxy = { target: backendTarget, changeOrigin: true };

  return {
    base: "/manager-v2/",
    // Vite runs on :5173 while Evolution GO runs on :8080. Proxying keeps the
    // Manager cookie on localhost and avoids cross-origin requests in dev mode.
    server: {
      proxy: {
        "/manager-v2/auth": backendProxy,
        "/manager-v2/settings": backendProxy,
        "/instance": backendProxy,
        "/call": backendProxy,
        "^/(?:server|instance|send|user|message|chat|group|call|community|label|newsletter|poll)(?:/|$)": backendProxy,
      },
    },
    build: {
      outDir: "dist",
      emptyOutDir: true,
      sourcemap: true,
    },
  };
});
