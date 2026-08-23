import { create } from "zustand";

interface SubscriptionGateState {
  readOnly: boolean;
  setReadOnly: (readOnly: boolean) => void;
}

// Flipped by http-client.ts's response interceptor the moment a 402
// `subscription_expired` response is seen (D14) — read by ReadOnlyBanner to
// show a persistent notice in the WO Console layout. Deliberately not
// fetched proactively on load: the backend's guard (D5/D10/D13) is the
// actual source of truth, this store just mirrors the first time it's
// actually enforced against a real write attempt.
export const useSubscriptionGateStore = create<SubscriptionGateState>((set) => ({
  readOnly: false,
  setReadOnly: (readOnly) => set({ readOnly }),
}));
