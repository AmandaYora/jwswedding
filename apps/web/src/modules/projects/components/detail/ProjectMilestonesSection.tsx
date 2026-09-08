import { useEffect, useMemo, useState } from "react";
import { Plus, Ban, CheckCircle2, Pencil, ArrowUp, ArrowDown, Eye, List as ListIcon, GanttChartSquare } from "lucide-react";
import { Card, CardHeader, CardContent } from "@/shared/components/ui/Card";
import { Button } from "@/shared/components/ui/Button";
import { Select } from "@/shared/components/ui/Input";
import { MonthSelect } from "@/shared/components/ui/MonthSelect";
import { SearchInput } from "@/shared/components/ui/SearchInput";
import { Table, THead, TBody, TR, TH, TD } from "@/shared/components/ui/Table";
import { CardList, CardListField } from "@/shared/components/ui/CardList";
import { Pagination } from "@/shared/components/ui/Pagination";
import { usePagination } from "@/shared/hooks/usePagination";
import { MilestoneRail, MilestoneRailLegend } from "@/shared/components/ui/MilestoneRail";
import { IconActionButton } from "@/shared/components/ui/IconActionButton";
import { ProjectMilestoneFormModal } from "@/modules/projects/components/ProjectMilestoneFormModal";
import {
  ProjectMilestoneEditModal,
  type ProjectMilestoneEditFields,
  type NewMilestoneEvidenceMeta,
} from "@/modules/projects/components/detail/ProjectMilestoneEditModal";
import { ProjectMilestoneGanttView } from "@/modules/projects/components/detail/ProjectMilestoneGanttView";
import type { ProjectMilestoneFormValues } from "@/modules/projects/schemas/project-milestone.schema";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import { categoryOptions } from "@/modules/projects/lib/milestone-categories";
import { monthOptionsFromDates } from "@/shared/lib/month-options";
import { computeMilestoneStats, isMilestoneOverdue } from "@/modules/projects/lib/dates";
import { compressFileForUpload } from "@/shared/lib/image-compression";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import type { ProjectMilestone, MilestoneStatus } from "@/modules/projects/types";
import { formatDate } from "@/shared/lib/formatters";
import { cn } from "@/shared/lib/cn";

const STATUS_OPTIONS: MilestoneStatus[] = ["Not Started", "In Progress", "Completed", "Blocked", "Cancelled"];

// Sentinel value for the "Tanpa Kategori" filter option — distinct from ""
// (which is "Semua Kategori" / no filter) so both can coexist in one Select.
const UNCATEGORIZED_FILTER = "__none__";

function sortMilestones(list: ProjectMilestone[]): ProjectMilestone[] {
  return [...list].sort((a, b) => {
    const aCancelled = a.status === "Cancelled";
    const bCancelled = b.status === "Cancelled";
    if (aCancelled !== bCancelled) return aCancelled ? 1 : -1;
    return a.order - b.order;
  });
}

export function ProjectMilestonesSection({ projectId }: { projectId: string }) {
  const milestones = useProjectStore((s) => s.milestones);
  const fetchMilestones = useProjectStore((s) => s.fetchMilestones);
  const createMilestone = useProjectStore((s) => s.createMilestone);
  const updateMilestoneStatus = useProjectStore((s) => s.updateMilestoneStatus);
  const updateMilestone = useProjectStore((s) => s.updateMilestone);
  const reorderMilestones = useProjectStore((s) => s.reorderMilestones);
  const evidence = useProjectStore((s) => s.evidence);
  const fetchEvidence = useProjectStore((s) => s.fetchEvidence);
  const uploadEvidence = useProjectStore((s) => s.uploadEvidence);
  const [addOpen, setAddOpen] = useState(false);
  const [editingMilestone, setEditingMilestone] = useState<ProjectMilestone | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<"list" | "gantt">("list");
  const [query, setQuery] = useState("");
  const [monthFilter, setMonthFilter] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("");
  const sortedMilestones = sortMilestones(milestones);
  const stats = computeMilestoneStats(milestones);
  const isFilterActive = query.trim().length > 0 || monthFilter.length > 0 || categoryFilter.length > 0;
  const monthOptions = monthOptionsFromDates(milestones.map((m) => m.targetDate));
  // Category filter options: only categories actually present on this project's
  // milestones, ordered built-ins-first (Blok B). "" (Tanpa Kategori) is
  // offered separately below when any milestone is uncategorized.
  const presentCategories = categoryOptions(milestones.map((m) => m.category)).filter((c) =>
    milestones.some((m) => m.category === c)
  );
  const hasUncategorized = milestones.some((m) => m.category === "");
  // Search cocok pada nama timeline saja -- satu-satunya teks bebas pada
  // ProjectMilestone (PLAN.md mom-25082026-item-belum item 4). Filter bulan
  // menyaring target_date, bukan tanggal selesai -- MOM-nya bicara "timeline
  // yg harus diselesaikan", yaitu target (item 15).
  const filteredMilestones = useMemo(() => {
    const q = query.trim().toLowerCase();
    return sortedMilestones.filter((m) => {
      const matchesQuery = q.length === 0 || m.name.toLowerCase().includes(q);
      const matchesMonth = monthFilter.length === 0 || m.targetDate.slice(0, 7) === monthFilter;
      const matchesCategory =
        categoryFilter.length === 0 ||
        (categoryFilter === UNCATEGORIZED_FILTER ? m.category === "" : m.category === categoryFilter);
      return matchesQuery && matchesMonth && matchesCategory;
    });
  }, [sortedMilestones, query, monthFilter, categoryFilter]);
  const { page, setPage, totalPages, totalItems, pageSize, pageItems } = usePagination(filteredMilestones);
  // Up/down only makes sense within the active (non-cancelled) set — a
  // cancelled milestone's on-screen position is always forced to the bottom
  // (see sortMilestones above) regardless of its stored sort order, so
  // reordering buttons on it (or past it) would be confusing. Computed from
  // the UNFILTERED list (not filteredMilestones) on purpose: handleMove
  // sends the full permutation of every milestone's ID to the backend, which
  // rejects anything less than a complete permutation (project_service.go's
  // ReorderMilestones) -- reordering buttons stay hidden while a
  // filter/search is active instead (see isFilterActive below).
  const activeOrder = sortedMilestones.filter((m) => m.status !== "Cancelled");

  useEffect(() => {
    void fetchMilestones(projectId);
    void fetchEvidence(projectId);
  }, [projectId, fetchMilestones, fetchEvidence]);

  // Mirrors ProjectVendorsSection.tsx's own evidenceFor(milestone) — one
  // fetchEvidence call for the whole project, filtered client-side per
  // timeline, same pattern as its vendor-milestone sibling.
  function evidenceFor(milestone: ProjectMilestone) {
    return evidence.filter((e) => e.relatedKind === "projectMilestone" && e.relatedId === milestone.id);
  }

  async function updateStatus(id: string, status: MilestoneStatus) {
    setActionError(null);
    try {
      await updateMilestoneStatus(projectId, id, status);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal memperbarui status timeline"));
    }
  }

  async function handleAddMilestone(values: ProjectMilestoneFormValues) {
    setActionError(null);
    try {
      await createMilestone(projectId, values);
      setAddOpen(false);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal menambahkan timeline"));
    }
  }

  async function handleEditSave(fields: ProjectMilestoneEditFields) {
    if (!editingMilestone) return;
    setActionError(null);
    try {
      await updateMilestone(projectId, editingMilestone.id, fields);
      setEditingMilestone(null);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal memperbarui timeline"));
    }
  }

  // No local try/catch here — matches ProjectVendorsSection.tsx's own
  // handleAddMilestoneEvidence: errors bubble up to
  // ProjectMilestoneEditModal's internal handleAddEvidence, which shows its
  // own uploadError inside the Lampiran section.
  async function handleAddEvidence(file: File, values: NewMilestoneEvidenceMeta) {
    if (!editingMilestone) return;
    const compressed = await compressFileForUpload(file);
    await uploadEvidence(projectId, {
      ...compressed,
      name: values.name,
      type: values.type,
      documentDate: values.documentDate,
      description: values.description,
      relatedKind: "projectMilestone",
      relatedId: editingMilestone.id,
      isClientVisible: values.isClientVisible,
    });
  }

  async function handleMove(milestoneId: string, direction: "up" | "down") {
    setActionError(null);
    const idx = activeOrder.findIndex((m) => m.id === milestoneId);
    const swapIdx = direction === "up" ? idx - 1 : idx + 1;
    if (idx === -1 || swapIdx < 0 || swapIdx >= activeOrder.length) return;
    const reordered = [...activeOrder];
    [reordered[idx], reordered[swapIdx]] = [reordered[swapIdx], reordered[idx]];
    const cancelled = sortedMilestones.filter((m) => m.status === "Cancelled");
    try {
      await reorderMilestones(projectId, [...reordered, ...cancelled].map((m) => m.id));
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal mengubah urutan timeline"));
    }
  }

  return (
    <div id="milestone">
      <Card>
        <CardHeader
          title="Timeline Persiapan Acara"
          subtitle="Progress keseluruhan didasarkan pada timeline yang benar-benar telah diselesaikan, bukan angka manual."
          action={
            <div className="flex items-center gap-2">
              <div className="flex items-center rounded-md border border-border p-0.5">
                <button
                  type="button"
                  onClick={() => setViewMode("list")}
                  className={cn(
                    "flex items-center gap-1.5 rounded px-2.5 py-1 text-[12.5px] font-semibold transition-colors",
                    viewMode === "list" ? "bg-navy-900 text-white" : "text-text-secondary hover:text-text-primary"
                  )}
                >
                  <ListIcon className="h-3.5 w-3.5" /> List
                </button>
                <button
                  type="button"
                  onClick={() => setViewMode("gantt")}
                  className={cn(
                    "flex items-center gap-1.5 rounded px-2.5 py-1 text-[12.5px] font-semibold transition-colors",
                    viewMode === "gantt" ? "bg-navy-900 text-white" : "text-text-secondary hover:text-text-primary"
                  )}
                >
                  <GanttChartSquare className="h-3.5 w-3.5" /> Gantt
                </button>
              </div>
              <Button size="sm" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setAddOpen(true)}>
                Tambah Timeline
              </Button>
            </div>
          }
        />
        <CardContent>
          {actionError && (
            <p className="mb-3 rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{actionError}</p>
          )}
          <div className="flex flex-col gap-2 border-b border-border-light pb-4">
            <MilestoneRail milestones={milestones} size="md" />
            <div className="flex flex-wrap items-center justify-between gap-2">
              <MilestoneRailLegend />
              {stats.overdue > 0 && (
                <span className="text-[12.5px] font-semibold text-danger">{stats.overdue} timeline terlambat dari target</span>
              )}
            </div>
          </div>

          {milestones.length > 0 && (
            <div className="flex flex-wrap items-center gap-2 pt-4">
              <SearchInput
                className="max-w-xs"
                placeholder="Cari nama timeline..."
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
            <p className="mt-4 rounded-md border border-dashed border-border px-4 py-6 text-center text-[13px] text-text-secondary">
              Belum ada timeline untuk project ini.
            </p>
          ) : filteredMilestones.length === 0 ? (
            <p className="mt-4 rounded-md border border-dashed border-border px-4 py-6 text-center text-[13px] text-text-secondary">
              Tidak ada timeline yang cocok dengan pencarian atau filter.
            </p>
          ) : viewMode === "gantt" ? (
            <div className="pt-4">
              <ProjectMilestoneGanttView milestones={filteredMilestones} />
            </div>
          ) : (
          <>
          <CardList
            className="sm:hidden"
            items={pageItems}
            keyFor={(m) => m.id}
            renderItem={(m) => {
              const overdue = isMilestoneOverdue(m.status, m.targetDate);
              const cancelled = m.status === "Cancelled";
              const activeIdx = activeOrder.findIndex((a) => a.id === m.id);
              return (
                <>
                  <div className="flex items-start justify-between gap-3">
                    <span className={cn("font-medium text-text-primary", cancelled && "line-through")}>{m.order}. {m.name}</span>
                    <div className="flex shrink-0 items-center gap-1">
                      {!cancelled && !isFilterActive && (
                        <>
                          <IconActionButton
                            icon={ArrowUp}
                            label="Naikkan urutan"
                            tone="neutral"
                            disabled={activeIdx <= 0}
                            onClick={() => void handleMove(m.id, "up")}
                          />
                          <IconActionButton
                            icon={ArrowDown}
                            label="Turunkan urutan"
                            tone="neutral"
                            disabled={activeIdx === -1 || activeIdx >= activeOrder.length - 1}
                            onClick={() => void handleMove(m.id, "down")}
                          />
                        </>
                      )}
                      <IconActionButton icon={Pencil} label="Edit timeline" tone="neutral" onClick={() => setEditingMilestone(m)} />
                      {cancelled ? (
                        <IconActionButton
                          icon={CheckCircle2}
                          label="Aktifkan kembali"
                          tone="success"
                          onClick={() => void updateStatus(m.id, "Not Started")}
                        />
                      ) : (
                        <IconActionButton
                          icon={Ban}
                          label="Batalkan timeline"
                          tone="danger"
                          onClick={() => void updateStatus(m.id, "Cancelled")}
                        />
                      )}
                    </div>
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <CardListField
                      label="Target Tanggal"
                      value={
                        <span className={overdue ? "font-semibold text-danger" : undefined}>
                          {formatDate(m.targetDate)}
                          {overdue && " · terlambat"}
                        </span>
                      }
                    />
                    <CardListField label="Kategori" value={m.category || "Tanpa Kategori"} />
                    <CardListField label="Tanggal Selesai" value={formatDate(m.completedDate)} />
                    <CardListField
                      label="Lampiran"
                      value={
                        evidenceFor(m).length > 0 ? (
                          <span className="inline-flex items-center gap-1.5">
                            <span className="font-medium text-text-primary">{evidenceFor(m).length}</span>
                            <IconActionButton icon={Eye} label="Lihat Lampiran" tone="info" onClick={() => setEditingMilestone(m)} />
                          </span>
                        ) : (
                          "-"
                        )
                      }
                    />
                  </div>
                  <Select
                    value={m.status}
                    onChange={(e) => void updateStatus(m.id, e.target.value as MilestoneStatus)}
                    className="h-9 w-full text-[13px]"
                  >
                    {STATUS_OPTIONS.map((s) => (
                      <option key={s} value={s}>{s}</option>
                    ))}
                  </Select>
                </>
              );
            }}
          />
          <div className="hidden sm:block">
          <Table className="mt-2">
            <THead>
              <TR>
                <TH>Timeline</TH>
                <TH>Kategori</TH>
                <TH>Status</TH>
                <TH>Target Tanggal</TH>
                <TH>Tanggal Selesai</TH>
                <TH>Lampiran</TH>
                <TH>Aksi</TH>
              </TR>
            </THead>
            <TBody>
              {pageItems.map((m) => {
                const overdue = isMilestoneOverdue(m.status, m.targetDate);
                const cancelled = m.status === "Cancelled";
                const activeIdx = activeOrder.findIndex((a) => a.id === m.id);
                return (
                  <TR key={m.id} className={cancelled ? "opacity-50" : undefined}>
                    <TD className={cn("font-medium", cancelled && "line-through")}>{m.order}. {m.name}</TD>
                    <TD className="text-text-secondary">{m.category || "Tanpa Kategori"}</TD>
                    <TD>
                      <Select
                        value={m.status}
                        onChange={(e) => void updateStatus(m.id, e.target.value as MilestoneStatus)}
                        className="h-8 w-40 text-[13px]"
                      >
                        {STATUS_OPTIONS.map((s) => (
                          <option key={s} value={s}>{s}</option>
                        ))}
                      </Select>
                    </TD>
                    <TD className={overdue ? "font-semibold text-danger" : undefined}>
                      {formatDate(m.targetDate)}
                      {overdue && " · terlambat"}
                    </TD>
                    <TD>{formatDate(m.completedDate)}</TD>
                    <TD>
                      {evidenceFor(m).length > 0 ? (
                        <span className="inline-flex items-center gap-1.5">
                          <span className="text-[12.5px] font-medium text-text-primary">{evidenceFor(m).length}</span>
                          <IconActionButton icon={Eye} label="Lihat Lampiran" tone="info" onClick={() => setEditingMilestone(m)} />
                        </span>
                      ) : (
                        <span className="text-text-secondary">-</span>
                      )}
                    </TD>
                    <TD>
                      <div className="flex items-center gap-1">
                        {!cancelled && !isFilterActive && (
                          <>
                            <IconActionButton
                              icon={ArrowUp}
                              label="Naikkan urutan"
                              tone="neutral"
                              disabled={activeIdx <= 0}
                              onClick={() => void handleMove(m.id, "up")}
                            />
                            <IconActionButton
                              icon={ArrowDown}
                              label="Turunkan urutan"
                              tone="neutral"
                              disabled={activeIdx === -1 || activeIdx >= activeOrder.length - 1}
                              onClick={() => void handleMove(m.id, "down")}
                            />
                          </>
                        )}
                        <IconActionButton icon={Pencil} label="Edit timeline" tone="neutral" onClick={() => setEditingMilestone(m)} />
                        {cancelled ? (
                          <IconActionButton
                            icon={CheckCircle2}
                            label="Aktifkan kembali"
                            tone="success"
                            onClick={() => void updateStatus(m.id, "Not Started")}
                          />
                        ) : (
                          <IconActionButton
                            icon={Ban}
                            label="Batalkan timeline"
                            tone="danger"
                            onClick={() => void updateStatus(m.id, "Cancelled")}
                          />
                        )}
                      </div>
                    </TD>
                  </TR>
                );
              })}
            </TBody>
          </Table>
          </div>
          <Pagination
            page={page}
            totalPages={totalPages}
            totalItems={totalItems}
            pageSize={pageSize}
            onPageChange={setPage}
            className="-mx-5 -mb-4 mt-1"
          />
          </>
          )}
        </CardContent>
      </Card>

      <ProjectMilestoneFormModal open={addOpen} onClose={() => setAddOpen(false)} onSubmit={(values) => void handleAddMilestone(values)} />

      {editingMilestone && (
        <ProjectMilestoneEditModal
          key={editingMilestone.id}
          open
          onClose={() => setEditingMilestone(null)}
          milestone={editingMilestone}
          projectId={projectId}
          evidenceList={evidenceFor(editingMilestone)}
          onSave={(fields) => void handleEditSave(fields)}
          onAddEvidence={handleAddEvidence}
        />
      )}
    </div>
  );
}
