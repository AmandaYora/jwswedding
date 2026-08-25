package domain

import (
	"sort"
	"testing"
)

func TestResolveCity(t *testing.T) {
	cases := []struct {
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
