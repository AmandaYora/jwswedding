import { describe, it, expect, afterEach, beforeEach } from "vitest";
import { cleanup, renderHook, waitFor } from "@testing-library/react";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import { useVendorStore } from "@/modules/vendors/stores/useVendorStore";
import { useVendorCategoryStore } from "@/modules/vendor-categories/stores/useVendorCategoryStore";
import { useProjectVendorPrefill } from "@/modules/rundowns/hooks/useProjectVendorPrefill";
import type { ProjectVendor } from "@/modules/projects/types";

afterEach(cleanup);

function engagement(vendorId: string, categoryId: string): ProjectVendor {
  return { vendorId, categoryId } as ProjectVendor;
}

// Satu promise per project yang bisa diselesaikan tes kapan saja — untuk
// mensimulasikan respons project lama yang datang TERLAMBAT.
const pending = new Map<string, (rows: ProjectVendor[]) => void>();

beforeEach(() => {
  pending.clear();
  useProjectStore.setState({
    fetchVendorEngagementsFor: (projectId: string) =>
      new Promise<ProjectVendor[]>((resolve) => pending.set(projectId, resolve)),
  });
  useVendorStore.setState({
    vendors: [
      { id: "v1", name: "Dekor Sejati" },
      { id: "v2", name: "Rias Ayu" },
    ] as never,
    fetchVendors: async () => {},
  });
  useVendorCategoryStore.setState({
    categories: [
      { id: "c1", name: "Dekorasi" },
      { id: "c2", name: "Rias" },
    ] as never,
    fetchCategories: async () => {},
  });
});

describe("useProjectVendorPrefill", () => {
  it("respons project lama yang datang terlambat tidak dipakai", async () => {
    const { result, rerender } = renderHook(({ id }) => useProjectVendorPrefill(id), {
      initialProps: { id: "A" },
    });
    rerender({ id: "B" });

    await waitFor(() => expect(pending.has("B")).toBe(true));
    pending.get("B")!([engagement("v2", "c2")]);
    // Respons A tiba SETELAH B — harus diabaikan.
    pending.get("A")!([engagement("v1", "c1")]);

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.rows).toEqual([{ categoryLabel: "RIAS", vendorName: "RIAS AYU" }]);
  });

  it("loading sampai vendor project termuat", async () => {
    const { result } = renderHook(() => useProjectVendorPrefill("A"));
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(pending.has("A")).toBe(true));
    pending.get("A")!([engagement("v1", "c1")]);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.rows).toEqual([{ categoryLabel: "DEKORASI", vendorName: "DEKOR SEJATI" }]);
  });
});
