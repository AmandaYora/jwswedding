-- Template Paket (per tenant, PLAN.md po-paket-client §3.1): master paket
-- yang dijual WO, disalin ke sebuah project lewat ApplyTemplate. Idiom sama
-- dengan project_milestone_templates (migrasi 000024) -- setelah disalin,
-- baris di sini tidak pernah dirujuk lagi oleh project tersebut (D6/D22).
--
-- default_terms ada di sini, BUKAN di tenants (D16): menaruhnya di tenants
-- memaksa memperluas platformcontracts.TenantProfile dan menyeret modul
-- platform ke dalam fitur ini. Di sini, seluruh fitur PO tidak menyentuh
-- satu modul pun selain projects -- dan S&K jadi bisa berbeda per tier paket.
CREATE TABLE package_templates (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(255) NOT NULL,
  base_price BIGINT UNSIGNED NOT NULL DEFAULT 0,
  default_terms TEXT NOT NULL,
  default_bonus_note TEXT NOT NULL,
  is_active TINYINT(1) NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_package_templates_tenant (tenant_id)
);

-- D20: satu baris di sini = satu BARIS TABEL di PDF, bukan satu item.
-- Kolom QTY dan BONUS di dokumen sumber (Invoice-PO.pdf) menempel pada baris
-- tabel, bukan pada item -- terbukti dari CATERING yang menempati dua baris
-- dengan bonus berbeda. Menaruhnya di level item membuat "bonus milik blok
-- ini yang mana?" tidak terdefinisi.
CREATE TABLE package_template_blocks (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  template_id BIGINT UNSIGNED NOT NULL,
  -- D4: VARCHAR biasa, bukan FK ke vendor_categories. Tabel itu milik modul
  -- vendors, dan kategori paket memuat BONUS/ADDITIONAL yang bukan kategori
  -- vendor. Nilai distinct per tenant cukup jadi datalist di UI.
  category VARCHAR(80) NOT NULL,
  -- body: daftar item, SATU PER BARIS. Baris ber-HURUF KAPITAL dirender
  -- tebal sebagai sub-judul (D21) -- tidak ada sintaks markup lain yang
  -- perlu dipelajari pengguna; mereka mengetik persis seperti di spreadsheet.
  body TEXT NOT NULL,
  -- D3: TEXT, bukan VARCHAR. Sel QTY bisa berisi beberapa baris sekaligus
  -- (blok STALL/GUBUKAN: empat baris "150 PORSI") dan bisa berisi rentang
  -- ("10-12 METER"). Dirender apa adanya, tidak pernah dihitung.
  qty_text TEXT NOT NULL,
  bonus_note TEXT NOT NULL,
  sort_order INT NOT NULL,
  INDEX idx_package_template_blocks_template (template_id),
  CONSTRAINT fk_package_template_blocks_template
    FOREIGN KEY (template_id) REFERENCES package_templates (id) ON DELETE CASCADE
);

-- Preset skema termin (D11). Disalin ke project_package_orders.terms_plan_json
-- saat ApplyTemplate (D22) -- Issue TIDAK PERNAH membaca tabel ini, supaya
-- template yang diedit 6 bulan kemudian tidak diam-diam mengubah rencana
-- termin kontrak yang sudah diteken.
CREATE TABLE package_template_terms (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  template_id BIGINT UNSIGNED NOT NULL,
  sequence INT NOT NULL,
  label VARCHAR(120) NOT NULL,
  type ENUM('DP', 'Termin', 'Pelunasan') NOT NULL,
  -- Tepat satu dari kedua kolom ini terisi: percent untuk tahap berbasis
  -- persentase (30%/50%), fixed_amount untuk DP bernominal tetap.
  percent DECIMAL(5,2) NULL,
  fixed_amount BIGINT UNSIGNED NULL,
  -- Offset hari terhadap projects.event_date. Positif = sebelum hari H,
  -- konvensi sama dengan project_milestone_templates.days_before_event.
  days_before_event INT NOT NULL,
  INDEX idx_package_template_terms_template (template_id),
  CONSTRAINT fk_package_template_terms_template
    FOREIGN KEY (template_id) REFERENCES package_templates (id) ON DELETE CASCADE
);
