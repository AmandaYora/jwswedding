-- Template Rundown per tenant (PLAN rundown-ux-ideal §6.4 / §7.1): isi standar
-- yang disalin ke rundown baru saat dibuat — List Nama, Panitia Keluarga,
-- Ruangan Makeup, Susunan Acara Akad & Resepsi, dan catatan Layout.
--
-- Satu baris per tenant; seluruh isi di satu kolom JSON karena template selalu
-- dibaca dan ditulis utuh, tidak pernah di-query per baris isinya. Preseden
-- kolom JSON: quotations.snapshot_json (000060).
--
-- Tidak berelasi dengan `rundowns`: isinya disalin (snapshot) saat rundown
-- dibuat, sama seperti prefill vendor. `tenant_id` adalah ID primitif tanpa FK
-- — `tenants` milik modul `platform` (.claude/rules/database.md).
--
-- Tanpa backfill: tenant yang belum punya baris diperlakukan sebagai template
-- kosong oleh repository.
CREATE TABLE rundown_templates (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  payload JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_rundown_templates_tenant (tenant_id)
);
