import type { BadgeTone } from "@/shared/components/ui/Badge";
import type { SubscriptionTransactionStatus } from "@/modules/subscription/stores/useSubscriptionStore";

export const TRANSACTION_STATUS_TONE: Record<SubscriptionTransactionStatus, BadgeTone> = {
  unpaid: "warning",
  pending: "warning",
  paid: "success",
  expired: "danger",
  granted: "navy",
  cancelled: "neutral",
};

export const TRANSACTION_STATUS_LABEL: Record<SubscriptionTransactionStatus, string> = {
  unpaid: "Menunggu Pembayaran",
  pending: "Menunggu Konfirmasi",
  paid: "Berhasil",
  expired: "Kedaluwarsa",
  granted: "Diaktifkan Admin",
  cancelled: "Dibatalkan",
};

export const TRANSACTION_TYPE_LABEL = {
  new: "Baru",
  renewal: "Perpanjangan",
} as const;
