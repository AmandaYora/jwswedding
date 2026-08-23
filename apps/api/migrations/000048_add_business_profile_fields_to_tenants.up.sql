ALTER TABLE tenants
  ADD COLUMN address TEXT NULL AFTER city,
  ADD COLUMN bank_name VARCHAR(100) NULL AFTER address,
  ADD COLUMN bank_account_number VARCHAR(50) NULL AFTER bank_name,
  ADD COLUMN bank_account_holder_name VARCHAR(150) NULL AFTER bank_account_number;
