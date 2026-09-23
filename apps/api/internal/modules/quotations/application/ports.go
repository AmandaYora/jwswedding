package application

import (
	"context"

	vendorscontracts "jwswedding/internal/modules/vendors/contracts"
)

// ClientDirectory adalah bentuk sempit yang dibutuhkan quotations dari
// `clients` (kop PO, daftar Penawaran): nama pasangan + telepon dari
// client_id, satu baris atau satu halaman per panggilan (§11). Interface
// lokal (bukan impor clients/contracts) supaya quotations tidak mengimpor
// clients — dipasang dari main.go lewat Module.SetClientDirectory, idiom
// two-phase yang sama dengan ClientAccessResolver. Signature primitif
// ([2]string{bride, groom}) disengaja agar cocok struktural tanpa impor tipe.
type ClientDirectory interface {
	CoupleNames(ctx context.Context, tenantID, clientID int64) (bride, groom string, err error)
	CoupleNamesBatch(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64][2]string, error)
	PhoneForClient(ctx context.Context, tenantID, clientID int64) (string, error)
	// SaveSpecimen menimpa specimen milik client (D6b/D12) — dipanggil setiap
	// penandatanganan dari jalur draw/upload (D10). role dan source string.
	SaveSpecimen(ctx context.Context, tenantID, clientID int64, role, signerName string, img []byte, mimeType, source string) error
	// SpecimenImage mengembalikan bytes gambar specimen untuk dipakai ulang.
	SpecimenImage(ctx context.Context, tenantID, clientID int64) ([]byte, error)
	// SpecimenMeta mengembalikan keterangan specimen sebagai
	// {role, signerName, source, updatedAt} + ada/tidak — bahan gerbang D12a
	// dan layar signature-options.
	SpecimenMeta(ctx context.Context, tenantID, clientID int64) ([4]string, bool, error)
	// SignerOptions mengembalikan opsi Atas Nama sebagai pasangan
	// {role, name} — dari clients, bukan client_contacts (T1).
	SignerOptions(ctx context.Context, tenantID, clientID int64) ([][2]string, error)
	// ClientIDForContact memetakan token portal (id kontak) ke master
	// client-nya — dipakai gerbang baca portal di GetForClientContact (PLAN
	// revisi-vendor-venue-portal §4.3/F2). Signature sama persis dengan milik
	// clients.Contracts, dan main.go sudah memasang SetClientDirectory —
	// tidak ada wiring baru, interface terpenuhi struktural.
	ClientIDForContact(ctx context.Context, tenantID, contactID int64) (int64, error)
}

// VenueResolver adalah bentuk sempit yang dibutuhkan quotations dari
// `vendors` (nama venue untuk snapshot Issue + kop PDF) — salinan idiom
// VenueResolver milik projects (ADR-0016), dipasang two-phase dari main.go.
type VenueResolver interface {
	GetVenueSummary(ctx context.Context, tenantID, venueID int64) (vendorscontracts.VenueSummary, error)
}

// StaffSignerResolver adalah bentuk sempit yang dibutuhkan quotations dari
// `staff` untuk blok tanda tangan kolom WO pada PDF Penawaran/PO (PLAN
// tanda-tangan-pengguna): nama + jabatan + gambar TTD milik staff yang
// MENERBITKAN dokumen, menggantikan nama pemilik usaha + TTD tenant yang dulu
// dipakai semua dokumen.
//
// Bentuknya identik dengan StaffNameResolver.GetSigner milik projects dan
// dijembatani adapter yang sama di main.go — primitif, tanpa mengimpor
// staff/contracts, mengikuti idiom yang sama seperti VenueResolver di atas.
//
// ok=false berarti staffID tidak ter-resolve (sentinel 0, baris staff sudah
// dihapus permanen, atau milik tenant lain) — kolom WO dicetak kosong, bukan
// jatuh ke orang lain (K6). signature nil berarti staff-nya ada tetapi belum
// mengisi TTD — namanya tetap tercetak (K4).
type StaffSignerResolver interface {
	GetSigner(ctx context.Context, tenantID, staffID int64) (name, title string, signature []byte, ok bool, err error)
}
