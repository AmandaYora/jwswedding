-- Penawaran-client-master (T3.5): anak komposisi ikut terhapus saat dokumennya
-- dihapus — hapus langsung, cascade project, maupun sapu Client. 000060
-- lupa membawa ON DELETE CASCADE dari tabel project_package_* yang diganti
-- (ditangkap TestAccept_HapusBerjenjangDanImpact, FK 1451).
--
-- DROP dan ADD dipisah per pernyataan: MySQL menolak memakai ulang nama
-- constraint yang sama dalam satu ALTER TABLE (Error 1826).
ALTER TABLE quotation_blocks DROP FOREIGN KEY fk_quotation_blocks_quotation;
ALTER TABLE quotation_blocks
  ADD CONSTRAINT fk_quotation_blocks_quotation
    FOREIGN KEY (quotation_id) REFERENCES quotations (id) ON DELETE CASCADE;

ALTER TABLE quotation_adjustments DROP FOREIGN KEY fk_quotation_adjustments_quotation;
ALTER TABLE quotation_adjustments
  ADD CONSTRAINT fk_quotation_adjustments_quotation
    FOREIGN KEY (quotation_id) REFERENCES quotations (id) ON DELETE CASCADE;
