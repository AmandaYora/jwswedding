CREATE TABLE client_invoices (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  project_id BIGINT UNSIGNED NOT NULL,
  invoice_number VARCHAR(40) NOT NULL,
  -- number_period/number_seq: sumber kebenaran penomoran. MAX(number_seq)+1
  -- per (tenant, period) -- tahan terhadap penghapusan invoice, tidak seperti
  -- COUNT(*) yang akan menghasilkan nomor duplikat. Idiom sama dengan
  -- project_milestones.sort_order (mysql_project_repository.go:450-457).
  number_period CHAR(6) NOT NULL,
  number_seq INT UNSIGNED NOT NULL,
  type ENUM('DP', 'Termin', 'Pelunasan', 'Tambahan') NOT NULL,
  description VARCHAR(255) NOT NULL DEFAULT '',
  amount BIGINT UNSIGNED NOT NULL,
  due_date DATE NOT NULL,
  status ENUM('Draft', 'Terkirim', 'Lunas', 'Dibatalkan') NOT NULL DEFAULT 'Draft',
  -- Sentinel 0 = belum tertaut. Sengaja BUKAN foreign key: ClientPayment bisa
  -- dihapus lewat "Batalkan Pelunasan" dan tautannya di-reset ke 0 di alur
  -- aplikasi yang sama, jadi FK constraint hanya akan menghalangi tanpa
  -- menambah jaminan. Konvensi sentinel sama dengan
  -- projects.pic_sales_staff_id.
  client_payment_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_by_staff_id BIGINT UNSIGNED NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_client_invoices_invoice_number (invoice_number),
  KEY idx_client_invoices_project (project_id),
  KEY idx_client_invoices_period (number_period),
  KEY idx_client_invoices_payment (client_payment_id),
  CONSTRAINT fk_client_invoices_project FOREIGN KEY (project_id) REFERENCES projects (id)
);
