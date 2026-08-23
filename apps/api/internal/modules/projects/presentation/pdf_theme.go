package presentation

import (
	"embed"
	"strings"

	"github.com/go-pdf/fpdf"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
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

// moneyLine renders one bold, accent-colored amount at the given size and
// alignment — the kwitansi nominal (§5.5.3, left-aligned inside its tinted
// panel). The invoice total (§5.4.7) mixes two different text styles
// (a 9pt secondary "TOTAL" label plus a differently-sized accent amount) on
// one line, which doesn't fit this single-style primitive, so it's composed
// directly in buildClientInvoicePDF instead.
func moneyLine(pdf *fpdf.Fpdf, theme pdfTheme, x, y, w, size float64, alignStr, amount string) {
	pdf.SetXY(x, y)
	pdf.SetFont(theme.Family, "B", size)
	pdf.SetTextColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
	pdf.CellFormat(w, size/2+2, amount, "", 0, alignStr, false, 0, "")
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
}

// sectionLabelRight is sectionLabel mirrored to align its right edge at
// xRight instead of starting from a left x — used for the "Jumlah" table
// header (§5.4.6), the only section label in either document that isn't
// naturally left-drawn from its column's left edge.
func sectionLabelRight(pdf *fpdf.Fpdf, theme pdfTheme, xRight, y float64, text string) {
	pdf.SetFont(theme.Family, "", 7.5)
	const letterSpacing = 0.3
	runes := []rune(strings.ToUpper(text))
	widths := make([]float64, len(runes))
	total := 0.0
	for i, r := range runes {
		widths[i] = pdf.GetStringWidth(string(r))
		total += widths[i]
		if i < len(runes)-1 {
			total += letterSpacing
		}
	}
	cx := xRight - total
	pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
	for i, r := range runes {
		s := string(r)
		pdf.SetXY(cx, y)
		pdf.CellFormat(widths[i], 4, s, "", 0, "L", false, 0, "")
		cx += widths[i] + letterSpacing
	}
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
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
