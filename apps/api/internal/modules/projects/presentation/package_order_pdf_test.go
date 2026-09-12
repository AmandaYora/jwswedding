package presentation

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
)

func poPercent(v float64) *float64 { return &v }
func poFixed(v int64) *int64       { return &v }

func testTermsPlan() []domain.TermPlanEntry {
	return []domain.TermPlanEntry{
		{Sequence: 1, Label: "Down Payment", Type: domain.PaymentDP, FixedAmount: poFixed(10_000_000), DaysBeforeEvent: 330},
		{Sequence: 2, Label: "Pembayaran 1", Type: domain.PaymentTermin, Percent: poPercent(30), DaysBeforeEvent: 210},
		{Sequence: 3, Label: "Pembayaran 2", Type: domain.PaymentTermin, Percent: poPercent(50), DaysBeforeEvent: 60},
		{Sequence: 4, Label: "Pelunasan", Type: domain.PaymentPelunasan, Percent: poPercent(100), DaysBeforeEvent: 30},
	}
}

// testPOBlocks mirrors the real shape of Invoice-PO.pdf: an ALL-CAPS
// sub-heading inside the body, a multi-line QTY cell, and a bonus cell taller
// than the body beside it.
func testPOBlocks() []domain.ProjectPackageBlock {
	return []domain.ProjectPackageBlock{
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
			Body:    "Pelaminan Nasional\nTaman Pelaminan\n1 Set Meja & Kursi Akad",
			QtyText: "10-12 METER",
			// Deliberately taller than the body beside it — row height must
			// follow the tallest column, not the first one.
			BonusNote: "BONUS :\n- Backdrop Photobooth\n- Mini Gallery 4 PCS\n- Pohon Cherry Blossom\n- Tirai Backdrop hitam\n- Lantai Kaca\n- Melaminto\n- Backdrop Pen. Tamu",
		},
	}
}

func testPOAdjustments() []domain.ProjectPackageAdjustment {
	return []domain.ProjectPackageAdjustment{
		{ID: 1, Description: "Takeout Busana akad Resepsi 1jt, busana ortu 500k", Amount: -1_500_000, SortOrder: 1},
		{ID: 2, Description: "Add 100 buffet x 99k", Amount: 9_900_000, SortOrder: 2},
	}
}

func testPrintData(status domain.PackageOrderStatus) packageOrderPrintData {
	return packageOrderPrintData{
		PONumber: "PO/202609/0001", Revision: 0, Status: status,
		BasePrice: 213_500_000, TermsText: "A. TAHAP PEMBAYARAN\na. DP untuk keep harga dan promo.",
		BonusNote: "BONUS TAMBAHAN :\n- 8 Box Hias Seserahan",
		Blocks:    testPOBlocks(), Adjustments: testPOAdjustments(), TermsPlan: testTermsPlan(),
		Event: domain.PackageOrderEventSnapshot{
			ClientName: "Rara & Dafa", Phone: "08170043310",
			EventDate: "2026-09-19", EventStart: "08.00", EventEnd: "13.00",
			Venue: "KLINK TOWER", Pax: 800,
		},
		IssuedAt:  time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC),
		Payments:  []domain.ClientPayment{testPayment()},
		TotalPaid: 5_000_000,
	}
}

func TestBuildPackageOrderPDF_LengkapDenganLogoDanTTD(t *testing.T) {
	pdf, err := buildPackageOrderPDF(testPrintData(domain.PackageOrderIssued), testProject(), testProfile(), validPNGBytes(t), validPNGBytes(t))
	if err != nil {
		t.Fatalf("buildPackageOrderPDF: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true setelah build: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildPackageOrderPDF_TanpaLogoTanpaTTD(t *testing.T) {
	pdf, err := buildPackageOrderPDF(testPrintData(domain.PackageOrderDraft), testProject(), testProfile(), nil, nil)
	if err != nil {
		t.Fatalf("buildPackageOrderPDF tanpa aset: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildPackageOrderPDF_AsetRusakTidakMenggagalkanDokumen(t *testing.T) {
	broken := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x00, 0x00}
	pdf, err := buildPackageOrderPDF(testPrintData(domain.PackageOrderIssued), testProject(), testProfile(), broken, broken)
	if err != nil {
		t.Fatalf("buildPackageOrderPDF dengan aset rusak: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true — aset rusak harus di-ClearError, bukan menggagalkan dokumen: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildPackageOrderPDF_SeluruhStatus(t *testing.T) {
	for _, status := range []domain.PackageOrderStatus{
		domain.PackageOrderDraft, domain.PackageOrderIssued, domain.PackageOrderCancelled,
	} {
		pdf, err := buildPackageOrderPDF(testPrintData(status), testProject(), testProfile(), nil, nil)
		if err != nil {
			t.Fatalf("status %v: %v", status, err)
		}
		if pdf.Err() {
			t.Fatalf("status %v: pdf.Err() true: %v", status, pdf.Error())
		}
		assertValidPDF(t, pdf)
	}
}

// A block taller than one page must split across pages instead of being
// clipped (PLAN.md B19) — the CATERING block in the source document is
// already close to half a page, so this is a real case.
func TestBuildPackageOrderPDF_BlokLebihTinggiDariSatuHalaman(t *testing.T) {
	lines := make([]string, 120)
	for i := range lines {
		lines[i] = "Item komposisi baris ke-" + strconv.Itoa(i+1)
	}
	data := testPrintData(domain.PackageOrderIssued)
	data.Blocks = []domain.ProjectPackageBlock{{
		ID: 1, Category: "CATERING", SortOrder: 1,
		Body: strings.Join(lines, "\n"), QtyText: "700 PORSI", BonusNote: "BONUS :\nMakanan After Akad",
	}}

	pdf, err := buildPackageOrderPDF(data, testProject(), testProfile(), nil, nil)
	if err != nil {
		t.Fatalf("buildPackageOrderPDF blok panjang: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	if pages := pdf.PageCount(); pages < 2 {
		t.Errorf("blok 120 baris muat dalam %d halaman — berarti isinya terpotong, bukan dipecah", pages)
	}
	assertValidPDF(t, pdf)
}

// Many blocks must paginate without the table header being orphaned or the
// document erroring.
func TestBuildPackageOrderPDF_BanyakBlok(t *testing.T) {
	var blocks []domain.ProjectPackageBlock
	for i := 0; i < 30; i++ {
		blocks = append(blocks, domain.ProjectPackageBlock{
			ID: int64(i + 1), Category: "KATEGORI " + strconv.Itoa(i+1), SortOrder: i + 1,
			Body: "Baris satu\nBaris dua\nBaris tiga", QtyText: "10 PCS", BonusNote: "BONUS :\nSesuatu",
		})
	}
	data := testPrintData(domain.PackageOrderIssued)
	data.Blocks = blocks

	pdf, err := buildPackageOrderPDF(data, testProject(), testProfile(), nil, nil)
	if err != nil {
		t.Fatalf("buildPackageOrderPDF banyak blok: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

// An empty PO — the StartBlank path (D23) — must still print a valid,
// signable document rather than erroring on its missing parts.
func TestBuildPackageOrderPDF_KosongTetapValid(t *testing.T) {
	data := packageOrderPrintData{
		Status: domain.PackageOrderDraft, IssuedAt: time.Now(),
		Event: domain.PackageOrderEventSnapshot{ClientName: "Rara & Dafa"},
	}
	pdf, err := buildPackageOrderPDF(data, testProject(), testProfile(), nil, nil)
	if err != nil {
		t.Fatalf("buildPackageOrderPDF kosong: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

func TestBuildPackageOrderPDF_TeksSangatPanjangTidakMenggagalkan(t *testing.T) {
	data := testPrintData(domain.PackageOrderIssued)
	data.TermsText = strings.Repeat("Ketentuan yang sangat panjang sekali. ", 200)
	data.BonusNote = strings.Repeat("Bonus tambahan berlimpah. ", 200)
	data.Adjustments = append(data.Adjustments, domain.ProjectPackageAdjustment{
		ID: 99, Description: strings.Repeat("Deskripsi penyesuaian amat panjang ", 20), Amount: -250_000, SortOrder: 3,
	})
	pdf, err := buildPackageOrderPDF(data, testProject(), testProfile(), nil, nil)
	if err != nil {
		t.Fatalf("buildPackageOrderPDF teks panjang: %v", err)
	}
	if pdf.Err() {
		t.Fatalf("pdf.Err() true: %v", pdf.Error())
	}
	assertValidPDF(t, pdf)
}

// D21 — the bold rule for sub-headings has to fire on exactly the lines the
// source spreadsheet already writes in capitals, and on nothing else.
func TestIsHeadingLine(t *testing.T) {
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
		{"150", false},   // angka saja bukan sub-judul
		{"- - -", false}, // pemisah bukan sub-judul
		{"2 FOTOGRAFER", true},
	}
	for _, c := range cases {
		if got := isHeadingLine(c.line); got != c.want {
			t.Errorf("isHeadingLine(%q) = %v, mau %v", c.line, got, c.want)
		}
	}
}

// D6/D27 — the single most consequential branch in the renderer: a Draft
// prints from the live tables, an issued PO from its frozen snapshot.
func TestBuildPackageOrderPrintData_DraftPakaiDataLive(t *testing.T) {
	view := &application.PackageOrderView{
		Order: &domain.PackageOrder{
			ProjectID: 1, Status: domain.PackageOrderDraft,
			BasePrice: 213_500_000, TermsText: "S&K live", TermsPlan: testTermsPlan(),
		},
		Blocks:      testPOBlocks(),
		Adjustments: testPOAdjustments(),
	}
	data := buildPackageOrderPrintData(view, testProject(), "Rara & Dafa", "08170043310", nil, 0)

	if data.TermsText != "S&K live" {
		t.Errorf("TermsText = %q, mau data live", data.TermsText)
	}
	if len(data.Blocks) != len(testPOBlocks()) {
		t.Errorf("Blocks = %d, mau %d dari tabel live", len(data.Blocks), len(testPOBlocks()))
	}
	// D27: tanpa ini blok "TAHAP PEMBAYARAN" hilang dari PDF negosiasi.
	if len(data.TermsPlan) == 0 {
		t.Error("TermsPlan kosong pada PO Draft — PDF negosiasi akan tercetak tanpa jadwal pembayaran sama sekali")
	}
	if data.Event.ClientName != "Rara & Dafa" || data.Event.Pax != testProject().Pax {
		t.Errorf("Event tidak diisi dari project live: %+v", data.Event)
	}
}

func TestBuildPackageOrderPrintData_TerbitPakaiSnapshot(t *testing.T) {
	issuedAt := time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC)
	snapshotBlocks := []domain.ProjectPackageBlock{{Category: "SNAPSHOT", Body: "Isi saat diteken"}}
	view := &application.PackageOrderView{
		Order: &domain.PackageOrder{
			ProjectID: 1, Status: domain.PackageOrderIssued, IssuedAt: &issuedAt,
			// Nilai live sengaja dibuat BERBEDA dari snapshot: kalau renderer
			// keliru membaca dari sini, tesnya gagal.
			BasePrice: 999_000_000, TermsText: "S&K yang sudah berubah",
			Snapshot: &domain.PackageOrderSnapshot{
				Current: domain.PackageOrderRevision{
					Revision: 0, IssuedAt: issuedAt, BasePrice: 213_500_000,
					TermsText: "S&K saat diteken", Blocks: snapshotBlocks,
					TermsPlan: testTermsPlan(),
					Event:     domain.PackageOrderEventSnapshot{ClientName: "Rara & Dafa", Venue: "KLINK TOWER"},
				},
			},
		},
		Blocks: testPOBlocks(),
	}
	data := buildPackageOrderPrintData(view, testProject(), "Nama Baru", "0800", nil, 0)

	if data.BasePrice != 213_500_000 {
		t.Errorf("BasePrice = %d, mau 213500000 dari snapshot — kontrak yang sudah diteken tidak boleh ikut berubah", data.BasePrice)
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

// Ledger pembayaran SELALU live, bahkan untuk PO yang sudah terbit — dokumen
// sumber membuktikannya: diteken Oktober 2025, memuat pembayaran sampai
// September 2026.
func TestBuildPackageOrderPrintData_LedgerSelaluLive(t *testing.T) {
	issuedAt := time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC)
	view := &application.PackageOrderView{
		Order: &domain.PackageOrder{
			ProjectID: 1, Status: domain.PackageOrderIssued, IssuedAt: &issuedAt,
			Snapshot: &domain.PackageOrderSnapshot{Current: domain.PackageOrderRevision{BasePrice: 213_500_000}},
		},
	}
	payments := []domain.ClientPayment{testPayment(), testPayment()}
	data := buildPackageOrderPrintData(view, testProject(), "Rara & Dafa", "0800", payments, 10_000_000)

	if len(data.Payments) != 2 || data.TotalPaid != 10_000_000 {
		t.Errorf("ledger tidak live: %d pembayaran, totalPaid %d", len(data.Payments), data.TotalPaid)
	}
}

func TestPackageOrderPrintData_SubtitleMenandaiRevisi(t *testing.T) {
	if got := (packageOrderPrintData{Revision: 0}).subtitle(); strings.Contains(got, "Revisi") {
		t.Errorf("revisi 0 tidak boleh berlabel revisi, got %q", got)
	}
	if got := (packageOrderPrintData{Revision: 2}).subtitle(); !strings.Contains(got, "Revisi 2") {
		t.Errorf("subtitle revisi 2 = %q, mau memuat \"Revisi 2\"", got)
	}
}

func TestFormatRupiahSigned(t *testing.T) {
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

// The merged category cell is what makes the printed table match the source
// document, where CATERING spans its two rows under a single centred label.
func TestCategorySpans(t *testing.T) {
	blocks := func(categories ...string) []domain.ProjectPackageBlock {
		out := make([]domain.ProjectPackageBlock, 0, len(categories))
		for _, c := range categories {
			out = append(out, domain.ProjectPackageBlock{Category: c})
		}
		return out
	}
	cases := []struct {
		name string
		in   []domain.ProjectPackageBlock
		want [][2]int
	}{
		{"dokumen sumber", blocks("CATERING", "CATERING", "DEKORASI"), [][2]int{{0, 1}, {2, 2}}},
		{"semua berbeda", blocks("A", "B", "C"), [][2]int{{0, 0}, {1, 1}, {2, 2}}},
		{"semua sama", blocks("A", "A", "A"), [][2]int{{0, 2}}},
		{"satu blok", blocks("A"), [][2]int{{0, 0}}},
		{"kosong", nil, nil},
		// Hanya yang BERURUTAN yang digabung -- urutan yang disusun WO adalah
		// urutan yang tercetak, tidak pernah disusun ulang diam-diam.
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
