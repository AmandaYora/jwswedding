-- Root cause error 500 "Terjadi kesalahan pada server" pada
-- POST /projects/{id}/vendors: scope adalah satu-satunya field teks bebas
-- di form yang masuk ke kolom sempit. Dengan STRICT_TRANS_TABLES aktif,
-- scope > 255 karakter menjadi Error 1406, bukan truncate. TEXT mengikuti
-- konvensi yang sudah dipakai vendors.social_media (migrasi 000023).
--
-- Dikonfirmasi dari log produksi 2026-09-22: 56 dari 56 `unhandled error`
-- dalam 45 jam adalah "Error 1406 Data too long for column 'scope'".
-- Lihat docs/plan/vendor-engagement-500/PLAN.md §3.3.
--
-- Tidak ada indeks pada `scope`, jadi MODIFY ini tidak butuh panjang
-- prefiks (project_vendors hanya punya idx_project_vendors_project dan
-- idx_project_vendors_vendor).
ALTER TABLE project_vendors
  MODIFY COLUMN scope TEXT NOT NULL;
