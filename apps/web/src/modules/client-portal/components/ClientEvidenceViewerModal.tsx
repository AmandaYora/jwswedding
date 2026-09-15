import { EvidenceViewerModal } from "@/shared/components/ui/EvidenceViewerModal";
import { clientEvidenceTypeLabel } from "@/modules/client-portal/lib/labels";
import type { Evidence } from "@/modules/projects/types";

// One place where the portal's five tabs get the shared viewer with a
// client-facing type label ("Bukti Transfer", not "TRANSFER PROOF") — rather
// than each of them repeating the same `typeLabel={...}` prop.
export function ClientEvidenceViewerModal({
  evidence,
  projectId,
  onClose,
}: {
  evidence: Evidence;
  projectId: string;
  onClose: () => void;
}) {
  return (
    <EvidenceViewerModal
      open
      onClose={onClose}
      projectId={projectId}
      evidence={evidence}
      typeLabel={clientEvidenceTypeLabel(evidence.type)}
    />
  );
}
