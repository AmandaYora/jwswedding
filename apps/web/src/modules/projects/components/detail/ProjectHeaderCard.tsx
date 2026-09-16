import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Pencil, AlertTriangle, Archive, ArchiveRestore, ArrowUpRight } from "lucide-react";
import { Card, CardContent } from "@/shared/components/ui/Card";
import { Button } from "@/shared/components/ui/Button";
import { Badge } from "@/shared/components/ui/Badge";
import { ProgressMeter } from "@/shared/components/ui/ProgressMeter";
import { ProjectStatusBadge, ConditionBadge } from "@/modules/projects/components/StatusBadges";
import { ProjectFormModal } from "@/modules/projects/components/ProjectFormModal";
import type { ProjectFormValues } from "@/modules/projects/schemas/project.schema";
import { useProjectStore, type ProjectDeleteImpact } from "@/modules/projects/stores/useProjectStore";
import { useStaffStore } from "@/modules/users/stores/useStaffStore";
import { ROLE_LABELS } from "@/modules/users/types";
import { useAuthStore } from "@/shared/stores/useAuthStore";
import { daysUntil } from "@/modules/projects/lib/dates";
import { formatCurrency, formatDate } from "@/shared/lib/formatters";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { ConfirmDialog } from "@/shared/components/ui/ConfirmDialog";
import { BudgetMeter } from "@/modules/projects/components/BudgetMeter";
import { activeVendorCost, budgetStateOf } from "@/modules/projects/lib/budget";

export function ProjectHeaderCard({ projectId }: { projectId: string }) {
  const navigate = useNavigate();
  const project = useProjectStore((s) => s.currentProject);
  const milestones = useProjectStore((s) => s.milestones);
  const vendorMilestones = useProjectStore((s) => s.vendorMilestones);
  const vendorEngagements = useProjectStore((s) => s.vendorEngagements);
  const clientPayments = useProjectStore((s) => s.clientPayments);
  const fetchMilestones = useProjectStore((s) => s.fetchMilestones);
  const fetchVendorSection = useProjectStore((s) => s.fetchVendorSection);
  const fetchClientPayments = useProjectStore((s) => s.fetchClientPayments);
  const updateProject = useProjectStore((s) => s.updateProject);
  const cancelProject = useProjectStore((s) => s.cancelProject);
  const toggleArchiveProject = useProjectStore((s) => s.toggleArchiveProject);
  const deleteProject = useProjectStore((s) => s.deleteProject);
  const fetchProjectDeleteImpact = useProjectStore((s) => s.fetchProjectDeleteImpact);
  const staff = useStaffStore((s) => s.staffSummaries);
  const fetchStaff = useStaffStore((s) => s.fetchStaffSummaries);
  const isOwner = useAuthStore((s) => s.session?.role === "Owner");
  const role = useAuthStore((s) => s.session?.role);
  // canEditGeneral gates every field in the Edit modal except Status and
  // Deskripsi — confirmed role rule, PLAN.md mom-25082026-item-sebagian §3c.
  const canEditGeneral = role === "Owner" || role === "Admin";
  // Margin/Keuntungan reveals vendor-cost/venue-cost business data — Wedding
  // Planner is not supposed to see this specific computed figure (confirmed
  // RBAC rule), even though the underlying vendor engagements and venue cost
  // snapshot it's derived from are already visible to every role via the
  // Vendor/Venue tabs themselves (no separate fetch to gate here anymore —
  // both now come from data already loaded for this project).
  const canSeeMargin = useAuthStore((s) => s.session?.role) !== "Staff";
  // Tautan ke penawaran menggantikan tab "Paket & PO", yang isinya sudah
  // tinggal ringkasan baca-saja plus tombol "Buka di Penawaran". Daftar peran
  // di sini WAJIB sama dengan dua gerbang yang sebenarnya menentukan:
  // `RequireRole` pada rute /quotations/:id dan `requireQuotationManager` di
  // backend. Wedding Planner tidak melihat tautan ini sama sekali — tab lama
  // tampil untuknya lalu selalu menjawab 403, dan dialah peran yang paling
  // sering membuka halaman ini.
  const canSeeQuotation = role === "Owner" || role === "Admin" || role === "Sales";

  const [editOpen, setEditOpen] = useState(false);
  const [confirmingCancel, setConfirmingCancel] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleteImpact, setDeleteImpact] = useState<ProjectDeleteImpact | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    void fetchMilestones(projectId);
    void fetchVendorSection(projectId);
    void fetchClientPayments(projectId);
    void fetchStaff();
  }, [projectId, fetchMilestones, fetchVendorSection, fetchClientPayments, fetchStaff]);

  if (!project) {
    return <div className="text-sm text-text-secondary">Project tidak ditemukan.</div>;
  }

  // Margin/Keuntungan = sisa anggaran: Nilai Kontrak − biaya vendor aktif −
  // biaya venue. Rumusnya pindah ke lib/budget agar form vendor bisa memakai
  // yang sama persis saat memproyeksikan komitmen yang sedang diketik; biaya
  // venue tetap dibaca dari snapshot milik project, tidak pernah dari harga
  // master venue yang hidup (PLAN.md "Financial Calculation Correctness").
  const budget = budgetStateOf(project, activeVendorCost(vendorEngagements));

  // Sisa Tagihan Client (PLAN.md §1.7/§3.9) — day-to-day operational status
  // ("has the client paid this installment"), visible to every role that
  // already sees Nilai Kontrak, unlike Margin/Keuntungan above.
  const totalReceived = clientPayments.reduce(
    (sum, p) => (p.type === "Refund" ? sum - p.amount : sum + p.amount),
    0
  );
  const outstanding = project.contractValue - totalReceived;

  const progress = project.progress;
  const segments = [...milestones, ...vendorMilestones];
  const totalMilestones = (progress?.projectMilestoneStats.total ?? 0) + (progress?.vendorMilestoneStats.total ?? 0);
  const completedMilestones = (progress?.projectMilestoneStats.completed ?? 0) + (progress?.vendorMilestoneStats.completed ?? 0);
  const pic = staff.find((s) => s.id === project.picStaffId);
  const picSales = staff.find((s) => s.id === project.picSalesStaffId);
  const d = daysUntil(project.eventDate);
  const isOpenProject = project.status !== "Completed" && project.status !== "Cancelled";

  async function handleEdit(values: ProjectFormValues) {
    setActionError(null);
    try {
      await updateProject(projectId, values);
      setEditOpen(false);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal menyimpan perubahan project"));
    }
  }

  async function handleCancelProject() {
    setActionError(null);
    try {
      await cancelProject(projectId);
      setConfirmingCancel(false);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal membatalkan project"));
    }
  }

  async function handleToggleArchive() {
    setActionError(null);
    try {
      await toggleArchiveProject(projectId);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal mengubah status arsip project"));
    }
  }

  async function handleDeleteProject() {
    setActionError(null);
    setIsDeleting(true);
    try {
      await deleteProject(projectId);
      navigate(ROUTE_PATHS.projects);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal menghapus project secara permanen"));
      setIsDeleting(false);
    }
  }

  // D14: dialog hapus menolak tampil kalau dampaknya gagal dibaca.
  async function openDeleteConfirm() {
    setActionError(null);
    try {
      const impact = await fetchProjectDeleteImpact(projectId);
      setDeleteImpact(impact);
      setConfirmingDelete(true);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal membaca dampak penghapusan"));
    }
  }

  return (
    <Card>
      <CardContent className="flex flex-col gap-5 py-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-xl font-bold text-text-primary">{project.name}</h1>
              <ProjectStatusBadge status={project.status} />
              {project.isArchived && <Badge tone="neutral">Diarsipkan</Badge>}
            </div>
            <p className="mt-1 text-[13px] text-text-secondary">
              {project.brideName} &amp; {project.groomName} · {project.venue}
            </p>
          </div>
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            <Button variant="secondary" size="sm" icon={<Pencil className="h-3.5 w-3.5" />} onClick={() => setEditOpen(true)}>
              Ubah Project
            </Button>
            <Button
              variant="secondary"
              size="sm"
              icon={project.isArchived ? <ArchiveRestore className="h-3.5 w-3.5" /> : <Archive className="h-3.5 w-3.5" />}
              onClick={() => void handleToggleArchive()}
            >
              {project.isArchived ? "Pulihkan dari Arsip" : "Arsipkan"}
            </Button>
            {isOpenProject && (
              <Button variant="danger" size="sm" onClick={() => setConfirmingCancel(true)}>
                Batalkan Project
              </Button>
            )}
            {isOwner && (
              <Button variant="danger" size="sm" onClick={() => void openDeleteConfirm()}>
                Hapus Permanen
              </Button>
            )}
          </div>
        </div>

        {actionError && (
          <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{actionError}</p>
        )}

        {/* Penawaran yang ditarik kembali ke Draft untuk direvisi membuat Nilai
            Kontrak di bawah menjadi angka yang BELUM disepakati klien. Selama
            itu, penerbitan Tagihan ditahan backend — dan alasannya harus
            terbaca di sini, bukan baru muncul sebagai galat saat orang sudah
            mengisi form tagihan. */}
        {project.quotationUnderRevision && (
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-warning/40 bg-warning-soft px-4 py-3">
            <span className="flex items-center gap-2 text-[13px] font-medium text-warning-strong">
              <AlertTriangle className="h-4 w-4 shrink-0" />
              Penawaran project ini sedang direvisi dan belum dikirim ulang. Nilai Kontrak di bawah belum disepakati klien, dan penerbitan
              Tagihan ditahan sampai penawarannya dikirim dan diterima kembali.
            </span>
            {canSeeQuotation && project.poNumber !== null && (
              <Link
                to={ROUTE_PATHS.quotationDetail(project.quotationId)}
                className="inline-flex shrink-0 items-center gap-1 text-[12.5px] font-semibold text-warning-strong underline-offset-2 hover:underline"
              >
                Buka Penawaran
                <ArrowUpRight className="h-3.5 w-3.5" />
              </Link>
            )}
          </div>
        )}

        <div className="grid grid-cols-2 gap-4 border-t border-border-light pt-4 sm:grid-cols-3 lg:grid-cols-6">
          <InfoField label="Tanggal Acara" value={formatDate(project.eventDate)} />
          <InfoField label="Jam Acara" value={formatEventHours(project.eventStartTime, project.eventEndTime)} />
          <InfoField
            label="Countdown"
            value={isOpenProject ? (d >= 0 ? `H-${d}` : `H+${Math.abs(d)}`) : project.status === "Completed" ? "Selesai" : "Dibatalkan"}
            emphasize
          />
          <InfoField label="Tanggal Booking" value={formatDate(project.prepStartDate)} />
          <InfoField label="Paket / Layanan" value={project.packageName} />
          <InfoField label="Nilai Kontrak" value={formatCurrency(project.contractValue)} />
          {/* Digerbangi poNumber, BUKAN quotationId: sebuah project bisa
              menyimpan quotation_id yang barisnya sudah tidak ada (itulah yang
              dulu membuat tab lama menjawab "Penawaran tidak ditemukan"), dan
              menautkannya hanya memindahkan 404 itu satu klik lebih jauh.
              Backend sudah membedakan keduanya — null berarti tidak ada yang
              bisa dituju, "" berarti ada tapi belum bernomor. */}
          {canSeeQuotation && project.poNumber !== null && (
            <InfoField
              label="Penawaran"
              value={project.poNumber || "Tanpa nomor"}
              to={ROUTE_PATHS.quotationDetail(project.quotationId)}
            />
          )}
          <InfoField label="Sisa Tagihan Client" value={formatCurrency(outstanding)} />
          <InfoField label={ROLE_LABELS.Staff} value={pic?.name ?? "Belum ditugaskan"} />
          <InfoField label={ROLE_LABELS.Sales} value={picSales?.name ?? "Belum ditugaskan"} />
        </div>

        {/* Menggantikan field "Margin/Keuntungan" yang dulu berdiri sebagai
            satu angka di grid di atas. Keduanya bilangan yang sama persis —
            Nilai Kontrak dikurangi seluruh biaya — jadi menampilkan dua-duanya
            hanya menyajikan angka yang sama dengan dua nama berbeda. Yang
            bertahan adalah yang memberi tahu lebih banyak: berapa yang sudah
            dikomitmenkan, dari berapa, dan berapa sisanya. Digerbangi peran
            yang sama seperti sebelumnya. */}
        {canSeeMargin && (
          <BudgetMeter
            budget={budget}
            quotationHref={
              canSeeQuotation && project.poNumber !== null ? ROUTE_PATHS.quotationDetail(project.quotationId) : undefined
            }
          />
        )}

        {project.description && (
          <p className="rounded-md bg-surface-muted px-4 py-3 text-[13px] text-text-secondary">{project.description}</p>
        )}

        {progress && (
          <div className="rounded-md bg-surface-muted/60 p-4">
            <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-3">
                <span className="text-2xl font-bold leading-none tabular-nums text-navy-900">{progress.overallPercent}%</span>
                <ConditionBadge condition={progress.condition} />
              </div>
              <span className="text-[12.5px] text-text-secondary">
                Timeline project {progress.projectMilestoneStats.completed}/{progress.projectMilestoneStats.total} · Timeline vendor{" "}
                {progress.vendorMilestoneStats.completed}/{progress.vendorMilestoneStats.total}
                {progress.overdueMilestoneCount > 0 && (
                  <span className="font-semibold text-danger"> · {progress.overdueMilestoneCount} terlambat</span>
                )}
              </span>
            </div>
            <ProgressMeter
              percent={progress.overallPercent}
              segments={segments}
              caption={`${completedMilestones}/${totalMilestones} timeline selesai berdasarkan pencapaian nyata — bukan estimasi manual.`}
            />
          </div>
        )}
      </CardContent>

      <ProjectFormModal
        open={editOpen}
        onClose={() => setEditOpen(false)}
        onSubmit={(values) => void handleEdit(values)}
        initialProject={project}
        canEditGeneral={canEditGeneral}
      />

      <ConfirmDialog
        open={confirmingCancel}
        onClose={() => setConfirmingCancel(false)}
        onConfirm={() => void handleCancelProject()}
        title="Batalkan Project"
        message="Yakin ingin membatalkan project ini?"
        details="Data yang sudah ada tidak akan dihapus — project hanya ditandai Dibatalkan dan berhenti dihitung sebagai project berjalan."
        confirmLabel="Ya, Batalkan"
      />

      {/* Dialog hapus hanya dirender setelah dampaknya terbaca (D14):
          deleteImpact yang masih null berarti angkanya belum ada, dan
          persetujuan atas penghapusan permanen tidak boleh diminta tanpa
          menyebut apa yang ikut hilang. */}
      <ConfirmDialog
        open={confirmingDelete && deleteImpact !== null}
        onClose={() => {
          setConfirmingDelete(false);
          setDeleteImpact(null);
        }}
        onConfirm={() => void handleDeleteProject()}
        title="Hapus Project Permanen"
        message={
          <>
            Yakin ingin menghapus <strong>{project.name}</strong> secara permanen?
          </>
        }
        details={
          <>
            <p>
              Seluruh timeline, vendor, pembayaran, kendala, dan evidence ikut terhapus dan tidak dapat dipulihkan.
              {deleteImpact?.poNumber
                ? ` Penawaran ${deleteImpact.poNumber} ikut terhapus.`
                : " Penawaran terkait (bila ada) ikut terhapus."}
            </p>
            {(deleteImpact?.paidInvoiceCount ?? 0) > 0 && (
              <p className="mt-1.5 font-semibold text-danger">
                Termasuk {deleteImpact?.paidInvoiceCount} tagihan yang sudah lunas senilai{" "}
                {formatCurrency(deleteImpact?.paidInvoiceTotal ?? 0)}.
              </p>
            )}
          </>
        }
        confirmLabel="Ya, Hapus Permanen"
        busyLabel="Menghapus..."
        busy={isDeleting}
      />
    </Card>
  );
}

// formatEventHours renders the project-level Jam Acara as "08.00 - 13.00"
// (dot separators, matching the display in gambar 1), or "Belum ditentukan"
// when either bound is unset — same empty-sentinel convention as PIC fields.
function formatEventHours(start: string | null, end: string | null): string {
  if (!start || !end) return "Belum ditentukan";
  return `${start.replace(":", ".")} - ${end.replace(":", ".")}`;
}

function InfoField({
  label,
  value,
  emphasize,
  to,
}: {
  label: string;
  value: string;
  emphasize?: boolean;
  /** Merender nilainya sebagai tautan internal, bukan teks biasa. */
  to?: string;
}) {
  return (
    <div className="min-w-0">
      <p className="text-[11.5px] font-medium uppercase tracking-wide text-text-secondary">{label}</p>
      {to ? (
        <Link
          to={to}
          className="mt-0.5 inline-flex max-w-full items-center gap-1 text-[13.5px] font-semibold text-navy-900 underline-offset-2 hover:underline"
        >
          <span className="truncate">{value}</span>
          <ArrowUpRight className="h-3.5 w-3.5 shrink-0" />
        </Link>
      ) : (
        <p
          className={
            emphasize
              ? "mt-0.5 text-[15px] font-bold tabular-nums text-navy-900"
              : "mt-0.5 text-[13.5px] font-medium text-text-primary"
          }
        >
          {value}
        </p>
      )}
    </div>
  );
}
