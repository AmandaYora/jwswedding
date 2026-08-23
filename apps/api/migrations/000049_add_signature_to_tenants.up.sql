-- Tanda tangan usaha (PNG) yang diunggah lewat "Profil Usaha", dicetak pada
-- Kwitansi hasil redesain (docs/plan/redesain-pdf-invoice-kwitansi/PLAN.md).
-- Nullable -- tenant yang belum pernah mengunggah tanda tangan tetap bisa
-- mencetak kwitansi, hanya menyisakan ruang kosong untuk tanda tangan basah.
ALTER TABLE tenants
  ADD COLUMN signature_storage_path VARCHAR(500) NULL AFTER logo_storage_path;
