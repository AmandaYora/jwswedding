import { useEffect, useMemo, useState } from "react";
import { useOutletContext } from "react-router-dom";
import { Check, ChevronDown, HeartHandshake, Eye } from "lucide-react";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import { DEFAULT_MILESTONE_CATEGORIES, categoryOptions, iconForCategory } from "@/modules/projects/lib/milestone-categories";
import { SearchInput } from "@/shared/components/ui/SearchInput";
import { Select } from "@/shared/components/ui/Input";
import { EvidenceViewerModal } from "@/shared/components/ui/EvidenceViewerModal";
import { formatDate } from "@/shared/lib/formatters";
import type { Evidence, ProjectMilestone } from "@/modules/projects/types";
import type { ClientPortalContext } from "@/modules/client-portal/layouts/ClientPortalLayout";

// Sentinel value for the "Tanpa Kategori" filter option — distinct from ""
// (which is "Semua Kategori" / no filter), same convention as the WO
// Console's own category filter (ProjectMilestonesSection.tsx, PLAN-1 B14).
const UNCATEGORIZED_FILTER = "__none__";

interface MilestoneGroup {
  category: string;
  label: string;
  items: ProjectMilestone[];
}

// buildGroups orders the timeline into per-category groups (gambar 2): built-in
// categories first (in their fixed order), then any extra categories
// alphabetically, then "Tanpa Kategori" last — and only when it actually has
// items. Blok B, revisi-putri-mom-25082026.
function buildGroups(milestones: ProjectMilestone[]): MilestoneGroup[] {
  const byCat = new Map<string, ProjectMilestone[]>();
  for (const m of milestones) {
    const key = m.category || "";
    const list = byCat.get(key);
    if (list) list.push(m);
    else byCat.set(key, [m]);
  }
  const ordered: MilestoneGroup[] = [];
  for (const cat of DEFAULT_MILESTONE_CATEGORIES) {
    const items = byCat.get(cat);
    if (items) {
      ordered.push({ category: cat, label: cat, items });
      byCat.delete(cat);
    }
  }
  const extras = [...byCat.keys()].filter((k) => k !== "").sort((a, b) => a.localeCompare(b, "id-ID"));
  for (const cat of extras) {
    ordered.push({ category: cat, label: cat, items: byCat.get(cat)! });
  }
  const uncategorized = byCat.get("");
  if (uncategorized && uncategorized.length > 0) {
    ordered.push({ category: "", label: "Tanpa Kategori", items: uncategorized });
  }
  return ordered;
}

// isGroupComplete reports whether every item in a group is Completed --
// D2's "already done" test for the accordion's default-open group.
function isGroupComplete(group: MilestoneGroup): boolean {
  return group.items.length > 0 && group.items.every((m) => m.status === "Completed");
}

// The detailed vertical milestone stepper that used to be the main content
// of the standalone "Ringkasan" tab — split out into its own "Timeline" tab
// as part of PLAN.md's Client Portal restructure. Now grouped per category
// (Blok B) with per-category progress, and each agenda with a client-visible
// lampiran shows an eye icon that opens the file (Blok E).
export default function TimelineTabPage() {
  const { projectId } = useOutletContext<ClientPortalContext>();
  const milestones = useProjectStore((s) => s.milestones);
  const fetchMilestones = useProjectStore((s) => s.fetchMilestones);
  const milestoneDocuments = useProjectStore((s) => s.milestoneDocuments);
  const fetchMilestoneDocuments = useProjectStore((s) => s.fetchMilestoneDocuments);
  const [viewingEvidence, setViewingEvidence] = useState<Evidence | null>(null);
  const [query, setQuery] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("");
  // Manual accordion overrides, keyed by category — only the categories the
  // client has actually clicked live here. Everything else falls back to
  // D2's computed default (see isExpanded below), so the default keeps
  // tracking "the group currently being worked on" even as milestones are
  // completed over time, instead of freezing at whatever was true on mount.
  const [manualExpanded, setManualExpanded] = useState<Record<string, boolean>>({});

  useEffect(() => {
    void fetchMilestones(projectId);
    void fetchMilestoneDocuments(projectId);
  }, [projectId, fetchMilestones, fetchMilestoneDocuments]);

  // Cancelled agenda is never shown to the client (pre-existing behavior) --
  // everything below (groups, category options, the search/filter) works off
  // this set, not the raw store value.
  const activeMilestones = useMemo(() => milestones.filter((m) => m.status !== "Cancelled"), [milestones]);

  // D3: category options are the categories actually present on THIS
  // project's timeline (not the global template list), same pattern the WO
  // Console already uses -- and deliberately computed from the unfiltered
  // set, so the dropdown's own options don't shrink while the client is
  // typing a search query.
  const presentCategories = useMemo(
    () => categoryOptions(activeMilestones.map((m) => m.category)).filter((c) => activeMilestones.some((m) => m.category === c)),
    [activeMilestones]
  );
  const hasUncategorized = useMemo(() => activeMilestones.some((m) => m.category === ""), [activeMilestones]);

  const filteredMilestones = useMemo(() => {
    const q = query.trim().toLowerCase();
    return activeMilestones.filter((m) => {
      const matchesQuery = q.length === 0 || m.name.toLowerCase().includes(q);
      const matchesCategory =
        categoryFilter.length === 0 ||
        (categoryFilter === UNCATEGORIZED_FILTER ? m.category === "" : m.category === categoryFilter);
      return matchesQuery && matchesCategory;
    });
  }, [activeMilestones, query, categoryFilter]);

  const groups = useMemo(() => buildGroups(filteredMilestones), [filteredMilestones]);

  // D2's default-open group computed from the UNFILTERED groups, so it stays
  // stable while the client is searching/filtering rather than jumping
  // around as the visible group set shrinks.
  const allGroups = useMemo(() => buildGroups(activeMilestones), [activeMilestones]);
  const defaultOpenCategory = useMemo(() => {
    const firstIncomplete = allGroups.find((g) => !isGroupComplete(g));
    return (firstIncomplete ?? allGroups[0])?.category ?? null;
  }, [allGroups]);

  // G5: when search/category filtering leaves exactly one group, that group
  // is forced open regardless of manual/default state -- an accordion
  // collapsed over the only visible result would look like an empty screen.
  function isExpanded(category: string): boolean {
    if (groups.length === 1) return true;
    if (category in manualExpanded) return manualExpanded[category];
    return category === defaultOpenCategory;
  }

  function toggleGroup(category: string) {
    setManualExpanded((prev) => ({ ...prev, [category]: !isExpanded(category) }));
  }

  // Client-visible attachments for one agenda, matched by relatedId. Only rows
  // the milestone-documents endpoint returned (already filtered to visible)
  // reach here.
  function attachmentsFor(milestoneId: string): Evidence[] {
    return milestoneDocuments.filter((d) => d.relatedId === milestoneId);
  }

  return (
    <section className="rounded-3xl border border-border bg-white p-6 sm:p-8 shadow-sm">
      <div className="mb-8 flex items-center gap-3 border-b border-border pb-5">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-navy-50 text-navy-600">
          <HeartHandshake className="h-5 w-5 shrink-0" />
        </div>
        <div>
          <h2 className="text-[18px] font-bold text-navy-950 sm:text-[20px]">Perjalanan Persiapan</h2>
          <p className="mt-0.5 text-[13px] text-text-secondary sm:text-[14px]">Tercatat secara real-time saat tim menyelesaikan tahapan.</p>
        </div>
      </div>

      {activeMilestones.length > 0 && (
        <div className="mb-6 flex flex-wrap items-center gap-2">
          <SearchInput
            className="max-w-xs"
            placeholder="Cari timeline..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <Select className="w-44" value={categoryFilter} onChange={(e) => setCategoryFilter(e.target.value)}>
            <option value="">Semua Kategori</option>
            {presentCategories.map((c) => (
              <option key={c} value={c}>{c}</option>
            ))}
            {hasUncategorized && <option value={UNCATEGORIZED_FILTER}>Tanpa Kategori</option>}
          </Select>
        </div>
      )}

      {milestones.length === 0 ? (
        <p className="rounded-xl border border-dashed border-border px-4 py-8 text-center text-[13px] text-text-secondary">
          Belum ada tahapan persiapan yang tercatat.
        </p>
      ) : groups.length === 0 ? (
        <p className="rounded-xl border border-dashed border-border px-4 py-8 text-center text-[13px] text-text-secondary">
          Tidak ada tahapan yang cocok dengan pencarian atau kategori.
        </p>
      ) : (
        <div className="flex flex-col gap-8">
          {groups.map((group) => {
            const total = group.items.length;
            const completed = group.items.filter((m) => m.status === "Completed").length;
            const percent = total > 0 ? Math.round((completed / total) * 100) : 0;
            const expanded = isExpanded(group.category);
            const CategoryIcon = iconForCategory(group.category);
            return (
              <div key={group.label}>
                <button
                  type="button"
                  onClick={() => toggleGroup(group.category)}
                  aria-expanded={expanded}
                  className="mb-4 flex w-full items-center gap-3 text-left"
                >
                  <CategoryIcon className="h-4 w-4 shrink-0 text-navy-600" />
                  <div className="flex-1">
                    <div className="flex items-baseline justify-between gap-3">
                      <h3 className="text-[15px] font-bold text-navy-950 sm:text-[16px]">{group.label}</h3>
                      <span className="shrink-0 text-[12.5px] font-semibold text-text-secondary tabular-nums">
                        {completed}/{total} selesai
                      </span>
                    </div>
                    <div className="mt-2 flex items-center gap-2">
                      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-surface-muted">
                        <div className="h-full rounded-full bg-navy-900 transition-all" style={{ width: `${percent}%` }} />
                      </div>
                      <span className="shrink-0 text-[12.5px] font-semibold text-navy-900 tabular-nums">{percent}%</span>
                    </div>
                  </div>
                  <ChevronDown className={`h-4 w-4 shrink-0 text-text-secondary transition-transform ${expanded ? "rotate-180" : ""}`} />
                </button>

                {expanded && (
                <ol className="flex flex-col ml-4 sm:ml-6">
                  {group.items.map((m, idx) => {
                    const isLast = idx === group.items.length - 1;
                    const isCompleted = m.status === "Completed";
                    const isBlocked = m.status === "Blocked";
                    const isInProgress = m.status === "In Progress";
                    const attachments = attachmentsFor(m.id);

                    return (
                      <li key={m.id} className="group flex gap-5 sm:gap-6 relative">
                        <div className="flex flex-col items-center">
                          <span
                            className={
                              isCompleted
                                ? "flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-navy-900 text-white shadow-md shadow-navy-900/20 z-10 transition-transform group-hover:scale-110"
                                : isBlocked
                                ? "flex h-8 w-8 shrink-0 items-center justify-center rounded-full border-[3px] border-danger bg-white text-danger z-10 transition-transform group-hover:scale-110"
                                : isInProgress
                                ? "flex h-8 w-8 shrink-0 items-center justify-center rounded-full border-[3px] border-navy-900 bg-white text-navy-900 z-10 transition-transform group-hover:scale-110"
                                : "flex h-8 w-8 shrink-0 items-center justify-center rounded-full border-2 border-border bg-surface-muted text-border z-10"
                            }
                          >
                            {isCompleted && <Check className="h-4 w-4" />}
                            {!isCompleted && !isBlocked && !isInProgress && <div className="h-2 w-2 rounded-full bg-border" />}
                            {isInProgress && <div className="h-2.5 w-2.5 rounded-full bg-navy-900 animate-pulse" />}
                          </span>
                          {!isLast && <span className="w-[2px] flex-1 bg-gradient-to-b from-border to-border/50 my-1" />}
                        </div>
                        <div className={isLast ? "pb-4" : "pb-10 sm:pb-12"}>
                          <div className="flex flex-col sm:flex-row sm:items-center gap-1.5 sm:gap-3">
                            <p className={isCompleted || isInProgress ? "text-[15px] font-bold text-navy-950 sm:text-[16px]" : "text-[15px] font-medium text-text-secondary sm:text-[16px]"}>
                              {m.name}
                            </p>
                            {isInProgress && (
                              <span className="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-[11px] font-bold text-blue-600 ring-1 ring-blue-500/20">
                                IN PROGRESS
                              </span>
                            )}
                          </div>
                          <p className="mt-1.5 text-[13px] text-text-secondary sm:text-[13.5px] bg-surface-muted/50 inline-block px-3 py-1.5 rounded-lg border border-border/50">
                            {isCompleted
                              ? `Diselesaikan pada ${formatDate(m.completedDate)}`
                              : isBlocked
                              ? "Sedang terhambat — tim kami sedang menindaklanjuti"
                              : isInProgress
                              ? "Sedang dikerjakan oleh tim"
                              : `Target: ${formatDate(m.targetDate)}`}
                          </p>
                          {attachments.length > 0 && (
                            <div className="mt-2 flex flex-wrap gap-2">
                              {attachments.map((a) => (
                                <button
                                  key={a.id}
                                  type="button"
                                  onClick={() => setViewingEvidence(a)}
                                  className="inline-flex items-center gap-1.5 rounded-lg border border-navy-200 bg-navy-50 px-2.5 py-1 text-[12.5px] font-semibold text-navy-700 transition-colors hover:bg-navy-100"
                                >
                                  <Eye className="h-3.5 w-3.5" />
                                  {a.name}
                                </button>
                              ))}
                            </div>
                          )}
                        </div>
                      </li>
                    );
                  })}
                </ol>
                )}
              </div>
            );
          })}
        </div>
      )}

      {viewingEvidence && (
        <EvidenceViewerModal
          open
          onClose={() => setViewingEvidence(null)}
          projectId={projectId}
          evidence={viewingEvidence}
        />
      )}
    </section>
  );
}
