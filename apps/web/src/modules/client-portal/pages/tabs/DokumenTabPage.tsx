import { useState } from "react";
import { useOutletContext } from "react-router-dom";
import { Download, FileText, FolderOpen, Eye, Loader2 } from "lucide-react";
import { Badge } from "@/shared/components/ui/Badge";
import { ClientEvidenceViewerModal } from "@/modules/client-portal/components/ClientEvidenceViewerModal";
import { PortalEmpty, PortalError, PortalLoading } from "@/modules/client-portal/components/PortalState";
import { usePortalSections } from "@/modules/client-portal/hooks/usePortalSections";
import { clientEvidenceTypeLabel } from "@/modules/client-portal/lib/labels";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import type { Evidence } from "@/modules/projects/types";
import { formatDate } from "@/shared/lib/formatters";
import { getApiErrorMessageFromBlob } from "@/shared/lib/api-error";
import { openPdfInNewTab } from "@/shared/lib/open-pdf";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import type { ClientPortalContext } from "@/modules/client-portal/layouts/ClientPortalLayout";

// Client Portal's own "Dokumen" tab — general project documents (rundown,
// buku acara, teks juru bicara, dll) with no other specific vendor/venue/
// timeline context. Backed by GET /projects/{id}/documents, which always
// only returns related_kind="general" AND is_client_visible=true documents
// (PLAN.md) — a document only ever shows up here after staff has explicitly
// opted it in from the WO Console's own Dokumen & Evidence section.
export default function DokumenTabPage() {
  const { projectId } = useOutletContext<ClientPortalContext>();
  const documents = useProjectStore((s) => s.documents);
  const project = useProjectStore((s) => s.currentProject);
  const [viewingEvidence, setViewingEvidence] = useState<Evidence | null>(null);
  const [poBusy, setPoBusy] = useState(false);
  const [poError, setPoError] = useState<string | null>(null);

  const { loading, error, reload } = usePortalSections(projectId, ["documents"]);

  const sortedDocuments = [...documents].sort((a, b) => (a.uploadedAt < b.uploadedAt ? 1 : -1));

  // Kartu "Purchase Order" tersemat (PLAN revisi-vendor-venue-portal §1.1 poin
  // 11): unduh PDF PO milik project ini, hanya bila backend menyatakan
  // tersedia (penawaran sudah Diterima — selama staff merevisi, kartunya
  // hilang sementara). Pengambilan blob inline di sini — JANGAN memakai store
  // quotations (itu store staff-only; menariknya ke portal menyeret state yang
  // tidak berhak dibaca klien). Idiom kartu tersemat mengikuti kartu Venue di
  // VendorTabPage.
  const showPoCard = Boolean(project?.quotationPdfAvailable && project?.quotationId);
  async function handleDownloadPo() {
    if (!project?.quotationId) return;
    setPoError(null);
    setPoBusy(true);
    try {
      // Must stay synchronous up to openPdfInNewTab's first line — it claims
      // the tab while the click's user activation is still alive (see
      // open-pdf.ts; the same reason PembayaranTabPage's receipt button does
      // this).
      await openPdfInNewTab(
        async () => {
          const res = await httpClient.get(API.quotations.pdf(project.quotationId), { responseType: "blob" });
          return res.data as Blob;
        },
        // poNumber berbentuk "026/..." — garis miring tidak sah pada
        // link.download di jalur fallback helper; pola persis
        // PembayaranTabPage. Cabang poNumber kosong bukan hipotetis: PO hasil
        // migrasi 000061 bisa tidak pernah bernomor.
        project.poNumber ? `PO-${project.poNumber.replace(/\//g, "-")}` : `PO-${project.name}`
      );
    } catch (err) {
      setPoError(await getApiErrorMessageFromBlob(err, "Gagal membuka Purchase Order"));
    } finally {
      setPoBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-6 sm:gap-8">
      {showPoCard && (
        <section>
          <h3 className="mb-3 text-[13px] font-semibold uppercase tracking-wide text-text-secondary">
            Purchase Order
          </h3>
          <button
            type="button"
            onClick={handleDownloadPo}
            disabled={poBusy}
            className="flex w-full items-start gap-3 rounded-2xl border border-border bg-white p-4 text-left shadow-sm transition-colors hover:border-navy-200 hover:bg-surface-muted/50 disabled:opacity-60 sm:max-w-xl"
          >
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-navy-800 text-white">
              {poBusy ? <Loader2 className="h-5 w-5 animate-spin" /> : <FileText className="h-5 w-5" />}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-[14px] font-semibold text-navy-950">
                {project?.poNumber ? `Purchase Order ${project.poNumber}` : "Purchase Order"}
              </p>
              <p className="mt-0.5 text-[12.5px] text-text-secondary">
                {poBusy ? "Menyiapkan dokumen..." : "Ketuk untuk membuka dokumen penawaran Anda."}
              </p>
              {poError && <p className="mt-1.5 text-[12.5px] text-danger">{poError}</p>}
            </div>
            <Download className="h-4 w-4 shrink-0 text-text-secondary" />
          </button>
        </section>
      )}
      <section>
        <div className="mb-4 flex items-center gap-3 border-b border-border pb-4 sm:mb-6">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-info-soft text-info">
            <FileText className="h-5 w-5 shrink-0" />
          </div>
          <div>
            <h2 className="text-[18px] font-bold text-navy-950 sm:text-[20px]">Dokumen</h2>
            <p className="mt-0.5 text-[13px] text-text-secondary sm:text-[14px]">
              Dokumen umum acara yang telah kami bagikan untuk Anda — rundown, buku acara, dan lainnya.
            </p>
          </div>
        </div>

        {loading ? (
          <PortalLoading label="Memuat dokumen..." />
        ) : error ? (
          <PortalError message={error} onRetry={reload} />
        ) : sortedDocuments.length === 0 ? (
          <PortalEmpty
            icon={FolderOpen}
            tone="info"
            title="Belum Ada Dokumen"
            description="Dokumen umum acara Anda akan muncul di sini setelah dibagikan oleh tim kami."
          />
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            {sortedDocuments.map((doc) => (
              <button
                key={doc.id}
                type="button"
                onClick={() => setViewingEvidence(doc)}
                className="flex items-start gap-3 rounded-2xl border border-border bg-white p-4 text-left shadow-sm transition-colors hover:border-navy-200 hover:bg-surface-muted/50"
              >
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-info-soft text-info">
                  <FileText className="h-5 w-5" />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[14px] font-semibold text-navy-950">{doc.name}</p>
                  <div className="mt-1 flex flex-wrap items-center gap-1.5 text-[12px] text-text-secondary">
                    <Badge tone="neutral">{clientEvidenceTypeLabel(doc.type)}</Badge>
                    <span>{formatDate(doc.documentDate)}</span>
                  </div>
                  {doc.description && (
                    <p className="mt-1.5 line-clamp-2 text-[12.5px] text-text-secondary">{doc.description}</p>
                  )}
                </div>
                <Eye className="h-4 w-4 shrink-0 text-text-secondary" />
              </button>
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
