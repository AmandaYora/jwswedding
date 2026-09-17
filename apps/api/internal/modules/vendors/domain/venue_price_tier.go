package domain

// Venue price tiers — PLAN revisi-vendor-venue-portal §1.1 poin 6 / §4.2.
//
// The three bands the user wrote down had 1-rupiah gaps (…25jt vs 26jt…,
// …60jt vs 61jt…) and no band above Rp100jt at all; per locked decision §2.1
// the gaps are closed and a fourth tier added on top. Derived purely from
// venues.rental_price — there is no DB column for this.
//
// Bounds (rental_price in rupiah):
//
//	Venue Budget    ≤ 25.000.000
//	Venue Value     25.000.001 – 60.000.000
//	Venue Prestige  60.000.001 – 100.000.000
//	Venue Luxury    > 100.000.000
var VenuePriceTiers = []string{
	"Venue Budget",
	"Venue Value",
	"Venue Prestige",
	"Venue Luxury",
}

const (
	venueBudgetMax   int64 = 25_000_000
	venueValueMax    int64 = 60_000_000
	venuePrestigeMax int64 = 100_000_000
)

// VenuePriceTierFor maps a rental price to its tier label. A nil price
// (venue without a set rental price) yields "" — such venues never match
// a tier filter (repo predicates require rental_price IS NOT NULL).
func VenuePriceTierFor(price *int64) string {
	if price == nil {
		return ""
	}
	switch p := *price; {
	case p <= venueBudgetMax:
		return "Venue Budget"
	case p <= venueValueMax:
		return "Venue Value"
	case p <= venuePrestigeMax:
		return "Venue Prestige"
	default:
		return "Venue Luxury"
	}
}

// VenuePriceTierRange translates a tier label into [min, max] rental-price
// bounds for SQL predicates. max == 0 means unbounded above (Venue Luxury).
// ok == false when the label is not one of VenuePriceTiers.
func VenuePriceTierRange(label string) (min, max int64, ok bool) {
	switch label {
	case "Venue Budget":
		return 0, venueBudgetMax, true
	case "Venue Value":
		return venueBudgetMax + 1, venueValueMax, true
	case "Venue Prestige":
		return venueValueMax + 1, venuePrestigeMax, true
	case "Venue Luxury":
		return venuePrestigeMax + 1, 0, true
	default:
		return 0, 0, false
	}
}

// IsValidVenuePriceTier reports whether label is one of VenuePriceTiers.
func IsValidVenuePriceTier(label string) bool {
	_, _, ok := VenuePriceTierRange(label)
	return ok
}
