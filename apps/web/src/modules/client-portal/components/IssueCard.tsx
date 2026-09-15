import { FileText } from "lucide-react";
import { ClientIssueImpactBadge, ClientIssueStatusBadge } from "@/modules/client-portal/components/ClientBadges";
import { clientEvidenceTypeLabel } from "@/modules/client-portal/lib/labels";
import type { VendorIssue, Evidence } from "@/modules/projects/types";
import { formatDate } from "@/shared/lib/formatters";

interface IssueCardProps {
  issue: VendorIssue;
  evidence: Evidence[];
  onViewEvidence: (evidence: Evidence) => void;
  showVendorName?: boolean;
  vendorName?: string;
  // Name of the specific vendor timeline this kendala is about, if any --
  // omitted/undefined shows "Kendala Umum" (general to the vendor).
  milestoneName?: string;
}

export function IssueCard({
  issue,
  evidence,
  onViewEvidence,
  showVendorName = true,
  vendorName,
  milestoneName,
}: IssueCardProps) {
  const isResolved = issue.status === "Resolved" || issue.status === "Closed";

  return (
    <div className="group relative overflow-hidden rounded-2xl border border-border bg-white p-5 shadow-sm transition-shadow hover:shadow-md sm:p-6">
      {/* Decorative side bar based on resolution status */}
      <div
        className={`absolute bottom-0 left-0 top-0 w-1.5 opacity-70 transition-opacity group-hover:opacity-100 ${
          isResolved ? "bg-emerald-500" : "bg-warning-strong"
        }`}
      ></div>

      <div className="flex flex-col gap-3 border-b border-border/50 pb-4 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
        <div className="min-w-0">
          <p className="text-[15px] font-bold text-navy-950 sm:text-[16px]">{issue.title}</p>
          <p className="mt-1 inline-block rounded-md bg-surface-muted px-2.5 py-1 text-[12.5px] font-medium text-text-secondary sm:text-[13px]">
            {showVendorName && <span className="font-semibold text-navy-700">{vendorName ?? "Vendor tidak diketahui"}</span>}
            {showVendorName && <span className="mx-1.5 text-border">|</span>}
            <span className="font-semibold text-navy-700">{milestoneName ?? "Kendala Umum"}</span>
            <span className="mx-1.5 text-border">|</span>
            Ditemukan {formatDate(issue.foundDate)}
          </p>
        </div>
        <div className="flex shrink-0 flex-wrap items-center gap-2">
          <ClientIssueImpactBadge impact={issue.impact} />
          <ClientIssueStatusBadge status={issue.status} />
        </div>
      </div>

      {issue.description && (
        <div className="mt-4">
          <p className="whitespace-pre-line text-[13.5px] leading-relaxed text-text-secondary sm:text-[14px]">
            {issue.description}
          </p>
        </div>
      )}

      {/* Guarded: an issue recorded without a plan yet rendered an empty
          blue panel with nothing but its heading in it. */}
      {issue.resolutionPlan && (
        <div className="relative mt-4 overflow-hidden rounded-xl border border-blue-100 bg-blue-50/40 p-4">
          <div className="absolute bottom-0 left-0 top-0 w-1 bg-blue-400 opacity-50"></div>
          <p className="text-[12.5px] font-bold uppercase tracking-wider text-navy-900">Rencana penanganan</p>
          <p className="mt-1.5 whitespace-pre-line text-[13.5px] leading-relaxed text-navy-900/80">{issue.resolutionPlan}</p>
        </div>
      )}

      {isResolved && (
        <div className="relative mt-3 overflow-hidden rounded-xl border border-emerald-100 bg-emerald-50/40 p-4">
          <div className="absolute bottom-0 left-0 top-0 w-1 bg-emerald-400 opacity-50"></div>
          <p className="text-[12.5px] font-bold uppercase tracking-wider text-emerald-800">
            Diselesaikan {formatDate(issue.resolvedDate)}
          </p>
          {issue.resolutionNotes && (
            <p className="mt-1.5 whitespace-pre-line text-[13.5px] leading-relaxed text-emerald-900/80">
              {issue.resolutionNotes}
            </p>
          )}
        </div>
      )}

      {evidence.length > 0 && (
        <div className="mt-5 flex flex-wrap items-center gap-2 border-t border-border/50 pt-4">
          <FileText className="h-4 w-4 shrink-0 text-text-tertiary" />
          <span className="mr-2 text-[13px] font-medium text-text-secondary">Dokumen Pendukung:</span>
          {evidence.map((e) => (
            <button
              key={e.id}
              type="button"
              onClick={() => onViewEvidence(e)}
              className="flex items-center gap-1.5 rounded-lg border border-border bg-surface px-3 py-1.5 text-[12px] font-semibold text-navy-900 shadow-sm transition-colors hover:border-navy-200 hover:bg-navy-50"
            >
              Lihat {clientEvidenceTypeLabel(e.type)}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
