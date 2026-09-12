package presentation

import (
	"os"
	"testing"

	"jwswedding/internal/modules/projects/domain"
)

// TestDumpPackageOrderPDF writes sample PO PDFs to disk for visual review. It
// asserts nothing and skips unless PO_DUMP names an output directory:
//
//	PO_DUMP=/tmp go test ./internal/modules/projects/presentation/ -run TestDumpPackageOrderPDF
//
// Kept deliberately. PLAN redesain-pdf-invoice-kwitansi-v2 settled its own
// millimetre constants by rendering real pages and looking at them, not on
// paper -- the orphaned "SYARAT & KETENTUAN" heading was found exactly this
// way, by a test suite that was already fully green. Rendered output lives in
// docs/plan/po-paket-client/mockup/.
func TestDumpPackageOrderPDF(t *testing.T) {
	if os.Getenv("PO_DUMP") == "" {
		t.Skip("set PO_DUMP=1 untuk menulis berkas contoh")
	}
	for _, c := range []struct {
		name   string
		status domain.PackageOrderStatus
	}{{"po-terbit", domain.PackageOrderIssued}, {"po-draft", domain.PackageOrderDraft}} {
		pdf, err := buildPackageOrderPDF(testPrintData(c.status), testProject(), testProfile(), validPNGBytes(t), validPNGBytes(t))
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(os.Getenv("PO_DUMP") + "/" + c.name + ".pdf")
		if err != nil {
			t.Fatal(err)
		}
		if err := pdf.Output(f); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
