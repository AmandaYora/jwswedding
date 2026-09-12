-- Dokumen PO Paket, satu per project (PLAN.md po-paket-client §3.3).
-- Barisnya lahir saat ApplyTemplate/StartBlank dalam status Draft, jauh
-- sebelum diterbitkan -- itulah sebabnya kolom penomoran NULL-able (D26).
CREATE TABLE project_package_orders (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  project_id BIGINT UNSIGNED NOT NULL,
  -- D26: NULL selama Draft. Deal yang batal tidak boleh menghabiskan nomor,
  -- dan NOT NULL di sini mustahil dipenuhi tanpa placeholder -- placeholder
  -- '' akan bertabrakan dengan uq_..._number begitu ada project kedua.
  -- MySQL mengizinkan banyak NULL dalam UNIQUE index, jadi ini aman.
  --
  -- Diisi SEKALI saat Issue pertama, lalu PERMANEN: Revise menaikkan
  -- `revision` dan TIDAK menomori ulang, supaya klien tidak pernah memegang
  -- dua dokumen bernomor beda untuk kontrak yang sama. Format PO/{YYYYMM}/{seq},
  -- penomoran MAX(number_seq)+1 per (tenant, period) -- idiom sama dengan
  -- client_invoices, tahan terhadap penghapusan baris.
  po_number VARCHAR(40) NULL,
  number_period CHAR(6) NULL,
  number_seq INT UNSIGNED NULL,
  revision INT UNSIGNED NOT NULL DEFAULT 0,
  -- "HARGA PAKET AWAL" pada blok B6. D23: saat ApplyTemplate dijalankan pada
  -- project yang contract_value-nya sudah > 0, nilai ITU yang dipakai --
  -- bukan base_price template. Harga yang disepakati adalah hasil negosiasi;
  -- base_price template hanyalah harga daftar.
  base_price BIGINT UNSIGNED NOT NULL DEFAULT 0,
  terms_text TEXT NOT NULL,
  bonus_note TEXT NOT NULL,
  -- D22: preset termin DISALIN ke sini saat ApplyTemplate. Issue membaca
  -- dari kolom ini, TIDAK PERNAH dari package_template_terms.
  terms_plan_json JSON NULL,
  -- D29: 'Dibatalkan' hanya dapat dicapai dari 'Terbit', oleh Owner/Admin,
  -- dan tidak bisa kembali ke 'Draft'. Nomornya tidak didaur ulang dan
  -- barisnya tetap ada sebagai jejak. Satu-satunya jalan kembali ke 'Draft'
  -- adalah lewat Revise.
  status ENUM('Draft', 'Terbit', 'Dibatalkan') NOT NULL DEFAULT 'Draft',
  -- D6/D30: blok + penyesuaian + identitas acara dibekukan di sini saat
  -- terbit. NULL selama Draft -- PDF Draft dirender dari tabel live.
  -- Bentuknya OBJEK, bukan array:
  --   { "current": {...}, "history": [ {"revision": 0, ...}, ... ] }
  -- `current` adalah yang dicetak; Revise memindahkannya ke akhir `history`.
  snapshot_json JSON NULL,
  issued_at TIMESTAMP NULL,
  created_by_staff_id BIGINT UNSIGNED NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_project_package_orders_number (po_number),
  -- Menegakkan "satu PO per project" di tingkat basis data, sehingga
  -- apply-template dua kali ditolak oleh MySQL, bukan oleh pemeriksaan
  -- aplikasi yang bisa terlewat pada race.
  UNIQUE KEY uq_project_package_orders_project (project_id),
  INDEX idx_project_package_orders_period (number_period),
  CONSTRAINT fk_project_package_orders_project
    FOREIGN KEY (project_id) REFERENCES projects (id)
);
