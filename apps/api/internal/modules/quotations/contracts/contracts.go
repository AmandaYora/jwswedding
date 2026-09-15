// Package contracts is the ONLY package other modules may import from
// quotations (PLAN penawaran-client-master, D9/T2.6).
package contracts

import (
	"context"

	"jwswedding/internal/modules/quotations/application"
)

// QuotationImpact adalah potongan sisi-penawaran untuk dialog konfirmasi
// hapus Client (D14, T3.5): berapa penawaran yang belum jadi project (yang
// sudah Diterima dilaporkan/dihapus lewat sisi project).
type QuotationImpact struct {
	Count   int
	Numbers []string
}

// QuotationCleaner adalah port sempit yang dipakai `clients` untuk hapus
// Client berjenjang — implementasinya dipasang saat modul quotations
// dibangun (lihat quotations.module.go).
type QuotationCleaner interface {
	ImpactForClient(ctx context.Context, tenantID, clientID int64) (QuotationImpact, error)
	DeleteForClient(ctx context.Context, tenantID, clientID int64) error
}

// Contracts adalah kontrak publik modul quotations. QuotationResolver
// (dibutuhkan projects: hapus penawaran milik project + baca nomor PO untuk
// delete-impact) ditambahkan bersama implementasinya di modul ini.
type Contracts interface {
	QuotationCleaner
	QuotationResolver
}

// QuotationResolver adalah port sempit yang dipakai `projects`: menghapus
// penawaran milik project yang dihapus dan membaca nomor PO-nya untuk dialog
// delete-impact project (T2.8, T3.5).
type QuotationResolver interface {
	DeleteQuotation(ctx context.Context, tenantID, quotationID int64) error
	PONumberForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error)
	// CompositionForQuotation memberi `projects` bahan tabel komposisi untuk
	// PDF Tagihan. Primitif dengan sengaja — lihat doc comment method-nya di
	// application untuk alasan lingkaran impornya.
	CompositionForQuotation(ctx context.Context, tenantID, quotationID int64) ([][4]string, error)
	// PackageNameForQuotation: "" berarti penawaran pra-000065, yang tidak
	// punya nama paket untuk didorong — satu-satunya keadaan di mana field
	// "Paket / Layanan" project boleh diketik manual lagi.
	PackageNameForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error)
	// StatusForQuotation: "Draft" pada penawaran yang sudah punya project
	// berarti sedang direvisi dan belum dikirim ulang.
	StatusForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error)
}

type impl struct {
	quotations *application.QuotationService
}

func New(quotations *application.QuotationService) Contracts {
	return &impl{quotations: quotations}
}

func (c *impl) ImpactForClient(ctx context.Context, tenantID, clientID int64) (QuotationImpact, error) {
	count, numbers, err := c.quotations.ImpactForClient(ctx, tenantID, clientID)
	if err != nil {
		return QuotationImpact{}, err
	}
	return QuotationImpact{Count: count, Numbers: numbers}, nil
}

func (c *impl) DeleteForClient(ctx context.Context, tenantID, clientID int64) error {
	return c.quotations.DeleteForClient(ctx, tenantID, clientID)
}

func (c *impl) DeleteQuotation(ctx context.Context, tenantID, quotationID int64) error {
	return c.quotations.DeleteQuotation(ctx, tenantID, quotationID)
}

func (c *impl) PONumberForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error) {
	return c.quotations.PONumberForQuotation(ctx, tenantID, quotationID)
}

func (c *impl) CompositionForQuotation(ctx context.Context, tenantID, quotationID int64) ([][4]string, error) {
	return c.quotations.CompositionForQuotation(ctx, tenantID, quotationID)
}

func (c *impl) PackageNameForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error) {
	return c.quotations.PackageNameForQuotation(ctx, tenantID, quotationID)
}

func (c *impl) StatusForQuotation(ctx context.Context, tenantID, quotationID int64) (string, error) {
	return c.quotations.StatusForQuotation(ctx, tenantID, quotationID)
}
