import { useEffect, useMemo, useState } from "react";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import { useVendorStore } from "@/modules/vendors/stores/useVendorStore";
import { useVendorCategoryStore } from "@/modules/vendor-categories/stores/useVendorCategoryStore";
import type { ProjectVendor } from "@/modules/projects/types";
import type { RundownVendor } from "@/modules/rundowns/types";

interface Loaded {
  projectId: string;
  engagements: ProjectVendor[];
}

/**
 * Daftar vendor sebuah project dalam bentuk halaman VENDORS buku acara.
 *
 * Dirakit DI SINI, bukan di backend: nama vendor dan kategori milik modul
 * `vendors`, dan konvensi repo ini (knowledge/MODULE_MAP.md) menaruh resolusi
 * nama tampilan di frontend yang store-nya memang sudah termuat. Backend
 * menerimanya sebagai snapshot — itu yang membuat buku acara yang sudah
 * dicetak tidak ikut berubah saat master vendor disunting.
 *
 * Hasil fetch disimpan bersama project asalnya dan hanya dipakai bila masih
 * cocok dengan `projectId` yang diminta. Tanpa ini, respons project lama yang
 * datang terlambat — atau vendor project yang terakhir dibuka di halaman
 * project — bisa tersalin ke rundown project lain (PLAN rundown-ux-ideal #11).
 */
export function useProjectVendorPrefill(projectId: string) {
  const fetchVendorEngagementsFor = useProjectStore((s) => s.fetchVendorEngagementsFor);
  const vendors = useVendorStore((s) => s.vendors);
  const fetchVendors = useVendorStore((s) => s.fetchVendors);
  const categories = useVendorCategoryStore((s) => s.categories);
  const fetchCategories = useVendorCategoryStore((s) => s.fetchCategories);

  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const [catalogReady, setCatalogReady] = useState(false);
  const [error, setError] = useState("");

  // Master vendor & kategori wajib termuat sebelum hasilnya dipakai: tanpa
  // nama vendor, barisnya tersaring habis dan halaman VENDORS jadi kosong.
  useEffect(() => {
    let cancelled = false;
    Promise.all([fetchVendors(), fetchCategories()])
      .then(() => {
        if (!cancelled) setCatalogReady(true);
      })
      .catch(() => {
        if (!cancelled) setError("Data vendor gagal dimuat.");
      });
    return () => {
      cancelled = true;
    };
  }, [fetchVendors, fetchCategories]);

  useEffect(() => {
    if (!projectId) return;
    let cancelled = false;
    setError("");
    fetchVendorEngagementsFor(projectId)
      .then((engagements) => {
        if (!cancelled) setLoaded({ projectId, engagements });
      })
      .catch(() => {
        if (!cancelled) setError("Daftar vendor project gagal dimuat.");
      });
    return () => {
      cancelled = true;
    };
  }, [projectId, fetchVendorEngagementsFor]);

  const current = loaded && loaded.projectId === projectId && catalogReady ? loaded : null;
  const loading = !!projectId && !current && !error;

  const rows: RundownVendor[] = useMemo(() => {
    if (!current) return [];
    const categoryName = new Map(categories.map((c) => [c.id, c.name]));
    const vendorName = new Map(vendors.map((v) => [v.id, v.name]));
    return (
      current.engagements
        .map((pv) => ({
          categoryLabel: (categoryName.get(pv.categoryId) ?? "").toUpperCase(),
          vendorName: (vendorName.get(pv.vendorId) ?? "").toUpperCase(),
        }))
        .filter((r) => r.vendorName)
        // Dikelompokkan per kategori supaya renderer hanya mencetak judul
        // kategorinya sekali, persis seperti berkas contoh.
        .sort((a, b) => a.categoryLabel.localeCompare(b.categoryLabel))
    );
  }, [current, categories, vendors]);

  return { rows, loading, error };
}
