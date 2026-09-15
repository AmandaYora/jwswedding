package presentation

// Tes TTD klien di PDF PO (§13 PLAN ttd-penawaran): kotak terisi bila ada
// salinan milik dokumen, kosong bila belum diteken, dan PDF tetap tercetak
// bila gambarnya tidak bisa dibaca.

import (
	"testing"

	"jwswedding/internal/modules/quotations/domain"
)

func TestQuotationPDF_DenganTTDKlien(t *testing.T) {
	data := testQuotationPrintData(domain.QuotationAccepted)
	data.ClientSignature = validQuotationPNGBytes(t)
	data.ClientSignerName = "Rara"

	pdf, err := buildQuotationPDF(data, testQuotationEventDate(), testQuotationProfile(), nil, validQuotationPNGBytes(t))
	if err != nil {
		t.Fatalf("buildQuotationPDF dengan TTD klien: %v", err)
	}
	assertValidQuotationPDF(t, pdf)
}

func TestQuotationPDF_TanpaTTDKlien(t *testing.T) {
	data := testQuotationPrintData(domain.QuotationOffered)

	pdf, err := buildQuotationPDF(data, testQuotationEventDate(), testQuotationProfile(), nil, nil)
	if err != nil {
		t.Fatalf("buildQuotationPDF tanpa TTD klien: %v", err)
	}
	assertValidQuotationPDF(t, pdf)
}

func TestQuotationPDF_GambarTTDKlienRusak(t *testing.T) {
	data := testQuotationPrintData(domain.QuotationAccepted)
	data.ClientSignature = []byte("bukan-gambar")
	data.ClientSignerName = "Rara"

	// Gambar gagal dibaca tidak boleh menggagalkan pencetakan — kotaknya
	// tercetak kosong.
	pdf, err := buildQuotationPDF(data, testQuotationEventDate(), testQuotationProfile(), nil, nil)
	if err != nil {
		t.Fatalf("buildQuotationPDF dengan gambar rusak: %v", err)
	}
	assertValidQuotationPDF(t, pdf)
}
