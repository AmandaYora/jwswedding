-- Best-effort: skema lama dikembalikan KOSONG + kolom bantu dikembalikan
-- kosong. Isi PO yang dibuang di `up` tidak bisa dipulihkan dari sini --
-- pulihkan dari cadangan (R1) bila benar-benar perlu mundur.
ALTER TABLE quotations
  ADD COLUMN legacy_project_id BIGINT UNSIGNED NULL,
  ADD INDEX idx_quotations_legacy_project (legacy_project_id);

CREATE TABLE project_package_orders (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  project_id BIGINT UNSIGNED NOT NULL,
  po_number VARCHAR(40) NULL,
  number_period CHAR(6) NULL,
  number_seq INT UNSIGNED NULL,
  revision INT UNSIGNED NOT NULL DEFAULT 0,
  base_price BIGINT UNSIGNED NOT NULL DEFAULT 0,
  terms_text TEXT NOT NULL,
  bonus_note TEXT NOT NULL,
  terms_plan_json JSON NULL,
  status ENUM('Draft', 'Terbit', 'Dibatalkan') NOT NULL DEFAULT 'Draft',
  snapshot_json JSON NULL,
  issued_at TIMESTAMP NULL,
  created_by_staff_id BIGINT UNSIGNED NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_project_package_orders_number (po_number),
  UNIQUE KEY uq_project_package_orders_project (project_id),
  INDEX idx_project_package_orders_period (number_period),
  CONSTRAINT fk_project_package_orders_project
    FOREIGN KEY (project_id) REFERENCES projects (id)
);

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

CREATE TABLE project_package_adjustments (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  project_id BIGINT UNSIGNED NOT NULL,
  description VARCHAR(255) NOT NULL,
  amount BIGINT NOT NULL,
  sort_order INT NOT NULL,
  INDEX idx_project_package_adjustments_project (project_id),
  CONSTRAINT fk_project_package_adjustments_project
    FOREIGN KEY (project_id) REFERENCES projects (id)
);
