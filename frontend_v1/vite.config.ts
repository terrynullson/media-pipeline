import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "/app/",
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080",
      "/upload": "http://localhost:8080",
      "/media": "http://localhost:8080",
      "/media-source": "http://localhost:8080",
      "/media-audio": "http://localhost:8080",
      "/media-preview": "http://localhost:8080",
      "/media-screenshots": "http://localhost:8080"
    }
  }
});
