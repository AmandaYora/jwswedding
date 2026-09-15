import { useEffect, useState } from "react";
import { useOutletContext } from "react-router-dom";
import { CheckCircle2, HeartHandshake } from "lucide-react";
import { ClientEvidenceViewerModal } from "@/modules/client-portal/components/ClientEvidenceViewerModal";
import { IssueCard } from "@/modules/client-portal/components/IssueCard";
import { PortalEmpty, PortalError, PortalLoading } from "@/modules/client-portal/components/PortalState";
import { usePortalSections } from "@/modules/client-portal/hooks/usePortalSections";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import { useVendorStore } from "@/modules/vendors/stores/useVendorStore";
import type { Evidence } from "@/modules/projects/types";
import type { ClientPortalContext } from "@/modules/client-portal/layouts/ClientPortalLayout";

export default function KendalaTabPage() {
  const { projectId } = useOutletContext<ClientPortalContext>();
  const issues = useProjectStore((s) => s.issues);
  const evidence = useProjectStore((s) => s.evidence);
  const vendorEngagements = useProjectStore((s) => s.vendorEngagements);
  const vendorMilestones = useProjectStore((s) => s.vendorMilestones);
  // Public-safe {id, name} only (ADR-0016-style split) -- GET /vendors is
  // staff-only and 403s for a client principal; useVendorStore.vendors'
  // own doc comment already flags this ("Client Portal must use
  // fetchVendorSummaries instead"), which this tab wasn't following.
  const vendors = useVendorStore((s) => s.vendorSummaries);
  const fetchVendorSummaries = useVendorStore((s) => s.fetchVendorSummaries);
  const [viewingEvidence, setViewingEvidence] = useState<Evidence | null>(null);

  const { loading, error, reload } = usePortalSections(projectId, ["issues", "evidence", "vendors"]);

  useEffect(() => {
    // Not project-scoped, so it stays outside usePortalSections. A failure
    // only costs the vendor NAME on a card, which already has its own
    // fallback — it must not take the whole tab down with it.
    void fetchVendorSummaries().catch(() => undefined);
  }, [fetchVendorSummaries]);

  const sortedIssues = [...issues].sort((a, b) => {
    const aOpen = a.status !== "Resolved" && a.status !== "Closed";
    const bOpen = b.status !== "Resolved" && b.status !== "Closed";
    if (aOpen !== bOpen) return aOpen ? -1 : 1;
    return a.foundDate < b.foundDate ? 1 : -1;
  });

  return (
    <div className="flex flex-col gap-6 sm:gap-8">
      <section>
        <div className="mb-4 flex items-center gap-3 border-b border-border pb-4 sm:mb-6">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-warning-soft text-warning-strong">
            <HeartHandshake className="h-5 w-5 shrink-0" />
          </div>
          <div>
            <h2 className="text-[18px] font-bold text-navy-950 sm:text-[20px]">Kendala yang Sedang Kami Tangani</h2>
            <p className="mt-0.5 text-[13px] text-text-secondary sm:text-[14px]">
              Kami percaya Anda berhak mengetahui setiap kendala dan penanganannya — sekecil apa pun itu.
            </p>
          </div>
        </div>

        {loading ? (
          <PortalLoading label="Memuat catatan kendala..." />
        ) : error ? (
          <PortalError message={error} onRetry={reload} />
        ) : sortedIssues.length === 0 ? (
          // Only shown once the request has actually SUCCEEDED. Rendered off
          // an empty store, this same reassurance appeared while the request
          // was still in flight, and stayed on screen permanently if it
          // failed -- telling the client everything was fine on the strength
          // of data that never arrived.
          <PortalEmpty
            icon={CheckCircle2}
            tone="positive"
            title="Semuanya Berjalan Lancar"
            description="Tidak ada kendala yang tercatat saat ini. Tim kami terus memantau persiapan pernikahan Anda."
          />
        ) : (
          <div className="flex flex-col gap-4 sm:gap-5">
            {sortedIssues.map((issue) => {
              const pv = vendorEngagements.find((v) => v.id === issue.projectVendorId);
              const vendorName = pv ? vendors.find((v) => v.id === pv.vendorId)?.name : undefined;
              return (
                <IssueCard
                  key={issue.id}
                  issue={issue}
                  evidence={evidence.filter((e) => e.relatedKind === "issue" && e.relatedId === issue.id)}
                  onViewEvidence={setViewingEvidence}
                  vendorName={vendorName}
                  milestoneName={vendorMilestones.find((m) => m.id === issue.vendorMilestoneId)?.name}
                />
              );
            })}
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
