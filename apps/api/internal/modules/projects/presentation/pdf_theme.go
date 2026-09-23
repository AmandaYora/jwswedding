package presentation

import (
	"bytes"
	"embed"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
)

// Shared content grid (PLAN.md redesain-pdf-invoice-kwitansi-v2 §5.1) —
// every block in both documents is laid out against this one grid rather
// than a per-block hardcoded width. The §3.1 bug this replaced (nama usaha
// panjang menabrak judul) came from exactly that: two independently
// hardcoded widths (leftW=105, rightX=123) that silently drifted apart.
const (
	pML = 15.0  // margin kiri / kanan konten
	pMR = 195.0 // batas kanan konten
	pCW = 180.0 // lebar konten (pMR - pML)
	// contentBottom is the last y a fixed-height block may start drawing
	// into before ensureSpace forces a new page — leaves room for the 14mm
	// footer below it plus its own gap.
	contentBottom = 277.0
)

//go:embed fonts/Inter-Regular.ttf fonts/Inter-SemiBold.ttf
var fontFS embed.FS

// embeddedFontFamily/coreFontFamily are the two possible values registerFonts
// can return — never mixed within one document. coreFontFamily is fpdf's
// built-in Arial (Latin-1 only, but always available with zero I/O), the
// degrade-to path when the embedded TTFs fail to parse (D11) — same
// "optional asset" contract this package already applies to logos/
// signatures (see fpdfImageType's callers).
const (
	embeddedFontFamily = "Inter"
	coreFontFamily     = "Arial"
)

// pdfPalette is the 3 accent shades a tenant's brand_color_preset resolves
// to (PLAN.md redesain-pdf-invoice-kwitansi §D8) — sourced from
// TenantProfile, which platform/contracts already resolved from the
// preset's hex table, so this package never needs to know what a "preset"
// is or that 21 of them exist.
type pdfPalette struct {
	Accent, AccentDark, AccentSoft [3]int
}

func newPalette(profile platformcontracts.TenantProfile) pdfPalette {
	return pdfPalette{
		Accent: profile.AccentRGB, AccentDark: profile.AccentDarkRGB, AccentSoft: profile.AccentSoftRGB,
	}
}

// Static colors (§5.1) — never vary with the tenant's brand preset.
var (
	colorTextPrimary   = [3]int{15, 23, 42}    // #0f172a
	colorTextSecondary = [3]int{100, 116, 139} // #64748b
	colorBorder        = [3]int{214, 224, 234} // #d6e0ea
	colorWhite         = [3]int{255, 255, 255}

	badgeDraftBg, badgeDraftText   = [3]int{241, 245, 249}, [3]int{100, 116, 139}
	badgeSentBg, badgeSentText     = [3]int{254, 243, 199}, [3]int{146, 64, 14}
	badgePaidBg, badgePaidText     = [3]int{220, 252, 231}, [3]int{22, 101, 52}
	badgeCancelBg, badgeCancelText = [3]int{254, 226, 226}, [3]int{153, 27, 27}
)

// pdfTheme bundles the font family actually in effect (embedded or degraded
// to core — D11) with the tenant's resolved accent palette, so every layout
// primitive below takes one value instead of two.
type pdfTheme struct {
	Family  string
	Palette pdfPalette
}

// registerFonts embeds Inter Regular + SemiBold (PLAN.md
// redesain-pdf-invoice-kwitansi §D6) and returns the font family name to use
// for the rest of this document. Verified this session against fpdf v0.9.0:
// both static TTF instances (not the variable font) parse and render
// successfully via AddUTF8FontFromBytes. Any parse failure degrades to
// coreFontFamily instead of failing the whole PDF (D11) — pdf.Err() is
// cleared so the failure doesn't leak into unrelated later calls.
func registerFonts(pdf *fpdf.Fpdf) string {
	regular, err := fontFS.ReadFile("fonts/Inter-Regular.ttf")
	if err != nil {
		return coreFontFamily
	}
	semibold, err := fontFS.ReadFile("fonts/Inter-SemiBold.ttf")
	if err != nil {
		return coreFontFamily
	}

	pdf.AddUTF8FontFromBytes(embeddedFontFamily, "", regular)
	if pdf.Err() {
		pdf.ClearError()
		return coreFontFamily
	}
	pdf.AddUTF8FontFromBytes(embeddedFontFamily, "B", semibold)
	if pdf.Err() {
		pdf.ClearError()
		return coreFontFamily
	}
	return embeddedFontFamily
}

// hairline draws a thin separator from x1 to x2 at y, in the static border
// color — used for table rules, the footer separator (full width, 18-192),
// and the kwitansi signature underline (a narrower 60mm range).
func hairline(pdf *fpdf.Fpdf, x1, x2, y float64) {
	pdf.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])
	pdf.SetLineWidth(0.2)
	pdf.Line(x1, y, x2, y)
}

// accentRule draws the full-width accent-colored separator below the kop
// surat (§5.4.3) and under the invoice total (§5.4.7) — thicker than
// hairline and colored from the tenant's resolved palette, not the static
// border color.
func accentRule(pdf *fpdf.Fpdf, theme pdfTheme, x1, x2, y float64) {
	pdf.SetDrawColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
	pdf.SetLineWidth(0.6)
	pdf.Line(x1, y, x2, y)
	pdf.SetLineWidth(0.2) // restore the default hairline weight for whatever draws next
}

// sectionLabel renders a small uppercase label with light manual
// letter-spacing (§5.3) — fpdf has no native character-tracking primitive,
// so each rune is drawn individually at x, advancing by its own width plus a
// fixed gap. Leaves the cursor at (x, y) unchanged; the label is drawn as a
// side effect only, matching how CellFormat/MultiCell calls around it
// already manage position explicitly.
func sectionLabel(pdf *fpdf.Fpdf, theme pdfTheme, x, y float64, text string) {
	pdf.SetFont(theme.Family, "", 7.5)
	pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
	const letterSpacing = 0.3
	cx := x
	for _, r := range strings.ToUpper(text) {
		s := string(r)
		w := pdf.GetStringWidth(s)
		pdf.SetXY(cx, y)
		pdf.CellFormat(w, 4, s, "", 0, "L", false, 0, "")
		cx += w + letterSpacing
	}
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
}

// fieldValue renders one line of body text at the given position/width using
// the theme's font family at the standard body size (10.5pt) — the "value"
// half of every label+value pair in §5.4/§5.5.
func fieldValue(pdf *fpdf.Fpdf, theme pdfTheme, x, y, w float64, text string) {
	pdf.SetXY(x, y)
	pdf.SetFont(theme.Family, "", 10.5)
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	pdf.CellFormat(w, 5, text, "", 0, "L", false, 0, "")
}

// statusBadge draws a rounded pill at (x, y) sized to fit label, using the
// fixed 4-state color mapping (§5.4.5) — never the tenant's accent palette,
// so a Lunas/Belum Dibayar/Dibatalkan badge always reads the same regardless
// of brand color. Returns the badge's width so the caller can position
// whatever follows it.
func statusBadge(pdf *fpdf.Fpdf, theme pdfTheme, x, y float64, label string, bg, fg [3]int) (width float64) {
	pdf.SetFont(theme.Family, "B", 8)
	textW := pdf.GetStringWidth(label)
	const padX, height = 3.0, 6.0
	width = textW + 2*padX

	pdf.SetFillColor(bg[0], bg[1], bg[2])
	pdf.RoundedRect(x, y, width, height, 1.5, "1234", "F")

	pdf.SetTextColor(fg[0], fg[1], fg[2])
	pdf.SetXY(x+padX, y+1)
	pdf.CellFormat(textW, height-2, label, "", 0, "L", false, 0, "")
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	return width
}

// fitImage scales (w, h) so the longer side fits within (maxW, maxH) without
// exceeding either bound, preserving aspect ratio exactly — used to place
// both the kop surat logo and the kwitansi signature without ever
// distorting them. A zero-sized source (which RegisterImageOptionsReader
// never actually returns, but a defensive floor all the same) yields
// (0, 0) rather than dividing by zero.
func fitImage(srcW, srcH, maxW, maxH float64) (w, h float64) {
	if srcW <= 0 || srcH <= 0 {
		return 0, 0
	}
	scale := maxW / srcW
	if alt := maxH / srcH; alt < scale {
		scale = alt
	}
	return srcW * scale, srcH * scale
}

// bulanIndonesia indexes 1 (Januari) .. 12 (Desember) — index 0 unused, kept
// so int(time.Month()) can index directly without an off-by-one.
var bulanIndonesia = [...]string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// formatTanggalPDF renders a date the long Indonesian way ("20 Agustus
// 2026") for PDF display (A4/§5.2.7) — never the short "2006-01-02"
// dateLayout the JSON API uses (dto.go), which read as bare ISO dates on the
// printed document (D1).
func formatTanggalPDF(t time.Time) string {
	return strconv.Itoa(t.Day()) + " " + bulanIndonesia[int(t.Month())] + " " + strconv.Itoa(t.Year())
}

// drawAccentBand paints the thin full-bleed accent strip along the very top
// edge of the current page (§5.2.1) — called once per page, both by
// newDocument's initial AddPage and by ensureSpace whenever it starts a new
// one, so every page of a multi-page document carries it.
func drawAccentBand(pdf *fpdf.Fpdf, theme pdfTheme) {
	pdf.SetFillColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
	pdf.Rect(0, 0, 210, 3, "F")
}

// ensureSpace (§5.2.9) is how this layout controls its own pagination now
// that auto page-break is off (A6) — every fixed-height block calls this
// before drawing itself. When the block would cross contentBottom, it starts
// a fresh page (redrawing the accent band) and returns the new page's top
// content y instead of the y the caller passed in. Without this, a long
// invoice description reliably corrupted the rest of the document (§3.3):
// fpdf's own auto page-break moved the cursor mid-MultiCell, but every block
// after it was still positioned by arithmetic off the old y.
func ensureSpace(pdf *fpdf.Fpdf, theme pdfTheme, y, h float64) float64 {
	if y+h <= contentBottom {
		return y
	}
	pdf.AddPage()
	drawAccentBand(pdf, theme)
	return 20.0
}

// badgeSpec bundles a status badge's label with its fixed color pair — the
// shape infoCard needs to draw one inline as part of a card, sourced from
// invoiceStatusBadge's existing 3 return values.
type badgeSpec struct {
	Label  string
	Bg, Fg [3]int
}

// infoCard draws one bordered, titled card containing a stack of
// label/value rows, with an optional status badge appended below the last
// row (§5.2.3) — the shared building block behind Invoice's "Ditagihkan
// Kepada"/"Detail Tagihan" pair and Kwitansi's "Diterima Dari"/"Detail
// Kwitansi" pair. Returns the y just below the card, so the caller can pair
// two cards side by side and continue from max(leftBottom, rightBottom).
func infoCard(pdf *fpdf.Fpdf, theme pdfTheme, x, y, w float64, title string, rows [][2]string, badge *badgeSpec) float64 {
	h := 8.0 + 2.5 + float64(len(rows))*8.0 + 3.5
	if badge != nil {
		h += 8
	}
	pdf.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])
	pdf.SetLineWidth(0.25)
	pdf.SetFillColor(252, 253, 254)
	pdf.RoundedRect(x, y, w, h, 2, "1234", "FD")

	pdf.SetFillColor(theme.Palette.AccentSoft[0], theme.Palette.AccentSoft[1], theme.Palette.AccentSoft[2])
	pdf.RoundedRect(x, y, w, 8, 2, "12", "F")
	sectionLabel(pdf, theme, x+4, y+2, title)

	cy := y + 10.5
	for _, row := range rows {
		pdf.SetXY(x+4, cy)
		pdf.SetFont(theme.Family, "", 7.5)
		pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
		pdf.CellFormat(w-8, 3.5, row[0], "", 2, "L", false, 0, "")
		pdf.SetX(x + 4)
		pdf.SetFont(theme.Family, "", 10)
		pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
		// Each row is a fixed 8mm vertical slot (single line, not MultiCell)
		// — truncateToFit keeps a long value (e.g. a long Acara/project
		// name) from overflowing past this card's own border into
		// whichever card sits beside it, since CellFormat itself neither
		// wraps nor clips.
		pdf.CellFormat(w-8, 4.5, truncateToFit(pdf, row[1], w-8), "", 2, "L", false, 0, "")
		cy += 8
	}
	if badge != nil {
		statusBadge(pdf, theme, x+4, cy, badge.Label, badge.Bg, badge.Fg)
	}
	return y + h
}

// truncateToFit shortens s with a trailing "..." until its rendered width,
// at the pdf's CURRENTLY ACTIVE font/size, fits within maxWidth — the
// caller must SetFont before calling this, since GetStringWidth measures
// against whatever font is active. ASCII "..." rather than a Unicode
// ellipsis glyph, since the degrade-to-Arial path (registerFonts) isn't
// guaranteed to carry one. Returns s unchanged when it already fits.
func truncateToFit(pdf *fpdf.Fpdf, s string, maxWidth float64) string {
	if pdf.GetStringWidth(s) <= maxWidth {
		return s
	}
	const suffix = "..."
	runes := []rune(s)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := string(runes) + suffix
		if pdf.GetStringWidth(candidate) <= maxWidth {
			return candidate
		}
	}
	return suffix
}

// summaryStrip draws the 3-cell Nilai Kontrak / Total Sudah Dibayar / Sisa
// Tagihan strip shared by Invoice (§5.3) and Kwitansi (§5.4) — the ringkasan
// keuangan proyek the user asked to add (K3). When paid exceeds contract,
// the third cell relabels itself "Lebih Bayar" and shows a positive amount
// rather than ever printing a negative Rupiah figure (A5). paid is floored
// at 0 for display — nothing validates a Refund's Amount against a
// project's prior payments (ClientPaymentService.Create/Update), so
// ClientPaymentService.TotalReceived can legitimately return a negative
// total; showing that as a negative "Total Sudah Dibayar" would read as
// nonsensical on a customer-facing document, and the two other cells stay
// arithmetically consistent with each other since both derive from this
// same clamped value. Returns the y just below the strip.
func summaryStrip(pdf *fpdf.Fpdf, theme pdfTheme, y float64, contract, paid int64) float64 {
	if paid < 0 {
		paid = 0
	}
	sisaLabel, sisaValue := "Sisa Tagihan", formatRupiah(contract-paid)
	if contract-paid < 0 {
		sisaLabel, sisaValue = "Lebih Bayar", formatRupiah(paid-contract)
	}
	cells := [3][2]string{
		{"Nilai Kontrak", formatRupiah(contract)},
		{"Total Sudah Dibayar", formatRupiah(paid)},
		{sisaLabel, sisaValue},
	}
	pdf.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])
	pdf.SetLineWidth(0.25)
	pdf.SetFillColor(249, 250, 252)
	pdf.RoundedRect(pML, y, pCW, 16, 2, "1234", "FD")
	for i, c := range cells {
		cx := pML + float64(i)*(pCW/3)
		if i > 0 {
			pdf.Line(cx, y+3, cx, y+13)
		}
		sectionLabel(pdf, theme, cx+5, y+3.5, c[0])
		pdf.SetXY(cx+5, y+8)
		pdf.SetFont(theme.Family, "B", 10.5)
		if i == 2 {
			pdf.SetTextColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
		}
		pdf.CellFormat(pCW/3-10, 5, c[1], "", 0, "L", false, 0, "")
		pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	}
	return y + 16
}

// signatureBlock draws the right-aligned "<Kota>, <tanggal>" / salutation /
// signature image / underline / signer-name stack shared by Invoice ("Hormat
// kami,") and Kwitansi ("Diterima oleh,") — §5.2.5. A fixed 20mm image slot
// is always reserved even when sign is nil (degrade-gracefully, same
// contract fpdfImageType's other callers already follow), so a printed copy
// still has room for a wet signature. Returns the y just below the block.
//
// signerName is the staff member who issued the document, not the business
// owner (PLAN tanda-tangan-pengguna). It is empty when that staff member
// can't be resolved at all — a pre-existing row carrying the 0 sentinel, or a
// hard-deleted account — and the name line is then printed blank (K6).
// Deliberately still printed as an empty cell rather than skipped: the cell
// advances the cursor, so the block keeps exactly the same height either way
// and callers' layout maths (sigBottom) stays valid.
func signatureBlock(pdf *fpdf.Fpdf, theme pdfTheme, x, y, w float64, profile platformcontracts.TenantProfile, sign []byte, date time.Time, salutation, signerName string) float64 {
	place := formatTanggalPDF(date)
	if profile.City != "" {
		place = profile.City + ", " + place
	}
	pdf.SetXY(x, y)
	pdf.SetFont(theme.Family, "", 9.5)
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	pdf.CellFormat(w, 5, place, "", 2, "C", false, 0, "")
	pdf.SetX(x)
	pdf.SetFont(theme.Family, "", 9)
	pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
	pdf.CellFormat(w, 5, salutation, "", 2, "C", false, 0, "")
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])

	const boxTop, boxH = 11.0, 20.0
	boxY := y + 10 + boxTop
	if tp, ok := fpdfImageType(sign); ok {
		info := pdf.RegisterImageOptionsReader("signature", fpdf.ImageOptions{ImageType: tp}, bytes.NewReader(sign))
		if pdf.Err() {
			pdf.ClearError()
		} else if info != nil {
			iw, ih := fitImage(info.Width(), info.Height(), w-10, boxH)
			pdf.ImageOptions("signature", x+(w-iw)/2, boxY+(boxH-ih)/2, iw, ih, false, fpdf.ImageOptions{ImageType: tp}, 0, "")
		}
	}
	lineY := boxY + boxH + 1
	hairline(pdf, x+6, x+w-6, lineY)
	pdf.SetXY(x, lineY+1.5)
	pdf.SetFont(theme.Family, "B", 10)
	pdf.CellFormat(w, 5, signerName, "", 2, "C", false, 0, "")
	return pdf.GetY()
}
