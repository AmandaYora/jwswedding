package presentation

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-pdf/fpdf"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
	projectscontracts "jwswedding/internal/modules/projects/contracts"
	"jwswedding/internal/modules/quotations/application"
	"jwswedding/internal/modules/quotations/domain"
)

// Port tes renderer dari projects/presentation/package_order_pdf_test.go —
// dokumen yang sama ("PURCHASE ORDER"), sumber data Quotation.

func testQuotationBlocks() []domain.QuotationBlock {
	return []domain.QuotationBlock{
		{
			ID: 1, Category: "CATERING", SortOrder: 1,
			Body:      "BUFFET\nNasi Putih\nAneka Nasi Goreng\nOlahan Ayam/Ikan\nDESSERT\nAneka Buah\nMINUMAN\nAir mineral",
			QtyText:   "700 PORSI",
			BonusNote: "BONUS :\nMakanan After Akad 100 PORSI",
		},
		{
			ID: 2, Category: "CATERING", SortOrder: 2,
			Body:      "STALL / GUBUKAN\nPilihan Nusantara 1\nPilihan Nusantara 2\nPilihan Asia\nPilihan Western",
			QtyText:   "150 PORSI\n150 PORSI\n150 PORSI\n150 PORSI",
			BonusNote: "BONUS :\nIce Cream 1 Galon\nSomay 100 Porsi",
		},
		{
			ID: 3, Category: "DEKORASI", SortOrder: 3,
			Body:      "Pelaminan Nasional\nTaman Pelaminan\n1 Set Meja & Kursi Akad",
			QtyText:   "10-12 METER",
			BonusNote: "BONUS :\n- Backdrop Photobooth\n- Mini Gallery 4 PCS\n- Pohon Cherry Blossom\n- Tirai Backdrop hitam\n- Lantai Kaca\n- Melaminto\n- Backdrop Pen. Tamu",
		},
	}
}

func testQuotationAdjustments() []domain.QuotationAdjustment {
	return []domain.QuotationAdjustment{
		{ID: 1, Description: "Takeout Busana akad Resepsi 1jt, busana ortu 500k", Amount: -1_500_000, SortOrder: 1},
		{ID: 2, Description: "Add 100 buffet x 99k", Amount: 9_900_000, SortOrder: 2},
	}
}

func testQuotationEventDate() time.Time {
	return time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
}

func testQuotationPrintData(status domain.QuotationStatus) quotationPrintData {
	return quotationPrintData{
		PONumber: "PO/202609/0001", Revision: 0, Status: status,
		BasePrice: 213_500_000, TermsText: "A. TAHAP PEMBAYARAN\na. DP untuk keep harga dan promo.",
		BonusNote: "BONUS TAMBAHAN :\n- 8 Box Hias Seserahan",
		Blocks:    testQuotationBlocks(), Adjustments: testQuotationAdjustments(),
		Event: domain.QuotationEventSnapshot{
			ClientName: "Rara & Dafa", Phone: "08170043310",
			EventDate: "2026-09-19", Venue: "KLINK TOWER", Pax: 800,
		},
		IssuedAt:  time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC),
		Payments:  []projectscontracts.ClientPaymentInfo{testLedgerPayment()},
		TotalPaid: 5_000_000,
	}
}

func testLedgerPayment() projectscontracts.ClientPaymentInfo {
	return projectscontracts.ClientPaymentInfo{
		Type: "DP", Amount: 5_000_000,
		PaymentDate: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		Method:      "Transfer Bank",
	}
}

func testQuotationProfile() platformcontracts.TenantProfile {
	return platformcontracts.TenantProfile{
		BusinessName: "JWS Wedding", OwnerName: "Dimas Prasetio", Phone: "0812-0000-0000",
		Email: "halo@jws.id", Address: "Jl. Melati No. 12, Bandung", City: "Bandung",
		BankName: "BCA", BankAccountNumber: "1234567890", BankAccountHolderName: "Dimas Prasetio",
		AccentRGB: [3]int{150, 105, 46}, AccentDarkRGB: [3]int{107, 74, 31}, AccentSoftRGB: [3]int{244, 228, 204},
	}
}

func validQuotationPNGBytes(t *testing.T) []byte {
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

func assertValidQuotationPDF(t *testing.T, pdf *fpdf.Fpdf) {
	t.Helper()
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("pdf.Output: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("keluaran PDF kosong")
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatal("keluaran bukan PDF")
	}
}

func TestBuildQuotationPDF_LengkapDenganLogoDanTTD(t *testing.T) {
	pdf, err := buildQuotationPDF(testQuotationPrintData(domain.QuotationAccepted), testQuotationEventDate(), testQuotationProfile(), validQuotationPNGBytes(t), validQuotationPNGBytes(t), "Anisa Putri", "Lead Planner")
	if err != nil {
		t.Fatalf("buildQuotationPDF: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true setelah build: %v", pdf.Error())
	}
	assertValidQuotationPDF(t, pdf)
}

func TestBuildQuotationPDF_TanpaLogoTanpaTTD(t *testing.T) {
	pdf, err := buildQuotationPDF(testQuotationPrintData(domain.QuotationDraft), testQuotationEventDate(), testQuotationProfile(), nil, nil, "Anisa Putri", "Lead Planner")
	if err != nil {
		t.Fatalf("buildQuotationPDF tanpa aset: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidQuotationPDF(t, pdf)
}

func TestBuildQuotationPDF_AsetRusakTidakMenggagalkanDokumen(t *testing.T) {
	broken := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x00, 0x00}
	pdf, err := buildQuotationPDF(testQuotationPrintData(domain.QuotationAccepted), testQuotationEventDate(), testQuotationProfile(), broken, broken, "Anisa Putri", "Lead Planner")
	if err != nil {
		t.Fatalf("buildQuotationPDF dengan aset rusak: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true — aset rusak harus di-ClearError, bukan menggagalkan dokumen: %v", pdf.Error())
	}
	assertValidQuotationPDF(t, pdf)
}

func TestBuildQuotationPDF_SeluruhStatus(t *testing.T) {
	for _, status := range []domain.QuotationStatus{
		domain.QuotationDraft, domain.QuotationOffered, domain.QuotationAccepted,
		domain.QuotationRejected, domain.QuotationExpired, domain.QuotationCancelled,
	} {
		pdf, err := buildQuotationPDF(testQuotationPrintData(status), testQuotationEventDate(), testQuotationProfile(), nil, nil, "Anisa Putri", "Lead Planner")
		if err != nil {
			t.Fatalf("status %v: %v", status, err)
		}
		if pdf.Err() {
			t.Fatalf("status %v: pdf.Err() true: %v", status, pdf.Error())
		}
		assertValidQuotationPDF(t, pdf)
	}
}

func TestBuildQuotationPDF_BlokLebihTinggiDariSatuHalaman(t *testing.T) {
	lines := make([]string, 120)
	for i := range lines {
		lines[i] = "Item komposisi baris ke-" + strconv.Itoa(i+1)
	}
	data := testQuotationPrintData(domain.QuotationAccepted)
	data.Blocks = []domain.QuotationBlock{{
		ID: 1, Category: "CATERING", SortOrder: 1,
		Body: strings.Join(lines, "\n"), QtyText: "700 PORSI", BonusNote: "BONUS :\nMakanan After Akad",
	}}

	pdf, err := buildQuotationPDF(data, testQuotationEventDate(), testQuotationProfile(), nil, nil, "Anisa Putri", "Lead Planner")
	if err != nil {
		t.Fatalf("buildQuotationPDF blok panjang: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	if pages := pdf.PageCount(); pages < 2 {
		t.Errorf("blok 120 baris muat dalam %d halaman — berarti isinya terpotong, bukan dipecah", pages)
	}
	assertValidQuotationPDF(t, pdf)
}

func TestBuildQuotationPDF_BanyakBlok(t *testing.T) {
	var blocks []domain.QuotationBlock
	for i := 0; i < 30; i++ {
		blocks = append(blocks, domain.QuotationBlock{
			ID: int64(i + 1), Category: "KATEGORI " + strconv.Itoa(i+1), SortOrder: i + 1,
			Body: "Baris satu\nBaris dua\nBaris tiga", QtyText: "10 PCS", BonusNote: "BONUS :\nSesuatu",
		})
	}
	data := testQuotationPrintData(domain.QuotationAccepted)
	data.Blocks = blocks

	pdf, err := buildQuotationPDF(data, testQuotationEventDate(), testQuotationProfile(), nil, nil, "Anisa Putri", "Lead Planner")
	if err != nil {
		t.Fatalf("buildQuotationPDF banyak blok: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidQuotationPDF(t, pdf)
}

func TestBuildQuotationPDF_KosongTetapValid(t *testing.T) {
	data := quotationPrintData{
		Status: domain.QuotationDraft, IssuedAt: time.Now(),
		Event: domain.QuotationEventSnapshot{ClientName: "Rara & Dafa"},
	}
	pdf, err := buildQuotationPDF(data, time.Time{}, testQuotationProfile(), nil, nil, "Anisa Putri", "Lead Planner")
	if err != nil {
		t.Fatalf("buildQuotationPDF kosong: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidQuotationPDF(t, pdf)
}

func TestBuildQuotationPDF_TeksSangatPanjangTidakMenggagalkan(t *testing.T) {
	data := testQuotationPrintData(domain.QuotationAccepted)
	data.TermsText = strings.Repeat("Ketentuan yang sangat panjang sekali. ", 200)
	data.BonusNote = strings.Repeat("Bonus tambahan berlimpah. ", 200)
	data.Adjustments = append(data.Adjustments, domain.QuotationAdjustment{
		ID: 99, Description: strings.Repeat("Deskripsi penyesuaian amat panjang ", 20), Amount: -250_000, SortOrder: 3,
	})
	pdf, err := buildQuotationPDF(data, testQuotationEventDate(), testQuotationProfile(), nil, nil, "Anisa Putri", "Lead Planner")
	if err != nil {
		t.Fatalf("buildQuotationPDF teks panjang: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidQuotationPDF(t, pdf)
}

func TestQuotationIsHeadingLine(t *testing.T) {
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
		if got := isHeadingLine(c.line); got != c.want {
			t.Errorf("isHeadingLine(%q) = %v, mau %v", c.line, got, c.want)
		}
	}
}

// Draft dirender dari tabel live; selain itu dari snapshot beku — cabang
// paling konsekuensial di renderer.
func TestBuildQuotationPrintData_DraftPakaiDataLive(t *testing.T) {
	eventDate := testQuotationEventDate()
	view := &application.QuotationView{
		Quotation: &domain.Quotation{
			TenantID: 1, ClientID: 7, Status: domain.QuotationDraft,
			BasePrice: 213_500_000, TermsText: "S&K live",
			EventDate: &eventDate,
		},
		Blocks:      testQuotationBlocks(),
		Adjustments: testQuotationAdjustments(),
	}
	data := buildQuotationPrintData(view, eventDate, "Rara & Dafa", "08170043310", "KLINK TOWER", nil, 0)

	if data.TermsText != "S&K live" {
		t.Errorf("TermsText = %q, mau data live", data.TermsText)
	}
	if len(data.Blocks) != len(testQuotationBlocks()) {
		t.Errorf("Blocks = %d, mau %d dari tabel live", len(data.Blocks), len(testQuotationBlocks()))
	}
	if data.Event.ClientName != "Rara & Dafa" || data.Event.Venue != "KLINK TOWER" {
		t.Errorf("Event tidak diisi dari data live: %+v", data.Event)
	}
}

func TestBuildQuotationPrintData_TerkirimPakaiSnapshot(t *testing.T) {
	issuedAt := time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC)
	snapshotBlocks := []domain.QuotationBlock{{Category: "SNAPSHOT", Body: "Isi saat diteken"}}
	view := &application.QuotationView{
		Quotation: &domain.Quotation{
			TenantID: 1, ClientID: 7, Status: domain.QuotationOffered, IssuedAt: &issuedAt,
			BasePrice: 999_000_000, TermsText: "S&K yang sudah berubah",
			Snapshot: &domain.QuotationSnapshot{
				Current: domain.QuotationRevision{
					Revision: 0, IssuedAt: issuedAt, BasePrice: 213_500_000,
					TermsText: "S&K saat diteken", Blocks: snapshotBlocks,
					Event: domain.QuotationEventSnapshot{ClientName: "Rara & Dafa", Venue: "KLINK TOWER"},
				},
			},
		},
		Blocks: testQuotationBlocks(),
	}
	data := buildQuotationPrintData(view, testQuotationEventDate(), "Nama Baru", "0800", "Venue Baru", nil, 0)

	if data.BasePrice != 213_500_000 {
		t.Errorf("BasePrice = %d, mau 213500000 dari snapshot — dokumen yang sudah dikirim tidak boleh ikut berubah", data.BasePrice)
	}
	if data.TermsText != "S&K saat diteken" {
		t.Errorf("TermsText = %q, mau isi snapshot", data.TermsText)
	}
	if len(data.Blocks) != 1 || data.Blocks[0].Category != "SNAPSHOT" {
		t.Errorf("Blocks diambil dari tabel live, seharusnya dari snapshot: %+v", data.Blocks)
	}
	if data.Event.Venue != "KLINK TOWER" {
		t.Errorf("Event.Venue = %q, mau dari snapshot", data.Event.Venue)
	}
}

// Ledger pembayaran SELALU live, bahkan untuk dokumen beku.
func TestBuildQuotationPrintData_LedgerSelaluLive(t *testing.T) {
	issuedAt := time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC)
	view := &application.QuotationView{
		Quotation: &domain.Quotation{
			TenantID: 1, ClientID: 7, Status: domain.QuotationAccepted, IssuedAt: &issuedAt,
			Snapshot: &domain.QuotationSnapshot{Current: domain.QuotationRevision{BasePrice: 213_500_000}},
		},
	}
	payments := []projectscontracts.ClientPaymentInfo{testLedgerPayment(), testLedgerPayment()}
	data := buildQuotationPrintData(view, testQuotationEventDate(), "Rara & Dafa", "0800", "", payments, 10_000_000)

	if len(data.Payments) != 2 || data.TotalPaid != 10_000_000 {
		t.Errorf("ledger tidak live: %d pembayaran, totalPaid %d", len(data.Payments), data.TotalPaid)
	}
}

func TestQuotationPrintData_SubtitleMenandaiRevisi(t *testing.T) {
	if got := (quotationPrintData{Revision: 0}).subtitle(); strings.Contains(got, "Revisi") {
		t.Errorf("revisi 0 tidak boleh berlabel revisi, got %q", got)
	}
	if got := (quotationPrintData{Revision: 2}).subtitle(); !strings.Contains(got, "Revisi 2") {
		t.Errorf("subtitle revisi 2 = %q, mau memuat Revisi 2", got)
	}
}

func TestQuotationFormatRupiahSigned(t *testing.T) {
	cases := map[int64]string{
		-1_500_000: "-Rp 1.500.000",
		9_900_000:  "Rp 9.900.000",
		0:          "Rp 0",
	}
	for amount, want := range cases {
		if got := formatRupiahSigned(amount); got != want {
			t.Errorf("formatRupiahSigned(%d) = %q, mau %q", amount, got, want)
		}
	}
}

func TestQuotationCategorySpans(t *testing.T) {
	blocks := func(categories ...string) []domain.QuotationBlock {
		out := make([]domain.QuotationBlock, 0, len(categories))
		for _, c := range categories {
			out = append(out, domain.QuotationBlock{Category: c})
		}
		return out
	}
	cases := []struct {
		name string
		in   []domain.QuotationBlock
		want [][2]int
	}{
		{"dokumen sumber", blocks("CATERING", "CATERING", "DEKORASI"), [][2]int{{0, 1}, {2, 2}}},
		{"semua berbeda", blocks("A", "B", "C"), [][2]int{{0, 0}, {1, 1}, {2, 2}}},
		{"semua sama", blocks("A", "A", "A"), [][2]int{{0, 2}}},
		{"satu blok", blocks("A"), [][2]int{{0, 0}}},
		{"kosong", nil, nil},
		{"sama tapi terpisah", blocks("A", "B", "A"), [][2]int{{0, 0}, {1, 1}, {2, 2}}},
	}
	for _, c := range cases {
		got := categorySpans(c.in)
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
