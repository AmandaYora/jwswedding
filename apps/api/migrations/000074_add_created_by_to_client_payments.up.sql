-- Identitas pengesah Kwitansi (PLAN tanda-tangan-pengguna, K2/K5).
--
-- quotations dan client_invoices sudah menyimpan created_by_staff_id; hanya
-- client_payments yang belum, padahal Kwitansi juga mencetak blok tanda tangan.
-- ClientPaymentService.Create SUDAH menerima actorStaffID — kolom ini hanya
-- membuatnya tersimpan.
--
-- DEFAULT 0 (berbeda dari client_invoices yang NOT NULL tanpa default) karena
-- baris lama harus punya nilai. 0 adalah sentinel "tidak ter-resolve": blok
-- nama dan tanda tangan dikosongkan total saat dicetak (K6).
ALTER TABLE client_payments
  ADD COLUMN created_by_staff_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER notes;
