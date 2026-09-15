-- Rollback penyalinan: putuskan tautan, buang hasil salinan (FK CASCADE ikut
-- membersihkan blok + penyesuaian). Tabel project_package_* tidak disentuh --
-- ia baru dibuang di 000062, jadi data sumbernya masih utuh di titik ini.
UPDATE projects SET quotation_id = NULL WHERE quotation_id IS NOT NULL;
-- Anak dihapus eksplisit, TIDAK mengandalkan ON DELETE CASCADE: cascade itu
-- baru ditambahkan 000063 dan sudah dicabut lagi oleh 000063.down yang jalan
-- lebih dulu saat rollback. Mengandalkannya membuat DELETE di bawah mati
-- dengan FK 1451 tepat di jalur mundur yang paling dibutuhkan saat cutover
-- gagal.
DELETE FROM quotation_adjustments;
DELETE FROM quotation_blocks;
DELETE FROM quotations;
