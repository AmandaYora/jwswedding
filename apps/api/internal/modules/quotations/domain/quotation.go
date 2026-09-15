// Package domain memegang entitas modul quotations (PLAN
// penawaran-client-master, D9): Penawaran sebagai PO pra-deal — pindahan dari
// PackageOrder milik projects, dengan projectID diganti quotationID dan fase
// diperluas (D6, D18).
package domain

import "time"

type QuotationStatus string

const (
	QuotationDraft     QuotationStatus = "Draft"
	QuotationOffered   QuotationStatus = "Ditawarkan"
	QuotationAccepted  QuotationStatus = "Diterima"
	QuotationRejected  QuotationStatus = "Ditolak"
	QuotationExpired   QuotationStatus = "Kedaluwarsa"
	QuotationCancelled QuotationStatus = "Dibatalkan"
)

// Quotation adalah satu dokumen penawaran dengan fase (D6): Draft →
// Ditawarkan → Diterima, plus Ditolak / Kedaluwarsa / Dibatalkan. Judul cetak
// tetap "PURCHASE ORDER" di semua fase. Lahir di fase penawaran — belum punya
// project — dan menjadi milik project begitu diterima (D11).
type Quotation struct {
	ID       int64
	TenantID int64
	// ClientID adalah relasi lintas modul ke master Client — ID primitif TANPA
	// foreign key (database.md).
	ClientID int64
	// PONumber adalah identitas tunggal penawaran (D7): "" sampai dikirim ke
	// klien (fase Ditawarkan), lalu permanen — revisi menaikkan Revision dan
	// tidak pernah menomori ulang (D8).
	PONumber     string
	NumberPeriod string
	NumberSeq    int
	Revision     int
	BasePrice    int64
	// PackageName adalah nama komersial paket yang dijual ("Silver"), DISALIN
	// dari Template Paket saat penawaran dibuat dan boleh diketik ulang
	// selama Draft. Inilah yang menjadi projects.package_name saat Accept,
	// dan yang didorong ulang tiap kali penawaran diedit — sejalur dengan
	// contract_value (D15), supaya PO dan Invoice tidak pernah menyebut nama
	// berbeda untuk paket yang sama.
	//
	// "" hanya mungkin pada penawaran yang lahir sebelum kolom ini ada:
	// Issue mewajibkannya, jadi tidak ada penawaran baru yang bisa sampai ke
	// Accept tanpa nama.
	PackageName string
	TermsText   string
	BonusNote   string
	Status      QuotationStatus
	// EventDate/Pax/VenueID pindah dari project: tanggal, pax, dan tempat
	// acara yang DITAWARKAN, sebelum project-nya ada. VenueID lintas modul
	// (vendors) — tanpa FK.
	EventDate *time.Time
	Pax       int
	VenueID   *int64
	// Snapshot membekukan komposisi + penyesuaian + identitas acara saat
	// dikirim (Issue). Nil selama Draft — PDF Draft dirender dari tabel live.
	Snapshot         *QuotationSnapshot
	IssuedAt         *time.Time
	AcceptedAt       *time.Time
	CreatedByStaffID int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsNumbered melaporkan apakah penawaran sudah melewati Issue pertama —
// pengaman satu-satunya panggilan NextPOSequence.
func (o *Quotation) IsNumbered() bool { return o.PONumber != "" }

// QuotationSnapshot adalah dokumen beku: `current` yang dicetak, `history`
// revisi yang digantikan. Bentuk objek eksplisit, bukan array telanjang.
type QuotationSnapshot struct {
	Current QuotationRevision   `json:"current"`
	History []QuotationRevision `json:"history,omitempty"`
}

// QuotationRevision adalah satu status beku kesepakatan. Identitas acara ikut
// dibekukan karena kontrak yang ditandatangani harus tetap menunjukkan venue,
// pax, dan tanggal seperti saat penandatanganan.
type QuotationRevision struct {
	Revision  int       `json:"revision"`
	IssuedAt  time.Time `json:"issuedAt"`
	BasePrice int64     `json:"basePrice"`
	// Ikut dibekukan: PO non-Draft dicetak dari snapshot, jadi mencetak ulang
	// dokumen yang sudah diteken harus menampilkan nama paket seperti saat
	// itu — bukan hasil revisi terbaru.
	PackageName string                `json:"packageName"`
	TermsText   string                `json:"termsText"`
	BonusNote   string                `json:"bonusNote"`
	Blocks      []QuotationBlock      `json:"blocks"`
	Adjustments []QuotationAdjustment `json:"adjustments"`
	// Signature adalah pembubuhan TTD klien pada revisi ini (TTD Penawaran,
	// D6c): SALINAN milik dokumen — bukan rujukan ke baris client_signatures
	// yang bisa ditimpa (D6b) atau dihapus (D6d). nil = revisi ini belum
	// diteken. Revisi baru lahir dengan nil (D5): arsip saat revisi sudah
	// berlaku sendiri lewat reopen yang mendorong Current ke History (T4),
	// jadi Issue/reopen tidak disentuh untuk urusan TTD.
	Signature *QuotationClientSignature `json:"signature,omitempty"`
	// Snapshot lama masih menyimpan kunci "termsPlan" — tidak ada field yang
	// memetakannya lagi, dan encoding/json memang mengabaikan kunci tak
	// dikenal, jadi dokumen yang sudah beku tetap terbaca tanpa migrasi.
	Event QuotationEventSnapshot `json:"event"`
}

// QuotationClientSignature adalah bukti kesepakatan pada satu revisi: siapa
// meneken, kapan, lewat jalur mana, dan di objek mana salinannya disimpan.
// Berdiri sendiri sepenuhnya — TANPA SignatureID: snapshot tidak boleh
// menunjuk baris yang bisa berubah atau hilang (§6.4).
type QuotationClientSignature struct {
	SignerName string `json:"signerName"`
	SignerRole string `json:"signerRole"`
	// StorageKey adalah objek milik DOKUMEN INI
	// (jwswedding/signature/quotation/...), bukan kunci specimen.
	StorageKey string    `json:"storageKey"`
	SignedAt   time.Time `json:"signedAt"`
	// Channel: magic_link | upload | specimen — membedakan TTD yang
	// dibubuhkan klien sendiri dari yang diunggah pengelola.
	Channel string `json:"channel"`
}

// QuotationEventSnapshot adalah kotak header B1 seperti saat dikirim.
type QuotationEventSnapshot struct {
	ClientName string `json:"clientName"`
	Phone      string `json:"phone"`
	EventDate  string `json:"eventDate"`
	Venue      string `json:"venue"`
	Pax        int    `json:"pax"`
}

// QuotationBlock adalah salinan komposisi milik penawaran — tanpa template_id:
// sekali disalin, kesepakatan independen dari master asalnya.
type QuotationBlock struct {
	ID          int64  `json:"-"`
	QuotationID int64  `json:"-"`
	Category    string `json:"category"`
	Body        string `json:"body"`
	QtyText     string `json:"qtyText"`
	BonusNote   string `json:"bonusNote"`
	SortOrder   int    `json:"sortOrder"`
}

// QuotationAdjustment adalah satu baris ADDITIONAL/TAKEOUT — satu-satunya
// bagian komposisi yang bernominal. Amount BERTANDA: negatif = takeout.
type QuotationAdjustment struct {
	ID          int64  `json:"-"`
	QuotationID int64  `json:"-"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	SortOrder   int    `json:"sortOrder"`
}

// TotalAdjustments menjumlah penyesuaian bertanda. Total penawaran =
// BasePrice + TotalAdjustments (= contract_value project setelah Accept).
func TotalAdjustments(adjustments []QuotationAdjustment) int64 {
	var total int64
	for _, a := range adjustments {
		total += a.Amount
	}
	return total
}
