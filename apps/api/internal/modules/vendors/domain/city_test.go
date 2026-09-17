package domain

import (
	"sort"
	"testing"
)

func TestResolveCity(t *testing.T) {	cases := []struct {
		name          string
		raw           string
		wantCanonical string
		wantCandidate []string
	}{
		{name: "sudah persis sama", raw: "Kota Bogor", wantCanonical: "Kota Bogor"},
		{name: "beda besar kecil huruf", raw: "kota bogor", wantCanonical: "Kota Bogor"},
		{name: "tanpa prefix, tunggal", raw: "Tangerang Selatan", wantCanonical: "Kota Tangerang Selatan"},
		{name: "tanpa prefix, ambigu", raw: "Tangerang", wantCandidate: []string{"Kabupaten Tangerang", "Kota Tangerang"}},
		{name: "salah ketik, tidak dikenal", raw: "Kabupten Bogor"},
		// Kota di luar Jabodetabek–Bali tidak lagi resolve (PLAN
		// revisi-vendor-venue-portal §4.1, poin 9).
		{name: "kota yang dibuang, Bandung", raw: "Kota Bandung"},
		{name: "kota yang dibuang, Surabaya", raw: "Kota Surabaya"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotCanonical, gotCandidates := ResolveCity(c.raw)
			if gotCanonical != c.wantCanonical {
				t.Errorf("ResolveCity(%q) canonical = %q, seharusnya %q", c.raw, gotCanonical, c.wantCanonical)
			}
			sort.Strings(gotCandidates)
			if len(gotCandidates) != len(c.wantCandidate) {
				t.Fatalf("ResolveCity(%q) candidates = %v, seharusnya %v", c.raw, gotCandidates, c.wantCandidate)
			}
			for i := range gotCandidates {
				if gotCandidates[i] != c.wantCandidate[i] {
					t.Errorf("ResolveCity(%q) candidates = %v, seharusnya %v", c.raw, gotCandidates, c.wantCandidate)
					break
				}
			}
		})
	}
}

// TestIsValidCity_JabodetabekBaliOnly mengunci daftar 23 kota (PLAN
// revisi-vendor-venue-portal §4.1, poin 9): yang bertahan valid, sisanya
// tidak — termasuk ResolveCity yang kini menolaknya.
func TestIsValidCity_JabodetabekBaliOnly(t *testing.T) {
	if len(AllowedCities) != 23 {
		t.Errorf("len(AllowedCities) = %d, seharusnya 23", len(AllowedCities))
	}
	for _, city := range []string{
		"Jakarta Pusat", "Jakarta Utara", "Jakarta Barat", "Jakarta Selatan", "Jakarta Timur",
		"Kepulauan Seribu", "Kota Bogor", "Kabupaten Bogor", "Kota Depok",
		"Kota Tangerang", "Kota Tangerang Selatan", "Kabupaten Tangerang",
		"Kota Bekasi", "Kabupaten Bekasi", "Kota Denpasar",
		"Kabupaten Badung", "Kabupaten Bangli", "Kabupaten Buleleng", "Kabupaten Gianyar",
		"Kabupaten Jembrana", "Kabupaten Karangasem", "Kabupaten Klungkung", "Kabupaten Tabanan",
	} {
		if !IsValidCity(city) {
			t.Errorf("IsValidCity(%q) = false, seharusnya true", city)
		}
	}
	for _, city := range []string{"Kota Bandung", "Kota Surabaya", "Kota Semarang", "Kota Yogyakarta", "Kota Serang", ""} {
		if IsValidCity(city) {
			t.Errorf("IsValidCity(%q) = true, seharusnya false", city)
		}
	}
}
