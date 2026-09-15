package domain

import "time"

// PackageTemplate adalah paket penjualan reusable milik tenant — pindahan
// utuh dari projects (T2.4, D13): tabelnya tetap, logikanya tetap. Harganya
// kini berlabel "Harga Standar": nilai awal saat paket dipilih di penawaran,
// boleh ditimpa Sales/Admin/Owner; satu sumber kebenaran (penawaran), satu
// nilai default (template).
type PackageTemplate struct {
	ID       int64
	TenantID int64
	Name     string
	// BasePrice adalah harga daftar ("Harga Standar"). Kalah dari nilai yang
	// disepakati: saat template diterapkan ke penawaran yang base price-nya
	// sudah diisi, nilai itu yang menang.
	BasePrice int64
	// DefaultTerms/DefaultBonusNote tinggal di sini, bukan di `tenants`, agar
	// fitur ini tidak pernah menjangkau modul `platform`, dan agar syarat bisa
	// berbeda per tier paket.
	DefaultTerms     string
	DefaultBonusNote string
	IsActive         bool
	SortOrder        int
	CreatedAt        time.Time
	UpdatedAt        time.Time

	// Blocks selalu terisi dari FindByID. List tidak menghasilkan tipe ini —
	// ia menghasilkan PackageTemplateSummary — supaya pemanggil tidak pernah
	// memegang komposisi yang "kebetulan belum dimuat" dan mengiranya
	// template yang memang kosong.
	Blocks []PackageTemplateBlock
}

// PackageTemplateSummary adalah satu baris daftar: header + HITUNGAN
// komposisi di belakangnya, bukan komposisinya.
type PackageTemplateSummary struct {
	ID               int64
	TenantID         int64
	Name             string
	BasePrice        int64
	DefaultTerms     string
	DefaultBonusNote string
	IsActive         bool
	SortOrder        int
	BlockCount       int
}

// PackageTemplateBlock adalah SATU BARIS tabel komposisi PDF, bukan satu item.
// Kategori teks bebas (bukan FK ke kategori vendor); baris ALL-CAPS dirender
// tebal sebagai sub-heading; QtyText teks bebas yang tidak pernah dijumlah.
type PackageTemplateBlock struct {
	ID         int64
	TemplateID int64
	Category   string
	Body       string
	QtyText    string
	BonusNote  string
	SortOrder  int
}
