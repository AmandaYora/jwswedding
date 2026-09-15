import { useEffect, useState } from "react";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import { getApiErrorMessage } from "@/shared/lib/api-error";

/**
 * One place for "this Client Portal surface needs these slices of the project".
 *
 * Two bugs made this worth extracting rather than leaving each tab to its own
 * `useEffect(() => { void fetchX(id) })`:
 *
 *  1. **Double fetching.** ClientProjectHeaderCard is persistent (it lives in
 *     the layout, above the tab strip) and fetches milestones + vendors +
 *     client payments + issues. Every tab then mounted and re-fetched an
 *     overlapping subset in the same commit, so e.g. opening "Kendala" fired
 *     GET .../issues twice — four times under React StrictMode. Concurrent
 *     requests for the same (section, project) now share one promise.
 *  2. **Empty state shown as fact.** Tabs rendered straight off the store, so
 *     before the first response landed the client was told "Belum ada
 *     dokumen" / "Semuanya berjalan lancar" — and if the request FAILED they
 *     were told exactly the same thing, permanently. `loading`/`error` below
 *     let each tab distinguish "not loaded yet", "failed", and "really empty".
 *
 * A refetch still happens on every mount (no cache, matching ADR-0009's "no
 * React Query, if a page needs fresh data it re-fetches"); `settled` only
 * decides whether a *spinner* is shown, so revisiting a tab shows the data
 * already in the store instead of flashing a placeholder over it.
 */
export type PortalSection =
  | "milestones"
  | "milestoneDocuments"
  | "vendors"
  | "clientPayments"
  | "issues"
  | "evidence"
  | "documents";

const FETCHERS: Record<PortalSection, (projectId: string) => Promise<void>> = {
  milestones: (id) => useProjectStore.getState().fetchMilestones(id),
  milestoneDocuments: (id) => useProjectStore.getState().fetchMilestoneDocuments(id),
  vendors: (id) => useProjectStore.getState().fetchVendorSection(id),
  clientPayments: (id) => useProjectStore.getState().fetchClientPayments(id),
  issues: (id) => useProjectStore.getState().fetchIssues(id),
  evidence: (id) => useProjectStore.getState().fetchEvidence(id),
  documents: (id) => useProjectStore.getState().fetchDocuments(id),
};

const inFlight = new Map<string, Promise<void>>();
const settled = new Set<string>();

function cacheKey(section: PortalSection, projectId: string) {
  return `${section}:${projectId}`;
}

function load(section: PortalSection, projectId: string): Promise<void> {
  const key = cacheKey(section, projectId);
  const existing = inFlight.get(key);
  if (existing) return existing;
  const promise = FETCHERS[section](projectId)
    .then(() => {
      settled.add(key);
    })
    .finally(() => {
      inFlight.delete(key);
    });
  inFlight.set(key, promise);
  return promise;
}

/**
 * Forgets which sections have ever loaded, so the next mount shows its
 * loading state again instead of a previous session's data. Called when the
 * portal layout unmounts — i.e. on logout, or on leaving /portal entirely.
 * In-flight promises are deliberately left alone: they clean up after
 * themselves and dropping them would only re-duplicate requests.
 */
export function resetPortalSectionCache() {
  settled.clear();
}

export interface PortalSectionsState {
  /** True until every requested section has loaded at least once. */
  loading: boolean;
  /** Message from the first failing request, or null. */
  error: string | null;
  /** Re-runs every requested section; used by the error state's retry. */
  reload: () => void;
}

export function usePortalSections(projectId: string, sections: PortalSection[]): PortalSectionsState {
  // The array is rebuilt on every render at each call site; its contents are
  // the real dependency, so they are what the effect keys off.
  const sectionKey = sections.join(",");
  const [attempt, setAttempt] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const list = sectionKey.split(",") as PortalSection[];
    let cancelled = false;
    const alreadyLoaded = list.every((s) => settled.has(cacheKey(s, projectId)));
    setError(null);
    setLoading(!alreadyLoaded);

    Promise.all(list.map((s) => load(s, projectId)))
      .then(() => {
        if (!cancelled) setError(null);
      })
      .catch((err) => {
        if (!cancelled) setError(getApiErrorMessage(err, "Gagal memuat data. Coba muat ulang halaman."));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [projectId, sectionKey, attempt]);

  return { loading, error, reload: () => setAttempt((a) => a + 1) };
}
