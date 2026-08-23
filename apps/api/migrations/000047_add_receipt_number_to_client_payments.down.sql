ALTER TABLE client_payments
  DROP KEY uq_client_payments_receipt_number,
  DROP KEY idx_client_payments_receipt_period,
  DROP COLUMN receipt_number,
  DROP COLUMN receipt_period,
  DROP COLUMN receipt_seq;
