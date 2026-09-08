import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Card } from "@/shared/components/ui/Card";
import { SearchInput } from "@/shared/components/ui/SearchInput";
import { Select } from "@/shared/components/ui/Input";
import { MonthSelect } from "@/shared/components/ui/MonthSelect";
import { Table, THead, TBody, TR, TH, TD } from "@/shared/components/ui/Table";
import { CardList, CardListField } from "@/shared/components/ui/CardList";
import { Pagination } from "@/shared/components/ui/Pagination";
import { usePagination } from "@/shared/hooks/usePagination";
import { EmptyState } from "@/shared/components/feedback/EmptyState";
import { MilestoneStatusBadge } from "@/modules/projects/components/StatusBadges";
import { useTimelineMonitorStore } from "@/modules/timeline-monitor/stores/useTimelineMonitorStore";
import { useStaffStore } from "@/modules/users/stores/useStaffStore";
import type { ClientTimeline } from "@/modules/timeline-monitor/types";
import type { MilestoneStatus } from "@/modules/projects/types";
import { isMilestoneOverdue } from "@/modules/projects/lib/dates";
import { formatDate } from "@/shared/lib/formatters";
import { monthOptionsFromDates } from "@/shared/lib/month-options";
import { nextSortDir, sortTimelines, type SortDir, type TimelineSortKey } from "@/modules/timeline-monitor/lib/timeline-sort";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { CalendarClock, ChevronDown, ChevronUp, ChevronsUpDown } from "lucide-react";

const STATUS_OPTIONS: MilestoneStatus[] = ["Not Started", "In Progress", "Completed", "Blocked", "Cancelled"];

// The 6 sortable columns (gambar 5, docs/plan/revisi-putri-lanjutan/PLAN.md
// Blok H), in table order — backs both the clickable TH headers (D11's
// desktop path) and the "Urutkan" dropdown (D11's HP path, T-8).
const SORT_COLUMNS: { key: TimelineSortKey; label: string; ascLabel: string; descLabel: string }[] = [
  { key: "name", label: "Timeline", ascLabel: "Timeline (A–Z)", descLabel: "Timeline (Z–A)" },
  { key: "pic", label: "PIC", ascLabel: "PIC (A–Z)", descLabel: "PIC (Z–A)" },
  { key: "project", label: "Project", ascLabel: "Project (A–Z)", descLabel: "Project (Z–A)" },
  { key: "status", label: "Status", ascLabel: "Status (A–Z)", descLabel: "Status (Z–A)" },
  { key: "targetDate", label: "Target Tanggal", ascLabel: "Target Tanggal (Terlama)", descLabel: "Target Tanggal (Terbaru)" },
  { key: "eventDate", label: "Tanggal Acara", ascLabel: "Tanggal Acara (Terlama)", descLabel: "Tanggal Acara (Terbaru)" },
];

// Standalone monitoring page (PLAN.md mom-25082026-item-belum item 17) — a
// Wedding Planner has no Dashboard access at all (T-1 in the plan), so this
// is its own route/menu entry rather than a dashboard card. Read-only: no
// write action lives here, just search/bulan/status/sort filters over one
// fetch already scoped server-side by role.
export default function TimelineMonitorPage() {
  const timelines = useTimelineMonitorStore((s) => s.timelines);
  const fetchTimelines = useTimelineMonitorStore((s) => s.fetchTimelines);
  const staffSummaries = useStaffStore((s) => s.staffSummaries);
  const fetchStaffSummaries = useStaffStore((s) => s.fetchStaffSummaries);

  const [query, setQuery] = useState("");
  const [monthFilter, setMonthFilter] = useState("");
  const [eventMonthFilter, setEventMonthFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<"Semua" | MilestoneStatus>("Semua");
  const [picFilter, setPicFilter] = useState("");
  const [sortKey, setSortKey] = useState<TimelineSortKey | null>(null);
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  useEffect(() => {
    void fetchTimelines();
    void fetchStaffSummaries();
  }, [fetchTimelines, fetchStaffSummaries]);

  // Staff display names are resolved client-side (MODULE_MAP.md) — the backend
  // returns only picStaffId. "0" and unknown ids render "Belum ditugaskan".
  const picNameFor = (id: string) => staffSummaries.find((s) => s.id === id)?.name ?? "Belum ditugaskan";

  // A Map, not picNameFor itself — sortTimelines' comparator runs O(n log n)
  // times, and resolving via Array.find inside it would turn into thousands
  // of linear scans at this screen's realistic volume (§8). Built once here.
  const picNameByStaffId = useMemo(() => new Map(staffSummaries.map((s) => [s.id, s.name])), [staffSummaries]);

  const monthOptions = monthOptionsFromDates(timelines.map((t) => t.targetDate));
  const eventMonthOptions = monthOptionsFromDates(timelines.map((t) => t.eventDate));

  // Unique PICs actually present in the loaded rows. The filter is only shown
  // when there's more than one (D8): a Wedding Planner only ever sees their own
  // projects, so a single-PIC dropdown would be pure noise.
  const picOptions = useMemo(() => [...new Set(timelines.map((t) => t.picStaffId))], [timelines]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return timelines.filter((t) => {
      const matchesQuery =
        q.length === 0 ||
        t.name.toLowerCase().includes(q) ||
        t.brideName.toLowerCase().includes(q) ||
        t.groomName.toLowerCase().includes(q) ||
        t.projectName.toLowerCase().includes(q);
      const matchesMonth = monthFilter.length === 0 || t.targetDate.slice(0, 7) === monthFilter;
      const matchesEventMonth = eventMonthFilter.length === 0 || t.eventDate.slice(0, 7) === eventMonthFilter;
      const matchesStatus = statusFilter === "Semua" || t.status === statusFilter;
      const matchesPic = picFilter.length === 0 || t.picStaffId === picFilter;
      return matchesQuery && matchesMonth && matchesEventMonth && matchesStatus && matchesPic;
    });
  }, [timelines, query, monthFilter, eventMonthFilter, statusFilter, picFilter]);

  // Sort is applied AFTER filtering and BEFORE pagination — sorting only
  // pageItems would sort a single page in isolation, correct-looking on page
  // 1 and visibly wrong everywhere else.
  const sorted = useMemo(
    () => sortTimelines(filtered, sortKey, sortDir, picNameByStaffId),
    [filtered, sortKey, sortDir, picNameByStaffId]
  );

  const { page, setPage, totalPages, totalItems, pageSize, pageItems } = usePagination(sorted);

  // D5: a sort change or a Bulan Acara change must return to page 1 — neither
  // is guaranteed to change usePagination's own totalItems (sorting never
  // does; a month filter usually does, but this is explicit rather than
  // incidental).
  useEffect(() => {
    setPage(1);
  }, [sortKey, sortDir, eventMonthFilter, setPage]);

  function handleSort(key: TimelineSortKey) {
    setSortDir(nextSortDir(sortKey === key ? sortDir : null));
    setSortKey(key);
  }

  function sortIcon(key: TimelineSortKey) {
    if (sortKey !== key) return <ChevronsUpDown className="h-3.5 w-3.5 text-text-secondary/50" />;
    return sortDir === "asc" ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />;
  }

  function ariaSortFor(key: TimelineSortKey): "ascending" | "descending" | "none" {
    if (sortKey !== key) return "none";
    return sortDir === "asc" ? "ascending" : "descending";
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-bold text-text-primary">Monitoring Timeline</h1>
        <p className="mt-1 text-[13px] text-text-secondary">
          Seluruh Timeline Project dari project yang Anda tangani, dalam satu tampilan.
        </p>
      </div>

      <div className="flex flex-wrap gap-3">
        <SearchInput
          className="max-w-xs"
          placeholder="Cari nama timeline, pengantin, atau project..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <MonthSelect
          className="w-44"
          value={monthFilter}
          onChange={setMonthFilter}
          options={monthOptions}
          allLabel="Semua Bulan Target"
        />
        <MonthSelect
          className="w-44"
          value={eventMonthFilter}
          onChange={setEventMonthFilter}
          options={eventMonthOptions}
          allLabel="Semua Bulan Acara"
        />
        <Select className="w-48" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value as "Semua" | MilestoneStatus)}>
          <option value="Semua">Semua Status</option>
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </Select>
        {picOptions.length > 1 && (
          <Select className="w-48" value={picFilter} onChange={(e) => setPicFilter(e.target.value)}>
            <option value="">Semua PIC</option>
            {picOptions.map((id) => (
              <option key={id} value={id}>{picNameFor(id)}</option>
            ))}
          </Select>
        )}
        {/* D11: the only sort path that survives on HP -- THead is hidden below
            `sm` (T-8), so a header-click-only sort would be unreachable there. */}
        <Select
          className="w-52"
          value={sortKey === null ? "" : `${sortKey}:${sortDir}`}
          onChange={(e) => {
            const value = e.target.value;
            if (value === "") {
              setSortKey(null);
              return;
            }
            const [key, dir] = value.split(":") as [TimelineSortKey, SortDir];
            setSortKey(key);
            setSortDir(dir);
          }}
        >
          <option value="">Urutan Bawaan</option>
          {SORT_COLUMNS.map((col) => (
            <optgroup key={col.key} label={col.label}>
              <option value={`${col.key}:asc`}>{col.ascLabel}</option>
              <option value={`${col.key}:desc`}>{col.descLabel}</option>
            </optgroup>
          ))}
        </Select>
      </div>

      <Card>
        {timelines.length === 0 ? (
          <EmptyState
            icon={<CalendarClock className="h-7 w-7" />}
            title="Belum ada timeline untuk dipantau"
            description="Timeline dari project yang Anda tangani akan muncul di sini."
          />
        ) : filtered.length === 0 ? (
          <EmptyState title="Tidak ada timeline yang cocok" description="Ubah kata kunci pencarian atau filter." />
        ) : (
          <>
            <CardList
              className="sm:hidden"
              items={pageItems}
              keyFor={(t) => t.id}
              renderItem={(t) => <TimelineCardContent timeline={t} picName={picNameFor(t.picStaffId)} />}
            />
            <div className="hidden sm:block">
              <Table>
                <THead>
                  <TR>
                    {SORT_COLUMNS.map((col) => (
                      <TH key={col.key} aria-sort={ariaSortFor(col.key)}>
                        <button
                          type="button"
                          onClick={() => handleSort(col.key)}
                          className="inline-flex items-center gap-1 uppercase tracking-wide hover:text-text-primary"
                        >
                          {col.label}
                          {sortIcon(col.key)}
                        </button>
                      </TH>
                    ))}
                  </TR>
                </THead>
                <TBody>
                  {pageItems.map((t) => {
                    const overdue = isMilestoneOverdue(t.status, t.targetDate);
                    return (
                      <TR key={t.id}>
                        <TD className="font-medium">{t.order}. {t.name}</TD>
                        <TD className="text-text-secondary">{picNameFor(t.picStaffId)}</TD>
                        <TD>
                          <Link
                            to={ROUTE_PATHS.projectDetail(t.projectId, "milestone")}
                            className="font-medium text-navy-900 hover:underline"
                          >
                            {t.brideName} &amp; {t.groomName}
                          </Link>
                          <p className="text-[12px] text-text-secondary">{t.projectName}</p>
                        </TD>
                        <TD><MilestoneStatusBadge status={t.status} /></TD>
                        <TD className={overdue ? "font-semibold text-danger" : undefined}>
                          {formatDate(t.targetDate)}
                          {overdue && " · terlambat"}
                        </TD>
                        <TD>{formatDate(t.eventDate)}</TD>
                      </TR>
                    );
                  })}
                </TBody>
              </Table>
            </div>
            <Pagination page={page} totalPages={totalPages} totalItems={totalItems} pageSize={pageSize} onPageChange={setPage} />
          </>
        )}
      </Card>
    </div>
  );
}

function TimelineCardContent({ timeline: t, picName }: { timeline: ClientTimeline; picName: string }) {
  const overdue = isMilestoneOverdue(t.status, t.targetDate);
  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <Link to={ROUTE_PATHS.projectDetail(t.projectId, "milestone")} className="font-medium text-text-primary hover:underline">
          {t.order}. {t.name}
        </Link>
        <MilestoneStatusBadge status={t.status} />
      </div>
      <div className="flex flex-col gap-1.5">
        <CardListField label="PIC" value={picName} />
        <CardListField label="Project" value={`${t.brideName} & ${t.groomName} — ${t.projectName}`} />
        <CardListField
          label="Target Tanggal"
          value={
            <span className={overdue ? "font-semibold text-danger" : undefined}>
              {formatDate(t.targetDate)}
              {overdue && " · terlambat"}
            </span>
          }
        />
        <CardListField label="Tanggal Acara" value={formatDate(t.eventDate)} />
      </div>
    </>
  );
}
