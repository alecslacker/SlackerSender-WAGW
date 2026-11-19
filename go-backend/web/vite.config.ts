import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    proxy: {
      // Regex, BUKAN prefix "/api": prefix string ikut menangkap rute SPA
      // /api-docs & /api-playground dan meneruskannya ke backend Go (yang
      // menyajikan dist/ lama → aset 404 → halaman kosong saat dev).
      "^/api(/|$)": "http://localhost:8080",
      "^/uploads(/|$)": "http://localhost:8080",
      // Endpoint kompatibel Slackersender hidup di ROOT backend (bukan /api).
      "^/(send-message|send-media|send-location|info-device|check-number)$": "http://localhost:8080",
    },
  },
  build: {
    outDir: "dist",
    rollupOptions: {
      // Dua entry: index.html (dashboard SPA) + landing.html (halaman "/" ringan).
      // router.go melayani exact "/" dari dist/landing.html — tanpa entry ini,
      // tiap build menghapus landing.html dan "/" jatuh ke dashboard.
      input: {
        main: path.resolve(__dirname, "index.html"),
        landing: path.resolve(__dirname, "landing.html"),
      },
    },
  },
});
