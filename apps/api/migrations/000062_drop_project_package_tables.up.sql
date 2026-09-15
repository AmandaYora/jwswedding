-- Fase 2 penawaran-client-master (T2.10): buang tabel lama + kolom bantu
-- cutover. Dijalankan PALING AKHIR di fase ini, setelah kode tidak lagi
-- membaca tabel lama. Urutan DROP anak-dulu (FK CASCADE ke projects).
DROP TABLE project_package_adjustments;
DROP TABLE project_package_blocks;
DROP TABLE project_package_orders;

ALTER TABLE quotations DROP INDEX idx_quotations_legacy_project;
ALTER TABLE quotations DROP COLUMN legacy_project_id;
