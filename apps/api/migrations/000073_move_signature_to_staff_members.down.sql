-- Kebalikan 000073: kembalikan TTD ke tingkat tenant.
--
-- Kolom dikembalikan ke posisi semula (AFTER logo_storage_path, sesuai 000049)
-- supaya skema hasil rollback identik dengan sebelum migrasi ini pernah jalan.
ALTER TABLE tenants
  ADD COLUMN signature_storage_path VARCHAR(500) NULL AFTER logo_storage_path;

-- Salin balik dari baris Owner. TTD milik staff non-Owner memang hilang di
-- jalur mundur ini — tenant memang hanya punya satu slot TTD sebelum 000073.
UPDATE tenants t
  JOIN staff_members s ON s.tenant_id = t.id AND s.role = 'Owner'
  SET t.signature_storage_path = s.signature_storage_path
  WHERE s.signature_storage_path IS NOT NULL;

ALTER TABLE staff_members DROP COLUMN signature_storage_path;
