package domain

import (
	"reflect"
	"testing"
)

// TestNormalizeVenueCategories mengunci perilaku daftar tetap multi-select
// (PLAN revisi-vendor-venue-portal §4.2/D-5).
func TestNormalizeVenueCategories(t *testing.T) {
	t.Run("duplikat dibuang, urutan kanonik dipertahankan", func(t *testing.T) {
		got, err := NormalizeVenueCategories([]string{"Restaurant", "Function Hall", "Restaurant", "  Function Hall "})
		if err != nil {
			t.Fatalf("NormalizeVenueCategories: %v", err)
		}
		want := []string{"Function Hall", "Restaurant"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("NormalizeVenueCategories = %v, seharusnya %v", got, want)
		}
	})
	t.Run("nilai tak dikenal ditolak", func(t *testing.T) {
		if _, err := NormalizeVenueCategories([]string{"Function Hall", "Gedung Serbaguna"}); err == nil {
			t.Error("NormalizeVenueCategories seharusnya galat untuk nilai tak dikenal")
		}
	})
	t.Run("kosong bukan galat dan mengosongkan", func(t *testing.T) {
		for _, raw := range [][]string{nil, {}, {""}, {"   "}} {
			got, err := NormalizeVenueCategories(raw)
			if err != nil {
				t.Errorf("NormalizeVenueCategories(%v): %v", raw, err)
			}
			if len(got) != 0 {
				t.Errorf("NormalizeVenueCategories(%v) = %v, seharusnya kosong", raw, got)
			}
		}
	})
	t.Run("enam nilai sah berurutan kanonik", func(t *testing.T) {
		raw := []string{"Restaurant", "Hotel Bintang 3", "Hotel Bintang 4", "Hotel Bintang 5", "Ballroom (carpet)", "Function Hall"}
		got, err := NormalizeVenueCategories(raw)
		if err != nil {
			t.Fatalf("NormalizeVenueCategories: %v", err)
		}
		if !reflect.DeepEqual(got, AllowedVenueCategories) {
			t.Errorf("NormalizeVenueCategories = %v, seharusnya %v", got, AllowedVenueCategories)
		}
	})
}

func TestIsValidVenueCategory(t *testing.T) {
	for _, c := range AllowedVenueCategories {
		if !IsValidVenueCategory(c) {
			t.Errorf("IsValidVenueCategory(%q) = false, seharusnya true", c)
		}
	}
	for _, c := range []string{"", "ballroom (carpet)", "Hotel", "NgawurBanget"} {
		if IsValidVenueCategory(c) {
			t.Errorf("IsValidVenueCategory(%q) = true, seharusnya false", c)
		}
	}
}
