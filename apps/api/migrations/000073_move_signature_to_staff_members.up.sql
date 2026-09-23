-- Tanda tangan pindah dari tenant ke pengguna (PLAN tanda-tangan-pengguna, K3).
--
-- Sebelum ini satu-satunya TTD yang dimiliki sistem adalah TTD usaha milik
-- tenant, dan SEMUA dokumen mencetaknya bersama tenants.owner_name — sehingga
-- setiap dokumen selalu tampak disahkan Owner. Sekarang TTD menjadi master data
-- per pengguna, dan dokumen mencetak nama + TTD staff yang menerbitkannya.
--
-- Nullable: pengguna tanpa TTD adalah keadaan sah — dokumennya tercetak dengan
-- ruang kosong untuk tanda tangan basah, persis kontrak degrade-gracefully yang
-- sudah dipakai signatureBlock hari ini.
ALTER TABLE staff_members
  ADD COLUMN signature_storage_path VARCHAR(500) NULL AFTER phone;

-- Pindahkan TTD tenant yang sudah ada ke baris Owner-nya. Kunci object storage
-- TIDAK berubah (string yang sama menunjuk objek yang sama), jadi tidak ada
-- berkas yang perlu disalin/dipindah di bucket.
--
-- Aman karena satu tenant tepat satu Owner: staff.CreateOwner hanya dipanggil
-- saat registrasi tenant, dan role 'Owner' tidak bisa diberikan lewat endpoint
-- staff mana pun (StaffService.Update menolaknya).
UPDATE staff_members s
  JOIN tenants t ON t.id = s.tenant_id
  SET s.signature_storage_path = t.signature_storage_path
  WHERE s.role = 'Owner' AND t.signature_storage_path IS NOT NULL;

ALTER TABLE tenants DROP COLUMN signature_storage_path;
