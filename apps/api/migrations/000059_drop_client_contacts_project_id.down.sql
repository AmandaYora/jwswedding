-- Best-effort: kolom dikembalikan kosong + kolom bantu cutover dikembalikan
-- kosong. Isi relasi project yang dibuang di `up` tidak bisa dipulihkan dari
-- sini -- pulihkan dari cadangan (R1) bila benar-benar perlu mundur.
ALTER TABLE clients
  ADD COLUMN legacy_project_id BIGINT UNSIGNED NULL,
  ADD INDEX idx_clients_legacy_project (legacy_project_id);

ALTER TABLE client_contacts
  ADD COLUMN project_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD INDEX idx_client_contacts_project (project_id);
