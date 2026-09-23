package presentation

// Blok tanda tangan Invoice & Kwitansi memakai staff PENERBIT dokumen, bukan
// pemilik usaha (PLAN tanda-tangan-pengguna T32).
//
// Dua hal yang dikunci di sini:
//  1. yang tercetak di bawah garis benar-benar mengikuti nama pengesah, bukan
//     lagi nama pemilik usaha;
//  2. tinggi bloknya tidak berubah saat nama dikosongkan (K6) — sigBottom
//     dipakai buildClientInvoicePDF untuk menghitung posisi blok berikutnya,
//     jadi tinggi yang menyusut akan menggeser seluruh sisa halaman.

import (
	"bytes"
	"testing"
	"time"

	"github.com/go-pdf/fpdf"
)

// renderedBytes mengembalikan keluaran PDF mentah dengan kompresi dimatikan.
//
// Teksnya TIDAK bisa dicari sebagai string biasa: fpdf memakai font subset,
// jadi "Anisa Putri" tersimpan sebagai indeks glyph, bukan huruf. Karena itu
// asersi di bawah membandingkan keluaran antar-skenario, bukan mencari kata —
// yang dibuktikan adalah nama pengesah benar-benar MENGUBAH hasil cetak, jadi
// parameternya tidak mungkin diam-diam diabaikan.
func renderedBytes(t *testing.T, pdf *fpdf.Fpdf) []byte {
	t.Helper()
	pdf.SetCompression(false)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("pdf.Output: %v", err)
	}
	return buf.Bytes()
}

func invoiceWith(t *testing.T, signerName string) []byte {
	t.Helper()
	pdf, err := buildClientInvoicePDF(testProject(), testInvoice(), testProfile(), nil, nil, 0, "", nil, signerName)
	if err != nil {
		t.Fatalf("buildClientInvoicePDF(%q): %v", signerName, err)
	}
	return renderedBytes(t, pdf)
}

func receiptWith(t *testing.T, signerName string) []byte {
	t.Helper()
	pdf, err := buildClientPaymentReceiptPDF(testProject(), testPayment(), "", testProfile(), nil, nil, 0, signerName)
	if err != nil {
		t.Fatalf("buildClientPaymentReceiptPDF(%q): %v", signerName, err)
	}
	return renderedBytes(t, pdf)
}

// Nama pengesah harus benar-benar dicetak: dua nama berbeda wajib menghasilkan
// dokumen berbeda, dan nama yang kebetulan sama dengan pemilik usaha tidak
// boleh diperlakukan istimewa.
func TestInvoicePDF_NamaPengesahMempengaruhiHasilCetak(t *testing.T) {
	a := invoiceWith(t, "Anisa Putri")
	b := invoiceWith(t, "Rara Koordinator")
	if bytes.Equal(a, b) {
		t.Fatal("Invoice identik untuk dua pengesah berbeda — signerName tidak tercetak")
	}
	if bytes.Equal(a, invoiceWith(t, "")) {
		t.Error("Invoice tanpa pengesah identik dengan yang ada pengesahnya")
	}
	// Dulu blok ini SELALU mencetak profile.OwnerName. Kalau perilaku lama masih
	// tersisa, keluaran untuk pengesah bernama lain akan sama dengan keluaran
	// untuk pengesah bernama persis pemilik usaha.
	if bytes.Equal(a, invoiceWith(t, testProfile().OwnerName)) {
		t.Error("Invoice masih mencetak nama pemilik usaha, bukan nama pengesah")
	}
}

func TestKwitansiPDF_NamaPengesahMempengaruhiHasilCetak(t *testing.T) {
	a := receiptWith(t, "Anisa Putri")
	if bytes.Equal(a, receiptWith(t, "Rara Koordinator")) {
		t.Fatal("Kwitansi identik untuk dua pengesah berbeda — signerName tidak tercetak")
	}
	if bytes.Equal(a, receiptWith(t, "")) {
		t.Error("Kwitansi tanpa pengesah identik dengan yang ada pengesahnya")
	}
	if bytes.Equal(a, receiptWith(t, testProfile().OwnerName)) {
		t.Error("Kwitansi masih mencetak nama pemilik usaha, bukan nama pengesah")
	}
}

// K6: pengesah tak ter-resolve tetap menghasilkan PDF yang sah, bukan galat.
func TestPDF_PengesahTakTerResolveTetapTerbit(t *testing.T) {
	if len(invoiceWith(t, "")) == 0 {
		t.Error("Invoice tanpa pengesah harus tetap terbit")
	}
	if len(receiptWith(t, "")) == 0 {
		t.Error("Kwitansi tanpa pengesah harus tetap terbit")
	}
}

// Regresi tata letak: tinggi blok WAJIB sama dengan atau tanpa nama. Kalau
// baris namanya dilewati alih-alih dicetak kosong, nilai kembaliannya
// mengecil dan seluruh blok di bawahnya ikut naik.
func TestSignatureBlock_TinggiBlokTidakBergantungPadaAdaTidaknyaNama(t *testing.T) {
	measure := func(signerName string, sign []byte) float64 {
		pdf, theme := newDocument(testProfile(), nil, "INVOICE", "Tagihan kepada Client")
		return signatureBlock(pdf, theme, 130.0, 60.0, 65.0, testProfile(), sign, time.Now(), "Hormat kami,", signerName)
	}

	withName := measure("Anisa Putri", nil)
	withoutName := measure("", nil)
	if withName != withoutName {
		t.Errorf("tinggi blok berubah saat nama kosong: dengan nama %v, tanpa nama %v", withName, withoutName)
	}

	// Slot gambar 20mm juga harus tetap dipesan walau TTD-nya tidak ada.
	withSignature := measure("Anisa Putri", validPNGBytes(t))
	if withSignature != withName {
		t.Errorf("tinggi blok berubah saat TTD ada: dengan TTD %v, tanpa TTD %v", withSignature, withName)
	}
}
