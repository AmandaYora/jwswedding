package presentation

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// templateSheetSpec tells styleTemplateSheet which of an Import template's
// columns need special handling beyond the plain-text default: currency
// columns get thousand-separated display, text columns are locked to Excel's
// Text format so an all-digit phone number keeps its leading zero instead of
// being read as a number, and required columns get a visually distinct
// header (see PLAN.md "perbaikan-import-bulk-vendor" S6.3) so a user can see
// at a glance which cells Import will actually reject if left blank.
type templateSheetSpec struct {
	currencyHeaders []string
	textHeaders     []string
	requiredHeaders []string
}

// styleTemplateSheet gives an Import template a clean, professional look
// without going past what a spreadsheet user already expects from one: a
// bold header row that stays pinned while scrolling, columns wide enough
// that a real value doesn't get clipped, thousand-separated formatting on
// the currency columns so a typed price reads back clearly, and Text format
// on phone columns so a leading "0" survives. No borders on data rows, no
// alternating row banding, no instructions sheet -- there's no data yet for
// any of that to describe.
func styleTemplateSheet(f *excelize.File, sheet string, headers []string, spec templateSheetSpec) error {
	requiredHeaderStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"1E3A5F"}},
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "left"},
		Border:    []excelize.Border{{Type: "bottom", Color: "172741", Style: 1}},
	})
	if err != nil {
		return err
	}
	// Optional columns keep a visibly lighter header than required ones, so
	// the mandatory set (also marked with the " *" templateHeaderLabel adds
	// to the printed text) reads as mandatory at a glance, not just in text.
	optionalHeaderStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"6B7280"}},
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "left"},
		Border:    []excelize.Border{{Type: "bottom", Color: "4B5563", Style: 1}},
	})
	if err != nil {
		return err
	}
	currencyStyle, err := f.NewStyle(&excelize.Style{NumFmt: 3}) // built-in "#,##0"
	if err != nil {
		return err
	}
	textStyle, err := f.NewStyle(&excelize.Style{NumFmt: 49}) // built-in "@" (Text)
	if err != nil {
		return err
	}

	currencySet := make(map[string]struct{}, len(spec.currencyHeaders))
	for _, h := range spec.currencyHeaders {
		currencySet[h] = struct{}{}
	}
	textSet := make(map[string]struct{}, len(spec.textHeaders))
	for _, h := range spec.textHeaders {
		textSet[h] = struct{}{}
	}
	requiredSet := make(map[string]struct{}, len(spec.requiredHeaders))
	for _, h := range spec.requiredHeaders {
		requiredSet[h] = struct{}{}
	}

	for i, header := range headers {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		if err := f.SetColWidth(sheet, col, col, columnWidthFor(header)); err != nil {
			return err
		}
		headerCell := fmt.Sprintf("%s1", col)
		_, required := requiredSet[header]
		headerStyle := optionalHeaderStyle
		if required {
			headerStyle = requiredHeaderStyle
		}
		if err := f.SetCellStyle(sheet, headerCell, headerCell, headerStyle); err != nil {
			return err
		}
		rangeStart := fmt.Sprintf("%s2", col)
		rangeEnd := fmt.Sprintf("%s%d", col, templateDropdownRows+1)
		if _, ok := currencySet[header]; ok {
			if err := f.SetCellStyle(sheet, rangeStart, rangeEnd, currencyStyle); err != nil {
				return err
			}
		} else if _, ok := textSet[header]; ok {
			if err := f.SetCellStyle(sheet, rangeStart, rangeEnd, textStyle); err != nil {
				return err
			}
		}
	}

	if err := f.SetRowHeight(sheet, 1, 20); err != nil {
		return err
	}
	// Keeps the header row visible no matter how far down a long list of
	// vendors/venues scrolls -- a plain, expected spreadsheet convenience.
	return f.SetPanes(sheet, &excelize.Panes{
		Freeze: true, Split: false, XSplit: 0, YSplit: 1,
		TopLeftCell: "A2", ActivePane: "bottomLeft",
	})
}

// templateHeaderLabel returns the text actually written into an Import
// template's header cell: required columns get a trailing " *" so a user
// sees which cells Import will reject if left blank, without changing the
// column's real name -- vendorTemplateHeaders/venueTemplateHeaders (the
// index every parser matches against) are never touched. Only Template()
// calls this; Export() keeps plain headers so an exported file re-uploads
// cleanly as Import input.
func templateHeaderLabel(header string, required bool) string {
	if required {
		return header + " *"
	}
	return header
}

// columnWidthFor sizes a column for the value it actually holds, not just its
// header text -- an "Email" or "Alamat" header is short, but real emails and
// addresses run much longer than that, so those columns get a fixed, roomier
// width instead of the header-length heuristic every other column uses.
func columnWidthFor(header string) float64 {
	switch header {
	case "Email", "Sosial Media":
		return 26
	case "Alamat", "Fasilitas", "Catatan":
		return 32
	case "Kota", "Nama Vendor", "Nama Venue":
		return 22
	case "No Tlp Vendor", "No Tlp PIC", "No Tlp Venue":
		return 16
	case "Harga Akad", "Harga Akad+Resepsi", "Harga Sewa", "Charge", "Kapasitas":
		return 16
	default:
		if width := float64(len(header)) + 6; width >= 14 {
			return width
		}
		return 14
	}
}
