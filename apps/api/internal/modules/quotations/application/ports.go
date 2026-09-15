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
}

// VenueResolver adalah bentuk sempit yang dibutuhkan quotations dari
// `vendors` (nama venue untuk snapshot Issue + kop PDF) — salinan idiom
// VenueResolver milik projects (ADR-0016), dipasang two-phase dari main.go.
type VenueResolver interface {
	GetVenueSummary(ctx context.Context, tenantID, venueID int64) (vendorscontracts.VenueSummary, error)
}
