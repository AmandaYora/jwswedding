import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config";

// Konfigurasi tes berdiri di atas vite.config.ts, bukan menduplikasinya:
// alias `@/*`, plugin React, dan envDir monorepo ikut apa adanya. Kalau salah
// satunya berubah di sana, tes tidak diam-diam memakai yang lama.
//
// globals sengaja dibiarkan mati. describe/it/expect diimpor eksplisit di tiap
// berkas tes, jadi tsc yang sama dengan build (`include: ["src"]`) tetap bisa
// memeriksa berkas-berkas itu tanpa perlu menambahkan `types` global.
export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      environment: "jsdom",
      globals: false,
      include: ["src/**/*.test.{ts,tsx}"],
    },
  })
);
