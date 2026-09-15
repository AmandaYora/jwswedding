-- TTD Penawaran (D6b/D12): specimen tanda tangan klien — SATU baris per
-- CLIENT, ditimpa saat ada yang baru. Bukan riwayat.
--
-- `role` hanya KETERANGAN pemilik specimen ("milik siapa"), bukan bagian
-- kunci — UNIQUE-nya (client_id) saja. Dokumen yang sudah diteken memegang
-- SALINANNYA SENDIRI di snapshot (D6c), jadi menghapus baris di sini tidak
-- pernah mengubah dokumen mana pun (D6d).
--
-- FK ke clients nyata (satu modul): specimen ikut terhapus saat client-nya
-- dihapus permanen, mengikuti preseden 000063_quotation_children_cascade.
CREATE TABLE client_signatures (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT UNSIGNED NOT NULL,
  client_id BIGINT UNSIGNED NOT NULL,
  role ENUM('Bride','Groom','Family Representative') NOT NULL,
  signer_name VARCHAR(150) NOT NULL,
  storage_key VARCHAR(500) NOT NULL,
  source ENUM('draw','upload') NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_client_signatures_client (client_id),
  INDEX idx_client_signatures_tenant (tenant_id),
  CONSTRAINT fk_client_signatures_client
    FOREIGN KEY (client_id) REFERENCES clients (id) ON DELETE CASCADE
);
