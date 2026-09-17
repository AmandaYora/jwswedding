-- Batasi wilayah operasi ke Jabodetabek + Bali (PLAN revisi-vendor-venue-portal
-- §4.1, poin 9): "hilangkan kota selain jabodetabek - bali, karena sementara
-- kita hanya mencakup area itu".
--
-- domain.AllowedCities dipangkas dari 128 menjadi 23 entri di kode. Tanpa
-- migrasi ini, setiap vendor/venue lama yang kotanya di luar 23 itu menjadi
-- TIDAK BISA DISIMPAN saat diedit: Combobox di frontend tidak menampilkan nilai
-- yang tidak ada di daftar, dan validateVenueCity/validateVendorCity di backend
-- menolaknya dengan 422. Mengosongkan kolomnya membuat baris itu tetap bisa
-- dibuka, diedit, dan diisi ulang kotanya.
--
-- Hanya kolom `city` yang disentuh; tidak ada baris yang dihapus.
--
-- Kedua tabel milik modul `vendors` yang sama, jadi ini bukan backfill lintas
-- modul.

UPDATE vendors SET city = NULL
WHERE city IS NOT NULL
  AND city NOT IN (
    'Jakarta Pusat', 'Jakarta Utara', 'Jakarta Barat', 'Jakarta Selatan', 'Jakarta Timur',
    'Kepulauan Seribu',
    'Kota Bogor', 'Kabupaten Bogor',
    'Kota Depok',
    'Kota Tangerang', 'Kota Tangerang Selatan', 'Kabupaten Tangerang',
    'Kota Bekasi', 'Kabupaten Bekasi',
    'Kota Denpasar', 'Kabupaten Badung', 'Kabupaten Bangli', 'Kabupaten Buleleng',
    'Kabupaten Gianyar', 'Kabupaten Jembrana', 'Kabupaten Karangasem',
    'Kabupaten Klungkung', 'Kabupaten Tabanan'
  );

UPDATE venues SET city = NULL
WHERE city IS NOT NULL
  AND city NOT IN (
    'Jakarta Pusat', 'Jakarta Utara', 'Jakarta Barat', 'Jakarta Selatan', 'Jakarta Timur',
    'Kepulauan Seribu',
    'Kota Bogor', 'Kabupaten Bogor',
    'Kota Depok',
    'Kota Tangerang', 'Kota Tangerang Selatan', 'Kabupaten Tangerang',
    'Kota Bekasi', 'Kabupaten Bekasi',
    'Kota Denpasar', 'Kabupaten Badung', 'Kabupaten Bangli', 'Kabupaten Buleleng',
    'Kabupaten Gianyar', 'Kabupaten Jembrana', 'Kabupaten Karangasem',
    'Kabupaten Klungkung', 'Kabupaten Tabanan'
  );
