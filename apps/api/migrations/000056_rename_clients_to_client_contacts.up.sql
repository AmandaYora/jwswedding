-- Fase 1 penawaran-client-master (D1, D3, T1.1): `clients` hari ini sebenarnya
-- akun kontak portal per project, bukan data klien -- rename supaya nama di kode
-- sejalan dengan "Client" di menu. RENAME TABLE, bukan create-and-copy, supaya
-- id baris TIDAK berubah: akun portal adalah Credential di modul identity
-- dengan PrincipalID = id baris itu, jadi id stabil = kredensial tak tersentuh.
RENAME TABLE clients TO client_contacts;

-- Nama indeks ikut diluruskan (fungsinya sama, hanya penamaannya).
ALTER TABLE client_contacts RENAME INDEX idx_clients_tenant TO idx_client_contacts_tenant;
ALTER TABLE client_contacts RENAME INDEX idx_clients_project TO idx_client_contacts_project;
