package presentation

import (
	"strings"
	"unicode"

	"github.com/go-pdf/fpdf"

	"jwswedding/internal/modules/projects/application"
)

// Tabel komposisi paket (KATEGORI/PRODUK/QTY/BONUS) untuk PDF Tagihan.
//
// Ini SALINAN dari renderer yang sama di quotations/presentation
// (quotation_pdf.go), bukan pemakaian bersama, dan itu keputusan sadar:
// `projects` tidak boleh mengimpor apa pun dari `quotations` selain
// contracts-nya, sedangkan renderer ini hidup di lapisan presentation
// modul itu. Kodebase ini sudah memakai idiom yang sama untuk kasus yang
// sama persis (dulu ComputeScheduleAmounts/SplitScheduleAmounts, dan
// pdf_theme.go yang memang kembar di kedua modul).
//
// Konsekuensinya harus dipikul jujur: kalau tabel di PDF PO berubah bentuk,
// yang di sini TIDAK ikut berubah. Komentar silang di kedua sisi menunjuk
// pasangannya, dan geometrinya sengaja dibuat identik (angka yang sama,
// urutan kolom yang sama) supaya perbedaan apa pun langsung terlihat saat
// kedua PDF ditaruh berdampingan.
const (
	compColCategory = 32.0
	compColProduct  = 84.0
	compColQty      = 26.0
	compColBonus    = 38.0
	compLineH       = 4.4
	compCellPadX    = 2.5
	compCellPadY    = 2.5
)

// isCompositionHeading menandai baris PRODUK yang seluruh hurufnya kapital —
// dicetak tebal sebagai sub-judul (D21), persis aturan yang dipakai PDF PO,
// supaya BUFFET/DESSERT/MINUMAN tercetak sama di kedua dokumen.
//
// Baris tanpa huruf sama sekali (angka telanjang, pemisah) bukan judul —
// tanpa syarat itu "150" ikut tercetak tebal.
func isCompositionHeading(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	hasLetter := false
	for _, r := range trimmed {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return hasLetter
}

// wrapCompositionCell memecah isi sel menjadi baris tercetak: pertama pada
// newline yang diketik penulisnya, lalu membungkus tiap potongan itu ke lebar
// kolom. Baris kosong dipertahankan — itu spasi yang memang disengaja.
func wrapCompositionCell(pdf *fpdf.Fpdf, text string, width float64) []string {
	var out []string
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(raw) == "" {
			out = append(out, "")
			continue
		}
		for _, seg := range pdf.SplitLines([]byte(raw), width) {
			out = append(out, string(seg))
		}
	}
	return out
}

func drawCompositionHeader(pdf *fpdf.Fpdf, theme pdfTheme, y float64) float64 {
	const h = 8.0
	pdf.SetFillColor(theme.Palette.AccentDark[0], theme.Palette.AccentDark[1], theme.Palette.AccentDark[2])
	pdf.Rect(pML, y, pCW, h, "F")
	pdf.SetFont(theme.Family, "B", 8)
	pdf.SetTextColor(colorWhite[0], colorWhite[1], colorWhite[2])

	x := pML
	for _, col := range []struct {
		w     float64
		label string
	}{
		{compColCategory, "KATEGORI"},
		{compColProduct, "PRODUK"},
		{compColQty, "QTY"},
		{compColBonus, "BONUS"},
	} {
		pdf.SetXY(x+compCellPadX, y+2)
		pdf.CellFormat(col.w-compCellPadX*2, 4, col.label, "", 0, "L", false, 0, "")
		x += col.w
	}
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	return y + h
}

// drawCompositionCategoryCell menulis label kategori yang digabung, rata
// tengah secara vertikal pada rentang yang ditempati blok-bloknya di halaman
// ini.
func drawCompositionCategoryCell(pdf *fpdf.Fpdf, theme pdfTheme, label string, top, bottom float64) {
	if label == "" || bottom <= top {
		return
	}
	pdf.SetFont(theme.Family, "B", 8)
	lines := pdf.SplitLines([]byte(label), compColCategory-compCellPadX*2)
	ty := top + (bottom-top-float64(len(lines))*compLineH)/2
	if ty < top+compCellPadY {
		ty = top + compCellPadY
	}
	for _, line := range lines {
		pdf.SetXY(pML+compCellPadX, ty)
		pdf.CellFormat(compColCategory-compCellPadX*2, compLineH, string(line), "", 0, "L", false, 0, "")
		ty += compLineH
	}
}

// drawCompositionFragment menggambar batas satu penggalan baris beserta tiga
// kolom teksnya. Pemisah kolom digambar per penggalan supaya blok yang
// terpotong halaman tetap terbaca sebagai tabel di halaman lanjutannya.
//
// mergeBottom menghilangkan garis penutup di kolom kategori — itulah yang
// membuat blok ini menyatu dengan blok berikutnya dalam kategori yang sama.
func drawCompositionFragment(pdf *fpdf.Fpdf, theme pdfTheme, y, h float64, mergeBottom bool, product, qty, bonus []string) {
	pdf.SetFillColor(255, 255, 255)
	pdf.Rect(pML, y, pCW, h, "F")
	pdf.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])
	pdf.SetLineWidth(0.2)
	x := pML
	pdf.Line(x, y, x, y+h)
	for _, w := range []float64{compColCategory, compColProduct, compColQty, compColBonus} {
		x += w
		pdf.Line(x, y, x, y+h)
	}
	if mergeBottom {
		pdf.Line(pML+compColCategory, y+h, pMR, y+h)
	} else {
		pdf.Line(pML, y+h, pMR, y+h)
	}

	ty := y + compCellPadY
	for _, line := range product {
		style := ""
		if isCompositionHeading(line) {
			style = "B"
		}
		pdf.SetFont(theme.Family, style, 8.5)
		pdf.SetXY(pML+compColCategory+compCellPadX, ty)
		pdf.CellFormat(compColProduct-compCellPadX*2, compLineH, line, "", 0, "L", false, 0, "")
		ty += compLineH
	}

	pdf.SetFont(theme.Family, "", 8.5)
	drawCompositionColumn(pdf, pML+compColCategory+compColProduct+compCellPadX, y+compCellPadY, compColQty-compCellPadX*2, qty)
	drawCompositionColumn(pdf, pML+compColCategory+compColProduct+compColQty+compCellPadX, y+compCellPadY, compColBonus-compCellPadX*2, bonus)
}

func drawCompositionColumn(pdf *fpdf.Fpdf, x, y, w float64, lines []string) {
	for _, line := range lines {
		pdf.SetXY(x, y)
		pdf.CellFormat(w, compLineH, line, "", 0, "L", false, 0, "")
		y += compLineH
	}
}

// takeCompositionLines memotong sampai n entri dari depan slice dan
// mengecilkannya di tempat — konsumsi aliran yang diandalkan pemecahan
// halaman di bawah.
func takeCompositionLines(lines *[]string, n int) []string {
	if n > len(*lines) {
		n = len(*lines)
	}
	head := (*lines)[:n]
	*lines = (*lines)[n:]
	return head
}

// compositionSpans mengelompokkan baris BERURUTAN yang berkategori sama
// menjadi pasangan indeks [awal, akhir] inklusif — rentang yang dirender
// sebagai satu sel gabungan.
//
// Berurutan saja, disengaja: urutan yang disusun WO adalah urutan yang
// tercetak, jadi dua blok CATERING yang dipisahkan blok DEKORASI tetap jadi
// dua sel, bukan diam-diam disatukan.
func compositionSpans(rows []application.QuotationCompositionRow) [][2]int {
	var spans [][2]int
	for i := 0; i < len(rows); {
		j := i
		for j+1 < len(rows) && rows[j+1].Category == rows[i].Category {
			j++
		}
		spans = append(spans, [2]int{i, j})
		i = j + 1
	}
	return spans
}

// drawCompositionTable menggambar tabel komposisi paket.
//
// Blok yang lebih tinggi dari satu halaman dipecah antar halaman, bukan
// dipotong: blok CATERING pada dokumen sumber saja sudah hampir setengah
// halaman, jadi ini kasus nyata. Ketiga kolom dikonsumsi sebagai aliran
// paralel — sebanyak yang muat dari masing-masing masuk halaman ini, sisanya
// lanjut di halaman berikutnya, sehingga tidak ada isi sel yang diam-diam
// hilang.
func drawCompositionTable(pdf *fpdf.Fpdf, theme pdfTheme, y float64, rows []application.QuotationCompositionRow) float64 {
	if len(rows) == 0 {
		return y
	}
	y = ensureSpace(pdf, theme, y, 20)
	y = drawCompositionHeader(pdf, theme, y)

	for _, span := range compositionSpans(rows) {
		i, j := span[0], span[1]
		category := rows[i].Category
		groupTop := y

		for k := i; k <= j; k++ {
			row := rows[k]
			pdf.SetFont(theme.Family, "", 8.5)
			product := wrapCompositionCell(pdf, row.Product, compColProduct-compCellPadX*2)
			qty := wrapCompositionCell(pdf, row.Qty, compColQty-compCellPadX*2)
			bonus := wrapCompositionCell(pdf, row.Bonus, compColBonus-compCellPadX*2)

			for first := true; ; first = false {
				remaining := len(product)
				if len(qty) > remaining {
					remaining = len(qty)
				}
				if len(bonus) > remaining {
					remaining = len(bonus)
				}
				if remaining == 0 && !first {
					break
				}
				avail := contentBottom - y - compCellPadY*2
				fit := int(avail / compLineH)
				// Sisa kurang dari dua baris tidak sepadan untuk sebuah
				// penggalan — tutup sel gabungannya untuk apa yang sudah ada
				// di halaman ini, lalu lanjut di halaman baru.
				if fit < 2 {
					drawCompositionCategoryCell(pdf, theme, category, groupTop, y)
					pdf.AddPage()
					drawAccentBand(pdf, theme)
					y = drawCompositionHeader(pdf, theme, 20.0)
					groupTop = y
					continue
				}
				take := remaining
				if take > fit {
					take = fit
				}
				if take < 1 {
					take = 1 // blok kosong pun tetap dapat satu baris ber-padding
				}

				lastFragment := remaining <= take
				rowH := float64(take)*compLineH + compCellPadY*2
				drawCompositionFragment(pdf, theme, y, rowH, lastFragment && k < j,
					takeCompositionLines(&product, take), takeCompositionLines(&qty, take), takeCompositionLines(&bonus, take))
				y += rowH
				if lastFragment {
					break
				}
			}
		}
		drawCompositionCategoryCell(pdf, theme, category, groupTop, y)
	}
	return y
}
