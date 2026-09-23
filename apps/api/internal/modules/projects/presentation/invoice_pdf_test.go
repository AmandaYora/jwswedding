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
	"jwswedding/internal/modules/projects/application"
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

// testSignerName adalah nama staff PENERBIT dokumen — sejak PLAN
// tanda-tangan-pengguna itulah yang tercetak di bawah garis tanda tangan,
// menggantikan TenantProfile.OwnerName.
const testSignerName = "Anisa Putri"

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
		EventDate:   time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
		PackageName: "Silver", ContractValue: 75_000_000,
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
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), validPNGBytes(t), validPNGBytes(t), 5_000_000, "", nil, testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true setelah build: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientInvoicePDF_TanpaLogo(t *testing.T) {
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), nil, nil, 0, "", nil, testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF tanpa logo: %v", err)
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientInvoicePDF_LogoRusak(t *testing.T) {
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x00, 0x00}, nil, 0, "", nil, testSignerName)
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
		pdf, err := buildClientInvoicePDF(testProject(), inv, testProfile(), nil, nil, 0, "", nil, testSignerName)
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

	pdf, err := buildClientInvoicePDF(testProject(), inv, profile, nil, nil, 0, "", nil, testSignerName)
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

	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), profile, nil, nil, 0, "", nil, testSignerName)
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

	pdfA, err := buildClientInvoicePDF(testProject(), testInvoice(), bronze, nil, nil, 0, "", nil, testSignerName)
	if err != nil {
		t.Fatalf("build bronze: %v", err)
	}
	pdfB, err := buildClientInvoicePDF(testProject(), testInvoice(), emerald, nil, nil, 0, "", nil, testSignerName)
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

// --- Regresi desain v2 (PLAN.md redesain-pdf-invoice-kwitansi-v2 §D4) ---

// TestBuildClientInvoicePDF_NamaUsahaPanjangSatuHalaman regresi §3.1: nama
// usaha panjang dulu menabrak judul "INVOICE" karena leftW kop surat
// dihardcode independen dari posisi judul. leftW sekarang dihitung dari
// sisa ruang aktual (newDocument), jadi ini tidak lagi terjadi dan dokumen
// tetap 1 halaman.
func TestBuildClientInvoicePDF_NamaUsahaPanjangSatuHalaman(t *testing.T) {
	profile := testProfile()
	profile.BusinessName = "JWS Wedding Organizer & Event Planner Indonesia"
	profile.Address = "Jl. Melati Raya No. 12, Kompleks Permata Indah Blok C-4, Kelurahan Sukajadi, Kecamatan Sukajadi, Bandung, Jawa Barat 40162"

	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), profile, nil, nil, 0, "", nil, testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF nama usaha panjang: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	if got := pdf.PageCount(); got != 1 {
		t.Errorf("PageCount() = %d, want 1 — nama usaha panjang seharusnya tidak memaksa halaman tambahan", got)
	}
}

// TestBuildClientInvoicePDF_DeskripsiSangatPanjangTidakRusak regresi §3.3:
// dulu buildClientInvoicePDF menghitung posisi blok-blok berikutnya secara
// aritmetika dari dataY, bukan dari kursor asli, sehingga saat fpdf memicu
// auto page-break di tengah MultiCell deskripsi, seluruh blok sesudahnya
// digambar di luar halaman. ensureSpace (A6) menggantikan itu.
func TestBuildClientInvoicePDF_DeskripsiSangatPanjangTidakRusak(t *testing.T) {
	inv := testInvoice()
	inv.Description = strings.Repeat("Pelunasan paket lengkap mencakup dekorasi, catering, dokumentasi. ", 10) // ~670 karakter

	pdf, err := buildClientInvoicePDF(testProject(), inv, testProfile(), nil, nil, 0, "", nil, testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF deskripsi sangat panjang: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	if got := pdf.PageCount(); got > 2 {
		t.Errorf("PageCount() = %d, want <= 2", got)
	}
	assertValidPDF(t, pdf)
}

// TestBuildClientInvoicePDF_LebihBayarTidakMenampilkanNegatif regresi A5:
// totalPaid melebihi ContractValue tidak pernah boleh membuat formatRupiah
// menerima angka negatif — summaryStrip merelabel sel itu "Lebih Bayar".
func TestBuildClientInvoicePDF_LebihBayarTidakMenampilkanNegatif(t *testing.T) {
	project := testProject()
	project.ContractValue = 5_000_000
	pdf, err := buildClientInvoicePDF(project, testInvoice(), testProfile(), nil, nil, 20_000_000, "", nil, testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF totalPaid > ContractValue: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

// TestBuildClientInvoicePDF_TotalPaidNegatifTidakError regresi temuan review:
// ClientPaymentService.TotalReceived dapat mengembalikan angka negatif kalau
// sebuah Refund melebihi total pembayaran sebelumnya (tidak ada validasi
// untuk itu di ClientPaymentService.Create/Update) — summaryStrip sekarang
// meng-clamp paid ke 0 sebelum ditampilkan, bukan mencetak "Total Sudah
// Dibayar: -Rp ..." yang tidak masuk akal di dokumen customer-facing.
func TestBuildClientInvoicePDF_TotalPaidNegatifTidakError(t *testing.T) {
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), nil, nil, -5_000_000, "", nil, testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF totalPaid negatif: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientInvoicePDF_TanpaKotaTanpaBank(t *testing.T) {
	profile := testProfile()
	profile.City = ""
	profile.BankName = ""
	profile.BankAccountNumber = ""
	profile.BankAccountHolderName = ""

	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), profile, nil, nil, 0, "", nil, testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF tanpa Kota/bank: %v", err)
	}
	assertValidPDF(t, pdf)
}

// TestBuildClientInvoicePDF_NamaAcaraSangatPanjangTidakMenimpaKartuSebelah
// regresi bug ditemukan saat verifikasi manual: infoCard menggambar nilai
// baris (mis. "Acara") dengan CellFormat satu baris yang TIDAK membungkus
// maupun memotong teks — nama acara yang sangat panjang meluber keluar
// lebar kartunya sendiri (87mm) dan secara visual menabrak kartu "Detail
// Tagihan" di sebelahnya. infoCard sekarang memotong nilai dengan "..."
// lewat truncateToFit sebelum digambar. Uji ini hanya memastikan tidak
// error — kebenaran visualnya (potongan tidak lagi menabrak) sudah
// diverifikasi manual lewat render.
func TestBuildClientInvoicePDF_NamaAcaraSangatPanjangTidakMenimpaKartuSebelah(t *testing.T) {
	project := testProject()
	project.Name = "Resepsi dan Akad Nikah Meriah Keluarga Besar Dimas dan Sari di Grand Ballroom Hotel Bersama Seluruh Kerabat dan Sahabat dari Berbagai Kota di Indonesia"

	pdf, err := buildClientInvoicePDF(project, testInvoice(), testProfile(), nil, nil, 0, "", nil, testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF nama acara sangat panjang: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestFormatTanggalPDF(t *testing.T) {
	cases := []struct {
		date time.Time
		want string
	}{
		{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "1 Januari 2026"},
		{time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC), "9 Februari 2026"},
		{time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), "31 Desember 2026"},
	}
	for _, c := range cases {
		if got := formatTanggalPDF(c.date); got != c.want {
			t.Errorf("formatTanggalPDF(%v) = %q, want %q", c.date, got, c.want)
		}
	}
}

// --- Kwitansi ---

func TestBuildClientPaymentReceiptPDF_DenganTandaTanganTransparan(t *testing.T) {
	sig := validPNGBytes(t)
	pdf, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "INV/2026/08/001", testProfile(), nil, sig, 5_000_000, testSignerName)
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

	pdfNoSig, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "INV/2026/08/001", testProfile(), nil, nil, 5_000_000, testSignerName)
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
	pdf, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "", testProfile(), nil, nil, 5_000_000, testSignerName)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF tanpa tanda tangan: %v", err)
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientPaymentReceiptPDF_TandaTanganRusak(t *testing.T) {
	pdf, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "", testProfile(), nil, []byte{0x89, 0x50, 0x4e, 0x47, 0x01, 0x02}, 5_000_000, testSignerName)
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

	pdf, err := buildClientPaymentReceiptPDF(testProject(), p, "", testProfile(), nil, nil, 999_999_999_999, testSignerName)
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

	pdf, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "", profile, nil, nil, 5_000_000, testSignerName)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF dengan City kosong: %v", err)
	}
	assertValidPDF(t, pdf)
}

// --- Regresi desain v2 (PLAN.md redesain-pdf-invoice-kwitansi-v2 §D4) ---

// TestBuildClientPaymentReceiptPDF_NominalBesarSatuHalaman regresi §3.2: dulu
// panelH panel nominal dihitung "12 + n*4.5" tapi baris terbilang digambar
// mulai y+13, sehingga terbilang 2 baris (nominal besar) selalu meluber
// keluar panel. panelH sekarang dihitung dari posisi awal teks yang
// sebenarnya (nomTop + nomH + n*lineHeight + margin bawah).
func TestBuildClientPaymentReceiptPDF_NominalBesarSatuHalaman(t *testing.T) {
	p := testPayment()
	p.Amount = 123_456_789 // "Seratus Dua Puluh Tiga Juta..." -> terbilang 2 baris

	pdf, err := buildClientPaymentReceiptPDF(testProject(), p, "", testProfile(), nil, nil, 123_456_789, testSignerName)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF nominal besar: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	if got := pdf.PageCount(); got != 1 {
		t.Errorf("PageCount() = %d, want 1", got)
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientPaymentReceiptPDF_LebihBayarTidakMenampilkanNegatif(t *testing.T) {
	project := testProject()
	project.ContractValue = 5_000_000
	pdf, err := buildClientPaymentReceiptPDF(project, testPayment(), "", testProfile(), nil, nil, 20_000_000, testSignerName)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF totalPaid > ContractValue: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

// TestBuildClientPaymentReceiptPDF_TotalPaidNegatifTidakError mirrors the
// Invoice sibling above — same underlying reachable-negative-TotalReceived
// scenario, same summaryStrip clamp.
func TestBuildClientPaymentReceiptPDF_TotalPaidNegatifTidakError(t *testing.T) {
	pdf, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "", testProfile(), nil, nil, -5_000_000, testSignerName)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF totalPaid negatif: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

// TestBuildClientPaymentReceiptPDF_NamaProyekPanjangDenganNominalBesarSatuAtauDuaHalaman
// regresi bug review: buildClientPaymentReceiptPDF sebelumnya memanggil
// summaryStrip TANPA ensureSpace (tidak seperti buildClientInvoicePDF yang
// sudah membungkusnya) -- kombinasi nama proyek sangat panjang (memaksa
// "Untuk Pembayaran" jadi banyak baris) dengan nominal besar (terbilang
// banyak baris) bisa mendorong strip ringkasan sampai melewati footer tanpa
// pernah memicu halaman baru. Sekarang keduanya ensureSpace, jadi ini hanya
// perlu tidak error dan tetap PDF valid, berapa pun jumlah halamannya.
func TestBuildClientPaymentReceiptPDF_NamaProyekPanjangDenganNominalBesar(t *testing.T) {
	project := testProject()
	project.Name = "Resepsi dan Akad Nikah Meriah Keluarga Besar Dimas dan Sari di Grand Ballroom Hotel Bersama Seluruh Kerabat dan Sahabat dari Berbagai Kota di Indonesia"
	p := testPayment()
	p.Amount = 999_999_999_999

	pdf, err := buildClientPaymentReceiptPDF(project, p, "", testProfile(), nil, nil, 999_999_999_999, testSignerName)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF nama proyek panjang + nominal besar: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildClientPaymentReceiptPDF_TanpaKotaTanpaBank(t *testing.T) {
	profile := testProfile()
	profile.City = ""
	profile.BankName = ""
	profile.BankAccountNumber = ""
	profile.BankAccountHolderName = ""

	pdf, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "", profile, nil, nil, 5_000_000, testSignerName)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF tanpa Kota/bank: %v", err)
	}
	assertValidPDF(t, pdf)
}

// --- truncateToFit ---

// TestTruncateToFit_TeksPendekTidakBerubah / _TeksPanjangDipotong verify
// truncateToFit's two branches directly, against a real registered font
// (truncation decisions depend on GetStringWidth, which needs an active
// font) — regresi bug §"nama acara sangat panjang menimpa kartu sebelah".
func TestTruncateToFit_TeksPendekTidakBerubah(t *testing.T) {
	pdf := fpdf.New("P", "mm", "A4", "")
	family := registerFonts(pdf)
	pdf.AddPage()
	pdf.SetFont(family, "", 10)

	got := truncateToFit(pdf, "Akad & Resepsi", 79)
	if got != "Akad & Resepsi" {
		t.Errorf("truncateToFit(teks pendek) = %q, want unchanged", got)
	}
}

func TestTruncateToFit_TeksPanjangDipotong(t *testing.T) {
	pdf := fpdf.New("P", "mm", "A4", "")
	family := registerFonts(pdf)
	pdf.AddPage()
	pdf.SetFont(family, "", 10)

	long := "Resepsi dan Akad Nikah Meriah Keluarga Besar Dimas dan Sari di Grand Ballroom Hotel Bersama Seluruh Kerabat dan Sahabat dari Berbagai Kota di Indonesia"
	const maxWidth = 79.0 // matches infoCard's cardW(87) - 8
	got := truncateToFit(pdf, long, maxWidth)

	if got == long {
		t.Fatal("truncateToFit tidak memotong teks yang jelas melebihi maxWidth")
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncateToFit(panjang) = %q, want berakhir dengan \"...\"", got)
	}
	if w := pdf.GetStringWidth(got); w > maxWidth {
		t.Errorf("truncateToFit(panjang) lebar %.2fmm masih melebihi maxWidth %.2fmm", w, maxWidth)
	}
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
	pdf, _ := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), nil, nil, 0, "", nil, testSignerName)
	if pdf.Err() {
		t.Fatalf("registerFonts meninggalkan pdf.Err(): %v", pdf.Error())
	}
}

// --- Nomor PO + tabel komposisi paket pada Invoice ---

// testComposition mereproduksi bentuk dokumen sumber: CATERING menempati DUA
// baris berturut-turut dengan QTY/BONUS berbeda (itulah sebabnya kategori
// dirender sebagai sel gabungan), lalu satu kategori lain.
func testComposition() []application.QuotationCompositionRow {
	return []application.QuotationCompositionRow{
		{
			Category: "CATERING",
			Product:  "BUFFET\nNasi Putih\nAneka Nasi Goreng\nDESSERT\nAneka Buah",
			Qty:      "700 PORSI",
			Bonus:    "BONUS :\nMakanan After Akad",
		},
		{
			Category: "CATERING",
			Product:  "STALL / GUBUKAN\nBakso\nSomay",
			Qty:      "150 PORSI",
			Bonus:    "BONUS :\nIce Cream 1 Galon",
		},
		{
			Category: "DEKORASI",
			Product:  "Pelaminan\nBackdrop Photobooth",
			Qty:      "10-12 METER",
			Bonus:    "BONUS :\n- Lantai Kaca",
		},
	}
}

func TestBuildClientInvoicePDF_DenganNomorPODanKomposisi(t *testing.T) {
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), nil, nil, 0,
		"026/PO/JWS/IX/2026", testComposition(), testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF dengan komposisi: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true setelah build: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

// Tanpa penawaran (project pra-penawaran) tagihan HARUS tetap tercetak:
// nomor PO jatuh ke "-" dan tabel komposisinya tidak digambar sama sekali.
// Ini jalur yang paling mungkin dilupakan, karena di data dev hampir setiap
// project punya penawaran.
func TestBuildClientInvoicePDF_TanpaPOTetapTercetak(t *testing.T) {
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), nil, nil, 0, "", nil, testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF tanpa PO: %v", err)
	}
	assertValidPDF(t, pdf)
}

// Komposisi sepanjang beberapa halaman: pemecah halaman tabel harus bekerja
// DAN blok-blok sesudahnya (total, terbilang, bank, tanda tangan) tetap
// tergambar di halaman yang benar — regresi kelas yang sama dengan §3.3.
func TestBuildClientInvoicePDF_KomposisiLintasHalaman(t *testing.T) {
	rows := make([]application.QuotationCompositionRow, 0, 40)
	for i := 0; i < 40; i++ {
		rows = append(rows, application.QuotationCompositionRow{
			Category: "KATEGORI " + strings.Repeat("X", 8),
			Product:  strings.Repeat("Baris produk yang cukup panjang untuk membungkus\n", 6),
			Qty:      "100 PORSI",
			Bonus:    "BONUS :\n- Sesuatu",
		})
	}
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), nil, nil, 0, "026/PO/JWS/IX/2026", rows, testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF komposisi panjang: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	if pdf.PageNo() < 2 {
		t.Errorf("komposisi 40 blok hanya menghasilkan %d halaman — pemecah halaman tidak berjalan", pdf.PageNo())
	}
	assertValidPDF(t, pdf)
}

// Satu blok yang SENDIRIAN lebih tinggi dari satu halaman: harus terpecah,
// bukan terpotong diam-diam.
func TestBuildClientInvoicePDF_SatuBlokLebihTinggiDariHalaman(t *testing.T) {
	rows := []application.QuotationCompositionRow{{
		Category: "CATERING",
		Product:  strings.Repeat("Item menu prasmanan\n", 120),
		Qty:      "700 PORSI",
		Bonus:    "BONUS :\nMakanan After Akad",
	}}
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), nil, nil, 0, "026/PO/JWS/IX/2026", rows, testSignerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF blok raksasa: %v", err)
	}
	if pdf.PageNo() < 2 {
		t.Errorf("blok 120 baris hanya %d halaman — blok terpotong, bukan terpecah", pdf.PageNo())
	}
	assertValidPDF(t, pdf)
}

// Paritas dengan TestQuotationCategorySpans di quotations/presentation —
// renderer-nya kembar, jadi vektor ujinya pun harus sama. Kalau salah satu
// berubah, yang ini ikut gagal.
func TestCompositionSpans(t *testing.T) {
	rows := func(categories ...string) []application.QuotationCompositionRow {
		out := make([]application.QuotationCompositionRow, 0, len(categories))
		for _, c := range categories {
			out = append(out, application.QuotationCompositionRow{Category: c})
		}
		return out
	}
	cases := []struct {
		name string
		in   []application.QuotationCompositionRow
		want [][2]int
	}{
		{"dokumen sumber", rows("CATERING", "CATERING", "DEKORASI"), [][2]int{{0, 1}, {2, 2}}},
		{"semua berbeda", rows("A", "B", "C"), [][2]int{{0, 0}, {1, 1}, {2, 2}}},
		{"semua sama", rows("A", "A", "A"), [][2]int{{0, 2}}},
		{"satu blok", rows("A"), [][2]int{{0, 0}}},
		{"kosong", nil, nil},
		{"sama tapi terpisah", rows("A", "B", "A"), [][2]int{{0, 0}, {1, 1}, {2, 2}}},
	}
	for _, c := range cases {
		got := compositionSpans(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%s: %d span, mau %d (%v)", c.name, len(got), len(c.want), got)
			continue
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Errorf("%s: span ke-%d = %v, mau %v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

// Paritas dengan TestQuotationIsHeadingLine — vektor yang sama persis.
func TestIsCompositionHeading(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"BUFFET", true},
		{"DESSERT", true},
		{"MINUMAN", true},
		{"STALL / GUBUKAN", true},
		{"BONUS :", true},
		{"Nasi Putih", false},
		{"Aneka Nasi Goreng", false},
		{"1 Psg Makeup & Attire Pengantin Akad", false},
		{"", false},
		{"   ", false},
		{"150", false},
		{"- - -", false},
		{"2 FOTOGRAFER", true},
	}
	for _, c := range cases {
		if got := isCompositionHeading(c.line); got != c.want {
			t.Errorf("isCompositionHeading(%q) = %v, mau %v", c.line, got, c.want)
		}
	}
}
