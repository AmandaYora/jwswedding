package presentation

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/go-pdf/fpdf"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
	"jwswedding/internal/modules/projects/domain"
)

// validPNGBytes builds a tiny real PNG with an alpha channel — good enough
// for fpdfImageType to sniff as "image/png" and for fpdf to actually decode
// and embed (unlike a hand-written byte literal, which risks being
// accidentally malformed).
func validPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 128})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func testProfile() platformcontracts.TenantProfile {
	return platformcontracts.TenantProfile{
		BusinessName: "JWS Wedding", OwnerName: "Dimas Prasetio", Phone: "0812-0000-0000",
		Email: "halo@jws.id", Address: "Jl. Melati No. 12, Bandung", City: "Bandung",
		BankName: "BCA", BankAccountNumber: "1234567890", BankAccountHolderName: "Dimas Prasetio",
		AccentRGB: [3]int{150, 105, 46}, AccentDarkRGB: [3]int{107, 74, 31}, AccentSoftRGB: [3]int{244, 228, 204},
	}
}

func testProject() domain.Project {
	return domain.Project{
		ID: 1, TenantID: 1, Name: "Akad & Resepsi Dimas-Sari",
		BrideName: "Sari", GroomName: "Dimas",
		EventDate: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
		PackageName: "Silver",
	}
}

func testInvoice() domain.ClientInvoice {
	return domain.ClientInvoice{
		ID: 1, ProjectID: 1, InvoiceNumber: "INV/2026/08/001",
		Type: domain.PaymentDP, Description: "Uang muka acara", Amount: 5_000_000,
		CreatedAt: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		DueDate:   time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
		Status:    domain.InvoiceSent,
	}
}

func testPayment() domain.ClientPayment {
	return domain.ClientPayment{
		ID: 1, ProjectID: 1, Type: domain.PaymentDP, Amount: 5_000_000,
		PaymentDate: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		Method:      "Transfer Bank", ReferenceNumber: "TRX-001",
		ReceiptNumber: "KWT/2026/08/001",
	}
}

func assertValidPDF(t *testing.T, pdf *fpdf.Fpdf) {
	t.Helper()
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("pdf.Output: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("keluaran PDF kosong")
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		n := 20
		if buf.Len() < n {
			n = buf.Len()
		}
		t.Fatalf("keluaran tidak diawali %%PDF- (bukan PDF valid), got: %q", buf.Bytes()[:n])
	}
}

// --- Invoice ---

func TestBuildClientInvoicePDF_LengkapDenganLogo(t *testing.T) {
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), validPNGBytes(t))
	if err != nil {
		t.Fatalf("buildClientInvoicePDF: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true setelah build: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientInvoicePDF_TanpaLogo(t *testing.T) {
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), nil)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF tanpa logo: %v", err)
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientInvoicePDF_LogoRusak(t *testing.T) {
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x00, 0x00})
	if err != nil {
		t.Fatalf("buildClientInvoicePDF dengan logo rusak: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true — logo rusak seharusnya di-ClearError, bukan menggagalkan dokumen: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientInvoicePDF_SeluruhStatus(t *testing.T) {
	for _, status := range []domain.InvoiceStatus{
		domain.InvoiceDraft, domain.InvoiceSent, domain.InvoicePaid, domain.InvoiceCancelled,
	} {
		inv := testInvoice()
		inv.Status = status
		pdf, err := buildClientInvoicePDF(testProject(), inv, testProfile(), nil)
		if err != nil {
			t.Fatalf("status %v: buildClientInvoicePDF: %v", status, err)
		}
		if pdf.Err() {
			t.Fatalf("status %v: pdf.Err() true: %v", status, pdf.Error())
		}
	}
}

func TestBuildClientInvoicePDF_DeskripsiDanAlamatPanjang(t *testing.T) {
	inv := testInvoice()
	inv.Description = strings.Repeat("Rincian pekerjaan dekorasi dan katering pernikahan. ", 20)
	profile := testProfile()
	profile.Address = strings.Repeat("Jalan yang sangat panjang sekali, ", 15) + "Bandung"

	pdf, err := buildClientInvoicePDF(testProject(), inv, profile, nil)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF dengan teks panjang: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true dengan teks panjang: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientInvoicePDF_TanpaDataBank(t *testing.T) {
	profile := testProfile()
	profile.BankName = ""
	profile.BankAccountNumber = ""
	profile.BankAccountHolderName = ""

	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), profile, nil)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF tanpa data bank: %v", err)
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientInvoicePDF_WarnaAksenBerbedaMenghasilkanKeluaranBerbeda(t *testing.T) {
	bronze := testProfile()
	emerald := testProfile()
	emerald.AccentRGB = [3]int{6, 78, 59}
	emerald.AccentDarkRGB = [3]int{2, 44, 34}
	emerald.AccentSoftRGB = [3]int{209, 250, 229}

	pdfA, err := buildClientInvoicePDF(testProject(), testInvoice(), bronze, nil)
	if err != nil {
		t.Fatalf("build bronze: %v", err)
	}
	pdfB, err := buildClientInvoicePDF(testProject(), testInvoice(), emerald, nil)
	if err != nil {
		t.Fatalf("build emerald: %v", err)
	}

	var bufA, bufB bytes.Buffer
	if err := pdfA.Output(&bufA); err != nil {
		t.Fatalf("output bronze: %v", err)
	}
	if err := pdfB.Output(&bufB); err != nil {
		t.Fatalf("output emerald: %v", err)
	}
	if bytes.Equal(bufA.Bytes(), bufB.Bytes()) {
		t.Fatal("dua profil dengan warna aksen berbeda menghasilkan PDF identik — warna aksen tampaknya di-hardcode, bukan diambil dari TenantProfile (D8)")
	}
}

// --- Kwitansi ---

func TestBuildClientPaymentReceiptPDF_DenganTandaTanganTransparan(t *testing.T) {
	sig := validPNGBytes(t)
	pdf, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "INV/2026/08/001", testProfile(), nil, sig)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF dengan tanda tangan: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	var withSig bytes.Buffer
	if err := pdf.Output(&withSig); err != nil {
		t.Fatalf("output: %v", err)
	}

	pdfNoSig, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "INV/2026/08/001", testProfile(), nil, nil)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF tanpa tanda tangan: %v", err)
	}
	var withoutSig bytes.Buffer
	if err := pdfNoSig.Output(&withoutSig); err != nil {
		t.Fatalf("output tanpa tanda tangan: %v", err)
	}

	if withSig.Len() <= withoutSig.Len() {
		t.Errorf("PDF dengan tanda tangan (%d bytes) seharusnya lebih besar dari tanpa tanda tangan (%d bytes) — gambar tampaknya tidak benar-benar tertanam", withSig.Len(), withoutSig.Len())
	}
}

func TestBuildClientPaymentReceiptPDF_TanpaTandaTangan(t *testing.T) {
	pdf, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "", testProfile(), nil, nil)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF tanpa tanda tangan: %v", err)
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientPaymentReceiptPDF_TandaTanganRusak(t *testing.T) {
	pdf, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "", testProfile(), nil, []byte{0x89, 0x50, 0x4e, 0x47, 0x01, 0x02})
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF dengan tanda tangan rusak: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true — tanda tangan rusak seharusnya di-ClearError: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientPaymentReceiptPDF_NominalBesarDanCatatanPanjang(t *testing.T) {
	p := testPayment()
	p.Amount = 999_999_999_999
	p.Notes = strings.Repeat("Catatan pembayaran tambahan yang cukup panjang. ", 10)

	pdf, err := buildClientPaymentReceiptPDF(testProject(), p, "", testProfile(), nil, nil)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF nominal besar: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true dengan nominal besar dan catatan panjang: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientPaymentReceiptPDF_CityKosong(t *testing.T) {
	profile := testProfile()
	profile.City = ""

	pdf, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "", profile, nil, nil)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF dengan City kosong: %v", err)
	}
	assertValidPDF(t, pdf)
}

// --- formatRupiah ---

func TestFormatRupiah(t *testing.T) {
	cases := []struct {
		amount int64
		want   string
	}{
		{0, "Rp 0"},
		{1000, "Rp 1.000"},
		{2_500_000, "Rp 2.500.000"},
		{-2_500_000, "-Rp 2.500.000"},
	}
	for _, c := range cases {
		if got := formatRupiah(c.amount); got != c.want {
			t.Errorf("formatRupiah(%d) = %q, want %q", c.amount, got, c.want)
		}
	}
}

// --- Font registration ---

func TestRegisterFonts_TidakMenyisakanError(t *testing.T) {
	pdf, _ := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), nil)
	if pdf.Err() {
		t.Fatalf("registerFonts meninggalkan pdf.Err(): %v", pdf.Error())
	}
}
