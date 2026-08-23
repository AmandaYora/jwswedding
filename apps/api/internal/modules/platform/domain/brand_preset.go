package domain

import "fmt"

// DefaultBrandColorPreset must match migration 000017's
// `brand_color_preset ... DEFAULT 'navy'` — used wherever a Tenant is
// constructed in Go without an explicit preset (e.g. Register), so the
// in-memory value agrees with what the DB will actually store.
const DefaultBrandColorPreset = "navy"

// AllowedBrandColorPresets is the fixed set of 21 brand colors a tenant may
// pick for its WO Console / Client Portal theme (see PLAN.md §3/§14) —
// free-form hex is deliberately not supported, so this is the single source
// of truth the application layer validates against. "navy" is the app's
// original, unbranded look and stays the default for every existing tenant.
var AllowedBrandColorPresets = []string{
	"navy", "gold", "orange", "blue", "emerald", "red", "purple",
	"teal", "indigo", "rose", "cyan", "fuchsia", "lime", "slate", "stone",
	"mustard", "green", "sky", "pink", "yellow", "bronze",
}

func IsValidBrandColorPreset(preset string) bool {
	for _, p := range AllowedBrandColorPresets {
		if p == preset {
			return true
		}
	}
	return false
}

// PresetHexTable mirrors apps/web/src/theme/brandPresets.ts's
// BRAND_COLOR_PRESETS exactly — same 21 keys, same 4 shade roles per key
// (950/900/800/soft), same hex values. It exists so the backend can resolve
// a tenant's brand_color_preset into RGB for server-rendered output (the
// Invoice/Kwitansi PDF, PLAN.md redesain-pdf-invoice-kwitansi §D8) — before
// that need existed, hex values lived only on the frontend (see this file's
// git history). Keep both tables in sync by hand; a mismatch would make the
// PDF's accent color diverge from what WO Console/Client Portal render for
// the same tenant.
var PresetHexTable = map[string]struct {
	Shade950, Shade900, Shade800, ShadeSoft string
}{
	"navy":    {"#172741", "#1e3a5f", "#24476f", "#e3ebf3"},
	"gold":    {"#92400e", "#b45309", "#d97706", "#fef3c7"},
	"orange":  {"#9a3412", "#c2410c", "#ea580c", "#ffedd5"},
	"blue":    {"#172554", "#1e3a8a", "#1e40af", "#dbeafe"},
	"emerald": {"#022c22", "#064e3b", "#065f46", "#d1fae5"},
	"red":     {"#450a0a", "#7f1d1d", "#991b1b", "#fee2e2"},
	"purple":  {"#2e1065", "#4c1d95", "#5b21b6", "#ede9fe"},
	"teal":    {"#042f2c", "#134e4a", "#115e59", "#ccfbf1"},
	"indigo":  {"#1e1b4b", "#312e81", "#3730a3", "#e0e7ff"},
	"rose":    {"#4c0519", "#881337", "#9f1239", "#ffe4e6"},
	"cyan":    {"#083344", "#164e63", "#155e75", "#cffafe"},
	"fuchsia": {"#4a044e", "#701a75", "#86198f", "#fae8ff"},
	"lime":    {"#1a2e05", "#365314", "#3f6212", "#ecfccb"},
	"slate":   {"#020617", "#0f172a", "#1e293b", "#f1f5f9"},
	"stone":   {"#0c0a09", "#1c1917", "#292524", "#f5f5f4"},
	"mustard": {"#4a4400", "#7a7000", "#9c8900", "#f5f0c9"},
	"green":   {"#14532d", "#15803d", "#16a34a", "#dcfce7"},
	"sky":     {"#075985", "#0369a1", "#0284c7", "#e0f2fe"},
	"pink":    {"#be185d", "#db2777", "#ec4899", "#fce7f3"},
	"yellow":  {"#854d0e", "#a16207", "#ca8a04", "#fef9c3"},
	"bronze":  {"#6b4a1f", "#96692e", "#ab8347", "#f4e4cc"},
}

// PresetRGB resolves a brand_color_preset key into the 3 shades the
// Invoice/Kwitansi PDF actually uses — accent (900), accent dark (950), and
// accent soft/tint — as [3]int{R, G, B}. An unknown or empty key (should
// only ever happen against manipulated data; the column is
// NOT NULL DEFAULT 'navy' and every write path validates via
// IsValidBrandColorPreset) falls back to "navy", mirroring the frontend's
// own fallback (apps/web/src/theme/useTenantBrandingStore.ts's
// isBrandColorPresetKey check, brandPresets.ts's isBrandColorPresetKey).
func PresetRGB(preset string) (accent, accentDark, accentSoft [3]int) {
	shades, ok := PresetHexTable[preset]
	if !ok {
		shades = PresetHexTable[DefaultBrandColorPreset]
	}
	return hexToRGB(shades.Shade900), hexToRGB(shades.Shade950), hexToRGB(shades.ShadeSoft)
}

// hexToRGB parses a "#rrggbb" literal from PresetHexTable into [3]int. Every
// value in that table is a hand-verified constant copied from
// brandPresets.ts, so a parse error here means the table itself is
// malformed — caught immediately by TestPresetHexTable_SemuaNilaiValid
// rather than surfacing as a silently-wrong color in production.
func hexToRGB(hex string) [3]int {
	var r, g, b int
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		panic(fmt.Sprintf("brand_preset: nilai hex tidak valid di PresetHexTable: %q: %v", hex, err))
	}
	return [3]int{r, g, b}
}
