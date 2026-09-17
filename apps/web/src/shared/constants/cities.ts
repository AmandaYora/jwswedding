// CITIES mirrors the backend's AllowedCities verbatim (see
// apps/api/internal/modules/vendors/domain/city.go) — the fixed set of 23
// official kota/kabupaten in the Jabodetabek + Bali operating area (PLAN
// revisi-vendor-venue-portal §4.1, poin 9 — down from 128 Java+Bali entries,
// ADR-0016).
// Both sides must agree on the exact strings, same discipline as
// brandPresets.ts vs. AllowedBrandColorPresets. Deliberately kota AND
// kabupaten, not kota alone: Bali has exactly one official kota (Denpasar),
// so a kota-only list would leave every venue/vendor in Ubud/Kuta/Nusa Dua/
// Seminyak (all kabupaten Gianyar/Badung) with no accurate city to pick.
// Originally lived under modules/venues/ (Venue-only); moved here once
// Vendor's own "Kota" field needed the exact same list — a cities list is
// domain-agnostic data, not Venue- or Vendor-specific, so both modules
// import it from this shared location.
export const CITIES: string[] = [
  // DKI Jakarta (5 kota administrasi + 1 kabupaten)
  "Jakarta Pusat", "Jakarta Utara", "Jakarta Barat", "Jakarta Selatan", "Jakarta Timur",
  "Kepulauan Seribu",
  // Bogor (1 kota + 1 kabupaten)
  "Kota Bogor", "Kabupaten Bogor",
  // Depok (1 kota)
  "Kota Depok",
  // Tangerang (2 kota + 1 kabupaten)
  "Kota Tangerang", "Kota Tangerang Selatan", "Kabupaten Tangerang",
  // Bekasi (1 kota + 1 kabupaten)
  "Kota Bekasi", "Kabupaten Bekasi",
  // Bali (1 kota + 8 kabupaten)
  "Kota Denpasar", "Kabupaten Badung", "Kabupaten Bangli", "Kabupaten Buleleng",
  "Kabupaten Gianyar", "Kabupaten Jembrana", "Kabupaten Karangasem", "Kabupaten Klungkung",
  "Kabupaten Tabanan",
];

const CITY_SET = new Set(CITIES);

export function isValidCity(city: string): boolean {
  return CITY_SET.has(city);
}
