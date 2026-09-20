// Package domain memuat entitas modul `rundowns` — "Panduan Acara Akad &
// Resepsi", buku acara yang dipegang tim WO di hari-H.
//
// Rundown adalah SATU aggregate: `Rundown` sebagai akar dan sebelas koleksi
// anak yang selalu dibaca dan ditulis bersama-sama. Ia juga dokumen snapshot —
// nama vendor, kategori, dan nama project disalin ke sini saat dibuat, jadi
// buku acara yang sudah dicetak tidak ikut berubah kalau master vendor
// disunting belakangan.
package domain

import "time"

// Rundown adalah akar aggregate: satu baris per project.
type Rundown struct {
	ID          int64
	TenantID    int64
	ProjectID   int64
	ProjectName string

	// Sampul. Semua label teks bebas, bukan tanggal/jam terstruktur, supaya
	// "Sabtu, 8 Agustus 2026" tercetak persis seperti yang diketik WO.
	GroomName       string
	GroomBirthOrder string
	GroomParents    string
	BrideName       string
	BrideBirthOrder string
	BrideParents    string
	EventDateLabel  string
	VenueLabel      string
	EventTimeLabel  string
	CoupleTitle     string

	WOPICName  string
	WOPICPhone string

	SiblingsBride  string
	SiblingsGroom  string
	SouvenirNote   string
	TableClothNote string

	PlaylistNotes string

	// LayoutImagePath kosong berarti template memakai gambar denah placeholder
	// bawaannya, bukan denah pasangan lain.
	LayoutImagePath string

	LastDocxEvidenceID int64
	LastPdfEvidenceID  int64

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Summary adalah proyeksi ringan untuk daftar rundown — sengaja tidak memuat
// satu pun koleksi anak, supaya halaman daftar tidak menarik dua belas tabel
// per baris.
type Summary struct {
	ID             int64
	ProjectID      int64
	ProjectName    string
	BrideName      string
	GroomName      string
	EventDateLabel string
	VenueLabel     string
	HasLayoutImage bool
	UpdatedAt      time.Time
}

// View adalah aggregate utuh: akar plus seluruh koleksi anaknya. Inilah yang
// dibaca renderer dokumen.
type View struct {
	Rundown     Rundown
	Vendors     []Vendor
	Roles       []Role
	Committees  []Committee
	MenuItems   []MenuItem
	MakeupRooms []MakeupRoom
	Items       []Item
	LayoutNotes []LayoutNote
	PhotoGroups []PhotoGroup
	VIPGuests   []VIPGuest
	Playlist    []PlaylistEntry
}

// Vendor: satu baris halaman VENDORS. Satu kategori boleh muncul di beberapa
// baris berurutan — renderer yang mengelompokkan dan hanya mencetak baris
// kategori saat kategorinya berganti.
type Vendor struct {
	ID            int64
	SortOrder     int
	CategoryLabel string
	VendorName    string
}

// Role: satu baris LIST NAMA (wali nikah, jubir, penghulu, saksi, qori, ...).
type Role struct {
	ID         int64
	SortOrder  int
	RoleLabel  string
	PersonName string
	Note       string
}

// Committee: satu baris PANITIA KELUARGA. PersonText dan JobDesc boleh
// multi-baris; tiap baris menjadi satu paragraf di dalam selnya.
type Committee struct {
	ID         int64
	SortOrder  int
	RoleLabel  string
	PersonText string
	JobDesc    string
}

// MenuItem: satu baris di salah satu dari tiga kolom DATA LAINNYA.
type MenuItem struct {
	ID        int64
	SortOrder int
	GroupKey  MenuGroup
	Style     MenuStyle
	Content   string
}

// MakeupRoom: satu ruangan pada LIST & RUANGAN MAKEUP, beserta baris isinya.
type MakeupRoom struct {
	ID        int64
	SortOrder int
	RoomLabel string
	Lines     []MakeupLine
}

// MakeupLine: satu baris di dalam sel keterangan sebuah ruangan makeup.
type MakeupLine struct {
	ID        int64
	RoomID    int64
	SortOrder int
	Style     MakeupStyle
	Content   string
}

// Item: satu baris SUSUNAN ACARA. NoLabel diisi ulang oleh service saat seksi
// disimpan, tidak pernah diketik WO.
type Item struct {
	ID        int64
	Section   Section
	SortOrder int
	NoLabel   string
	TimeLabel string
	Item      string
	PIC       string
	Note      string
}

// LayoutNote: aturan tamu di bawah denah (Rule) atau baris tabel keterangan
// bernomor (Legend).
type LayoutNote struct {
	ID          int64
	Kind        LayoutNoteKind
	SortOrder   int
	NumberLabel string
	Content     string
}

// PhotoGroup: satu baris LIST FOTO TAMU.
type PhotoGroup struct {
	ID        int64
	SortOrder int
	GroupName string
}

// VIPGuest: satu baris LIST TAMU VIP.
type VIPGuest struct {
	ID        int64
	SortOrder int
	FullName  string
	Position  string
}

// PlaylistEntry: satu baris PLAYLIST REQUEST LAGU.
type PlaylistEntry struct {
	ID        int64
	SortOrder int
	Title     string
	Artist    string
}
