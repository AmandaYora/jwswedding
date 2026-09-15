import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { "@": path.resolve(__dirname, "src") } },
  // Satu `.env` untuk seluruh monorepo, di root repo — bukan satu per app.
  // Backend sudah memakai file yang sama (`config.Load()` membaca `.env` lalu
  // `../../.env`, dan `npm run dev:api` berjalan dari apps/api), dan
  // `.env.example` di root memang sudah mendaftarkan VITE_API_BASE_URL di
  // samping variabel API.
  //
  // Tanpa baris ini Vite hanya melihat `apps/web/.env`, sehingga
  // VITE_API_BASE_URL yang sudah terisi di root diam-diam terbaca `undefined`
  // — axios lalu memakai baseURL kosong dan setiap panggilan API mendarat di
  // dev server Vite sendiri (port 5173), yang membalas index.html untuk rute
  // apa pun. Gejalanya: frontend dan backend tampak "tidak terintegrasi"
  // padahal keduanya jalan dan konfigurasinya benar.
  //
  // Aman untuk rahasia: Vite hanya menyuntikkan variabel ber-awalan `VITE_`
  // ke `import.meta.env`. DB_PASSWORD/JWT_SECRET/S3_SECRET_KEY yang ada di
  // file yang sama TIDAK pernah masuk bundle (diverifikasi dengan menggeledah
  // hasil build). Beri awalan `VITE_` hanya pada nilai yang memang boleh
  // dibaca siapa pun yang membuka DevTools.
  //
  // Docker tidak terpengaruh: `.dockerignore` membuang `.env`, jadi di sana
  // VITE_API_BASE_URL memang kosong dan frontend memakai path relatif —
  // persis yang diinginkan, karena satu container menyajikan API dan web di
  // origin yang sama.
  envDir: path.resolve(__dirname, "..", ".."),
  server: { port: 5173 },
});
