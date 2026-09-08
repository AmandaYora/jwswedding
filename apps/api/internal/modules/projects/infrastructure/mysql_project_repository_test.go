package infrastructure

import (
	"testing"
	"time"
)

// eventMonthRange's two bounds are exactly what the ?eventMonth=YYYY-MM
// project filter puts on the wire, so these cases pin the boundaries that
// actually decide whether a project shows up: the 1st of the filtered month
// (must be INSIDE the half-open range) and the 1st of the next month (must be
// OUTSIDE it).
func TestEventMonthRange_BatasSetengahTerbuka(t *testing.T) {
	cases := []struct {
		in        string
		wantStart string
		wantEnd   string
	}{
		{"2026-09", "2026-09-01", "2026-10-01"},
		{"2026-02", "2026-02-01", "2026-03-01"}, // Februari tahun biasa
		{"2024-02", "2024-02-01", "2024-03-01"}, // Februari tahun kabisat
		{"2026-12", "2026-12-01", "2027-01-01"}, // batas pindah tahun
	}
	for _, c := range cases {
		start, end, err := eventMonthRange(c.in)
		if err != nil {
			t.Fatalf("eventMonthRange(%q): error tak terduga: %v", c.in, err)
		}
		if start != c.wantStart || end != c.wantEnd {
			t.Errorf("eventMonthRange(%q) = (%q, %q), ingin (%q, %q)", c.in, start, end, c.wantStart, c.wantEnd)
		}
	}
}

// The bounds must come out identical no matter which timezone the API process
// happens to run in -- the entire reason they are date strings and not
// time.Time (see eventMonthRange's doc comment). Without this property the
// filter silently shifts by the host's UTC offset: on a UTC+7 host it drops
// every event on the 1st of the filtered month and pulls in the 1st of the
// next one, which is invisible in a Docker image with no tzdata (Local = UTC)
// and reproducible only on a developer's own machine.
func TestEventMonthRange_TidakBergantungTimezoneHost(t *testing.T) {
	original := time.Local
	defer func() { time.Local = original }()

	for _, zone := range []*time.Location{
		time.UTC,
		time.FixedZone("WIB", 7*60*60),
		time.FixedZone("Pasifik", -11*60*60),
	} {
		time.Local = zone
		start, end, err := eventMonthRange("2026-09")
		if err != nil {
			t.Fatalf("zona %s: error tak terduga: %v", zone, err)
		}
		if start != "2026-09-01" || end != "2026-10-01" {
			t.Errorf("zona %s: eventMonthRange = (%q, %q), ingin (%q, %q)", zone, start, end, "2026-09-01", "2026-10-01")
		}
	}
}

// Presentation already answers a malformed ?eventMonth with 422 before the
// repository is reached (parseEventMonthFilter), so this path is defense in
// depth -- it must surface an error rather than fall through to an unfiltered
// query, which would quietly return every project in the tenant.
func TestEventMonthRange_FormatSalah_Error(t *testing.T) {
	for _, in := range []string{"", "September", "2026", "2026-13", "2026-09-01"} {
		if _, _, err := eventMonthRange(in); err == nil {
			t.Errorf("eventMonthRange(%q): ingin error, dapat nil", in)
		}
	}
}
