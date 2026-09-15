-- Fase 1 penawaran-client-master (T1.3): satu baris `clients` per project dari
-- bride_name/groom_name + telepon kontak pertama yang punya nomor. Berjalan
-- atas ratusan baris dalam pernyataan INSERT ... SELECT, bukan loop (§11).
--
-- Pemetaan memakai clients.legacy_project_id (lihat 000057): deterministik
-- lewat id project, tidak pernah ditebak dari nama yang bisa kembar.
INSERT INTO clients (tenant_id, bride_name, groom_name, phone, email, notes, legacy_project_id)
SELECT
  p.tenant_id,
  p.bride_name,
  p.groom_name,
  COALESCE((
    SELECT cc.phone FROM client_contacts cc
    WHERE cc.project_id = p.id AND cc.phone <> ''
    ORDER BY cc.id LIMIT 1
  ), ''),
  '',
  '',
  p.id
FROM projects p;

-- Tautkan kontak lama ke master barunya.
UPDATE client_contacts cc
JOIN clients c ON c.legacy_project_id = cc.project_id AND c.tenant_id = cc.tenant_id
SET cc.client_id = c.id;

-- Tautkan project ke master barunya (sumber relasi mulai sekarang).
UPDATE projects p
JOIN clients c ON c.legacy_project_id = p.id
SET p.client_id = c.id;
