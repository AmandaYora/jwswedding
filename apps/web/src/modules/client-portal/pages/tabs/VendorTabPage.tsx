import { useEffect, useState } from "react";
import { useOutletContext } from "react-router-dom";
import { ChevronDown, Users, FileText, AlertTriangle, Building2, MapPin } from "lucide-react";
import { ClientEngagementStatusBadge, ClientMilestoneStatusBadge } from "@/modules/client-portal/components/ClientBadges";
import { MilestoneRail } from "@/shared/components/ui/MilestoneRail";
import { ClientEvidenceViewerModal } from "@/modules/client-portal/components/ClientEvidenceViewerModal";
import { IssueCard } from "@/modules/client-portal/components/IssueCard";
import { PortalEmpty, PortalError, PortalLoading } from "@/modules/client-portal/components/PortalState";
import { usePortalSections } from "@/modules/client-portal/hooks/usePortalSections";
import { clientEvidenceTypeLabel } from "@/modules/client-portal/lib/labels";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import { useVendorStore } from "@/modules/vendors/stores/useVendorStore";
import { useVendorCategoryStore } from "@/modules/vendor-categories/stores/useVendorCategoryStore";
import type { ProjectVendor, VendorMilestone, VendorIssue, Evidence, ProjectVenueSummary } from "@/modules/projects/types";
import { formatDate } from "@/shared/lib/formatters";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import type { ClientPortalContext } from "@/modules/client-portal/layouts/ClientPortalLayout";

// Venue merged into this same tab (client-facing only — WO Console keeps its
// own separate Vendor/Venue tabs, and the underlying data model is
// untouched). Venue has no engagement/milestone/issue concept at all, so it
// renders as one pinned, full-width card ahead of the vendor grid rather
// than pretending to have progress it doesn't have — see VenueCard below.
export default function VendorTabPage() {
  const { projectId } = useOutletContext<ClientPortalContext>();
  const vendorEngagements = useProjectStore((s) => s.vendorEngagements);
  const vendorMilestones = useProjectStore((s) => s.vendorMilestones);
  const issues = useProjectStore((s) => s.issues);
  const evidence = useProjectStore((s) => s.evidence);
  const fetchProjectVenueSummary = useProjectStore((s) => s.fetchProjectVenueSummary);
  // Public-safe {id, name} only (ADR-0016-style split) — every other vendor
  // field (PIC, phone, email, price, lampiran) is staff-only now, and this
  // tab never needed more than the name anyway (category name comes from
  // the separate `vendor-categories` store below).
  const vendors = useVendorStore((s) => s.vendorSummaries);
  const fetchVendorSummaries = useVendorStore((s) => s.fetchVendorSummaries);
  const categories = useVendorCategoryStore((s) => s.categories);
  const fetchCategories = useVendorCategoryStore((s) => s.fetchCategories);
  const [viewingEvidence, setViewingEvidence] = useState<Evidence | null>(null);

  const [venue, setVenue] = useState<ProjectVenueSummary | null>(null);
  const [venuePhotoUrl, setVenuePhotoUrl] = useState<string | null>(null);
  const [sideLoading, setSideLoading] = useState(true);

  // The project-scoped slices go through the shared hook, so the persistent
  // ClientProjectHeaderCard above and this tab issue one request each rather
  // than two. Venue + the vendor/category name lookups are not project-scoped
  // (or return a value instead of storing one), so they stay local.
  const { loading: sectionsLoading, error, reload } = usePortalSections(projectId, ["vendors", "issues", "evidence"]);

  useEffect(() => {
    let cancelled = false;
    let createdUrl: string | null = null;
    setSideLoading(true);
    // Cleared up front: the previous project's photo lives in state as an
    // object URL that the cleanup below has already revoked, so leaving it
    // there renders a broken image until the new one arrives.
    setVenuePhotoUrl(null);

    const venueLoad = fetchProjectVenueSummary(projectId).then(async (summary) => {
      if (cancelled) return;
      setVenue(summary);
      if (!summary || !summary.hasVisibleAttachment) return;
      try {
        const res = await httpClient.get(API.venues.attachment(summary.id), { responseType: "blob" });
        if (cancelled) return;
        createdUrl = URL.createObjectURL(res.data as Blob);
        setVenuePhotoUrl(createdUrl);
      } catch {
        // Swallow -- the venue card still renders fine without a photo.
      }
    });

    void Promise.all([venueLoad, fetchVendorSummaries(), fetchCategories()])
      .catch(() => {
        // Names degrade to "Vendor tidak diketahui"; not worth blocking the
        // whole tab, which has its own error path for the data that matters.
      })
      .finally(() => {
        if (!cancelled) setSideLoading(false);
      });

    return () => {
      cancelled = true;
      if (createdUrl) URL.revokeObjectURL(createdUrl);
    };
  }, [projectId, fetchProjectVenueSummary, fetchVendorSummaries, fetchCategories]);

  const loading = sectionsLoading || sideLoading;
  const isEmpty = !venue && vendorEngagements.length === 0;

  return (
    <div className="flex flex-col gap-6 sm:gap-8">
      <section>
        <div className="mb-4 flex items-center gap-2 sm:mb-5">
          <Users className="h-5 w-5 shrink-0 text-navy-900" />
          <h2 className="text-base font-bold text-text-primary sm:text-lg">
            Venue &amp; Vendor yang Mempersiapkan Hari Bahagia Anda
          </h2>
        </div>
        <p className="mb-5 max-w-2xl text-[13px] leading-relaxed text-text-secondary sm:mb-6 sm:text-[13.5px]">
          Venue dan seluruh vendor yang bekerja untuk mewujudkan hari bahagia Anda, beserta progress tahapan kerja
          masing-masing yang benar-benar telah diselesaikan oleh tim kami — lengkap dengan dokumen pendukungnya.
        </p>

        {loading ? (
          <PortalLoading label="Memuat venue dan vendor..." />
        ) : error ? (
          <PortalError message={error} onRetry={reload} />
        ) : isEmpty ? (
          <PortalEmpty
            icon={Users}
            title="Venue dan vendor belum ditentukan"
            description="Begitu tim kami mengunci venue dan vendor untuk pernikahan Anda, semuanya akan muncul di sini."
          />
        ) : (
          <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
            {venue ? (
              <VenueCard venue={venue} photoUrl={venuePhotoUrl} />
            ) : (
              vendorEngagements.length > 0 && <VenuePlaceholderCard />
            )}

            {vendorEngagements.map((pv) => (
              <VendorCard
                key={pv.id}
                projectVendor={pv}
                vendorName={vendors.find((v) => v.id === pv.vendorId)?.name ?? "Vendor tidak diketahui"}
                categoryName={categories.find((c) => c.id === pv.categoryId)?.name ?? "Kategori tidak diketahui"}
                milestones={vendorMilestones.filter((m) => m.projectVendorId === pv.id)}
                issues={issues.filter((i) => i.projectVendorId === pv.id)}
                evidence={evidence}
                onViewEvidence={setViewingEvidence}
              />
            ))}
          </div>
        )}
      </section>

      {viewingEvidence && (
        <ClientEvidenceViewerModal
          evidence={viewingEvidence}
          projectId={projectId}
          onClose={() => setViewingEvidence(null)}
        />
      )}
    </div>
  );
}

// Pinned ahead of the vendor grid, spanning both columns -- venue has no
// engagement status, milestones, or issues in the data model, so this
// deliberately doesn't reuse VendorCard's progress rail/status badge/
// accordion slots, only its outer card shell (rounded-3xl border shadow-sm)
// so the two read as one visual family. "Venue Pernikahan" fills the
// pill-badge slot a vendor card would use for its category.
function VenueCard({ venue, photoUrl }: { venue: ProjectVenueSummary; photoUrl: string | null }) {
  const hasExtra = Boolean(photoUrl || venue.facilities || venue.socialMedia);
  return (
    <div className="flex flex-col overflow-hidden rounded-3xl border border-border bg-white shadow-sm transition-all hover:shadow-xl hover:shadow-navy-900/5 sm:flex-row md:col-span-2">
      <div className="flex shrink-0 flex-col gap-3 border-b border-border/50 bg-gradient-to-br from-surface-muted/50 to-white p-5 sm:w-[320px] sm:border-b-0 sm:border-r sm:p-6">
        <div>
          <h3 className="text-[16px] font-bold text-navy-950 sm:text-[18px]">{venue.name}</h3>
          <p className="mt-1.5 inline-flex items-center gap-1.5 rounded-full border border-border bg-white px-2.5 py-0.5 text-[13px] font-medium text-text-secondary sm:text-[13.5px]">
            <span className="h-1.5 w-1.5 rounded-full bg-navy-900" /> Venue Pernikahan
          </p>
        </div>
        <div className="flex flex-col gap-1.5">
          <p className="flex items-start gap-1.5 text-[13px] text-text-secondary sm:text-[13.5px]">
            <MapPin className="h-3.5 w-3.5 shrink-0 translate-y-0.5" />{" "}
            {[venue.address, venue.city].filter(Boolean).join(", ") || "-"}
          </p>
          {venue.capacity !== null && (
            <p className="flex items-center gap-1.5 text-[13px] text-text-secondary sm:text-[13.5px]">
              <Users className="h-3.5 w-3.5 shrink-0" /> Kapasitas {venue.capacity} orang
            </p>
          )}
        </div>
      </div>

      <div className="flex min-w-0 flex-1 flex-col gap-5 p-5 sm:p-6">
        {photoUrl && <img src={photoUrl} alt={venue.name} className="max-h-56 w-full rounded-xl object-cover" />}

        {venue.facilities && (
          <div>
            <p className="mb-1.5 text-[12px] font-semibold text-text-secondary">Fasilitas</p>
            <p className="whitespace-pre-line text-[13px] text-text-primary">{venue.facilities}</p>
          </div>
        )}

        {venue.socialMedia && (
          <div>
            <p className="mb-1.5 text-[12px] font-semibold text-text-secondary">Sosial Media</p>
            <p className="whitespace-pre-line break-words text-[13px] text-text-primary">{venue.socialMedia}</p>
          </div>
        )}

        {!hasExtra && <p className="text-[13px] text-text-secondary">Tidak ada informasi tambahan untuk venue ini.</p>}
      </div>
    </div>
  );
}

// Shown in the venue's pinned slot only when at least one vendor already
// exists but no venue has been picked yet -- a visible "not decided yet"
// placeholder, not a silently missing card, so the client isn't left
// wondering whether it was simply forgotten.
function VenuePlaceholderCard() {
  return (
    <div className="rounded-3xl border border-dashed border-border bg-white p-8 text-center shadow-sm md:col-span-2">
      <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-surface-muted">
        <Building2 className="h-6 w-6 text-text-secondary opacity-50" />
      </div>
      <p className="text-[13.5px] font-medium text-text-secondary">Venue untuk pernikahan Anda belum ditentukan.</p>
    </div>
  );
}

function VendorCard({
  projectVendor,
  vendorName,
  categoryName,
  milestones,
  issues,
  evidence,
  onViewEvidence,
}: {
  projectVendor: ProjectVendor;
  vendorName: string;
  categoryName: string;
  milestones: VendorMilestone[];
  issues: VendorIssue[];
  evidence: Evidence[];
  onViewEvidence: (evidence: Evidence) => void;
}) {
  // Cancelled tahapan are hidden from the client everywhere on this card.
  // The rail already filtered them; the "Lihat rincian tahapan" list did
  // NOT, so a vendor with cancelled work showed e.g. "3 dari 5 tahapan
  // selesai" above a list of 7 rows, two of them cancelled.
  const relevantMilestones = milestones.filter((m) => m.status !== "Cancelled");
  const completedCount = relevantMilestones.filter((m) => m.status === "Completed").length;

  const sortedIssues = [...issues].sort((a, b) => {
    const aOpen = a.status !== "Resolved" && a.status !== "Closed";
    const bOpen = b.status !== "Resolved" && b.status !== "Closed";
    if (aOpen !== bOpen) return aOpen ? -1 : 1;
    return a.foundDate < b.foundDate ? 1 : -1;
  });
  const openIssueCount = issues.filter((i) => i.status !== "Resolved" && i.status !== "Closed").length;

  return (
    <div className="flex flex-col overflow-hidden rounded-3xl border border-border bg-white shadow-sm transition-all hover:-translate-y-1 hover:shadow-xl hover:shadow-navy-900/5">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-border/50 bg-gradient-to-br from-surface-muted/50 to-white p-5 sm:p-6">
        <div className="min-w-0">
          <h3 className="text-[16px] font-bold text-navy-950 sm:text-[18px]">{vendorName}</h3>
          <p className="mt-1 inline-flex flex-wrap items-center gap-1.5 rounded-full border border-border bg-white px-2.5 py-0.5 text-[13px] font-medium text-text-secondary sm:text-[13.5px]">
            <span className="h-1.5 w-1.5 rounded-full bg-blue-400"></span> {categoryName}
            {projectVendor.scope && (
              <>
                <span className="mx-1 text-border">|</span> {projectVendor.scope}
              </>
            )}
          </p>
        </div>
        <ClientEngagementStatusBadge status={projectVendor.engagementStatus} />
      </div>

      <div className="flex flex-col p-5 sm:p-6">
        {openIssueCount > 0 && (
          <div className="mb-5 flex items-center gap-2 rounded-xl border border-warning/20 bg-warning-soft/50 px-3.5 py-2.5 text-[13px] font-semibold text-warning-strong shadow-sm">
            <AlertTriangle className="h-4 w-4 shrink-0" />
            <span>
              {openIssueCount} kendala aktif sedang kami tangani
            </span>
          </div>
        )}

        <div>
          {relevantMilestones.length === 0 ? (
            <p className="rounded-xl border border-dashed border-border bg-surface py-4 text-center text-[13px] text-text-secondary sm:text-[13.5px]">
              Belum ada progress yang tercatat untuk vendor ini.
            </p>
          ) : (
            <div className="rounded-2xl border border-border bg-surface/30 p-4">
              <MilestoneRail milestones={relevantMilestones} size="md" showCount={false} wrap />
              <p className="mt-3 text-center text-[13px] font-medium text-navy-900">
                {completedCount} dari {relevantMilestones.length} tahapan selesai
              </p>
            </div>
          )}
        </div>

        {relevantMilestones.length > 0 && (
          <details className="group mt-5 overflow-hidden rounded-2xl border border-border bg-white shadow-sm [&_summary::-webkit-details-marker]:hidden">
            <summary className="flex cursor-pointer items-center justify-between bg-surface-muted/30 px-4 py-3 text-[13.5px] font-bold text-navy-950 transition-colors hover:bg-surface-muted/60">
              Lihat rincian tahapan
              <span className="flex h-6 w-6 items-center justify-center rounded-full border border-border bg-white text-navy-900 transition-transform group-open:rotate-180">
                <ChevronDown className="h-4 w-4 shrink-0" />
              </span>
            </summary>
            <ul className="flex flex-col gap-3 border-t border-border bg-surface-muted/10 p-4">
              {relevantMilestones.map((m) => (
                <MilestoneRow
                  key={m.id}
                  milestone={m}
                  evidence={evidence.filter((e) => e.relatedKind === "vendorMilestone" && e.relatedId === m.id)}
                  onViewEvidence={onViewEvidence}
                />
              ))}
            </ul>
          </details>
        )}

        {sortedIssues.length > 0 && (
          <details className="group mt-3 overflow-hidden rounded-2xl border border-warning/30 bg-white shadow-sm [&_summary::-webkit-details-marker]:hidden">
            <summary className="flex cursor-pointer items-center justify-between bg-warning-soft/30 px-4 py-3 text-[13.5px] font-bold text-warning-strong transition-colors hover:bg-warning-soft/60">
              Lihat detail kendala ({sortedIssues.length})
              <span className="flex h-6 w-6 items-center justify-center rounded-full border border-warning/30 bg-white text-warning-strong transition-transform group-open:rotate-180">
                <ChevronDown className="h-4 w-4 shrink-0" />
              </span>
            </summary>
            <div className="flex flex-col gap-4 border-t border-warning/20 bg-warning-soft/10 p-4">
              {sortedIssues.map((issue) => (
                <IssueCard
                  key={issue.id}
                  issue={issue}
                  evidence={evidence.filter((e) => e.relatedKind === "issue" && e.relatedId === issue.id)}
                  onViewEvidence={onViewEvidence}
                  showVendorName={false}
                  vendorName={vendorName}
                  milestoneName={milestones.find((m) => m.id === issue.vendorMilestoneId)?.name}
                />
              ))}
            </div>
          </details>
        )}
      </div>
    </div>
  );
}

function MilestoneRow({
  milestone: m,
  evidence,
  onViewEvidence,
}: {
  milestone: VendorMilestone;
  evidence: Evidence[];
  onViewEvidence: (evidence: Evidence) => void;
}) {
  return (
    <li className="flex flex-col gap-2.5 rounded-xl border border-border bg-white px-4 py-3.5 shadow-sm transition-colors hover:border-navy-900/20">
      <div className="flex flex-col items-start gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
        <span className="text-[13.5px] font-semibold text-navy-950">{m.name}</span>
        <span className="flex flex-wrap items-center gap-2">
          <ClientMilestoneStatusBadge status={m.status} />
          <span className="whitespace-nowrap rounded-md bg-surface-muted px-2 py-1 text-[12px] font-medium text-text-secondary">
            {m.status === "Completed" ? `Selesai ${formatDate(m.completedDate)}` : `Target ${formatDate(m.targetDate)}`}
          </span>
        </span>
      </div>
      {evidence.length > 0 && (
        <div className="mt-1 flex flex-wrap items-center gap-2">
          <FileText className="h-3.5 w-3.5 shrink-0 text-text-tertiary" />
          {evidence.map((e) => (
            <button
              key={e.id}
              type="button"
              onClick={() => onViewEvidence(e)}
              className="group flex items-center gap-1.5 rounded-lg border border-border bg-surface px-2.5 py-1 text-[11.5px] font-semibold text-navy-900 transition-colors hover:border-navy-200 hover:bg-navy-50"
            >
              Lihat {clientEvidenceTypeLabel(e.type)}
            </button>
          ))}
        </div>
      )}
    </li>
  );
}
