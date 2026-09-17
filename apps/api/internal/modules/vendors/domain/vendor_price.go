package domain

// VendorPriceColumn is the whitelist mapping the frontend's package-type
// filter (PLAN revisi-vendor-venue-portal §1.1 poin 1 / §4.2 — "pilih
// jenis paket dulu, lalu Min–Maks") to the real vendors table column. The
// ONLY place a user-supplied price-kind string may become SQL; the
// repository must never interpolate the raw query param.
func VendorPriceColumn(kind string) (string, bool) {
	switch kind {
	case "akad":
		return "price_akad", true
	case "akadResepsi":
		return "price_akad_resepsi", true
	case "resepsi":
		return "price_resepsi", true
	default:
		return "", false
	}
}

// IsValidVendorPriceKind reports whether kind is one of the three filterable
// package types. "" is NOT valid here — it means "no price filter" and is
// handled by the caller before reaching the whitelist.
func IsValidVendorPriceKind(kind string) bool {
	_, ok := VendorPriceColumn(kind)
	return ok
}
