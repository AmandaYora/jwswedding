-- Fase 1 penawaran-client-master (D1, D2, T1.2): master pasangan `clients` yang
-- baru, lepas dari project. Satu clients = satu pasangan, punya 1..n
-- client_contacts berperan (Bride / Groom / Family Representative).
--
-- `legacy_project_id` adalah kolom bantu cutover SATU ARAH (R1, ADR-0027):
-- ia memegang pemetaan project -> clients selama 000058 mengisi
-- client_contacts.client_id dan projects.client_id, lalu DIBUANG di 000059.
-- Tanpa ini pemetaannya harus ditebak dari nama mempelai yang bisa kembar.
CREATE TABLE clients (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT UNSIGNED NOT NULL,
  bride_name VARCHAR(150) NOT NULL DEFAULT '',
  groom_name VARCHAR(150) NOT NULL DEFAULT '',
  phone VARCHAR(30) NOT NULL DEFAULT '',
  email VARCHAR(150) NOT NULL DEFAULT '',
  notes TEXT NOT NULL,
  legacy_project_id BIGINT UNSIGNED NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_clients_tenant (tenant_id),
  INDEX idx_clients_legacy_project (legacy_project_id)
);

-- Dalam satu modul => FK nyata (§5): client_contacts.client_id -> clients.id.
-- NULL-able: baris yatim pra-cutover (project_id menunjuk project yang sudah
-- tidak ada) tetap lolos backfill tanpa menggagalkan migrasi produksi; kode
-- aplikasi wajib mengisi client_id di setiap pembuatan kontak baru.
ALTER TABLE client_contacts
  ADD COLUMN client_id BIGINT UNSIGNED NULL,
  ADD INDEX idx_client_contacts_client (client_id),
  ADD CONSTRAINT fk_client_contacts_client
    FOREIGN KEY (client_id) REFERENCES clients (id);

-- Lintas modul => ID primitif TANPA FK (database.md): projects.client_id.
ALTER TABLE projects
  ADD COLUMN client_id BIGINT UNSIGNED NULL,
  ADD INDEX idx_projects_client (client_id);
