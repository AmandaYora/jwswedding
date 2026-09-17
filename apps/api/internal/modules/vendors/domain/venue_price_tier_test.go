package domain

import "testing"

func int64ptr(n int64) *int64 { return &n }

// TestVenuePriceTier_Boundaries mengunci batas tier (PLAN
// revisi-vendor-venue-portal §4.2): tepat di setiap perbatasan.
func TestVenuePriceTier_Boundaries(t *testing.T) {
	cases := []struct {
		price *int64
		want  string
	}{
		{int64ptr(0), "Venue Budget"},
		{int64ptr(25_000_000), "Venue Budget"},
		{int64ptr(25_000_001), "Venue Value"},
		{int64ptr(60_000_000), "Venue Value"},
		{int64ptr(60_000_001), "Venue Prestige"},
		{int64ptr(100_000_000), "Venue Prestige"},
		{int64ptr(100_000_001), "Venue Luxury"},
		{int64ptr(999_999_999), "Venue Luxury"},
		{nil, ""},
	}
	for _, c := range cases {
		var label string
		if c.price == nil {
			label = "nil"
		} else {
			label = itoa(*c.price)
		}
		t.Run(label, func(t *testing.T) {
			if got := VenuePriceTierFor(c.price); got != c.want {
				t.Errorf("VenuePriceTierFor(%v) = %q, seharusnya %q", c.price, got, c.want)
			}
		})
	}
}

func TestVenuePriceTierRange(t *testing.T) {
	cases := []struct {
		label       string
		wantMin     int64
		wantMax     int64
		wantUnbound bool
		wantOK      bool
	}{
		{"Venue Budget", 0, 25_000_000, false, true},
		{"Venue Value", 25_000_001, 60_000_000, false, true},
		{"Venue Prestige", 60_000_001, 100_000_000, false, true},
		{"Venue Luxury", 100_000_001, 0, true, true},
		{"Venue Mahal", 0, 0, false, false},
		{"", 0, 0, false, false},
	}
	for _, c := range cases {
		t.Run("range:"+c.label, func(t *testing.T) {
			min, max, ok := VenuePriceTierRange(c.label)
			if ok != c.wantOK {
				t.Fatalf("VenuePriceTierRange(%q) ok = %v, seharusnya %v", c.label, ok, c.wantOK)
			}
			if !ok {
				return
			}
			if min != c.wantMin || max != c.wantMax {
				t.Errorf("VenuePriceTierRange(%q) = [%d, %d], seharusnya [%d, %d]", c.label, min, max, c.wantMin, c.wantMax)
			}
			if c.wantUnbound && max != 0 {
				t.Errorf("VenuePriceTierRange(%q) max = %d, seharusnya 0 (tanpa plafon)", c.label, max)
			}
		})
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
