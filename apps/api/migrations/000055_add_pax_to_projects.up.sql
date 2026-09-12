-- Jumlah Pax (PLAN.md po-paket-client §3.4, blok B1 dokumen sumber).
-- Aditif ber-DEFAULT 0; 0 = "belum ditentukan", konvensi sentinel yang sama
-- dengan projects.pic_sales_staff_id. Tidak ada backfill -- baris lama
-- memang belum punya nilainya.
ALTER TABLE projects
  ADD COLUMN pax INT UNSIGNED NOT NULL DEFAULT 0 AFTER event_end_time;
