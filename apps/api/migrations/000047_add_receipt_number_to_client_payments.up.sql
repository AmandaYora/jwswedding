ALTER TABLE client_payments
  ADD COLUMN receipt_number VARCHAR(40) NULL AFTER reference_number,
  ADD COLUMN receipt_period CHAR(6) NULL AFTER receipt_number,
  ADD COLUMN receipt_seq INT UNSIGNED NULL AFTER receipt_period,
  -- MySQL mengizinkan banyak baris NULL pada UNIQUE key -- persis yang
  -- dibutuhkan penomoran malas: semua pembayaran lama tetap NULL tanpa
  -- backfill, tapi dua kwitansi tidak akan pernah bisa punya nomor sama.
  ADD UNIQUE KEY uq_client_payments_receipt_number (receipt_number),
  ADD KEY idx_client_payments_receipt_period (receipt_period);
