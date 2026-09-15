package presentation

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-pdf/fpdf"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
)

// Helper format PDF — salinan dari projects/presentation/invoice_pdf.go.
// Diduplikasi (bukan diimpor lintas modul) mengikuti pola codebase ini:
// presentation helper adalah milik modulnya masing-masing.

// formatRupiah renders an amount as "Rp 2.500.000" — dot thousands separator,
// id-ID convention. A negative amount renders with a leading "-".
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
// types (jpg/png/gif) — degrade gracefully, never trust the caller's claimed
// type.
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

// orDash renders an empty optional field as "-" rather than a blank label
// with nothing under it.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// newDocument builds the kop surat (logo + business identity + document
// title/subtitle) and the shared footer — salinan dari
// projects/presentation/invoice_pdf.go (alasan yang sama dengan helper lain
// di berkas ini).
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
	// block needs a fresh page instead.
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
