import { AlertTriangle } from "lucide-react";
import { Link } from "react-router-dom";
import { useSubscriptionGateStore } from "@/shared/stores/useSubscriptionGateStore";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

// Persistent notice once the backend's read-only subscription guard
// (D5/D10/D13) has rejected a write with 402 (D14) — mounted in the WO
// Console layout only (AppLayout), never Client Portal: the guard itself
// only ever applies to the `staff` principal (D13), so a client would never
// see this trip anyway. GET requests still succeed while this is showing —
// only the next write attempt would 402 again.
export function ReadOnlyBanner() {
  const readOnly = useSubscriptionGateStore((s) => s.readOnly);
  if (!readOnly) return null;

  return (
    <div className="flex items-center gap-2.5 border-b border-danger/30 bg-danger-soft px-4 py-2.5 text-[13px] text-danger sm:px-6">
      <AlertTriangle className="h-4 w-4 shrink-0" />
      <p className="flex-1">
        Langganan Anda sudah kedaluwarsa. Data masih bisa dilihat, tapi aksi simpan/ubah dinonaktifkan sampai pembayaran
        diselesaikan.
      </p>
      <Link to={ROUTE_PATHS.subscription} className="shrink-0 font-semibold underline hover:no-underline">
        Selesaikan Pembayaran
      </Link>
    </div>
  );
}
