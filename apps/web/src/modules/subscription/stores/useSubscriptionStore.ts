import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import { toPaginationMeta, EMPTY_PAGINATION_META, type PaginationMeta, type RawPaginationMeta } from "@/shared/types/pagination";

export type TenantSubscriptionStatus = "active" | "expiring_soon" | "expired" | "pending_payment";

// The tenant fields SubscriptionPage actually reads — a deliberately
// narrower slice than the old platform-admin `Tenant` type (which also
// carried Platform-Console-only fields like isSuspended/customDomain, all
// gone with the Platform Console itself — D2).
export interface MyTenant {
  id: string;
  planId: string | null;
  subscriptionStatus: TenantSubscriptionStatus;
  subscriptionExpiresAt: string | null;
}

interface RawTenant {
  id: number;
  planId: number | null;
  subscriptionStatus: TenantSubscriptionStatus;
  subscriptionExpiresAt: string | null;
}

function toTenant(raw: RawTenant): MyTenant {
  return {
    id: String(raw.id),
    planId: raw.planId !== null ? String(raw.planId) : null,
    subscriptionStatus: raw.subscriptionStatus,
    subscriptionExpiresAt: raw.subscriptionExpiresAt,
  };
}

export type SubscriptionTransactionType = "new" | "renewal";
export type SubscriptionTransactionStatus = "unpaid" | "pending" | "paid" | "expired" | "granted" | "cancelled";

export interface SubscriptionTransaction {
  id: string;
  tenantId: string;
  type: SubscriptionTransactionType;
  amount: number;
  paymentMethod: string;
  paymentReference: string;
  createdAt: string;
  status: SubscriptionTransactionStatus;
  paidAt: string | null;
}

interface RawTransaction {
  id: number;
  tenantId: number;
  type: SubscriptionTransactionType;
  amount: number;
  paymentMethod: string;
  paymentReference: string;
  status: SubscriptionTransactionStatus;
  createdAt: string;
  paidAt: string | null;
}

function toTransaction(raw: RawTransaction): SubscriptionTransaction {
  return { ...raw, id: String(raw.id), tenantId: String(raw.tenantId) };
}

// Returned by Pay — a real charge created at ElProof (QRIS by default). The
// subscription is NOT active yet at this point; it only activates once
// ElProof's webhook confirms payment (or the reconciler catches up).
export interface PaymentCharge {
  orderRef: string;
  providerRef: string;
  channel: string;
  qrImageUrl: string;
  payCode: string;
  checkoutUrl: string;
  amount: number;
  feeAmount: number;
  expiresAt: string;
  status: string;
}

interface SubscriptionState {
  myTenant: MyTenant | null;
  transactionPage: SubscriptionTransaction[];
  transactionPageMeta: PaginationMeta;

  fetchMyTenant: () => Promise<void>;
  fetchTransactionPage: (page: number, status: string) => Promise<void>;

  // Tenant Owner's own self-service "Bayar Sekarang" — scoped to their own
  // tenant server-side via the JWT claim, not a request parameter. Returns
  // the created charge — the subscription activates later, once ElProof's
  // webhook confirms payment (or the reconciler catches up), not
  // synchronously here.
  paySubscription: (planId: string) => Promise<PaymentCharge>;
  fetchPendingCharge: () => Promise<PaymentCharge | null>;
  cancelPendingCharge: () => Promise<void>;
}

// Backs SubscriptionPage only — the WO Console's "Langganan" self-service
// page. Fetch-then-set, no client cache (ADR-0009).
export const useSubscriptionStore = create<SubscriptionState>((set, get) => ({
  myTenant: null,
  transactionPage: [],
  transactionPageMeta: EMPTY_PAGINATION_META,

  fetchMyTenant: async () => {
    const res = await httpClient.get(API.platform.tenantMe);
    set({ myTenant: toTenant(res.data.data as RawTenant) });
  },

  fetchTransactionPage: async (page, status) => {
    const res = await httpClient.get(API.billing.transactions, { params: { page, status: status || undefined } });
    set({
      transactionPage: (res.data.data as RawTransaction[]).map(toTransaction),
      transactionPageMeta: toPaginationMeta(res.data.meta as RawPaginationMeta),
    });
  },

  paySubscription: async (planId) => {
    const res = await httpClient.post(API.platform.subscriptionsPay, { planId: Number(planId) });
    return res.data.data as PaymentCharge;
  },

  // Lets the Owner re-view a still-pending charge (e.g. after accidentally
  // closing its QR modal) — `data` is null when there's nothing pending,
  // which is the common case, not an error.
  fetchPendingCharge: async () => {
    const res = await httpClient.get(API.platform.subscriptionsPendingCharge);
    return (res.data.data as PaymentCharge | null) ?? null;
  },

  cancelPendingCharge: async () => {
    await httpClient.post(API.platform.subscriptionsCancelPendingCharge);
    await get().fetchTransactionPage(1, "");
  },
}));
