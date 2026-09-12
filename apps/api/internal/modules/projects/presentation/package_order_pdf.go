package presentation

import (
	"bytes"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-pdf/fpdf"

	platformcontracts "jwswedding/internal/modules/platform/contracts"
	"jwswedding/internal/modules/projects/application"
	"jwswedding/internal/modules/projects/domain"
)

// Column grid for the composition table (blok B2). Sums to pCW (180mm) —
// derived from the source document's own proportions: the item list needs
// roughly half the width, the two narrow cells carry short phrases.
const (
	poColCategory = 32.0
	poColBody     = 84.0
	poColQty      = 26.0
	poColBonus    = 38.0
	poLineH       = 4.4
	poCellPadX    = 2.5
	poCellPadY    = 2.5
)

// formatRupiahSigned renders an adjustment the way the source document does:
// "-Rp 1.500.000" for a takeout, "Rp 9.900.000" for an addition. formatRupiah
// already emits the minus for a negative amount, so this only has to force a
// value through unchanged — it exists as its own name so the sign-carrying
// intent is explicit at the call site, and so formatRupiah itself (used by
// Invoice and Kwitansi, where amounts are never negative) stays untouched.
func formatRupiahSigned(amount int64) string {
	return formatRupiah(amount)
}

// packageOrderPrintData is the normalized shape the renderer draws from,
// whichever source it came from.
//
// This indirection is the whole of D27/D6: a Draft PO renders from the live
// tables (so it can be reprinted freely mid-negotiation), while an issued one
// renders from its frozen snapshot (so a signed contract cannot drift when
// master data moves). The renderer below must never know which it got.
type packageOrderPrintData struct {
	PONumber    string
	Revision    int
	Status      domain.PackageOrderStatus
	BasePrice   int64
	TermsText   string
	BonusNote   string
	Blocks      []domain.ProjectPackageBlock
	Adjustments []domain.ProjectPackageAdjustment
	TermsPlan   []domain.TermPlanEntry
	Event       domain.PackageOrderEventSnapshot
	IssuedAt    time.Time
	// Payments and TotalPaid are ALWAYS live, even for an issued PO — the
	// source document proves this is the intent: it was signed in October 2025
	// and still lists payments received through September 2026.
	Payments  []domain.ClientPayment
	TotalPaid int64
}

func (d packageOrderPrintData) total() int64 {
	return d.BasePrice + domain.TotalAdjustments(d.Adjustments)
}

// title renders "PURCHASE ORDER" plus the revision marker. Revision 0 is the
// original and carries no marker at all.
func (d packageOrderPrintData) subtitle() string {
	if d.Revision > 0 {
		return "Kontrak Paket · Revisi " + strconv.Itoa(d.Revision)
	}
	return "Kontrak Paket"
}

// buildPackageOrderPrintData chooses between the live tables and the frozen
// snapshot, and folds in the always-live payment ledger.
func buildPackageOrderPrintData(
	view *application.PackageOrderView,
	project domain.Project,
	clientName, phone string,
	payments []domain.ClientPayment,
	totalPaid int64,
) packageOrderPrintData {
	o := view.Order
	data := packageOrderPrintData{
		PONumber: o.PONumber, Revision: o.Revision, Status: o.Status,
		Payments: payments, TotalPaid: totalPaid,
	}
	if o.IssuedAt != nil {
		data.IssuedAt = *o.IssuedAt
	} else {
		data.IssuedAt = time.Now()
	}

	if o.Status != domain.PackageOrderDraft && o.Snapshot != nil {
		s := o.Snapshot.Current
		data.BasePrice, data.TermsText, data.BonusNote = s.BasePrice, s.TermsText, s.BonusNote
		data.Blocks, data.Adjustments, data.TermsPlan = s.Blocks, s.Adjustments, s.TermsPlan
		data.Event = s.Event
		return data
	}

	data.BasePrice, data.TermsText, data.BonusNote = o.BasePrice, o.TermsText, o.BonusNote
	data.Blocks, data.Adjustments, data.TermsPlan = view.Blocks, view.Adjustments, o.TermsPlan
	data.Event = domain.PackageOrderEventSnapshot{
		ClientName: clientName, Phone: phone,
		EventDate:  project.EventDate.Format(dateLayout),
		EventStart: derefStringPDF(project.EventStartTime),
		EventEnd:   derefStringPDF(project.EventEndTime),
		Venue:      project.Venue, Pax: project.Pax,
	}
	return data
}

// isHeadingLine reports whether a body line should render bold as a
// sub-heading (D21). The rule is "the line is entirely upper-case", which is
// how the source spreadsheet already writes BUFFET / DESSERT / MINUMAN /
// STALL - GUBUKAN, so nobody has to learn a markup syntax to reproduce it.
//
// A line with no letters at all (a bare number, a separator) is never a
// heading — otherwise "150" would come out bold.
func isHeadingLine(line string) bool {
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

// wrapCell splits a cell's text into rendered lines: first on its own
// newlines (the author's deliberate line breaks), then wrapping each of those
// to the column width. Blank lines are preserved — they are spacing the
// author typed on purpose.
func wrapCell(pdf *fpdf.Fpdf, text string, width float64) []string {
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

func drawPOTableHeader(pdf *fpdf.Fpdf, theme pdfTheme, y float64) float64 {
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
		{poColCategory, "KATEGORI"},
		{poColBody, "PRODUK"},
		{poColQty, "QTY"},
		{poColBonus, "BONUS"},
	} {
		pdf.SetXY(x+poCellPadX, y+2)
		pdf.CellFormat(col.w-poCellPadX*2, 4, col.label, "", 0, "L", false, 0, "")
		x += col.w
	}
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	return y + h
}

// drawCompositionTable renders blok B2/B3.
//
// A block taller than one page is split across pages rather than clipped
// (PLAN.md B19) — the CATERING block in the source document is already close
// to half a page, so this is a real case, not a defensive one. The three
// columns are consumed as parallel streams: whatever fits of each goes on
// this page and the rest continues on the next, so no cell's content is ever
// silently dropped.
func drawCompositionTable(pdf *fpdf.Fpdf, theme pdfTheme, y float64, blocks []domain.ProjectPackageBlock) float64 {
	if len(blocks) == 0 {
		return y
	}
	y = ensureSpace(pdf, theme, y, 20)
	y = drawPOTableHeader(pdf, theme, y)

	// Consecutive blocks sharing a category render as ONE merged cell, the way
	// the source document prints CATERING spanning its two rows: no horizontal
	// rule crosses the category column between them, and the label sits
	// vertically centred in the span. Drawn after the rows rather than before,
	// because the span's height is only known once they are laid out.
	for _, span := range categorySpans(blocks) {
		i, j := span[0], span[1]
		category := blocks[i].Category
		groupTop := y

		for k := i; k <= j; k++ {
			block := blocks[k]
			pdf.SetFont(theme.Family, "", 8.5)
			body := wrapCell(pdf, block.Body, poColBody-poCellPadX*2)
			qty := wrapCell(pdf, block.QtyText, poColQty-poCellPadX*2)
			bonus := wrapCell(pdf, block.BonusNote, poColBonus-poCellPadX*2)

			for first := true; ; first = false {
				remaining := maxInt(len(body), maxInt(len(qty), len(bonus)))
				if remaining == 0 && !first {
					break
				}
				avail := contentBottom - y - poCellPadY*2
				fit := int(avail / poLineH)
				// Fewer than two lines left on the page is not worth a
				// fragment — close the merged cell for what is already on this
				// page, then continue on a fresh one.
				if fit < 2 {
					drawPOCategoryCell(pdf, theme, category, groupTop, y)
					pdf.AddPage()
					drawAccentBand(pdf, theme)
					y = drawPOTableHeader(pdf, theme, 20.0)
					groupTop = y
					continue
				}
				take := remaining
				if take > fit {
					take = fit
				}
				if take < 1 {
					take = 1 // an empty block still gets one padded row
				}

				lastFragment := remaining <= take
				rowH := float64(take)*poLineH + poCellPadY*2
				drawPOBlockFragment(pdf, theme, y, rowH, lastFragment && k < j,
					takeLines(&body, take), takeLines(&qty, take), takeLines(&bonus, take))
				y += rowH
				if lastFragment {
					break
				}
			}
		}
		drawPOCategoryCell(pdf, theme, category, groupTop, y)
	}
	return y
}

// categorySpans groups CONSECUTIVE blocks sharing a category into inclusive
// [start, end] index pairs — the spans that render as one merged cell.
//
// Consecutive only, deliberately: the order the WO arranged is the order that
// prints, so two CATERING blocks separated by a DEKORASI block stay separate
// cells rather than being silently reordered to sit together.
func categorySpans(blocks []domain.ProjectPackageBlock) [][2]int {
	var spans [][2]int
	for i := 0; i < len(blocks); {
		j := i
		for j+1 < len(blocks) && blocks[j+1].Category == blocks[i].Category {
			j++
		}
		spans = append(spans, [2]int{i, j})
		i = j + 1
	}
	return spans
}

// drawPOCategoryCell writes the merged category label centred in the vertical
// span its blocks occupy on the current page.
func drawPOCategoryCell(pdf *fpdf.Fpdf, theme pdfTheme, label string, top, bottom float64) {
	if label == "" || bottom <= top {
		return
	}
	pdf.SetFont(theme.Family, "B", 8)
	lines := pdf.SplitLines([]byte(label), poColCategory-poCellPadX*2)
	ty := top + (bottom-top-float64(len(lines))*poLineH)/2
	if ty < top+poCellPadY {
		ty = top + poCellPadY
	}
	for _, line := range lines {
		pdf.SetXY(pML+poCellPadX, ty)
		pdf.CellFormat(poColCategory-poCellPadX*2, poLineH, string(line), "", 0, "L", false, 0, "")
		ty += poLineH
	}
}

// drawPOBlockFragment paints one fragment's borders and its three text
// columns. Column separators are drawn per fragment so a split block still
// reads as a table on the continuation page.
//
// mergeBottom omits the closing rule across the category column, which is what
// visually merges this block with the next one in the same category. The
// category text itself is written separately by drawPOCategoryCell.
func drawPOBlockFragment(pdf *fpdf.Fpdf, theme pdfTheme, y, h float64, mergeBottom bool, body, qty, bonus []string) {
	pdf.SetFillColor(255, 255, 255)
	pdf.Rect(pML, y, pCW, h, "F")
	pdf.SetDrawColor(colorBorder[0], colorBorder[1], colorBorder[2])
	pdf.SetLineWidth(0.2)
	// Verticals run the full fragment height on both sides and at each column
	// boundary; consecutive fragments join into one continuous rule.
	x := pML
	pdf.Line(x, y, x, y+h)
	for _, w := range []float64{poColCategory, poColBody, poColQty, poColBonus} {
		x += w
		pdf.Line(x, y, x, y+h)
	}
	if mergeBottom {
		pdf.Line(pML+poColCategory, y+h, pMR, y+h)
	} else {
		pdf.Line(pML, y+h, pMR, y+h)
	}

	// Body lines carry the ALL-CAPS bold rule (D21); the other two columns are
	// short phrases and stay regular throughout.
	ty := y + poCellPadY
	for _, line := range body {
		style := ""
		if isHeadingLine(line) {
			style = "B"
		}
		pdf.SetFont(theme.Family, style, 8.5)
		pdf.SetXY(pML+poColCategory+poCellPadX, ty)
		pdf.CellFormat(poColBody-poCellPadX*2, poLineH, line, "", 0, "L", false, 0, "")
		ty += poLineH
	}

	pdf.SetFont(theme.Family, "", 8.5)
	drawPlainColumn(pdf, pML+poColCategory+poColBody+poCellPadX, y+poCellPadY, poColQty-poCellPadX*2, qty)
	drawPlainColumn(pdf, pML+poColCategory+poColBody+poColQty+poCellPadX, y+poCellPadY, poColBonus-poCellPadX*2, bonus)
}

func drawPlainColumn(pdf *fpdf.Fpdf, x, y, w float64, lines []string) {
	for _, line := range lines {
		pdf.SetXY(x, y)
		pdf.CellFormat(w, poLineH, line, "", 0, "L", false, 0, "")
		y += poLineH
	}
}

// takeLines pops up to n entries off the front of a slice, shrinking it in
// place — the stream consumption drawCompositionTable's split relies on.
func takeLines(lines *[]string, n int) []string {
	if n > len(*lines) {
		n = len(*lines)
	}
	head := (*lines)[:n]
	*lines = (*lines)[n:]
	return head
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// sectionReserve is the height a titled section must have available before
// its label may be drawn: the gap, the label itself, and its first few rows.
// Every section below uses it so no heading can ever be stranded alone at the
// bottom of a page while its content starts on the next one.
func sectionReserve(rows int) float64 {
	const gap, label, rowH = 6.0, 5.0, 7.0
	if rows > 3 {
		rows = 3
	}
	if rows < 1 {
		rows = 1
	}
	return gap + label + float64(rows)*rowH
}

// drawAdjustments renders blok B4 — the only priced part of the composition.
func drawAdjustments(pdf *fpdf.Fpdf, theme pdfTheme, y float64, adjustments []domain.ProjectPackageAdjustment) float64 {
	if len(adjustments) == 0 {
		return y
	}
	y = ensureSpace(pdf, theme, y, sectionReserve(len(adjustments))) + 6
	sectionLabel(pdf, theme, pML, y, "Additional & Takeout")
	y += 5

	for _, a := range adjustments {
		y = ensureSpace(pdf, theme, y, 7)
		pdf.SetFont(theme.Family, "", 9)
		pdf.SetXY(pML+2, y)
		pdf.CellFormat(pCW-52, 5.5, truncateToFit(pdf, a.Description, pCW-54), "", 0, "L", false, 0, "")
		pdf.SetFont(theme.Family, "B", 9)
		// A takeout is printed in the "cancelled" red so a reduction reads as
		// a reduction at a glance, not as another charge.
		if a.Amount < 0 {
			pdf.SetTextColor(badgeCancelText[0], badgeCancelText[1], badgeCancelText[2])
		}
		pdf.SetXY(pMR-50, y)
		pdf.CellFormat(50, 5.5, formatRupiahSigned(a.Amount), "", 0, "R", false, 0, "")
		pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
		y += 5.5
		hairline(pdf, pML, pMR, y)
		y += 1.5
	}
	return y
}

// drawScheduleBlock renders blok B6a — "TAHAP PEMBAYARAN".
//
// D27: while the PO is Draft no invoice exists yet, so the schedule is
// computed from the frozen TermsPlan. Sourcing it from invoices alone would
// leave the negotiation copy — the one actually handed to the client — with
// no payment plan on it at all.
func drawScheduleBlock(pdf *fpdf.Fpdf, theme pdfTheme, y float64, data packageOrderPrintData, eventDate time.Time) float64 {
	if len(data.TermsPlan) == 0 {
		return y
	}
	y = ensureSpace(pdf, theme, y, sectionReserve(len(data.TermsPlan))) + 6
	sectionLabel(pdf, theme, pML, y, "Tahap Pembayaran (Rencana)")
	y += 5

	amounts := application.ComputeScheduleAmounts(data.total(), data.TermsPlan)
	for i, entry := range data.TermsPlan {
		y = ensureSpace(pdf, theme, y, 7)
		due := eventDate.AddDate(0, 0, -entry.DaysBeforeEvent)
		label := entry.Label
		if label == "" {
			label = string(entry.Type)
		}
		pdf.SetFont(theme.Family, "", 9)
		pdf.SetXY(pML+2, y)
		pdf.CellFormat(pCW-52, 5.5, truncateToFit(pdf, label+"  ·  jatuh tempo "+formatTanggalPDF(due), pCW-54), "", 0, "L", false, 0, "")
		pdf.SetFont(theme.Family, "B", 9)
		pdf.SetXY(pMR-50, y)
		pdf.CellFormat(50, 5.5, formatRupiah(amounts[i]), "", 0, "R", false, 0, "")
		y += 5.5
		hairline(pdf, pML, pMR, y)
		y += 1.5
	}
	return y
}

// drawLedgerAndTotals renders blok B6b.
//
// Each ledger line carries the payment's `method` — that is the "BCA PT" the
// source document prints beside every instalment, and it was already stored
// (client_payments.method) long before this feature existed.
func drawLedgerAndTotals(pdf *fpdf.Fpdf, theme pdfTheme, y float64, data packageOrderPrintData) float64 {
	total := data.total()
	y = ensureSpace(pdf, theme, y, 40) + 6

	rows := [][2]string{
		{"HARGA PAKET AWAL", formatRupiah(data.BasePrice)},
		{"ADDITIONAL & TAKEOUT", formatRupiahSigned(domain.TotalAdjustments(data.Adjustments))},
	}
	for _, row := range rows {
		pdf.SetFont(theme.Family, "", 9)
		pdf.SetXY(pML+2, y)
		pdf.CellFormat(pCW-52, 5.5, row[0], "", 0, "L", false, 0, "")
		pdf.SetXY(pMR-50, y)
		pdf.CellFormat(50, 5.5, row[1], "", 0, "R", false, 0, "")
		y += 5.5
		hairline(pdf, pML, pMR, y)
		y += 1.5
	}

	pdf.SetFillColor(theme.Palette.Accent[0], theme.Palette.Accent[1], theme.Palette.Accent[2])
	pdf.RoundedRect(pML, y, pCW, 11, 2, "1234", "F")
	pdf.SetTextColor(colorWhite[0], colorWhite[1], colorWhite[2])
	pdf.SetFont(theme.Family, "B", 9.5)
	pdf.SetXY(pML+4, y+3)
	pdf.CellFormat(80, 5, "TOTAL PEMBAYARAN", "", 0, "L", false, 0, "")
	pdf.SetFont(theme.Family, "B", 12)
	pdf.SetXY(pMR-84, y+2.5)
	pdf.CellFormat(80, 6, formatRupiah(total), "", 0, "R", false, 0, "")
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	y += 15

	if len(data.Payments) > 0 {
		y = ensureSpace(pdf, theme, y, sectionReserve(len(data.Payments)))
		sectionLabel(pdf, theme, pML, y, "Pembayaran Diterima")
		y += 5
		for _, p := range data.Payments {
			y = ensureSpace(pdf, theme, y, 7)
			label := string(p.Type) + " (" + p.PaymentDate.Format("02-01-2006") + ")"
			if p.Method != "" {
				label += "  " + p.Method
			}
			pdf.SetFont(theme.Family, "", 9)
			pdf.SetXY(pML+2, y)
			pdf.CellFormat(pCW-52, 5.5, truncateToFit(pdf, label, pCW-54), "", 0, "L", false, 0, "")
			pdf.SetXY(pMR-50, y)
			pdf.CellFormat(50, 5.5, formatRupiah(p.Amount), "", 0, "R", false, 0, "")
			y += 5.5
			hairline(pdf, pML, pMR, y)
			y += 1.5
		}
	}

	remaining := total - data.TotalPaid
	y = ensureSpace(pdf, theme, y, 14) + 2
	pdf.SetFillColor(255, 237, 213) // amber-100, the source document's own "SISA PEMBAYARAN" highlight
	pdf.RoundedRect(pML, y, pCW, 11, 2, "1234", "F")
	pdf.SetFont(theme.Family, "B", 9.5)
	pdf.SetXY(pML+4, y+3)
	pdf.CellFormat(80, 5, "SISA PEMBAYARAN", "", 0, "L", false, 0, "")
	pdf.SetFont(theme.Family, "B", 12)
	pdf.SetXY(pMR-84, y+2.5)
	pdf.CellFormat(80, 6, formatRupiah(remaining), "", 0, "R", false, 0, "")
	return y + 15
}

// drawTermsAndBonus renders blok B5.
func drawTermsAndBonus(pdf *fpdf.Fpdf, theme pdfTheme, y float64, data packageOrderPrintData) float64 {
	for _, section := range []struct{ title, text string }{
		{"Syarat & Ketentuan", data.TermsText},
		{"Bonus Tambahan", data.BonusNote},
	} {
		if strings.TrimSpace(section.text) == "" {
			continue
		}
		pdf.SetFont(theme.Family, "", 8.5)
		lines := wrapCell(pdf, section.text, pCW-4)
		// Reserve the label AND its first lines together. Reserving only the
		// label's own height left "SYARAT & KETENTUAN" stranded at the foot of
		// page 1 with its text starting on page 2 — the same orphaned-header
		// class of bug PLAN redesain-pdf-invoice-kwitansi-v2 §3.3 fixed for the
		// invoice table, caught here by visual inspection of a rendered page.
		y = ensureSpace(pdf, theme, y, sectionReserve(len(lines))) + 6
		sectionLabel(pdf, theme, pML, y, section.title)
		y += 5
		for _, line := range lines {
			y = ensureSpace(pdf, theme, y, poLineH)
			style := ""
			if isHeadingLine(line) {
				style = "B"
			}
			pdf.SetFont(theme.Family, style, 8.5)
			pdf.SetXY(pML+2, y)
			pdf.CellFormat(pCW-4, poLineH, line, "", 0, "L", false, 0, "")
			y += poLineH
		}
	}
	return y
}

// dualSignatureBlock renders blok B7: the client on the left, the WO on the
// right, both wet-signed.
//
// Deliberately NOT built on signatureBlock, which hardcodes
// profile.OwnerName as the name under the line (pdf_theme.go) — calling it
// twice would print the WO owner's name in the client's column too. It reuses
// the lower-level primitives instead (fitImage, hairline, formatTanggalPDF)
// and leaves signatureBlock itself untouched for Invoice and Kwitansi.
func dualSignatureBlock(pdf *fpdf.Fpdf, theme pdfTheme, y float64, profile platformcontracts.TenantProfile, signature []byte, date time.Time, clientName string) float64 {
	const colW = 80.0
	const boxH = 20.0

	place := formatTanggalPDF(date)
	if profile.City != "" {
		place = profile.City + ", " + place
	}
	pdf.SetXY(pML, y)
	pdf.SetFont(theme.Family, "", 9.5)
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	pdf.CellFormat(pCW, 5, place, "", 0, "R", false, 0, "")
	y += 8

	columns := []struct {
		x       float64
		name    string
		role    string
		withSig bool
	}{
		{pML, clientName, "KLIEN", false},
		{pMR - colW, profile.OwnerName, "WEDDING CONSULTANT", true},
	}
	for _, col := range columns {
		// Only the WO side gets an embedded signature image; the client signs
		// the printout by hand, so their box stays deliberately empty.
		if col.withSig {
			if tp, ok := fpdfImageType(signature); ok {
				info := pdf.RegisterImageOptionsReader("po-signature", fpdf.ImageOptions{ImageType: tp}, bytes.NewReader(signature))
				if pdf.Err() {
					pdf.ClearError()
				} else if info != nil {
					iw, ih := fitImage(info.Width(), info.Height(), colW-10, boxH)
					pdf.ImageOptions("po-signature", col.x+(colW-iw)/2, y+(boxH-ih)/2, iw, ih, false, fpdf.ImageOptions{ImageType: tp}, 0, "")
				}
			}
		}
		lineY := y + boxH + 1
		hairline(pdf, col.x+6, col.x+colW-6, lineY)
		pdf.SetXY(col.x, lineY+1.5)
		pdf.SetFont(theme.Family, "B", 10)
		pdf.CellFormat(colW, 5, "("+orDash(col.name)+")", "", 2, "C", false, 0, "")
		pdf.SetX(col.x)
		pdf.SetFont(theme.Family, "", 8)
		pdf.SetTextColor(colorTextSecondary[0], colorTextSecondary[1], colorTextSecondary[2])
		pdf.CellFormat(colW, 4.5, col.role, "", 2, "C", false, 0, "")
		pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	}
	return y + boxH + 14
}

// drawDraftWatermark stamps "DRAFT" diagonally behind the content (D12).
//
// Registered as fpdf's header func so it repaints on every page fpdf opens,
// including the ones ensureSpace adds mid-document — and it runs before any
// content on that page, so the stamp always sits behind, never over the text.
// Page 1 is already open by the time newDocument returns, so the caller
// paints that one explicitly.
func drawDraftWatermark(pdf *fpdf.Fpdf, theme pdfTheme) {
	x, y := pdf.GetXY()
	pdf.SetAlpha(0.08, "Normal")
	pdf.TransformBegin()
	pdf.TransformRotate(45, 105, 160)
	pdf.SetFont(theme.Family, "B", 80)
	pdf.SetTextColor(30, 41, 59)
	pdf.SetXY(10, 140)
	pdf.CellFormat(190, 40, "DRAFT", "", 0, "C", false, 0, "")
	pdf.TransformEnd()
	pdf.SetAlpha(1, "Normal")
	pdf.SetTextColor(colorTextPrimary[0], colorTextPrimary[1], colorTextPrimary[2])
	pdf.SetXY(x, y)
}

func packageOrderStatusBadge(status domain.PackageOrderStatus) (label string, bg, fg [3]int) {
	switch status {
	case domain.PackageOrderIssued:
		return "TERBIT", badgePaidBg, badgePaidText
	case domain.PackageOrderCancelled:
		return "DIBATALKAN", badgeCancelBg, badgeCancelText
	default:
		return "DRAFT", badgeDraftBg, badgeDraftText
	}
}

// buildPackageOrderPDF renders the whole document (PLAN.md §4, blok B1-B7).
func buildPackageOrderPDF(
	data packageOrderPrintData,
	project domain.Project,
	profile platformcontracts.TenantProfile,
	logo, signature []byte,
) (*fpdf.Fpdf, error) {
	pdf, theme := newDocument(profile, logo, "PURCHASE ORDER", data.subtitle())
	if data.Status == domain.PackageOrderDraft {
		pdf.SetHeaderFunc(func() { drawDraftWatermark(pdf, theme) })
		drawDraftWatermark(pdf, theme) // page 1 is already open
	}
	y := pdf.GetY()

	// --- B1: identity box -------------------------------------------------
	eventTime := "Belum ditentukan"
	if data.Event.EventStart != "" || data.Event.EventEnd != "" {
		eventTime = data.Event.EventStart + " - " + data.Event.EventEnd
	}
	pax := "Belum ditentukan"
	if data.Event.Pax > 0 {
		pax = strconv.Itoa(data.Event.Pax) + " Pax"
	}
	leftRows := [][2]string{
		{"Nama", orDash(data.Event.ClientName)},
		{"No Telp", orDash(data.Event.Phone)},
		{"Hari / Tgl Acara", formatTanggalPDF(project.EventDate)},
		{"Waktu Acara", eventTime},
	}
	// "Harga" shows the TOTAL, not BasePrice. The source document printed the
	// pre-adjustment figure here while the client actually owed the higher
	// total — a discrepancy on its own front page.
	rightRows := [][2]string{
		{"Tempat", orDash(data.Event.Venue)},
		{"Jumlah Pax", pax},
		{"Nomor PO", orDash(data.PONumber)},
		{"Harga", formatRupiah(data.total())},
	}
	label, bg, fg := packageOrderStatusBadge(data.Status)
	const cardW = 87.0
	y1 := infoCard(pdf, theme, pML, y, cardW, "Data Client & Acara", leftRows, nil)
	y2 := infoCard(pdf, theme, pMR-cardW, y, cardW, "Detail Kontrak", rightRows, &badgeSpec{Label: label, Bg: bg, Fg: fg})
	y = max(y1, y2) + 8

	// --- B2/B3, B4, B5, B6a, B6b, B7 -------------------------------------
	y = drawCompositionTable(pdf, theme, y, data.Blocks)
	y = drawAdjustments(pdf, theme, y, data.Adjustments)
	y = drawTermsAndBonus(pdf, theme, y, data)
	y = drawScheduleBlock(pdf, theme, y, data, project.EventDate)
	y = drawLedgerAndTotals(pdf, theme, y, data)

	y = ensureSpace(pdf, theme, y+6, 46)
	dualSignatureBlock(pdf, theme, y, profile, signature, data.IssuedAt, data.Event.ClientName)

	return pdf, nil
}

func derefStringPDF(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
