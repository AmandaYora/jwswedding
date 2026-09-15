import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Download, X, FileWarning } from "lucide-react";
import { Badge } from "@/shared/components/ui/Badge";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import { getEvidenceFileKind } from "@/shared/lib/evidence-file";
import { useBodyScrollLock } from "@/shared/lib/body-scroll-lock";
import { formatDate } from "@/shared/lib/formatters";
import type { EvidenceType } from "@/modules/projects/types";

interface EvidenceViewerModalProps {
  open: boolean;
  onClose: () => void;
  projectId: string;
  evidence: {
    id: string;
    name: string;
    type: EvidenceType;
    documentDate: string | null;
    fileName: string;
  };
  contextLabel?: string;
  /**
   * Overrides the type badge's text. The Client Portal passes an Indonesian
   * label here (see client-portal/lib/labels.ts); the WO Console leaves it
   * unset and keeps the raw enum it works with everywhere else.
   */
  typeLabel?: string;
}

// Streams the real evidence file from the object-storage-backed download
// endpoint. That endpoint requires the staff Bearer token (ADR-0004), so a
// bare <img>/<iframe> src can't hit it directly — the file is fetched as a
// blob through the authenticated httpClient instead, then shown via a
// short-lived object URL.
//
// Rendered through a portal on <body>, NOT in place. `z-50` only outranks the
// page chrome if the two share a stacking context, and in the Client Portal
// they did not: its <main> carried `relative z-10`, which boxes every
// descendant's z-index inside it, so this overlay painted UNDER the z-30
// sticky header. Portalling also means the overlay can never be clipped by an
// ancestor's `overflow`.
export function EvidenceViewerModal({
  open,
  onClose,
  projectId,
  evidence,
  contextLabel,
  typeLabel,
}: EvidenceViewerModalProps) {
  const [fileUrl, setFileUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const dialogRef = useRef<HTMLDivElement>(null);

  useBodyScrollLock(open);

  useEffect(() => {
    if (!open) return;
    let objectUrl: string | null = null;
    let cancelled = false;
    setFileUrl(null);
    setError(null);

    httpClient
      .get(API.projects.evidenceFile(projectId, evidence.id), { responseType: "blob" })
      .then((res) => {
        if (cancelled) return;
        objectUrl = URL.createObjectURL(res.data as Blob);
        setFileUrl(objectUrl);
      })
      .catch(() => {
        if (!cancelled) setError("Gagal memuat berkas. Silakan coba lagi atau hubungi tim kami.");
      });

    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [open, projectId, evidence.id]);

  useEffect(() => {
    if (!open) return;
    // Focus moves into the dialog so Esc and Tab act on it rather than on the
    // page still visible behind the overlay, and returns to whatever opened it
    // once the dialog closes.
    const previouslyFocused = document.activeElement as HTMLElement | null;
    dialogRef.current?.focus();
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      previouslyFocused?.focus?.();
    };
  }, [open, onClose]);

  if (!open) return null;

  const kind = getEvidenceFileKind(evidence.fileName);

  return createPortal(
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-navy-950/60 p-0 sm:p-4 md:p-8"
      // Clicking the dimmed area closes. Comparing target to currentTarget on
      // mousedown keeps a click that merely ENDED on the backdrop (a text
      // selection dragged out of the dialog, say) from closing it.
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={evidence.name}
        tabIndex={-1}
        className="flex h-full w-full flex-col overflow-hidden bg-surface shadow-xl outline-none sm:h-[88vh] sm:max-w-3xl sm:rounded-xl"
      >
        <div className="flex items-start justify-between gap-3 border-b border-border-light px-4 py-3 sm:px-6 sm:py-4">
          <div className="min-w-0">
            <p className="truncate text-[14.5px] font-semibold text-text-primary sm:text-[15px]">{evidence.name}</p>
            <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-[12px] text-text-secondary">
              <Badge tone="neutral">{typeLabel ?? evidence.type}</Badge>
              <span>{formatDate(evidence.documentDate)}</span>
              {contextLabel && <span className="hidden sm:inline">· {contextLabel}</span>}
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {fileUrl && (
              <a
                href={fileUrl}
                download={evidence.fileName}
                className="flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1.5 text-[12.5px] font-semibold text-navy-900 transition-colors hover:bg-surface-muted"
                aria-label="Unduh dokumen"
              >
                <Download className="h-3.5 w-3.5" />
                <span className="hidden sm:inline">Unduh</span>
              </a>
            )}
            <button
              onClick={onClose}
              aria-label="Tutup"
              className="rounded-md p-1.5 text-text-secondary hover:bg-surface-muted"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>

        <div className="relative flex-1 overflow-hidden bg-surface-muted/50">
          {error ? (
            <div className="flex h-full flex-col items-center justify-center gap-2 p-8 text-center">
              <FileWarning className="h-8 w-8 text-danger" />
              <p className="text-[13px] text-text-secondary">{error}</p>
            </div>
          ) : !fileUrl ? (
            <div className="flex h-full items-center justify-center text-[13px] text-text-secondary">Memuat berkas...</div>
          ) : kind === "pdf" ? (
            <iframe src={fileUrl} title={evidence.name} className="h-full w-full border-0" />
          ) : kind === "image" ? (
            <div className="flex h-full items-center justify-center p-4 sm:p-8">
              <img
                src={fileUrl}
                alt={evidence.name}
                className="max-h-full max-w-full rounded-md border border-border-light bg-white p-2 object-contain shadow-sm"
              />
            </div>
          ) : (
            <div className="flex h-full flex-col items-center justify-center gap-2 p-8 text-center text-[13px] text-text-secondary">
              Pratinjau tidak tersedia untuk jenis berkas ini — gunakan tombol Unduh.
            </div>
          )}
        </div>
      </div>
    </div>,
    document.body
  );
}
