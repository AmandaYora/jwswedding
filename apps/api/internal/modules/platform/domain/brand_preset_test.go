package domain

import "testing"

// TestPresetHexTable_SemuaKunciPunyaEntri locks the invariant that
// AllowedBrandColorPresets and PresetHexTable can never drift apart (PLAN.md
// redesain-pdf-invoice-kwitansi §D8) — a preset that's a valid selection but
// has no hex entry would silently fall back to navy for that one tenant.
func TestPresetHexTable_SemuaKunciPunyaEntri(t *testing.T) {
	for _, preset := range AllowedBrandColorPresets {
		if _, ok := PresetHexTable[preset]; !ok {
			t.Errorf("preset %q ada di AllowedBrandColorPresets tapi tidak ada di PresetHexTable", preset)
		}
	}
}

// TestPresetHexTable_SemuaNilaiValid guards hexToRGB's panic path — every
// literal in PresetHexTable must actually parse as "#rrggbb".
func TestPresetHexTable_SemuaNilaiValid(t *testing.T) {
	for preset := range PresetHexTable {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("preset %q memicu panic saat resolusi RGB: %v", preset, r)
				}
			}()
			PresetRGB(preset)
		}()
	}
}

func TestPresetRGB_Bronze(t *testing.T) {
	accent, accentDark, accentSoft := PresetRGB("bronze")
	if accent != [3]int{150, 105, 46} {
		t.Errorf("bronze accent: got %v, want {150,105,46} (#96692e)", accent)
	}
	if accentDark != [3]int{107, 74, 31} {
		t.Errorf("bronze accent dark: got %v, want {107,74,31} (#6b4a1f)", accentDark)
	}
	if accentSoft != [3]int{244, 228, 204} {
		t.Errorf("bronze accent soft: got %v, want {244,228,204} (#f4e4cc)", accentSoft)
	}
}

func TestPresetRGB_KunciTakDikenalJatuhKeNavy(t *testing.T) {
	navyAccent, navyDark, navySoft := PresetRGB(DefaultBrandColorPreset)

	for _, bad := range []string{"", "chartreuse", "BRONZE", "navy "} {
		accent, dark, soft := PresetRGB(bad)
		if accent != navyAccent || dark != navyDark || soft != navySoft {
			t.Errorf("preset tak dikenal %q: got accent=%v dark=%v soft=%v, want navy's %v/%v/%v",
				bad, accent, dark, soft, navyAccent, navyDark, navySoft)
		}
	}
}

func TestPresetRGB_SeluruhPresetMenghasilkanRGBValid(t *testing.T) {
	for _, preset := range AllowedBrandColorPresets {
		accent, dark, soft := PresetRGB(preset)
		for _, triple := range [][3]int{accent, dark, soft} {
			for _, c := range triple {
				if c < 0 || c > 255 {
					t.Errorf("preset %q menghasilkan komponen RGB di luar 0-255: %v", preset, triple)
				}
			}
		}
	}
}
