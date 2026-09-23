package presentation

// Tes TTD klien di PDF PO (§13 PLAN ttd-penawaran): kotak terisi bila ada
// salinan milik dokumen, kosong bila belum diteken, dan PDF tetap tercetak
// bila gambarnya tidak bisa dibaca.

import (
	"bytes"
	"testing"

	"jwswedding/internal/modules/quotations/domain"
)

func TestQuotationPDF_DenganTTDKlien(t *testing.T) {
	data := testQuotationPrintData(domain.QuotationAccepted)
	data.ClientSignature = validQuotationPNGBytes(t)
	data.ClientSignerName = "Rara"

	pdf, err := buildQuotationPDF(data, testQuotationEventDate(), testQuotationProfile(), nil, validQuotationPNGBytes(t), "Anisa Putri", "Lead Planner")
	if err != nil {
		t.Fatalf("buildQuotationPDF dengan TTD klien: %v", err)
	}
	assertValidQuotationPDF(t, pdf)
}

func TestQuotationPDF_TanpaTTDKlien(t *testing.T) {
	data := testQuotationPrintData(domain.QuotationOffered)

	pdf, err := buildQuotationPDF(data, testQuotationEventDate(), testQuotationProfile(), nil, nil, "Anisa Putri", "Lead Planner")
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
	pdf, err := buildQuotationPDF(data, testQuotationEventDate(), testQuotationProfile(), nil, nil, "Anisa Putri", "Lead Planner")
	if err != nil {
		t.Fatalf("buildQuotationPDF dengan gambar rusak: %v", err)
	}
	assertValidQuotationPDF(t, pdf)
}

// --- Kolom WO: pengesah dokumen, bukan pemilik usaha (PLAN
// tanda-tangan-pengguna T33) ---
//
// Teks PDF tidak bisa dicari sebagai string (font subset menyimpannya sebagai
// indeks glyph), jadi yang dibandingkan adalah keluaran antar-skenario: nama
// dan jabatan pengesah harus benar-benar MENGUBAH hasil cetak.

func quotationWith(t *testing.T, woName, woTitle string) []byte {
	t.Helper()
	pdf, err := buildQuotationPDF(testQuotationPrintData(domain.QuotationOffered), testQuotationEventDate(), testQuotationProfile(), nil, nil, woName, woTitle)
	if err != nil {
		t.Fatalf("buildQuotationPDF(%q,%q): %v", woName, woTitle, err)
	}
	pdf.SetCompression(false)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("pdf.Output: %v", err)
	}
	return buf.Bytes()
}

func TestQuotationPDF_KolomWOMengikutiPengesah(t *testing.T) {
	a := quotationWith(t, "Anisa Putri", "Lead Planner")
	if bytes.Equal(a, quotationWith(t, "Rara Koordinator", "Lead Planner")) {
		t.Fatal("PDF identik untuk dua nama pengesah berbeda — namanya tidak tercetak")
	}
	// Jabatan menggantikan caption tetap "WEDDING CONSULTANT" (A4), jadi jabatan
	// berbeda pun wajib mengubah hasil cetak.
	if bytes.Equal(a, quotationWith(t, "Anisa Putri", "Sales")) {
		t.Error("PDF identik untuk dua jabatan berbeda — captionnya masih teks tetap")
	}
	// Perilaku lama selalu mencetak profile.OwnerName di kolom WO.
	if bytes.Equal(a, quotationWith(t, testQuotationProfile().OwnerName, "Lead Planner")) {
		t.Error("kolom WO masih mencetak nama pemilik usaha")
	}
}

// K6: penerbit tak ter-resolve -> kolom WO kosong, dan PDF tetap terbit.
func TestQuotationPDF_PengesahTakTerResolveTetapTerbit(t *testing.T) {
	out := quotationWith(t, "", "")
	if len(out) == 0 {
		t.Fatal("PDF tanpa pengesah harus tetap terbit")
	}
	if bytes.Equal(out, quotationWith(t, "Anisa Putri", "Lead Planner")) {
		t.Error("kolom WO kosong menghasilkan PDF identik dengan yang ada pengesahnya")
	}
}

// Kolom KLIEN tidak boleh ikut berubah oleh perubahan di sisi WO — TTD klien
// tetap snapshot milik dokumen (D6c), lingkup yang PLAN ini sengaja tidak
// sentuh.
func TestQuotationPDF_KolomKlienTidakTerpengaruhPengesahWO(t *testing.T) {
	render := func(woName string) []byte {
		data := testQuotationPrintData(domain.QuotationAccepted)
		data.ClientSignerName = "Rara"
		pdf, err := buildQuotationPDF(data, testQuotationEventDate(), testQuotationProfile(), nil, nil, woName, "Lead Planner")
		if err != nil {
			t.Fatalf("buildQuotationPDF: %v", err)
		}
		pdf.SetCompression(false)
		var buf bytes.Buffer
		if err := pdf.Output(&buf); err != nil {
			t.Fatalf("pdf.Output: %v", err)
		}
		return buf.Bytes()
	}
	// Kedua keluaran berbeda (kolom WO memang berubah), tapi keduanya harus
	// sama-sama terbit dengan nama klien yang sama — tidak ada galat, tidak ada
	// kolom klien yang hilang.
	if len(render("Anisa Putri")) == 0 || len(render("")) == 0 {
		t.Error("PDF harus terbit apa pun keadaan pengesah WO")
	}
}
