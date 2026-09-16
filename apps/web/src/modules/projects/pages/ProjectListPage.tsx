import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Plus, X } from "lucide-react";
import { Button } from "@/shared/components/ui/Button";
import { SearchInput } from "@/shared/components/ui/SearchInput";
import { Select } from "@/shared/components/ui/Input";
import { MonthSelect } from "@/shared/components/ui/MonthSelect";
import { Pagination } from "@/shared/components/ui/Pagination";
import { EmptyState } from "@/shared/components/feedback/EmptyState";
import { AcceptProjectDialog } from "@/modules/quotations/components/AcceptProjectDialog";
import { ProjectCard } from "@/modules/projects/components/ProjectCard";
import { PROJECT_STATUS_OPTIONS } from "@/modules/projects/schemas/project.schema";
import { useProjectStore, type ProjectListFilters } from "@/modules/projects/stores/useProjectStore";
import { useStaffStore } from "@/modules/users/stores/useStaffStore";
import { ROLE_LABELS } from "@/modules/users/types";
import { staffOptionLabel, staffOptionsForRole } from "@/modules/users/lib/staff-label";
import { useAuthStore } from "@/shared/stores/useAuthStore";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { useDebouncedValue } from "@/shared/hooks/useDebouncedValue";
import { monthOptionsRange } from "@/shared/lib/month-options";
import { formatMonth } from "@/shared/lib/formatters";

// D7 (docs/plan/revisi-putri-lanjutan/PLAN.md): this page is server-side
// paginated, so options derived from the loaded rows (like
// monthOptionsFromDates elsewhere) would lie -- only whichever page happens
// to be open would be represented. A fixed window is used instead: 12 months
// back (a project can be Completed/in the past) to 24 months forward.
const EVENT_MONTH_OPTIONS = monthOptionsRange(12, 24);

export default function ProjectListPage() {
  const navigate = useNavigate();
  const role = useAuthStore((s) => s.session?.role);
  // Project baru SELALU lahir dari penawaran Diterima (D11) — Owner/Admin/
  // Sales membuka dialog Tambah Project (T3.7); Wedding Planner tidak.
  const canCreate = role === "Owner" || role === "Admin" || role === "Sales";
  // PIC/Sales filters mirror the backend gate (D6): for Staff/Sales the query
  // param is silently ignored server-side, so showing the control would only
  // mislead — it would look like a working filter that does nothing.
  const canFilterByPIC = role === "Owner" || role === "Admin";
  const projects = useProjectStore((s) => s.projectPage);
  const meta = useProjectStore((s) => s.projectPageMeta);
  const fetchProjectPage = useProjectStore((s) => s.fetchProjectPage);
  const staffSummaries = useStaffStore((s) => s.staffSummaries);
  const fetchStaffSummaries = useStaffStore((s) => s.fetchStaffSummaries);

  const [query, setQuery] = useState("");
  const debouncedQuery = useDebouncedValue(query);
  const [statusFilter, setStatusFilter] = useState<string>("Semua");
  const [showArchived, setShowArchived] = useState(false);
  const [picFilter, setPicFilter] = useState("");
  const [picSalesFilter, setPicSalesFilter] = useState("");
  const [eventMonthFilter, setEventMonthFilter] = useState("");
  const [page, setPage] = useState(1);
  const [acceptOpen, setAcceptOpen] = useState(false);

  useEffect(() => {
    if (canFilterByPIC) void fetchStaffSummaries();
  }, [canFilterByPIC, fetchStaffSummaries]);

  const filters: ProjectListFilters = useMemo(
    () => ({
      search: debouncedQuery,
      status: statusFilter === "Semua" ? "" : statusFilter,
      showArchived,
      picStaffId: picFilter,
      picSalesStaffId: picSalesFilter,
      eventMonth: eventMonthFilter,
    }),
    [debouncedQuery, statusFilter, showArchived, picFilter, picSalesFilter, eventMonthFilter]
  );

  useEffect(() => {
    setPage(1);
  }, [debouncedQuery, statusFilter, showArchived, picFilter, picSalesFilter, eventMonthFilter]);

  useEffect(() => {
    void fetchProjectPage(page, filters);
  }, [fetchProjectPage, page, filters]);

  // Chip filter aktif + Reset (D9) — inline here, not lifted to shared/: this
  // is the only screen with the pattern today (T-6).
  const activeChips = useMemo(() => {
    const chips: { key: string; label: string; onRemove: () => void }[] = [];
    if (debouncedQuery) chips.push({ key: "query", label: `Cari: "${debouncedQuery}"`, onRemove: () => setQuery("") });
    if (statusFilter !== "Semua") chips.push({ key: "status", label: `Status: ${statusFilter}`, onRemove: () => setStatusFilter("Semua") });
    if (picFilter) {
      const found = staffSummaries.find((s) => s.id === picFilter);
      const label = picFilter === "0" ? "Belum ditugaskan" : found ? staffOptionLabel(found) : picFilter;
      chips.push({ key: "pic", label: `${ROLE_LABELS.Staff}: ${label}`, onRemove: () => setPicFilter("") });
    }
    if (picSalesFilter) {
      const found = staffSummaries.find((s) => s.id === picSalesFilter);
      const label = picSalesFilter === "0" ? "Belum ditugaskan" : found ? staffOptionLabel(found) : picSalesFilter;
      chips.push({ key: "picSales", label: `${ROLE_LABELS.Sales}: ${label}`, onRemove: () => setPicSalesFilter("") });
    }
    if (eventMonthFilter) chips.push({ key: "eventMonth", label: `Bulan Event: ${formatMonth(eventMonthFilter)}`, onRemove: () => setEventMonthFilter("") });
    if (showArchived) chips.push({ key: "archived", label: "Diarsipkan", onRemove: () => setShowArchived(false) });
    return chips;
  }, [debouncedQuery, statusFilter, picFilter, picSalesFilter, eventMonthFilter, showArchived, staffSummaries]);

  function resetAllFilters() {
    setQuery("");
    setStatusFilter("Semua");
    setShowArchived(false);
    setPicFilter("");
    setPicSalesFilter("");
    setEventMonthFilter("");
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold text-text-primary">Project</h1>
          <p className="mt-1 text-[13px] text-text-secondary">Kelola seluruh project pernikahan yang sedang dan pernah ditangani.</p>
        </div>
        {canCreate && (
          <Button icon={<Plus className="h-4 w-4" />} onClick={() => setAcceptOpen(true)}>
            Tambah Project
          </Button>
        )}
      </div>

      <div className="flex flex-wrap gap-3">
        <SearchInput
          className="max-w-xs"
          placeholder="Cari nama project, pasangan, atau venue..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <Select className="w-48" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
          <option value="Semua">Semua Status</option>
          {PROJECT_STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </Select>
        {canFilterByPIC && (
          <Select className="w-48" value={picFilter} onChange={(e) => setPicFilter(e.target.value)}>
            <option value="">Semua {ROLE_LABELS.Staff}</option>
            <option value="0">Belum ditugaskan</option>
            {staffOptionsForRole(staffSummaries, "Staff").map((s) => (
              <option key={s.id} value={s.id}>{staffOptionLabel(s)}</option>
            ))}
          </Select>
        )}
        {canFilterByPIC && (
          <Select className="w-48" value={picSalesFilter} onChange={(e) => setPicSalesFilter(e.target.value)}>
            <option value="">Semua {ROLE_LABELS.Sales}</option>
            <option value="0">Belum ditugaskan</option>
            {staffOptionsForRole(staffSummaries, "Sales").map((s) => (
              <option key={s.id} value={s.id}>{staffOptionLabel(s)}</option>
            ))}
          </Select>
        )}
        <MonthSelect
          className="w-44"
          value={eventMonthFilter}
          onChange={setEventMonthFilter}
          options={EVENT_MONTH_OPTIONS}
          allLabel="Semua Bulan Event"
        />
        <label className="flex items-center gap-2 rounded-md border border-border px-3 text-[13px] text-text-secondary">
          <input type="checkbox" checked={showArchived} onChange={(e) => setShowArchived(e.target.checked)} />
          Tampilkan yang diarsipkan
        </label>
      </div>

      {activeChips.length > 0 && (
        <div className="flex flex-wrap items-center gap-2">
          {activeChips.map((chip) => (
            <button
              key={chip.key}
              type="button"
              onClick={chip.onRemove}
              className="inline-flex items-center gap-1.5 rounded-full border border-navy-200 bg-navy-50 px-3 py-1 text-[12.5px] font-medium text-navy-700 transition-colors hover:bg-navy-100"
            >
              {chip.label}
              <X className="h-3 w-3" />
            </button>
          ))}
          <button
            type="button"
            onClick={resetAllFilters}
            className="text-[12.5px] font-semibold text-text-secondary underline-offset-2 hover:text-text-primary hover:underline"
          >
            Hapus Semua
          </button>
        </div>
      )}

      {projects.length === 0 ? (
        <div className="rounded-lg border border-border bg-surface">
          <EmptyState
            title={showArchived ? "Tidak ada project yang diarsipkan" : "Tidak ada project ditemukan"}
            description="Ubah kata kunci pencarian atau filter status."
          />
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {projects.map((project) => (
              <ProjectCard key={project.id} project={project} />
            ))}
          </div>
          <div className="rounded-lg border border-border bg-surface">
            <Pagination page={meta.page} totalPages={meta.totalPages} totalItems={meta.total} pageSize={meta.limit} onPageChange={setPage} />
          </div>
        </>
      )}

      {acceptOpen && (
        <AcceptProjectDialog
          onClose={() => setAcceptOpen(false)}
          onAccepted={(newProjectId) => {
            setAcceptOpen(false);
            void fetchProjectPage(page, filters);
            navigate(ROUTE_PATHS.projectDetail(newProjectId));
          }}
        />
      )}
    </div>
  );
}
