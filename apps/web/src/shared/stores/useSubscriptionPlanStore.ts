import { create } from "zustand";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import type { SubscriptionPlan } from "@/shared/data/subscriptionPlans";

interface RawPlan {
  id: number;
  name: string;
  durationMonths: number;
  price: number;
  isActive: boolean;
  features?: string[] | null;
}

function toPlan(raw: RawPlan): SubscriptionPlan {
  return {
    id: String(raw.id),
    name: raw.name,
    durationMonths: raw.durationMonths,
    price: raw.price,
    features: raw.features ?? [],
    isActive: raw.isActive,
  };
}

interface SubscriptionPlanState {
  plans: SubscriptionPlan[];
  isLoading: boolean;
  fetchPlans: () => Promise<void>;
}

// Read-only plan catalog (D7) — the plan catalog now lives at ElProof, not
// in this app's own database; `billing` just proxies it through. Fetch-then-
// set, no client cache.
export const useSubscriptionPlanStore = create<SubscriptionPlanState>((set) => ({
  plans: [],
  isLoading: false,

  fetchPlans: async () => {
    set({ isLoading: true });
    try {
      const res = await httpClient.get(API.billing.plans);
      set({ plans: (res.data.data as RawPlan[]).map(toPlan), isLoading: false });
    } catch (err) {
      set({ isLoading: false });
      throw err;
    }
  },
}));
