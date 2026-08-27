import { useNavigate } from "react-router-dom";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";
import { useAuthStore } from "@/shared/stores/useAuthStore";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

interface IncompleteProfileDialogProps {
  open: boolean;
  onClose: () => void;
  missingFields: string[];
  /** Which document the caller was trying to print — "Tagihan" or
   * "Kwitansi" — so the message reads naturally regardless of which action
   * triggered the gate. */
  docLabel: string;
}

// IncompleteProfileDialog is the gate the user asked for: before Invoice/
// Kwitansi generation is allowed, the tenant's business profile must be
// complete (PLAN.md redesain-pdf-invoice-kwitansi-v2 §6.3.2). Only an Owner
// can actually reach /profil-usaha (RequireRole allow={["Owner"]},
// protected.routes.tsx) — showing a "Lengkapi Profil Usaha" button to
// Admin/Staff/Sales would just be a dead end, so they get a different
// message with no navigation button instead (K5).
export function IncompleteProfileDialog({ open, onClose, missingFields, docLabel }: IncompleteProfileDialogProps) {
  const navigate = useNavigate();
  const role = useAuthStore((s) => s.session?.role);
  const isOwner = role === "Owner";

  function handleGoToProfile() {
    onClose();
    navigate(ROUTE_PATHS.companyProfile);
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Profil Usaha Belum Lengkap"
      size="sm"
      footer={isOwner ? <Button onClick={handleGoToProfile}>Lengkapi Profil Usaha</Button> : undefined}
    >
      <div className="flex flex-col gap-3 text-sm text-text-primary">
        {isOwner ? (
          <p>
            Profil Usaha belum lengkap, jadi <strong>{docLabel}</strong> belum bisa dicetak. Lengkapi field berikut
            terlebih dahulu:
          </p>
        ) : (
          <p>
            <strong>{docLabel}</strong> belum bisa dicetak karena Profil Usaha belum lengkap. Minta akun Owner
            melengkapi field berikut terlebih dahulu:
          </p>
        )}
        <ul className="list-disc space-y-1 pl-5 text-text-secondary">
          {missingFields.map((field) => (
            <li key={field}>{field}</li>
          ))}
        </ul>
      </div>
    </Modal>
  );
}
