-- Kategori timeline (PLAN.md revisi-putri-mom-25082026, Blok B / item 14):
-- satu kolom VARCHAR pengelompokan tampilan pada milestone project & template.
-- Aditif ber-DEFAULT '' -- baris lama tampil sebagai grup "Tanpa Kategori".
-- Bukan tabel master (D1), bukan reuse vendor_categories (D2).
ALTER TABLE project_milestones
  ADD COLUMN category VARCHAR(50) NOT NULL DEFAULT '' AFTER name;

ALTER TABLE project_milestone_templates
  ADD COLUMN category VARCHAR(50) NOT NULL DEFAULT '' AFTER name;
