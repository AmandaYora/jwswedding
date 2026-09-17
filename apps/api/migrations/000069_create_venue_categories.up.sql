-- Kategori Venue (PLAN revisi-vendor-venue-portal §4.1, poin 4): satu venue
-- boleh membawa LEBIH DARI SATU kategori — sebuah gedung bisa sekaligus
-- "Ballroom (carpet)" dan "Hotel Bintang 5" — jadi relasinya tabel tersendiri,
-- bukan satu kolom di `venues`.
--
-- JANGAN tertukar dengan `vendor_categories`: tabel itu adalah master milik
-- tenant dengan CRUD sendiri (menu "Kategori Vendor", Owner-only). Tabel ini
-- murni tabel label — `category` selalu salah satu dari daftar tetap di
-- domain.AllowedVenueCategories, dan tidak ada layar untuk menambah nilai baru.
--
-- FK ke `venues` diizinkan karena keduanya milik modul `vendors` yang sama
-- (knowledge/DATABASE_GUIDE.md §Conventions melarang FK lintas modul, bukan FK
-- di dalam satu modul). ON DELETE CASCADE membuat VenueService.Delete tetap
-- satu DELETE tanpa pembersihan manual.
CREATE TABLE venue_categories (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  venue_id BIGINT UNSIGNED NOT NULL,
  category VARCHAR(50) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  -- Menjadikan penulisan ulang idempoten, DAN melayani dua jalur baca
  -- terpanas sekaligus (venue_id di posisi terdepan): predikat EXISTS pada
  -- filter kategori, dan `WHERE venue_id IN (...)` milik loadCategories.
  UNIQUE KEY uq_venue_categories_venue_category (venue_id, category),
  -- Arah sebaliknya: "venue mana saja yang berkategori X dalam tenant ini".
  KEY idx_venue_categories_tenant_category (tenant_id, category),
  CONSTRAINT fk_venue_categories_venue FOREIGN KEY (venue_id) REFERENCES venues (id) ON DELETE CASCADE
);
