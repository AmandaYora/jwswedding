package presentation

import (
	"os"
	"testing"

	"jwswedding/internal/modules/quotations/domain"
)

// TestDumpQuotationPDF writes sample quotation PDFs to disk for visual
// review. It asserts nothing and skips unless QUOTATION_DUMP names an output
// directory:
//
//	QUOTATION_DUMP=/tmp go test ./internal/modules/quotations/presentation/ -run TestDumpQuotationPDF
//
// Pindahan dari projects (visual review yang sama untuk dokumen yang sama).
func TestDumpQuotationPDF(t *testing.T) {
	if os.Getenv("QUOTATION_DUMP") == "" {
		t.Skip("set QUOTATION_DUMP=1 untuk menulis berkas contoh")
	}
	for _, c := range []struct {
		name   string
		status domain.QuotationStatus
	}{{"penawaran-diterima", domain.QuotationAccepted}, {"penawaran-draft", domain.QuotationDraft}, {"penawaran-ditawarkan", domain.QuotationOffered}} {
		pdf, err := buildQuotationPDF(testQuotationPrintData(c.status), testQuotationEventDate(), testQuotationProfile(), validQuotationPNGBytes(t), validQuotationPNGBytes(t), "Anisa Putri", "Lead Planner")
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(os.Getenv("QUOTATION_DUMP") + "/" + c.name + ".pdf")
		if err != nil {
			t.Fatal(err)
		}
		if err := pdf.Output(f); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
