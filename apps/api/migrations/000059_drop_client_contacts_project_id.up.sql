-- Fase 1 penawaran-client-master (T1.4): lepas client_contacts dari project.
-- Dijalankan SETELAH 000058 terverifikasi (R1), bukan di migrasi yang sama.
-- Mulai sini satu-satunya tempat relasi itu hidup adalah projects.client_id.
ALTER TABLE client_contacts DROP INDEX idx_client_contacts_project;
ALTER TABLE client_contacts DROP COLUMN project_id;

-- Kolom bantu cutover selesai tugasnya (lihat 000057).
ALTER TABLE clients DROP INDEX idx_clients_legacy_project;
ALTER TABLE clients DROP COLUMN legacy_project_id;
