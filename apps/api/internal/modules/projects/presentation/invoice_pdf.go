package presentation

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-pdf/fpdf"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/terbilang"
)

// formatRupiah renders a non-negative amount as "Rp 2.500.000" — dot
// thousands separator, id-ID convention (mirrors apps/web's
// formatCurrency), no shared Go equivalent existed in this codebase yet.
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
// Also used for the Kwitansi signature image (PLAN.md
// redesain-pdf-invoice-kwitansi §D4/§D10), even though UploadSignature only
// ever stores PNG — sniffing again here costs nothing and keeps this
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

// newDocument builds the kop surat, accent rule, and footer shared by both
// Invoice and Kwitansi (PLAN.md redesain-pdf-invoice-kwitansi §5.4.1-3,
// §5.4.9, §5.5) — rightLines is the doc-specific block printed right under
// the title (just the invoice number for Invoice; receipt number + payment
// date for Kwitansi). Returns the theme alongside *fpdf.Fpdf so callers never
// need to re-derive the font family or re-resolve the accent palette.
// Leaves the cursor at (18, y) just below the accent rule, ready for the
// caller's own body content.
func newDocument(profile platformcontracts.TenantProfile, logo []byte, title string, rightLines []string) (*fpdf.Fpdf, pdfTheme) {
	pdf := fpdf.New("P", "mm", "A4", "")
	family := registerFonts(pdf)
	theme := pdfTheme{Family: family, Palette: newPalette(profile)}

	businessName := profile.BusinessName
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		hairline(pdf, 18, 192, pdf.GetY())
		pdf.SetXY(18, pdf.GetY()+2)
		pdf.SetFont(theme.Family, "", 8)
		pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
		pdf.CellFormat(90, 5, businessName, "", 0, "L", false, 0, "")
		pdf.CellFormat(84, 5, fmt.Sprintf("Halaman %d dari {nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
		pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	})
	pdf.AliasNbPages("")
	pdf.SetMargins(18, 16, 18)
	pdf.AddPage()

	textX := 18.0
	logoBottom := 14.0
	if tp, ok := fpdfImageType(logo); ok {
		info := pdf.RegisterImageOptionsReader("logo", fpdf.ImageOptions{ImageType: tp}, bytes.NewReader(logo))
		if pdf.Err() {
			// Sniffed as jpg/png/gif but fpdf still couldn't decode it (e.g.
			// truncated/corrupt bytes) — degrade to no logo rather than
			// failing the whole PDF, same "logo is optional" contract as a
			// tenant with no logo at all.
			pdf.ClearError()
		} else if info != nil {
			w, h := fitImage(info.Width(), info.Height(), 20, 16)
			pdf.ImageOptions("logo", 18, 14, w, h, false, fpdf.ImageOptions{ImageType: tp}, 0, "")
			textX = 42
			logoBottom = 14 + h
		}
	}

	const leftW = 105.0
	pdf.SetXY(textX, 14)
	pdf.SetFont(theme.Family, "B", 12)
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	pdf.MultiCell(leftW, 5.5, businessName, "", "L", false)
	leftBottom := pdf.GetY()

	if profile.Address != "" {
		pdf.SetXY(textX, leftBottom)
		pdf.SetFont(theme.Family, "", 9)
		pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
		pdf.MultiCell(leftW, 4.5, profile.Address, "", "L", false)
		leftBottom = pdf.GetY()
	}

	contact := profile.Phone
	if profile.Email != "" {
		if contact != "" {
			contact += " · "
		}
		contact += profile.Email
	}
	if contact != "" {
		pdf.SetXY(textX, leftBottom)
		pdf.SetFont(theme.Family, "", 9)
		pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
		pdf.MultiCell(leftW, 4.5, contact, "", "L", false)
		leftBottom = pdf.GetY()
	}
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])

	const rightX, rightW = 123.0, 69.0
	pdf.SetXY(rightX, 14)
	pdf.SetFont(theme.Family, "B", 20)
	pdf.SetTextColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
	pdf.CellFormat(rightW, 8, title, "", 2, "R", false, 0, "")
	pdf.SetX(rightX)
	rightBottom := pdf.GetY()

	pdf.SetFont(theme.Family, "", 9)
	pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
	for _, line := range rightLines {
		pdf.SetX(rightX)
		pdf.CellFormat(rightW, 5, line, "", 2, "R", false, 0, "")
		pdf.SetX(rightX)
		rightBottom = pdf.GetY()
	}
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])

	ruleY := leftBottom
	if rightBottom > ruleY {
		ruleY = rightBottom
	}
	if logoBottom > ruleY {
		ruleY = logoBottom
	}
	ruleY += 4
	accentRule(pdf, theme, 18, 192, ruleY)
	pdf.SetXY(18, ruleY+6)

	return pdf, theme
}

// invoiceStatusBadge maps an InvoiceStatus to its badge label and 4-state
// color pair (§5.4.5) — Terkirim reads as "BELUM DIBAYAR" on the badge since
// that's the status's actual meaning to whoever reads the printed document,
// even though the domain enum's own label is "Terkirim".
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

// field prints one section-label + value pair at (x, y) and returns the y
// coordinate for whatever comes next in the same column — the repeated
// "label above value" unit both the left (client/event) and right
// (tanggal terbit/jatuh tempo) info blocks in §5.4.4 are built from.
func field(pdf *fpdf.Fpdf, theme pdfTheme, x, y, w float64, label, value string) float64 {
	sectionLabel(pdf, theme, x, y, label)
	fieldValue(pdf, theme, x, y+4, w, value)
	return y + 4 + 5.5
}

// buildClientInvoicePDF renders an Invoice/Tagihan — PLAN.md
// redesain-pdf-invoice-kwitansi §5.4. Not itemized (a single
// Type+Description+Amount line, an earlier decision reaffirmed at that
// PLAN.md's §D2 — "no line items").
func buildClientInvoicePDF(project domain.Project, inv domain.ClientInvoice, profile platformcontracts.TenantProfile, logo []byte) (*fpdf.Fpdf, error) {
	pdf, theme := newDocument(profile, logo, "INVOICE / TAGIHAN", []string{inv.InvoiceNumber})

	const colL, colR, colW = 18.0, 111.0, 81.0
	y := pdf.GetY()

	leftY := field(pdf, theme, colL, y, colW, "Kepada Yth.", project.BrideName+" & "+project.GroomName)
	leftY = field(pdf, theme, colL, leftY, colW, "Acara", project.Name)
	leftY = field(pdf, theme, colL, leftY, colW, "Tanggal Acara", project.EventDate.Format(dateLayout))
	leftY = field(pdf, theme, colL, leftY, colW, "Paket", project.PackageName)

	rightY := field(pdf, theme, colR, y, colW, "Tanggal Terbit", inv.CreatedAt.Format(dateLayout))
	rightY = field(pdf, theme, colR, rightY, colW, "Jatuh Tempo", inv.DueDate.Format(dateLayout))

	badgeLabel, badgeBg, badgeFg := invoiceStatusBadge(inv.Status)
	statusBadge(pdf, theme, colR, rightY, badgeLabel, badgeBg, badgeFg)
	rightY += 6 + 5.5

	tableY := leftY
	if rightY > tableY {
		tableY = rightY
	}
	tableY += 2

	// Table columns: Jenis (18-48), Keterangan (48-142), Jumlah (142-192).
	sectionLabel(pdf, theme, 18, tableY, "Jenis")
	sectionLabel(pdf, theme, 48, tableY, "Keterangan")
	sectionLabelRight(pdf, theme, 192, tableY, "Jumlah")
	hairline(pdf, 18, 192, tableY+5)

	dataY := tableY + 7
	pdf.SetFont(theme.Family, "", 10.5)
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])

	pdf.SetXY(18, dataY)
	pdf.CellFormat(28, 5, string(inv.Type), "", 0, "L", false, 0, "")

	pdf.SetXY(48, dataY)
	pdf.MultiCell(90, 5, inv.Description, "", "L", false)
	descBottom := pdf.GetY()

	pdf.SetXY(142, dataY)
	pdf.CellFormat(50, 5, formatRupiah(inv.Amount), "", 0, "R", false, 0, "")

	rowBottom := dataY + 5
	if descBottom > rowBottom {
		rowBottom = descBottom
	}
	hairline(pdf, 18, 192, rowBottom+2)

	totalY := rowBottom + 6
	pdf.SetFont(theme.Family, "", 9)
	pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
	pdf.SetXY(18, totalY)
	pdf.CellFormat(124, 6, "TOTAL", "", 0, "R", false, 0, "")
	pdf.SetFont(theme.Family, "B", 12)
	pdf.SetTextColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
	pdf.SetXY(142, totalY)
	pdf.CellFormat(50, 6, formatRupiah(inv.Amount), "", 0, "R", false, 0, "")
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	accentRule(pdf, theme, 142, 192, totalY+7)

	bottomY := totalY + 12
	if profile.BankName != "" || profile.BankAccountNumber != "" {
		pdf.SetXY(18, bottomY)
		sectionLabel(pdf, theme, 18, bottomY, "Pembayaran Ditransfer Ke")
		line := profile.BankName
		if profile.BankAccountNumber != "" {
			if line != "" {
				line += " · "
			}
			line += profile.BankAccountNumber
		}
		if profile.BankAccountHolderName != "" {
			if line != "" {
				line += " · a.n. "
			}
			line += profile.BankAccountHolderName
		}
		fieldValue(pdf, theme, 18, bottomY+4, 174, line)
	}

	return pdf, nil
}

// buildClientPaymentReceiptPDF renders a Kwitansi — PLAN.md
// redesain-pdf-invoice-kwitansi §5.5. invoiceNumber is "" when this payment
// was recorded manually, never through a ClientInvoice. signature is nil
// when the tenant never uploaded one (PLAN.md §D4/§D7) — the signature
// block still reserves its 22mm of vertical space either way, for a wet
// signature.
func buildClientPaymentReceiptPDF(project domain.Project, p domain.ClientPayment, invoiceNumber string, profile platformcontracts.TenantProfile, logo, signature []byte) (*fpdf.Fpdf, error) {
	pdf, theme := newDocument(profile, logo, "KWITANSI", []string{p.ReceiptNumber, p.PaymentDate.Format(dateLayout)})

	y := pdf.GetY()
	y = field(pdf, theme, 18, y, 174, "Telah Terima Dari", project.BrideName+" & "+project.GroomName)
	y += 2

	pdf.SetFillColor(theme.Palette.AccentSoft[0], theme.Palette.AccentSoft[1], theme.Palette.AccentSoft[2])
	pdf.SetFont(theme.Family, "", 9)
	terbilangLines := pdf.SplitLines([]byte("Terbilang: "+terbilang.Rupiah(p.Amount)), 166)
	panelH := 12.0 + float64(len(terbilangLines))*4.5
	pdf.RoundedRect(18, y, 174, panelH, 2, "1234", "F")

	moneyLine(pdf, theme, 22, y+3, 166, 20, "L", formatRupiah(p.Amount))

	pdf.SetXY(22, y+13)
	pdf.SetFont(theme.Family, "", 9)
	pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
	pdf.MultiCell(166, 4.5, "Terbilang: "+terbilang.Rupiah(p.Amount), "", "L", false)
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])

	y += panelH + 6

	untuk := "Pembayaran " + string(p.Type)
	if invoiceNumber != "" {
		untuk += " (Tagihan " + invoiceNumber + ")"
	}
	if p.Notes != "" {
		untuk += " — " + p.Notes
	}
	sectionLabel(pdf, theme, 18, y, "Untuk Pembayaran")
	pdf.SetXY(18, y+4)
	pdf.SetFont(theme.Family, "", 10.5)
	pdf.MultiCell(174, 5, untuk, "", "L", false)
	y = pdf.GetY() + 2

	rowY := y
	y = field(pdf, theme, 18, rowY, 81, "Metode", orDash(p.Method))
	field(pdf, theme, 111, rowY, 81, "No. Referensi", orDash(p.ReferenceNumber))

	// Signature block, right-aligned, 60mm wide (§5.5.6).
	const sigX, sigW = 132.0, 60.0
	sigY := y + 6
	if profile.City != "" {
		fieldValue(pdf, theme, sigX, sigY, sigW, profile.City+", "+p.PaymentDate.Format(dateLayout))
	} else {
		fieldValue(pdf, theme, sigX, sigY, sigW, p.PaymentDate.Format(dateLayout))
	}
	pdf.SetXY(sigX, sigY+6)
	pdf.SetFont(theme.Family, "", 9)
	pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
	pdf.CellFormat(sigW, 5, "Diterima oleh,", "", 0, "L", false, 0, "")
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])

	const sigBoxTop = 12.0 // gap below "Diterima oleh," before the signature image/space
	const sigBoxH = 22.0
	if tp, ok := fpdfImageType(signature); ok {
		info := pdf.RegisterImageOptionsReader("signature", fpdf.ImageOptions{ImageType: tp}, bytes.NewReader(signature))
		if pdf.Err() {
			pdf.ClearError()
		} else if info != nil {
			w, h := fitImage(info.Width(), info.Height(), sigW, sigBoxH)
			pdf.ImageOptions("signature", sigX+(sigW-w)/2, sigY+6+sigBoxTop+(sigBoxH-h)/2, w, h, false, fpdf.ImageOptions{ImageType: tp}, 0, "")
		}
	}

	lineY := sigY + 6 + sigBoxTop + sigBoxH
	hairline(pdf, sigX, sigX+sigW, lineY)
	pdf.SetXY(sigX, lineY+1.5)
	pdf.SetFont(theme.Family, "", 10.5)
	pdf.CellFormat(sigW, 5, profile.OwnerName, "", 0, "L", false, 0, "")

	return pdf, nil
}

// orDash renders an empty optional field as "-" rather than a blank label
// with nothing under it (§5.6).
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
