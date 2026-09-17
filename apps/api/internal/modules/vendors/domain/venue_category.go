package domain

import (
	"fmt"
	"strings"
)

// AllowedVenueCategories is the fixed multi-select list for Menu Venue
// (PLAN revisi-vendor-venue-portal §1.1 poin 4 / §4.2) — in the exact
// order the user listed them. A venue may carry more than one, so they live in
// the venue_categories table, NOT as a column on venues and NOT as the
// tenant-owned vendor_categories master (which has its own CRUD).
var AllowedVenueCategories = []string{
	"Function Hall",
	"Ballroom (carpet)",
	"Hotel Bintang 5",
	"Hotel Bintang 4",
	"Hotel Bintang 3",
	"Restaurant",
}

// canonicalVenueCategoryIndex maps each allowed value to its position in
// AllowedVenueCategories — the canonical order NormalizeVenueCategories
// sorts into.
var canonicalVenueCategoryIndex = buildVenueCategoryIndex()

func buildVenueCategoryIndex() map[string]int {
	idx := make(map[string]int, len(AllowedVenueCategories))
	for i, c := range AllowedVenueCategories {
		idx[c] = i
	}
	return idx
}

// IsValidVenueCategory reports whether name is one of the six fixed values.
// Matching is exact (case-sensitive), same discipline as IsValidCity.
func IsValidVenueCategory(name string) bool {
	_, ok := canonicalVenueCategoryIndex[name]
	return ok
}

// NormalizeVenueCategories trims, drops empties and duplicates, sorts into
// canonical order, and rejects unknown values. An empty (or all-blank)
// input is valid and yields nil — a venue with no category is legitimate,
// and per D-5 an empty Import cell clears the categories.
func NormalizeVenueCategories(raw []string) ([]string, error) {
	seen := make(map[string]struct{}, len(raw))
	var out []string
	for _, r := range raw {
		name := strings.TrimSpace(r)
		if name == "" {
			continue
		}
		if !IsValidVenueCategory(name) {
			return nil, fmt.Errorf("kategori venue tidak dikenal: %q — pilih dari: %s", r, strings.Join(AllowedVenueCategories, ", "))
		}
		if _, dup := seen[name]; !dup {
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	// Insertion-style ordering by canonical index (at most 6 items).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && canonicalVenueCategoryIndex[out[j]] < canonicalVenueCategoryIndex[out[j-1]]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}
