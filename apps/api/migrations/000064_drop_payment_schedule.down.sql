-- Membangun ulang bentuk tabel/kolomnya saja. ISINYA tidak bisa dikembalikan:
-- rencana termin tidak disalin ke mana pun sebelum dihapus, jadi turun dari
-- migrasi ini menghasilkan skema yang benar dengan data kosong.
CREATE TABLE package_template_terms (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  template_id BIGINT UNSIGNED NOT NULL,
  sequence INT NOT NULL,
  label VARCHAR(120) NOT NULL,
  type ENUM('DP', 'Termin', 'Pelunasan') NOT NULL,
  percent DECIMAL(5,2) NULL,
  fixed_amount BIGINT UNSIGNED NULL,
  days_before_event INT NOT NULL,
  INDEX idx_package_template_terms_template (template_id),
  CONSTRAINT fk_package_template_terms_template
    FOREIGN KEY (template_id) REFERENCES package_templates (id) ON DELETE CASCADE
);

ALTER TABLE quotations ADD COLUMN terms_plan_json JSON NULL AFTER bonus_note;
