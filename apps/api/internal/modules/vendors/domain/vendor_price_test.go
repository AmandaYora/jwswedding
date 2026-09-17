package domain

import "testing"

// TestVendorPriceColumn mengunci daftar putih kolom harga (PLAN
// revisi-vendor-venue-portal §4.2): satu-satunya tempat nama kolom boleh
// masuk SQL.
func TestVendorPriceColumn(t *testing.T) {
	cases := []struct {
		kind       string
		wantColumn string
		wantOK     bool
	}{
		{"akad", "price_akad", true},
		{"akadResepsi", "price_akad_resepsi", true},
		{"resepsi", "price_resepsi", true},
		{"", "", false},
		{"Price_Akad", "", false},
		{"price_akad; DROP TABLE vendors", "", false},
		{"price_akad", "", false},
	}
	for _, c := range cases {
		t.Run("kind:"+c.kind, func(t *testing.T) {
			col, ok := VendorPriceColumn(c.kind)
			if ok != c.wantOK || col != c.wantColumn {
				t.Errorf("VendorPriceColumn(%q) = (%q, %v), seharusnya (%q, %v)", c.kind, col, ok, c.wantColumn, c.wantOK)
			}
		})
	}
}
