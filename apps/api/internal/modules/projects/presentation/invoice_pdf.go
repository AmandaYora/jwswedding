package presentation

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-pdf/fpdf"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/terbilang"
)

// formatRupiah renders a non-negative amount as "Rp 2.500.000" — dot
// thousands separator, id-ID convention (mirrors apps/web's
// formatCurrency), no shared Go equivalent existed in this codebase yet.
// A negative amount (only ever theoretically reachable — summaryStrip
// relabels its "Sisa Tagihan" cell to "Lebih Bayar" with a positive amount
// before this is ever called with one, A5) still renders with a leading
// "-" rather than panicking, as a defensive floor.
func formatRupiah(amount int64) string {
	s := strconv.FormatInt(amount, 10)
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, c)
	}
	sign := ""
	if neg {
		sign = "-"
	}
	return sign + "Rp " + string(out)
}

// fpdfImageType maps sniffed content bytes to one of fpdf's supported embed
// types (jpg/png/gif) — fpdf doesn't support WEBP, one of the 3 MIME types
// TenantService.UploadLogo's whitelist accepts, so a WEBP (or otherwise
// undecodable) logo simply isn't embedded rather than failing the whole PDF.
// Also used for the Kwitansi signature image, even though UploadSignature
// only ever stores PNG — sniffing again here costs nothing and keeps this
// function's contract ("degrade gracefully, never trust the caller's
// claimed type") uniform across both assets.
func fpdfImageType(data []byte) (tp string, ok bool) {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return "jpg", true
	case "image/png":
		return "png", true
	case "image/gif":
		return "gif", true
	default:
		return "", false
	}
}

// newDocument builds the kop surat (logo + business identity + document
// title/subtitle) and the shared footer for both Invoice and Kwitansi
// (PLAN.md redesain-pdf-invoice-kwitansi-v2 §5.2.1/§5.2.2/§5.2.6). Returns
// the theme alongside *fpdf.Fpdf so callers never need to re-derive the font
// family or re-resolve the accent palette. Leaves the cursor at (pML, y)
// just below the accent rule, ready for the caller's own body content.
//
// leftW (the business-identity block's width) is computed from the actual
// remaining space before the title column, never hardcoded independently of
// it — the fix for §3.1's bug, where a long business name collided with the
// document title because the two blocks' widths could silently drift apart.
func newDocument(profile platformcontracts.TenantProfile, logo []byte, title, subtitle string) (*fpdf.Fpdf, pdfTheme) {
	pdf := fpdf.New("P", "mm", "A4", "")
	family := registerFonts(pdf)
	theme := pdfTheme{Family: family, Palette: newPalette(profile)}

	businessName := profile.BusinessName
	pdf.SetFooterFunc(func() {
		pdf.SetY(-14)
		hairline(pdf, pML, pMR, pdf.GetY())
		pdf.SetXY(pML, pdf.GetY()+2)
		pdf.SetFont(theme.Family, "", 7.5)
		pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
		pdf.CellFormat(120, 4.5, "Dokumen ini diterbitkan secara elektronik oleh "+businessName+".", "", 0, "L", false, 0, "")
		pdf.CellFormat(60, 4.5, fmt.Sprintf("Halaman %d dari {nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
		pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	})
	pdf.AliasNbPages("")
	pdf.SetMargins(pML, 14, pML)
	// Auto page-break is deliberately OFF: this layout draws fixed-height
	// boxes computed by arithmetic, and ensureSpace is what decides when a
	// block needs a fresh page instead (A6) — letting fpdf's own
	// mid-MultiCell auto-break fire left every block drawn after a long
	// description positioned off the wrong page (§3.3).
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()
	drawAccentBand(pdf, theme)

	const headTop = 14.0
	const rightX, rightW = 120.0, 75.0 // 120..195, right edge aligned to pMR

	textX := pML
	logoBottom := headTop
	if tp, ok := fpdfImageType(logo); ok {
		info := pdf.RegisterImageOptionsReader("logo", fpdf.ImageOptions{ImageType: tp}, bytes.NewReader(logo))
		if pdf.Err() {
			// Sniffed as jpg/png/gif but fpdf still couldn't decode it (e.g.
			// truncated/corrupt bytes) — degrade to no logo rather than
			// failing the whole PDF, same "logo is optional" contract as a
			// tenant with no logo at all.
			pdf.ClearError()
		} else if info != nil {
			w, h := fitImage(info.Width(), info.Height(), 22, 18)
			pdf.ImageOptions("logo", pML, headTop, w, h, false, fpdf.ImageOptions{ImageType: tp}, 0, "")
			textX = pML + 22 + 5
			logoBottom = headTop + h
		}
	}
	leftW := rightX - 6 - textX

	pdf.SetXY(textX, headTop)
	pdf.SetFont(theme.Family, "B", 12.5)
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	pdf.MultiCell(leftW, 5.5, businessName, "", "L", false)
	leftBottom := pdf.GetY() + 1

	pdf.SetFont(theme.Family, "", 8.5)
	pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
	if profile.Address != "" {
		pdf.SetXY(textX, leftBottom)
		pdf.MultiCell(leftW, 4, profile.Address, "", "L", false)
		leftBottom = pdf.GetY()
	}
	contact := profile.Phone
	if profile.Email != "" {
		if contact != "" {
			contact += "  ·  "
		}
		contact += profile.Email
	}
	if contact != "" {
		pdf.SetXY(textX, leftBottom)
		pdf.MultiCell(leftW, 4, contact, "", "L", false)
		leftBottom = pdf.GetY()
	}
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])

	pdf.SetXY(rightX, headTop-1)
	pdf.SetFont(theme.Family, "B", 24)
	pdf.SetTextColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
	pdf.CellFormat(rightW, 10, title, "", 2, "R", false, 0, "")
	rightBottom := pdf.GetY()
	if subtitle != "" {
		pdf.SetX(rightX)
		pdf.SetFont(theme.Family, "", 8.5)
		pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
		pdf.CellFormat(rightW, 4.5, subtitle, "", 2, "R", false, 0, "")
		rightBottom = pdf.GetY()
	}
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])

	y := max(leftBottom, max(rightBottom, logoBottom)) + 4
	accentRule(pdf, theme, pML, pMR, y)
	pdf.SetXY(pML, y+7)
	return pdf, theme
}

// invoiceStatusBadge maps an InvoiceStatus to its badge label and 4-state
// color pair — Terkirim reads as "BELUM DIBAYAR" on the badge since that's
// the status's actual meaning to whoever reads the printed document, even
// though the domain enum's own label is "Terkirim".
func invoiceStatusBadge(status domain.InvoiceStatus) (label string, bg, fg [3]int) {
	switch status {
	case domain.InvoicePaid:
		return "LUNAS", badgePaidBg, badgePaidText
	case domain.InvoiceCancelled:
		return "DIBATALKAN", badgeCancelBg, badgeCancelText
	case domain.InvoiceSent:
		return "BELUM DIBAYAR", badgeSentBg, badgeSentText
	default: // domain.InvoiceDraft, and any future/unknown value
		return "DRAFT", badgeDraftBg, badgeDraftText
	}
}

// drawInvoiceTableRowsHeader draws the item table's accent-filled header row
// at the given y — factored out because the table can restart on a fresh
// page mid-document when a long description doesn't fit (§3.3), and the
// restarted table needs this same header redrawn above its row.
func drawInvoiceTableHeader(pdf *fpdf.Fpdf, theme pdfTheme, y float64) {
	pdf.SetFillColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
	pdf.Rect(pML, y, pCW, 8, "F")
	pdf.SetTextColor(colorWhite[0], colorWhite[1], colorWhite[2])
	pdf.SetFont(theme.Family, "B", 8)
	pdf.SetXY(pML+3, y+2)
	pdf.CellFormat(28, 4, "JENIS", "", 0, "L", false, 0, "")
	pdf.SetXY(48, y+2)
	pdf.CellFormat(90, 4, "KETERANGAN", "", 0, "L", false, 0, "")
	pdf.SetXY(pMR-50, y+2)
	pdf.CellFormat(50, 4, "JUMLAH", "", 0, "R", false, 0, "")
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
}

// buildClientInvoicePDF renders an Invoice/Tagihan (PLAN.md
// redesain-pdf-invoice-kwitansi-v2 §5.3).
//
// Yang DITAGIHKAN tetap satu baris Jenis+Keterangan+Jumlah — keputusan lama
// "no line items" masih berlaku, karena satu Tagihan menagih satu termin,
// bukan daftar barang. Di ATASNYA kini tercetak tabel KOMPOSISI PAKET
// (KATEGORI/PRODUK/QTY/BONUS) yang sama persis dengan PDF PO, plus Nomor PO
// di kartu detail: keduanya KONTEKS, bukan yang ditagih. Tanpa itu, penerima
// tagihan tidak punya cara tahu tagihan ini milik PO yang mana, atau paket
// apa yang sedang dicicilnya, tanpa membuka dokumen kedua.
//
// poNumber "" dan composition nil adalah keadaan sah: project lama yang lahir
// sebelum fase penawaran tidak punya PO sama sekali. Barisnya lalu jatuh ke
// "-" dan tabelnya tidak tercetak, bukan gagal.
//
// totalPaid (ClientPaymentService.TotalReceived) feeds the "Nilai
// Kontrak / Total Sudah Dibayar / Sisa Tagihan" summary strip (K3) —
// deliberately a caller-supplied parameter rather than this function
// querying `projects`' own payment repository itself, since a PDF builder
// has no business reaching past its own arguments into another service.
// poNumber/composition ikut konvensi yang sama: disodorkan pemanggil.
func buildClientInvoicePDF(project domain.Project, inv domain.ClientInvoice, profile platformcontracts.TenantProfile, logo, signature []byte, totalPaid int64, poNumber string, composition []application.QuotationCompositionRow) (*fpdf.Fpdf, error) {
	pdf, theme := newDocument(profile, logo, "INVOICE", "Tagihan kepada Client")
	y := pdf.GetY()

	const cardW = 87.0
	leftRows := [][2]string{
		{"Nama Client", project.BrideName + " & " + project.GroomName},
		{"Acara", project.Name},
		{"Tanggal Acara", formatTanggalPDF(project.EventDate)},
		{"Paket", project.PackageName},
	}
	label, bg, fg := invoiceStatusBadge(inv.Status)
	rightRows := [][2]string{
		{"Nomor Invoice", inv.InvoiceNumber},
		{"Nomor PO", orDash(poNumber)},
		{"Tanggal Terbit", formatTanggalPDF(inv.CreatedAt)},
		{"Jatuh Tempo", formatTanggalPDF(inv.DueDate)},
	}
	badge := &badgeSpec{Label: label, Bg: bg, Fg: fg}
	y1 := infoCard(pdf, theme, pML, y, cardW, "Ditagihkan Kepada", leftRows, nil)
	y2 := infoCard(pdf, theme, pMR-cardW, y, cardW, "Detail Tagihan", rightRows, badge)
	y = max(y1, y2) + 8

	// Komposisi paket lebih dulu: ia KONTEKS untuk baris tagihan di bawahnya,
	// jadi pembaca melihat "paket apa" sebelum "berapa yang ditagih sekarang".
	// Label seksinya wajib — tanpa itu tabel ini gampang disalahbaca sebagai
	// rincian yang sedang ditagihkan.
	if len(composition) > 0 {
		y = ensureSpace(pdf, theme, y, 20)
		sectionLabel(pdf, theme, pML, y, "Komposisi Paket")
		y = drawCompositionTable(pdf, theme, y+5, composition)
		y = ensureSpace(pdf, theme, y+8, 20)
		sectionLabel(pdf, theme, pML, y, "Yang Ditagihkan")
		y += 5
	}

	// Measure the row BEFORE drawing anything — fpdf renders immediately
	// (no retained-mode undo), so the header must not be painted until we
	// know which page it's actually landing on. Drawing it first and only
	// then checking whether the row fits left an orphaned header (with no
	// row under it) on the old page whenever the row didn't fit, plus a
	// duplicate header on the new one.
	pdf.SetFont(theme.Family, "", 10)
	descLines := pdf.SplitLines([]byte(inv.Description), 88)
	rowH := max(12, 6+float64(len(descLines))*5)
	if y+8+rowH > contentBottom {
		// The row itself won't fit before the footer — move the whole row
		// (header + data) to a fresh page rather than letting it split
		// mid-description across two pages (§3.3).
		y = ensureSpace(pdf, theme, contentBottom, 8+rowH)
	}
	drawInvoiceTableHeader(pdf, theme, y)
	rowY := y + 8
	pdf.SetFillColor(255, 255, 255)
	pdf.Rect(pML, rowY, pCW, rowH, "F")
	pdf.SetXY(pML+3, rowY+3.5)
	pdf.CellFormat(28, 5, string(inv.Type), "", 0, "L", false, 0, "")
	pdf.SetXY(48, rowY+3.5)
	pdf.MultiCell(88, 5, inv.Description, "", "L", false)
	pdf.SetXY(pMR-50, rowY+3.5)
	pdf.CellFormat(50, 5, formatRupiah(inv.Amount), "", 0, "R", false, 0, "")
	y = rowY + rowH
	hairline(pdf, pML, pMR, y)

	const totalX, totalW = 105.0, 90.0
	// Reserved height is the larger of 26mm (the right-side total box's own
	// fixed requirement) and the left-side Terbilang text's actual wrapped
	// height — terbilang.Rupiah has no upper bound on inv.Amount, so a
	// very large amount that wraps to several lines must be measured for
	// real rather than assumed to fit inside a hardcoded reservation.
	pdf.SetFont(theme.Family, "", 9)
	terbilangReserveLines := pdf.SplitLines([]byte(terbilang.Rupiah(inv.Amount)), 85)
	terbilangReserveHeight := 9.5 + float64(len(terbilangReserveLines))*4.5
	y = ensureSpace(pdf, theme, y, max(26, terbilangReserveHeight))
	totalY := y + 4
	pdf.SetFont(theme.Family, "", 9)
	pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
	pdf.SetXY(totalX, totalY)
	pdf.CellFormat(45, 5, "Subtotal", "", 0, "L", false, 0, "")
	pdf.SetFont(theme.Family, "", 10)
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	pdf.CellFormat(45, 5, formatRupiah(inv.Amount), "", 0, "R", false, 0, "")
	totalY += 7

	pdf.SetFillColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
	pdf.RoundedRect(totalX, totalY, totalW, 11, 2, "1234", "F")
	pdf.SetTextColor(colorWhite[0], colorWhite[1], colorWhite[2])
	pdf.SetFont(theme.Family, "B", 9)
	pdf.SetXY(totalX+4, totalY+3)
	pdf.CellFormat(36, 5, "TOTAL TAGIHAN", "", 0, "L", false, 0, "")
	pdf.SetFont(theme.Family, "B", 12)
	pdf.SetXY(totalX+40, totalY+2.5)
	pdf.CellFormat(totalW-44, 6, formatRupiah(inv.Amount), "", 0, "R", false, 0, "")
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	totalY += 11

	sectionLabel(pdf, theme, pML, y+5, "Terbilang")
	pdf.SetXY(pML, y+9.5)
	pdf.SetFont(theme.Family, "", 9)
	pdf.MultiCell(85, 4.5, terbilang.Rupiah(inv.Amount), "", "L", false)
	y = max(totalY, pdf.GetY()) + 8

	y = summaryStrip(pdf, theme, ensureSpace(pdf, theme, y, 16), project.ContractValue, totalPaid) + 8

	y = ensureSpace(pdf, theme, y, 58)
	bankBottom := y
	if profile.BankName != "" || profile.BankAccountNumber != "" {
		pdf.SetFillColor(theme.Palette.AccentSoft[0], theme.Palette.AccentSoft[1], theme.Palette.AccentSoft[2])
		pdf.RoundedRect(pML, y, 100, 26, 2, "1234", "F")
		sectionLabel(pdf, theme, pML+5, y+4, "Pembayaran Ditransfer Ke")
		pdf.SetXY(pML+5, y+9)
		pdf.SetFont(theme.Family, "B", 11)
		pdf.CellFormat(90, 5, profile.BankName+"  "+profile.BankAccountNumber, "", 2, "L", false, 0, "")
		pdf.SetX(pML + 5)
		pdf.SetFont(theme.Family, "", 9)
		pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
		pdf.CellFormat(90, 5, "a.n. "+profile.BankAccountHolderName, "", 2, "L", false, 0, "")
		pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
		bankBottom = y + 26
	}
	sigBottom := signatureBlock(pdf, theme, 130.0, y, 65.0, profile, signature, inv.CreatedAt, "Hormat kami,")
	y = ensureSpace(pdf, theme, max(bankBottom, sigBottom)+8, 12)

	hairline(pdf, pML, pMR, y)
	pdf.SetXY(pML, y+3)
	pdf.SetFont(theme.Family, "", 8)
	pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
	pdf.MultiCell(pCW, 4, "Mohon cantumkan nomor Invoice pada berita transfer. Kwitansi resmi diterbitkan setelah pembayaran diterima.", "", "L", false)
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])

	return pdf, nil
}

// buildClientPaymentReceiptPDF renders a Kwitansi (PLAN.md
// redesain-pdf-invoice-kwitansi-v2 §5.4). invoiceNumber is "" when this
// payment was recorded manually, never through a ClientInvoice. signature is
// nil when the tenant never uploaded one — the signature block still
// reserves its 20mm of vertical space either way, for a wet signature.
// totalPaid mirrors buildClientInvoicePDF's own parameter (K3).
func buildClientPaymentReceiptPDF(project domain.Project, p domain.ClientPayment, invoiceNumber string, profile platformcontracts.TenantProfile, logo, signature []byte, totalPaid int64) (*fpdf.Fpdf, error) {
	pdf, theme := newDocument(profile, logo, "KWITANSI", "Tanda Terima Pembayaran")
	y := pdf.GetY()

	const cardW = 87.0
	leftRows := [][2]string{
		{"Nama Client", project.BrideName + " & " + project.GroomName},
		{"Acara", project.Name},
		{"Tanggal Acara", formatTanggalPDF(project.EventDate)},
	}
	rightRows := [][2]string{
		{"Nomor Kwitansi", p.ReceiptNumber},
		{"Tanggal Pembayaran", formatTanggalPDF(p.PaymentDate)},
		{"Nomor Tagihan", orDash(invoiceNumber)},
	}
	y1 := infoCard(pdf, theme, pML, y, cardW, "Diterima Dari", leftRows, nil)
	y2 := infoCard(pdf, theme, pMR-cardW, y, cardW, "Detail Kwitansi", rightRows, nil)
	y = max(y1, y2) + 8

	pdf.SetFont(theme.Family, "", 9)
	terbilangText := "Terbilang: " + terbilang.Rupiah(p.Amount)
	terbilangLines := pdf.SplitLines([]byte(terbilangText), pCW-16)
	const nomTop, nomH, tbLineHeight = 6.0, 13.0, 4.5
	panelH := nomTop + nomH + float64(len(terbilangLines))*tbLineHeight + 6
	pdf.SetFillColor(theme.Palette.AccentSoft[0], theme.Palette.AccentSoft[1], theme.Palette.AccentSoft[2])
	pdf.RoundedRect(pML, y, pCW, panelH, 3, "1234", "F")
	pdf.SetDrawColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
	pdf.SetLineWidth(1.2)
	pdf.Line(pML+1, y+3, pML+1, y+panelH-3)
	pdf.SetLineWidth(0.2)

	sectionLabel(pdf, theme, pML+8, y+4, "Jumlah Diterima")
	pdf.SetXY(pML+8, y+nomTop+2)
	pdf.SetFont(theme.Family, "B", 22)
	pdf.SetTextColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
	pdf.CellFormat(pCW-16, nomH, formatRupiah(p.Amount), "", 0, "L", false, 0, "")
	pdf.SetXY(pML+8, y+nomTop+nomH+1)
	pdf.SetFont(theme.Family, "", 9)
	pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
	pdf.MultiCell(pCW-16, tbLineHeight, terbilangText, "", "L", false)
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	y += panelH + 8

	sectionLabel(pdf, theme, pML, y, "Untuk Pembayaran")
	pdf.SetXY(pML, y+4.5)
	pdf.SetFont(theme.Family, "", 10.5)
	pdf.MultiCell(pCW, 5, "Pembayaran "+string(p.Type)+" — "+project.Name, "", "L", false)
	y = pdf.GetY() + 4

	rowY := y
	sectionLabel(pdf, theme, pML, rowY, "Metode Pembayaran")
	fieldValue(pdf, theme, pML, rowY+4.5, 85, orDash(p.Method))
	sectionLabel(pdf, theme, 105, rowY, "No. Referensi")
	fieldValue(pdf, theme, 105, rowY+4.5, 85, orDash(p.ReferenceNumber))
	y = rowY + 18

	y = summaryStrip(pdf, theme, ensureSpace(pdf, theme, y, 16), project.ContractValue, totalPaid) + 10

	note := p.Notes
	if note == "" {
		note = "Kwitansi ini diterbitkan sebagai bukti sah penerimaan pembayaran atas acara di atas."
	}
	// Reserved height is the larger of 58mm (signatureBlock's own fixed
	// requirement — place/salutation + 20mm image slot + line + name, drawn
	// alongside Catatan at this same y) and the note's actual wrapped
	// height — p.Notes is an unbounded TEXT column with no length cap
	// anywhere in the stack, so a long note must be measured for real
	// rather than assumed to fit inside a hardcoded reservation.
	pdf.SetFont(theme.Family, "", 9.5)
	noteLines := pdf.SplitLines([]byte(note), 105)
	noteHeight := 4.5 + float64(len(noteLines))*4.5 + 4
	y = ensureSpace(pdf, theme, y, max(58, noteHeight))
	sectionLabel(pdf, theme, pML, y, "Catatan")
	pdf.SetXY(pML, y+4.5)
	pdf.SetFont(theme.Family, "", 9.5)
	pdf.MultiCell(105, 4.5, note, "", "L", false)

	signatureBlock(pdf, theme, 130.0, y, 65.0, profile, signature, p.PaymentDate, "Diterima oleh,")

	return pdf, nil
}

// orDash renders an empty optional field as "-" rather than a blank label
// with nothing under it.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
