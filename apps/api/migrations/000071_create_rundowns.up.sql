-- Modul `rundowns` — "Panduan Acara Akad & Resepsi" (buku acara hari-H).
-- Lihat docs/plan/rundown-generator/PLAN.md §7.
--
-- Satu project = satu rundown (uq_rundowns_project). Relasi ke `projects`
-- disimpan sebagai ID primitif TANPA foreign key: `projects` modul lain
-- (knowledge/DATABASE_GUIDE.md §Conventions). FK antar tabel `rundown*` di
-- bawah ini justru diizinkan karena semuanya milik modul yang sama —
-- preseden persis: 000069_create_venue_categories.up.sql.
--
-- Rundown adalah dokumen SNAPSHOT: nama vendor, kategori, dan nama project
-- disalin ke sini saat dibuat, bukan di-join saat dicetak. Itu juga yang
-- membuat buku acara yang sudah dicetak tidak berubah isinya ketika master
-- vendor di modul `vendors` disunting belakangan.

CREATE TABLE rundowns (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  project_id BIGINT UNSIGNED NOT NULL,
  -- Snapshot nama project; disegarkan tiap RundownProjectContext dipanggil
  -- (buka detail / generate), jadi paling lama basi sampai rundown dibuka lagi.
  project_name VARCHAR(150) NOT NULL DEFAULT '',

  -- Sampul. Disimpan sebagai label teks bebas, bukan DATE/TIME, supaya
  -- "Sabtu, 8 Agustus 2026" bisa tercetak persis seperti yang diketik WO.
  groom_name VARCHAR(255) NOT NULL DEFAULT '',
  groom_birth_order VARCHAR(255) NOT NULL DEFAULT '',
  groom_parents VARCHAR(255) NOT NULL DEFAULT '',
  bride_name VARCHAR(255) NOT NULL DEFAULT '',
  bride_birth_order VARCHAR(255) NOT NULL DEFAULT '',
  bride_parents VARCHAR(255) NOT NULL DEFAULT '',
  event_date_label VARCHAR(255) NOT NULL DEFAULT '',
  venue_label VARCHAR(255) NOT NULL DEFAULT '',
  event_time_label VARCHAR(255) NOT NULL DEFAULT '',
  -- Judul di lampiran: «THE WEDDING OF DINDA & REZA».
  couple_title VARCHAR(255) NOT NULL DEFAULT '',

  -- Blok ORGANIZED BY di halaman VENDORS.
  wo_pic_name VARCHAR(100) NOT NULL DEFAULT '',
  wo_pic_phone VARCHAR(100) NOT NULL DEFAULT '',

  -- DATA LAINNYA. siblings_* mengisi dua kotak teks pada shape di template,
  -- satu nama per baris.
  siblings_bride TEXT NULL,
  siblings_groom TEXT NULL,
  souvenir_note VARCHAR(255) NOT NULL DEFAULT '',
  table_cloth_note VARCHAR(255) NOT NULL DEFAULT '',

  playlist_notes TEXT NULL,

  -- Diagram denah akad: PNG yang diunggah WO, disimpan di object storage.
  -- Kosong = template memakai gambar placeholder netral bawaannya.
  layout_image_path VARCHAR(255) NOT NULL DEFAULT '',

  -- Dokumen hasil generate terakhir per format, supaya generate berikutnya
  -- MENGGANTI berkas lama di tab Dokumen alih-alih menumpuk (PLAN.md D6).
  -- 0 = belum pernah di-generate.
  last_docx_evidence_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  last_pdf_evidence_id BIGINT UNSIGNED NOT NULL DEFAULT 0,

  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  -- Satu project hanya boleh punya satu buku acara: kalau boleh lebih, WO di
  -- lapangan tidak punya cara tahu mana yang berlaku.
  UNIQUE KEY uq_rundowns_project (project_id),
  KEY idx_rundowns_tenant (tenant_id)
);

-- Halaman VENDORS. Satu kategori boleh punya beberapa vendor ("VENUE &
-- CATERING" punya 2, "RIAS & BUSANA" punya 3); renderer yang mengelompokkan,
-- jadi di sini cukup daftar datar yang urut.
CREATE TABLE rundown_vendors (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rundown_id BIGINT UNSIGNED NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  category_label VARCHAR(100) NOT NULL DEFAULT '',
  vendor_name VARCHAR(150) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_rundown_vendors_rundown (rundown_id, sort_order),
  CONSTRAINT fk_rundown_vendors_rundown FOREIGN KEY (rundown_id) REFERENCES rundowns (id) ON DELETE CASCADE
);

-- LIST NAMA: peran akad (wali nikah, jubir, penghulu, saksi, qori, ...).
CREATE TABLE rundown_roles (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rundown_id BIGINT UNSIGNED NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  role_label VARCHAR(150) NOT NULL DEFAULT '',
  person_name VARCHAR(150) NOT NULL DEFAULT '',
  note VARCHAR(255) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_rundown_roles_rundown (rundown_id, sort_order),
  CONSTRAINT fk_rundown_roles_rundown FOREIGN KEY (rundown_id) REFERENCES rundowns (id) ON DELETE CASCADE
);

-- PANITIA KELUARGA. person_text boleh multi-baris (satu nama/kontak per
-- baris); renderer mencetak satu paragraf per baris.
CREATE TABLE rundown_committees (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rundown_id BIGINT UNSIGNED NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  role_label VARCHAR(150) NOT NULL DEFAULT '',
  person_text TEXT NULL,
  job_desc TEXT NULL,
  PRIMARY KEY (id),
  KEY idx_rundown_committees_rundown (rundown_id, sort_order),
  CONSTRAINT fk_rundown_committees_rundown FOREIGN KEY (rundown_id) REFERENCES rundowns (id) ON DELETE CASCADE
);

-- DATA LAINNYA: tiga kolom menu. `style` menentukan paragraf-template mana di
-- dalam sel yang dipakai, supaya daftar bernomor/berpoin di berkas asli
-- tereproduksi apa adanya, bukan jadi teks datar.
CREATE TABLE rundown_menu_items (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rundown_id BIGINT UNSIGNED NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  group_key ENUM('Stall', 'Buffet', 'AfterAkad') NOT NULL,
  style ENUM('Heading', 'Numbered', 'Bullet', 'Plain') NOT NULL DEFAULT 'Numbered',
  content TEXT NULL,
  PRIMARY KEY (id),
  KEY idx_rundown_menu_items_rundown (rundown_id, sort_order),
  CONSTRAINT fk_rundown_menu_items_rundown FOREIGN KEY (rundown_id) REFERENCES rundowns (id) ON DELETE CASCADE
);

-- LIST & RUANGAN MAKEUP.
CREATE TABLE rundown_makeup_rooms (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rundown_id BIGINT UNSIGNED NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  room_label VARCHAR(150) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_rundown_makeup_rooms_rundown (rundown_id, sort_order),
  CONSTRAINT fk_rundown_makeup_rooms_rundown FOREIGN KEY (rundown_id) REFERENCES rundowns (id) ON DELETE CASCADE
);

-- Baris isi tiap ruangan makeup. `rundown_id` ikut disimpan meski sudah bisa
-- ditelusuri lewat makeup_room_id: itu yang membuat pemuatan aggregate cukup
-- satu query per tabel, bukan satu query per ruangan (N+1).
CREATE TABLE rundown_makeup_lines (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rundown_id BIGINT UNSIGNED NOT NULL,
  makeup_room_id BIGINT UNSIGNED NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  style ENUM('Heading', 'Numbered', 'Dash', 'Note') NOT NULL DEFAULT 'Numbered',
  content TEXT NULL,
  PRIMARY KEY (id),
  KEY idx_rundown_makeup_lines_rundown (rundown_id, sort_order),
  KEY idx_rundown_makeup_lines_room (makeup_room_id, sort_order),
  CONSTRAINT fk_rundown_makeup_lines_rundown FOREIGN KEY (rundown_id) REFERENCES rundowns (id) ON DELETE CASCADE,
  CONSTRAINT fk_rundown_makeup_lines_room FOREIGN KEY (makeup_room_id) REFERENCES rundown_makeup_rooms (id) ON DELETE CASCADE
);

-- SUSUNAN ACARA (akad dan resepsi). `no_label` sengaja VARCHAR, bukan angka:
-- baris pembuka "04.00 - 14.00 Checking Dekor" memang tampil tanpa nomor, dan
-- itu tidak bisa diwakili kolom numerik. Penomorannya sendiri diisi ulang
-- server saat seksi disimpan, bukan diketik WO.
CREATE TABLE rundown_items (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rundown_id BIGINT UNSIGNED NOT NULL,
  section ENUM('Akad', 'Resepsi') NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  no_label VARCHAR(10) NOT NULL DEFAULT '',
  time_label VARCHAR(50) NOT NULL DEFAULT '',
  item TEXT NULL,
  pic TEXT NULL,
  note TEXT NULL,
  PRIMARY KEY (id),
  KEY idx_rundown_items_rundown (rundown_id, section, sort_order),
  CONSTRAINT fk_rundown_items_rundown FOREIGN KEY (rundown_id) REFERENCES rundowns (id) ON DELETE CASCADE
);

-- Halaman LAYOUT AKAD: 'Rule' = tiga poin aturan tamu di bawah denah,
-- 'Legend' = tabel keterangan bernomor.
CREATE TABLE rundown_layout_notes (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rundown_id BIGINT UNSIGNED NOT NULL,
  kind ENUM('Legend', 'Rule') NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  number_label VARCHAR(10) NOT NULL DEFAULT '',
  content TEXT NULL,
  PRIMARY KEY (id),
  KEY idx_rundown_layout_notes_rundown (rundown_id, kind, sort_order),
  CONSTRAINT fk_rundown_layout_notes_rundown FOREIGN KEY (rundown_id) REFERENCES rundowns (id) ON DELETE CASCADE
);

-- LIST FOTO TAMU.
CREATE TABLE rundown_photo_groups (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rundown_id BIGINT UNSIGNED NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  group_name VARCHAR(255) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_rundown_photo_groups_rundown (rundown_id, sort_order),
  CONSTRAINT fk_rundown_photo_groups_rundown FOREIGN KEY (rundown_id) REFERENCES rundowns (id) ON DELETE CASCADE
);

-- LIST TAMU VIP.
CREATE TABLE rundown_vip_guests (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rundown_id BIGINT UNSIGNED NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  full_name VARCHAR(150) NOT NULL DEFAULT '',
  position VARCHAR(150) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_rundown_vip_guests_rundown (rundown_id, sort_order),
  CONSTRAINT fk_rundown_vip_guests_rundown FOREIGN KEY (rundown_id) REFERENCES rundowns (id) ON DELETE CASCADE
);

-- PLAYLIST REQUEST LAGU.
CREATE TABLE rundown_playlist (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  rundown_id BIGINT UNSIGNED NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  title VARCHAR(255) NOT NULL DEFAULT '',
  artist VARCHAR(150) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_rundown_playlist_rundown (rundown_id, sort_order),
  CONSTRAINT fk_rundown_playlist_rundown FOREIGN KEY (rundown_id) REFERENCES rundowns (id) ON DELETE CASCADE
);
