import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: "../static/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/_/": "http://localhost:6275",
    },
  },
});
