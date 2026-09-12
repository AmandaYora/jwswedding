-- Komposisi paket per project (PLAN.md po-paket-client §3.2). Cermin
-- package_template_blocks, disalin saat ApplyTemplate lalu hidup sendiri --
-- sengaja TIDAK menyimpan template_id (D6/D22), supaya template yang diedit
-- kemudian tidak pernah mengubah isi kontrak yang sudah disepakati.
CREATE TABLE project_package_blocks (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  project_id BIGINT UNSIGNED NOT NULL,
  category VARCHAR(80) NOT NULL,
  body TEXT NOT NULL,
  qty_text TEXT NOT NULL,
  bonus_note TEXT NOT NULL,
  sort_order INT NOT NULL,
  INDEX idx_project_package_blocks_project (project_id),
  CONSTRAINT fk_project_package_blocks_project
    FOREIGN KEY (project_id) REFERENCES projects (id)
);

-- Blok ADDITIONAL & TAKEOUT (B4 di dokumen sumber): satu-satunya bagian
-- komposisi yang bernominal.
CREATE TABLE project_package_adjustments (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  project_id BIGINT UNSIGNED NOT NULL,
  description VARCHAR(255) NOT NULL,
  -- D2: BIGINT BERTANDA -- sengaja BUKAN UNSIGNED, satu-satunya kolom uang
  -- di basis data ini yang begitu (client_payments.amount,
  -- projects.contract_value, vendor_payments.amount semuanya UNSIGNED).
  -- Negatif = takeout/cashback. Menyalin-tempel UNSIGNED ke sini akan
  -- membuat SETIAP baris takeout gagal tersimpan, dan pesan errornya tidak
  -- akan menunjuk ke baris ini. Lihat PLAN.md §9 R1.
  amount BIGINT NOT NULL,
  sort_order INT NOT NULL,
  INDEX idx_project_package_adjustments_project (project_id),
  CONSTRAINT fk_project_package_adjustments_project
    FOREIGN KEY (project_id) REFERENCES projects (id)
);
