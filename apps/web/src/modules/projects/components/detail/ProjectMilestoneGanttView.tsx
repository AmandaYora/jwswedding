import { useRef, useState } from "react";
import { createPortal } from "react-dom";
import { isMilestoneOverdue } from "@/modules/projects/lib/dates";
import type { ProjectMilestone, MilestoneStatus } from "@/modules/projects/types";
import { Badge, type BadgeTone } from "@/shared/components/ui/Badge";
import { formatDate, todayISO, MONTHS_ID } from "@/shared/lib/formatters";
import { cn } from "@/shared/lib/cn";

// Redesain dari library @svar-ui/react-gantt ke komponen timeline custom —
// lihat knowledge/decisions/ADR-0026-timeline-titik-waktu-custom-gantt.md.
// Setiap item Timeline hanya punya satu titik waktu (targetDate, atau
// completedDate begitu selesai), bukan rentang, jadi ini adalah timeline
// titik-waktu (marker), bukan Gantt bar-berdurasi.

function parseISODate(dateISO: string): Date {
  const [y, m, d] = dateISO.split("-").map(Number);
  return new Date(y, (m ?? 1) - 1, d ?? 1);
}

function sortForGantt(milestones: ProjectMilestone[]): ProjectMilestone[] {
  return [...milestones].sort((a, b) => a.order - b.order);
}

function effectiveDate(m: ProjectMilestone): Date {
  return parseISODate(m.completedDate ?? m.targetDate);
}

interface TimelineRange {
  start: Date;
  end: Date;
}

const DAY_MS = 24 * 60 * 60 * 1000;

function buildTimelineRange(milestones: ProjectMilestone[]): TimelineRange {
  const times = milestones.map((m) => effectiveDate(m).getTime());
  const minTime = Math.min(...times);
  const maxTime = Math.max(...times);

  if (minTime === maxTime) {
    return { start: new Date(minTime - 14 * DAY_MS), end: new Date(maxTime + 14 * DAY_MS) };
  }

  const span = maxTime - minTime;
  const padding = Math.max(span * 0.05, 3 * DAY_MS);
  return { start: new Date(minTime - padding), end: new Date(maxTime + padding) };
}

// Posisi visual saja (0-100, di-clamp) — untuk keputusan tampil/sembunyi
// (mis. TodayLine) pakai isDateWithinRange, bukan nilai yang sudah di-clamp
// ini.
function datePercent(date: Date, range: TimelineRange): number {
  const span = range.end.getTime() - range.start.getTime();
  if (span <= 0) return 0;
  const raw = ((date.getTime() - range.start.getTime()) / span) * 100;
  return Math.min(100, Math.max(0, raw));
}

function isDateWithinRange(date: Date, range: TimelineRange): boolean {
  return date.getTime() >= range.start.getTime() && date.getTime() <= range.end.getTime();
}

interface MonthTick {
  key: string;
  label: string;
  percent: number;
}

function buildMonthTicks(range: TimelineRange): MonthTick[] {
  const ticks: MonthTick[] = [];
  const cursor = new Date(range.start.getFullYear(), range.start.getMonth(), 1);
  const last = new Date(range.end.getFullYear(), range.end.getMonth(), 1);
  while (cursor.getTime() <= last.getTime()) {
    ticks.push({
      key: `${cursor.getFullYear()}-${cursor.getMonth()}`,
      label: `${MONTHS_ID[cursor.getMonth()]} ${cursor.getFullYear()}`,
      percent: datePercent(cursor, range),
    });
    cursor.setMonth(cursor.getMonth() + 1);
  }
  return ticks;
}

const STATUS_TONE: Record<MilestoneStatus, BadgeTone> = {
  "Not Started": "neutral",
  "In Progress": "info",
  Completed: "navy",
  Blocked: "danger",
  Cancelled: "neutral",
};

const STATUS_MARKER_BASE: Record<MilestoneStatus, string> = {
  Completed: "border-navy-900 bg-navy-900",
  "In Progress": "border-info bg-info-soft",
  Blocked: "border-danger bg-danger-soft",
  "Not Started": "border-border bg-white",
  Cancelled: "border-border-light bg-neutral-soft opacity-60",
};

function statusMarkerClasses(status: MilestoneStatus, overdue: boolean): string {
  return cn(STATUS_MARKER_BASE[status], overdue && "ring-2 ring-danger/40");
}

function TimelineTooltip({ milestone, overdue }: { milestone: ProjectMilestone; overdue: boolean }) {
  return (
    <div className="w-56 rounded-md border border-border bg-surface p-3 text-left shadow-md">
      <p className="text-[13px] font-semibold text-text-primary">
        {milestone.order}. {milestone.name}
      </p>
      <div className="mt-1.5">
        <Badge tone={STATUS_TONE[milestone.status]}>{milestone.status}</Badge>
      </div>
      <dl className="mt-2 space-y-1 text-[12px]">
        <div className="flex items-center justify-between gap-2">
          <dt className="text-text-secondary">Target</dt>
          <dd className={overdue ? "font-semibold text-danger" : "text-text-primary"}>
            {formatDate(milestone.targetDate)}
            {overdue && " · terlambat"}
          </dd>
        </div>
        {milestone.completedDate && (
          <div className="flex items-center justify-between gap-2">
            <dt className="text-text-secondary">Selesai</dt>
            <dd className="text-text-primary">{formatDate(milestone.completedDate)}</dd>
          </div>
        )}
      </dl>
    </div>
  );
}

function TimelineMarker({
  milestone,
  percent,
  overdue,
}: {
  milestone: ProjectMilestone;
  percent: number;
  overdue: boolean;
}) {
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [anchor, setAnchor] = useState<{ top: number; left: number } | null>(null);

  function showTooltip() {
    const rect = buttonRef.current?.getBoundingClientRect();
    if (rect) setAnchor({ top: rect.top, left: rect.left + rect.width / 2 });
  }
  function hideTooltip() {
    setAnchor(null);
  }

  return (
    <div className="absolute top-1/2" style={{ left: `${percent}%` }}>
      <button
        ref={buttonRef}
        type="button"
        onMouseEnter={showTooltip}
        onMouseLeave={hideTooltip}
        onFocus={showTooltip}
        onBlur={hideTooltip}
        aria-label={`${milestone.order}. ${milestone.name} — ${milestone.status}`}
        className={cn(
          "block h-3.5 w-3.5 -translate-x-1/2 -translate-y-1/2 rotate-45 rounded-[2px] border-2 transition-transform hover:scale-125 focus:scale-125 focus:outline-none",
          statusMarkerClasses(milestone.status, overdue)
        )}
      />
      {anchor &&
        createPortal(
          <div
            className="fixed z-50 -translate-x-1/2 -translate-y-full pb-2"
            style={{ top: anchor.top, left: anchor.left }}
          >
            <TimelineTooltip milestone={milestone} overdue={overdue} />
          </div>,
          document.body
        )}
    </div>
  );
}

function TodayLine({ percent }: { percent: number }) {
  return (
    <div className="absolute inset-y-0 z-10 border-l border-dashed border-navy-900/50" style={{ left: `${percent}%` }}>
      <span className="absolute -top-5 -translate-x-1/2 whitespace-nowrap rounded-full bg-navy-900 px-2 py-0.5 text-[10px] font-semibold text-white">
        Hari ini
      </span>
    </div>
  );
}

function TodayDivider() {
  return (
    <div className="flex items-center gap-2 py-1">
      <span className="h-0 flex-1 border-t border-dashed border-navy-900/50" />
      <span className="whitespace-nowrap rounded-full bg-navy-900 px-2 py-0.5 text-[10px] font-semibold text-white">
        Hari ini
      </span>
      <span className="h-0 flex-1 border-t border-dashed border-navy-900/50" />
    </div>
  );
}

function DesktopTimeline({ milestones, range }: { milestones: ProjectMilestone[]; range: TimelineRange }) {
  const ticks = buildMonthTicks(range);
  const today = parseISODate(todayISO());
  const todayPercent = datePercent(today, range);
  const showTodayLine = isDateWithinRange(today, range);
  const minWidth = Math.max(640, ticks.length * 96);

  return (
    <div className="hidden overflow-x-auto sm:block">
      <div style={{ minWidth: `${minWidth}px` }}>
        <div className="relative h-6 border-b border-border-light">
          {ticks.map((tick) => (
            <span
              key={tick.key}
              className="absolute top-0 -translate-x-1/2 whitespace-nowrap text-[11px] font-medium text-text-secondary"
              style={{ left: `${tick.percent}%` }}
            >
              {tick.label}
            </span>
          ))}
        </div>
        <div className="relative h-16">
          {ticks.map((tick) => (
            <span
              key={tick.key}
              className="absolute inset-y-0 border-l border-border-light"
              style={{ left: `${tick.percent}%` }}
            />
          ))}
          <span className="absolute inset-x-0 top-1/2 h-px bg-border" />
          <span className="absolute inset-y-0 left-0 bg-navy-900/5" style={{ width: `${todayPercent}%` }} />
          {milestones.map((m) => (
            <TimelineMarker
              key={m.id}
              milestone={m}
              percent={datePercent(effectiveDate(m), range)}
              overdue={isMilestoneOverdue(m.status, m.targetDate)}
            />
          ))}
          {showTodayLine && <TodayLine percent={todayPercent} />}
        </div>
      </div>
    </div>
  );
}

function MobileTimeline({ milestones }: { milestones: ProjectMilestone[] }) {
  const today = parseISODate(todayISO());
  const todayIndex = milestones.findIndex((m) => effectiveDate(m).getTime() > today.getTime());
  const insertAt = todayIndex === -1 ? milestones.length : todayIndex;

  return (
    <div className="relative sm:hidden">
      <div className="absolute bottom-2 left-[6px] top-2 w-px bg-border" />
      <ul className="flex flex-col gap-3">
        {insertAt === 0 && (
          <li>
            <TodayDivider />
          </li>
        )}
        {milestones.map((m, idx) => {
          const overdue = isMilestoneOverdue(m.status, m.targetDate);
          const cancelled = m.status === "Cancelled";
          return (
            <li key={m.id} className="flex flex-col gap-3">
              <div className="flex items-start gap-3">
                <span
                  className={cn(
                    "mt-1 h-3.5 w-3.5 shrink-0 rounded-full border-2",
                    statusMarkerClasses(m.status, overdue)
                  )}
                />
                <div className="min-w-0 flex-1">
                  <p className={cn("text-[13px] font-medium text-text-primary", cancelled && "text-text-secondary line-through")}>
                    {m.order}. {m.name}
                  </p>
                  <div className="mt-1 flex flex-wrap items-center gap-2">
                    <Badge tone={STATUS_TONE[m.status]}>{m.status}</Badge>
                    <span className={cn("text-[12px]", overdue ? "font-semibold text-danger" : "text-text-secondary")}>
                      {formatDate(m.targetDate)}
                      {overdue && " · terlambat"}
                    </span>
                  </div>
                  {m.completedDate && (
                    <p className="mt-0.5 text-[12px] text-text-secondary">Selesai: {formatDate(m.completedDate)}</p>
                  )}
                </div>
              </div>
              {idx + 1 === insertAt && <TodayDivider />}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export function ProjectMilestoneGanttView({ milestones }: { milestones: ProjectMilestone[] }) {
  if (milestones.length === 0) {
    return (
      <p className="rounded-md border border-dashed border-border px-4 py-6 text-center text-[13px] text-text-secondary">
        Belum ada timeline untuk ditampilkan.
      </p>
    );
  }

  const sorted = sortForGantt(milestones);
  const range = buildTimelineRange(sorted);

  return (
    <div className="rounded-md border border-border p-4">
      <DesktopTimeline milestones={sorted} range={range} />
      <MobileTimeline milestones={sorted} />
    </div>
  );
}
