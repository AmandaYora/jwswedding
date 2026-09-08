// Month-option builders for the MonthSelect dropdown (Blok C,
// revisi-putri-mom-25082026). Each returns a list of "YYYY-MM" strings; the
// two shapes differ deliberately because the screens filter different dates
// (§1.3 #5):
//  - monthOptionsFromDates: derived from data already loaded, so it naturally
//    includes past months (used by the target_date filters, items 15 & 17).
//  - monthOptionsForward: generated forward from a starting month, no past
//    months at all (used by the dashboard's event_date filter, item 16 / T-3).

// monthOptionsFromDates collects the distinct "YYYY-MM" months present in a set
// of ISO date strings, sorted DESCENDING (most recent first, matching gambar 3).
export function monthOptionsFromDates(dates: string[]): string[] {
  const set = new Set<string>();
  for (const d of dates) {
    if (!d) continue;
    const ym = d.slice(0, 7);
    if (ym.length === 7) set.add(ym);
  }
  return [...set].sort().reverse();
}

// monthOptionsForward returns `count` consecutive "YYYY-MM" months starting at
// fromMonth (inclusive), sorted ascending — nearest month first, since every
// option is in the future.
export function monthOptionsForward(fromMonth: string, count: number): string[] {
  const [y, m] = fromMonth.split("-").map(Number);
  if (!y || !m) return [];
  const out: string[] = [];
  for (let i = 0; i < count; i++) {
    const d = new Date(y, m - 1 + i, 1);
    out.push(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`);
  }
  return out;
}

// monthOptionsRange returns a fixed window of consecutive "YYYY-MM" months
// centered on the current month, ascending (oldest first) — used by a
// server-side-paginated screen's month filter (D7, Blok I,
// docs/plan/revisi-putri-lanjutan/PLAN.md), where the loaded rows are only
// ever one page and deriving options from them (like monthOptionsFromDates
// does) would silently limit the dropdown to whatever page happens to be
// open. monthsBack/monthsForward are both inclusive of the current month.
export function monthOptionsRange(monthsBack: number, monthsForward: number, from: Date = new Date()): string[] {
  const y = from.getFullYear();
  const m = from.getMonth();
  const out: string[] = [];
  for (let i = -monthsBack; i <= monthsForward; i++) {
    const d = new Date(y, m + i, 1);
    out.push(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`);
  }
  return out;
}
