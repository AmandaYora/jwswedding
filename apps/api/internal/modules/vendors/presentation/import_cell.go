package presentation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// groupCommaPattern/groupDotPattern/plainNumberPattern classify a spreadsheet
// cell's *display* text (excelize's f.Rows()/Columns() returns the formatted
// string, not the raw numeric value -- see styleTemplateSheet) into the three
// shapes an Import template's currency/count columns can come back as: Excel's
// own "#,##0"-style grouping (comma-grouped, optional dot-decimal), the id-ID
// habit of typing thousands with dots (dot-grouped, optional comma-decimal),
// or a plain number typed with no separators at all.
var (
	groupCommaPattern  = regexp.MustCompile(`^\d{1,3}(,\d{3})+(\.\d+)?$`)
	groupDotPattern    = regexp.MustCompile(`^\d{1,3}(\.\d{3})+(,\d+)?$`)
	plainNumberPattern = regexp.MustCompile(`^\d+([.,]\d+)?$`)
)

// parseNumberCell reads one Import template numeric cell (a price or a
// count) into an int64, replacing stripThousandsSeparators -- which only
// stripped commas and silently treated anything else as unparseable (see
// PLAN.md "perbaikan-import-bulk-vendor" S2/S6.1). A blank cell is not an
// error -- several of these columns are optional (Venue's RentalPrice/
// Charge/Capacity) -- the caller's own required-field check is the real
// gate for the columns that must be present. Anything that isn't a
// recognizable number ("nego", "5jt", "-") is a hard error instead of a
// silently-swallowed nil, so the caller can report exactly which cell
// didn't read rather than a generic "required fields missing" message.
func parseNumberCell(raw string) (*int64, error) {
	t := strings.TrimSpace(strings.ReplaceAll(raw, " ", " "))
	if t == "" {
		return nil, nil
	}

	up := strings.ToUpper(t)
	for _, prefix := range []string{"RP.", "RP", "IDR"} {
		if strings.HasPrefix(up, prefix) {
			t = strings.TrimSpace(t[len(prefix):])
			break
		}
	}
	t = strings.ReplaceAll(t, " ", "")
	if t == "" {
		return nil, fmt.Errorf("nilai %q bukan angka yang valid", raw)
	}

	switch {
	case groupCommaPattern.MatchString(t):
		t = strings.ReplaceAll(t, ",", "")
		if i := strings.IndexByte(t, '.'); i >= 0 {
			t = t[:i]
		}
	case groupDotPattern.MatchString(t):
		t = strings.ReplaceAll(t, ".", "")
		if i := strings.IndexByte(t, ','); i >= 0 {
			t = t[:i]
		}
	case plainNumberPattern.MatchString(t):
		if i := strings.IndexAny(t, ".,"); i >= 0 {
			t = t[:i]
		}
	default:
		return nil, fmt.Errorf("nilai %q bukan angka yang valid", raw)
	}

	n, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("nilai %q bukan angka yang valid", raw)
	}
	// price_akad/rental_price/charge/capacity are all UNSIGNED in MySQL -- a
	// negative value that slipped past this parser would fail the whole
	// batch at the DB layer instead of being reported on its own row, so
	// reject it here as a normal per-cell parse error.
	if n < 0 {
		return nil, fmt.Errorf("nilai %q tidak boleh negatif", raw)
	}
	return &n, nil
}

// allDigitsPattern matches a phone cell that Excel stored as a pure number
// (no leading zero survives that round-trip -- Excel drops it as an
// insignificant leading digit) -- normalizePhoneCell only ever rewrites a
// cell shaped like this; anything containing a "+", a space, a dash, or
// already starting with "0" is left exactly as read.
var allDigitsPattern = regexp.MustCompile(`^\d+$`)

// normalizePhoneCell restores the leading "0" an Indonesian phone number
// loses when a spreadsheet cell was left in Excel's default "General" number
// format instead of Text -- proven to have silently corrupted every single
// phone number in both real files this bug was reported against (see
// PLAN.md S2). Only touches all-digit cells; anything already textual
// (a leading zero, a "+62", spacing/dashes) passes through unchanged.
func normalizePhoneCell(raw string) string {
	t := strings.TrimSpace(raw)
	if t == "" || !allDigitsPattern.MatchString(t) {
		return t
	}
	switch {
	case strings.HasPrefix(t, "62") && len(t) >= 10:
		return "0" + t[2:]
	case strings.HasPrefix(t, "8") && len(t) >= 8:
		return "0" + t
	default:
		return t
	}
}
