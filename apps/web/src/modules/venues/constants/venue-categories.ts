// VENUE_CATEGORIES mirrors the backend's AllowedVenueCategories verbatim
// (see apps/api/internal/modules/vendors/domain/venue_category.go) — the
// fixed multi-select list for Menu Venue (PLAN revisi-vendor-venue-portal
// §1.1 poin 4 / §4.2). Both sides must agree on the exact strings, same
// discipline as CITIES vs. AllowedCities.
export const VENUE_CATEGORIES: string[] = [
  "Function Hall",
  "Ballroom (carpet)",
  "Hotel Bintang 5",
  "Hotel Bintang 4",
  "Hotel Bintang 3",
  "Restaurant",
];
