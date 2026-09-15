-- Fase 2 penawaran-client-master (D6, D9, T2.1): PO lahir di fase penawaran --
-- belum punya project -- dan menjadi milik project begitu diterima. Modul
-- baru `quotations` memiliki dokumen + komposisi + penyesuaiannya sendiri;
-- Template Paket pindah modul tanpa pindah tabel (T2.4).
--
-- Relasi lintas modul sebagai ID primitif TANPA FK (database.md):
-- quotations.client_id, quotations.venue_id, projects.quotation_id.
-- Relasi dalam satu modul tetap FK nyata: quotation_blocks/adjustments.
CREATE TABLE quotations (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT UNSIGNED NOT NULL,
  client_id BIGINT UNSIGNED NOT NULL,
  -- D7: nomor PO = identitas tunggal penawaran. NULL sampai dikirim ke klien
  -- (fase Ditawarkan). MySQL mengizinkan banyak NULL dalam UNIQUE index.
  po_number VARCHAR(40) NULL,
  number_period CHAR(6) NULL,
  number_seq INT UNSIGNED NULL,
  revision INT UNSIGNED NOT NULL DEFAULT 0,
  -- D6/D18: satu dokumen dengan fase. Judul cetak tetap "PURCHASE ORDER".
  status ENUM('Draft', 'Ditawarkan', 'Diterima', 'Ditolak', 'Kedaluwarsa', 'Dibatalkan') NOT NULL DEFAULT 'Draft',
  base_price BIGINT UNSIGNED NOT NULL DEFAULT 0,
  terms_text TEXT NOT NULL,
  bonus_note TEXT NOT NULL,
  terms_plan_json JSON NULL,
  -- Pindah dari project (dibawa saat Accept, dibaca saat Issue): tanggal dan
  -- tempat acara yang ditawarkan, sebelum project-nya ada.
  event_date DATE NULL,
  pax INT UNSIGNED NOT NULL DEFAULT 0,
  venue_id BIGINT UNSIGNED NULL,
  snapshot_json JSON NULL,
  issued_at TIMESTAMP NULL,
  accepted_at TIMESTAMP NULL,
  created_by_staff_id BIGINT UNSIGNED NOT NULL,
  -- Kolom bantu cutover satu arah (pola yang sama dengan 000057): pemetaan
  -- project -> quotations selama 000061, DIBUANG di 000062.
  legacy_project_id BIGINT UNSIGNED NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_quotations_number (po_number),
  INDEX idx_quotations_tenant_status (tenant_id, status),
  INDEX idx_quotations_client (client_id),
  INDEX idx_quotations_legacy_project (legacy_project_id)
);

CREATE TABLE quotation_blocks (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  quotation_id BIGINT UNSIGNED NOT NULL,
  category VARCHAR(80) NOT NULL,
  body TEXT NOT NULL,
  qty_text TEXT NOT NULL,
  bonus_note TEXT NOT NULL,
  sort_order INT NOT NULL,
  INDEX idx_quotation_blocks_quotation (quotation_id),
  CONSTRAINT fk_quotation_blocks_quotation
    FOREIGN KEY (quotation_id) REFERENCES quotations (id)
);

CREATE TABLE quotation_adjustments (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  quotation_id BIGINT UNSIGNED NOT NULL,
  description VARCHAR(255) NOT NULL,
  -- BIGINT BERTANDA seperti pendahulunya (D2 PO Paket): negatif = takeout.
  amount BIGINT NOT NULL,
  sort_order INT NOT NULL,
  INDEX idx_quotation_adjustments_quotation (quotation_id),
  CONSTRAINT fk_quotation_adjustments_quotation
    FOREIGN KEY (quotation_id) REFERENCES quotations (id)
);

-- Lintas modul => TANPA FK. UNIQUE menegakkan "1 penawaran Diterima =
-- 1 project" di tingkat basis data sekaligus membuat Accept idempoten (T3.2):
-- Accept yang diulang mengembalikan project yang sudah ada.
ALTER TABLE projects
  ADD COLUMN quotation_id BIGINT UNSIGNED NULL,
  ADD UNIQUE KEY uq_projects_quotation (quotation_id);
