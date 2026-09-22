-- PERINGATAN: rollback ini MEMOTONG data. Baris yang memanfaatkan lebar
-- baru harus dipangkas lebih dulu, jika tidak ALTER-nya sendiri gagal
-- dengan Error 1406 yang sama.
UPDATE project_vendors SET scope = LEFT(scope, 255) WHERE CHAR_LENGTH(scope) > 255;

ALTER TABLE project_vendors
  MODIFY COLUMN scope VARCHAR(255) NOT NULL;
