import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Card } from "@/shared/components/ui/Card";
import { SearchInput } from "@/shared/components/ui/SearchInput";
import { Select, Input } from "@/shared/components/ui/Input";
import { Table, THead, TBody, TR, TH, TD } from "@/shared/components/ui/Table";
import { CardList, CardListField } from "@/shared/components/ui/CardList";
import { Pagination } from "@/shared/components/ui/Pagination";
import { usePagination } from "@/shared/hooks/usePagination";
import { EmptyState } from "@/shared/components/feedback/EmptyState";
import { MilestoneStatusBadge } from "@/modules/projects/components/StatusBadges";
import { useTimelineMonitorStore } from "@/modules/timeline-monitor/stores/useTimelineMonitorStore";
import type { ClientTimeline } from "@/modules/timeline-monitor/types";
import type { MilestoneStatus } from "@/modules/projects/types";
import { isMilestoneOverdue } from "@/modules/projects/lib/dates";
import { formatDate } from "@/shared/lib/formatters";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { CalendarClock } from "lucide-react";

const STATUS_OPTIONS: MilestoneStatus[] = ["Not Started", "In Progress", "Completed", "Blocked", "Cancelled"];

// Standalone monitoring page (PLAN.md mom-25082026-item-belum item 17) — a
// Wedding Planner has no Dashboard access at all (T-1 in the plan), so this
// is its own route/menu entry rather than a dashboard card. Read-only: no
// write action lives here, just search/bulan/status filters over one fetch
// already scoped server-side by role.
export default function TimelineMonitorPage() {
  const timelines = useTimelineMonitorStore((s) => s.timelines);
  const fetchTimelines = useTimelineMonitorStore((s) => s.fetchTimelines);

  const [query, setQuery] = useState("");
  const [monthFilter, setMonthFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<"Semua" | MilestoneStatus>("Semua");

  useEffect(() => {
    void fetchTimelines();
  }, [fetchTimelines]);

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
      const matchesStatus = statusFilter === "Semua" || t.status === statusFilter;
      return matchesQuery && matchesMonth && matchesStatus;
    });
  }, [timelines, query, monthFilter, statusFilter]);

  const { page, setPage, totalPages, totalItems, pageSize, pageItems } = usePagination(filtered);

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
        <Input type="month" className="w-40" value={monthFilter} onChange={(e) => setMonthFilter(e.target.value)} />
        <Select className="w-48" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value as "Semua" | MilestoneStatus)}>
          <option value="Semua">Semua Status</option>
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>{s}</option>
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
              renderItem={(t) => <TimelineCardContent timeline={t} />}
            />
            <div className="hidden sm:block">
              <Table>
                <THead>
                  <TR>
                    <TH>Timeline</TH>
                    <TH>Project</TH>
                    <TH>Status</TH>
                    <TH>Target Tanggal</TH>
                    <TH>Tanggal Acara</TH>
                  </TR>
                </THead>
                <TBody>
                  {pageItems.map((t) => {
                    const overdue = isMilestoneOverdue(t.status, t.targetDate);
                    return (
                      <TR key={t.id}>
                        <TD className="font-medium">{t.order}. {t.name}</TD>
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

function TimelineCardContent({ timeline: t }: { timeline: ClientTimeline }) {
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
