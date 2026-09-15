-- Nama paket yang sudah diketik hilang bersama kolomnya. Yang sudah
-- terlanjur disalin ke projects.package_name saat Accept tetap utuh — kolom
-- itu tidak disentuh migrasi ini, naik maupun turun.
ALTER TABLE quotations DROP COLUMN package_name;
