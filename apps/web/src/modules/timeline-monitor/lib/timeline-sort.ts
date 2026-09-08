import type { ClientTimeline } from "@/modules/timeline-monitor/types";

// Sort helper for Monitoring Timeline (Blok H, gambar 5,
// docs/plan/revisi-putri-lanjutan/PLAN.md). Lives in this module, not
// shared/ (D10): the comparator needs to know ClientTimeline's own shape
// (which column is a date, which is a resolved-elsewhere PIC name), which
// makes it a timeline-monitor concept, not a domain-agnostic one.
export type TimelineSortKey = "name" | "pic" | "project" | "status" | "targetDate" | "eventDate";
export type SortDir = "asc" | "desc";

// nextSortDir cycles a column's direction on repeated clicks: unsorted -> asc
// -> desc -> asc ... (never back to unsorted once a column has been clicked,
// matching the gambar 5 header-click convention -- "Urutan Bawaan" in the
// dropdown, D11, is the only way back to unsorted).
export function nextSortDir(current: SortDir | null): SortDir {
  return current === "asc" ? "desc" : "asc";
}

// sortTimelines sorts a COPY of rows (never mutates its input) by the given
// key/direction. picNameByStaffId is a pre-resolved Map, not the picNameFor
// function the page also has -- resolving inside the comparator would run an
// O(n) Array.find on every one of the O(n log n) comparisons sort() makes
// (§8: ~19,000 linear scans at this screen's realistic volume). The map is
// built once by the caller before this runs.
export function sortTimelines(
  rows: ClientTimeline[],
  key: TimelineSortKey | null,
  dir: SortDir,
  picNameByStaffId: Map<string, string>
): ClientTimeline[] {
  if (key === null) return rows;
  const activeKey: TimelineSortKey = key;
  const sign = dir === "asc" ? 1 : -1;
  const picName = (id: string) => picNameByStaffId.get(id) ?? "Belum ditugaskan";

  function valueFor(t: ClientTimeline): string {
    switch (activeKey) {
      case "name":
        return t.name;
      case "pic":
        return picName(t.picStaffId);
      case "project":
        return t.projectName;
      case "status":
        return t.status;
      case "targetDate":
        return t.targetDate;
      case "eventDate":
        return t.eventDate;
    }
  }

  const isDateColumn = activeKey === "targetDate" || activeKey === "eventDate";
  return [...rows].sort((a, b) => {
    const va = valueFor(a);
    const vb = valueFor(b);
    // "YYYY-MM-DD" sorts correctly as a plain string comparison -- no Date
    // parsing needed. localeCompare is reserved for the human-language
    // columns (name/pic/project/status), where "id-ID" collation matters.
    if (isDateColumn) return sign * (va < vb ? -1 : va > vb ? 1 : 0);
    return sign * va.localeCompare(vb, "id-ID");
  });
}
