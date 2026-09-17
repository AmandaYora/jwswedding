// VENUE_PRICE_TIERS mirrors the backend's VenuePriceTiers verbatim (see
// apps/api/internal/modules/vendors/domain/venue_price_tier.go) — the tier
// labels derived server-side from rental_price (PLAN
// revisi-vendor-venue-portal §1.1 poin 6 / §4.2). Both sides must agree on
// the exact strings, same discipline as CITIES vs. AllowedCities.
export const VENUE_PRICE_TIERS: string[] = [
  "Venue Budget",
  "Venue Value",
  "Venue Prestige",
  "Venue Luxury",
];
