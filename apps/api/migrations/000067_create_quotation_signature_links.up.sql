-- TTD Penawaran (D8): token magic link tanda tangan — 24 jam, sekali
-- pakai, boleh diterbitkan ulang; link lama mati (revoked) saat yang baru
-- terbit. Token mentah tidak pernah disimpan: yang tersimpan SHA-256-nya.
--
-- FK ke quotations nyata (satu modul): link ikut terhapus saat penawarannya
-- dihapus permanen, mengikuti preseden 000063_quotation_children_cascade.
-- Tidak ada job pembersih: beberapa baris per penawaran, puluhan per tahun.
CREATE TABLE quotation_signature_links (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT UNSIGNED NOT NULL,
  quotation_id BIGINT UNSIGNED NOT NULL,
  token_hash CHAR(64) NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  used_at TIMESTAMP NULL DEFAULT NULL,
  revoked_at TIMESTAMP NULL DEFAULT NULL,
  created_by_staff_id BIGINT UNSIGNED NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_qsl_token (token_hash),
  INDEX idx_qsl_quotation (quotation_id),
  CONSTRAINT fk_qsl_quotation
    FOREIGN KEY (quotation_id) REFERENCES quotations (id) ON DELETE CASCADE
);
