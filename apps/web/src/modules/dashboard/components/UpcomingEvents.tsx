import { Link } from "react-router-dom";
import { Card, CardHeader, CardContent } from "@/shared/components/ui/Card";
import { EmptyState } from "@/shared/components/feedback/EmptyState";
import { Input } from "@/shared/components/ui/Input";
import type { Project } from "@/modules/projects/types";
import { daysUntil } from "@/modules/projects/lib/dates";
import { formatDate, todayISO } from "@/shared/lib/formatters";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { CalendarClock } from "lucide-react";

interface UpcomingEventsProps {
  projects: Project[];
  month: string;
  onMonthChange: (month: string) => void;
}

// PLAN.md mom-25082026-item-belum item 16: month ("YYYY-MM") switches this
// card from the default "5 acara terdekat" to every open project in that
// exact calendar month. min is pinned to the current month -- a past month
// isn't just unhelpful here, the backend's own upcoming-mode semantics for
// a past month would collide with the "no calendar-day floor" behavior a
// month filter intentionally has (see DashboardService.matchesUpcoming's
// doc comment), so keeping the picker from ever offering one avoids a
// confusing result rather than a broken one.
export function UpcomingEvents({ projects, month, onMonthChange }: UpcomingEventsProps) {
  const currentMonth = todayISO().slice(0, 7);
  const subtitle = month ? "Seluruh acara pada bulan yang dipilih." : "Diurutkan berdasarkan tanggal acara.";
  return (
    <Card>
      <CardHeader
        title="Acara Terdekat"
        subtitle={subtitle}
        action={
          <Input
            type="month"
            className="h-8 w-36 text-[12.5px]"
            min={currentMonth}
            value={month}
            onChange={(e) => onMonthChange(e.target.value)}
          />
        }
      />
      <CardContent className="p-0">
        {projects.length === 0 ? (
          <EmptyState
            icon={<CalendarClock className="h-7 w-7" />}
            title={month ? "Tidak ada acara pada bulan ini" : "Belum ada acara mendatang"}
          />
        ) : (
          <ul className="divide-y divide-border-light">
            {projects.map((p) => {
              const remaining = daysUntil(p.eventDate);
              return (
                <li key={p.id}>
                  <Link to={ROUTE_PATHS.projectDetail(p.id)} className="flex items-center justify-between gap-3 px-5 py-3 hover:bg-surface-muted">
                    <span className="min-w-0">
                      <span className="block truncate text-[13.5px] font-semibold text-text-primary">{p.name}</span>
                      <span className="block text-[12.5px] text-text-secondary">{formatDate(p.eventDate)} · {p.venue}</span>
                    </span>
                    <span className="shrink-0 rounded-full bg-navy-900/10 px-2.5 py-1 text-[11px] font-bold tabular-nums text-navy-900">
                      {remaining >= 0 ? `H-${remaining}` : "Sudah berlangsung"}
                    </span>
                  </Link>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
