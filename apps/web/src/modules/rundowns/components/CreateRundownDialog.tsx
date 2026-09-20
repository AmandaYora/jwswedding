import { useEffect, useMemo, useState } from "react";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";
import { Field, Input, Select } from "@/shared/components/ui/Input";
import { useRundownStore } from "@/modules/rundowns/stores/useRundownStore";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import { useVendorStore } from "@/modules/vendors/stores/useVendorStore";
import { useVendorCategoryStore } from "@/modules/vendor-categories/stores/useVendorCategoryStore";
import { useStaffStore } from "@/modules/users/stores/useStaffStore";
import type { RundownVendor } from "@/modules/rundowns/types";
import { getApiErrorMessage } from "@/shared/lib/api-error";

interface Props {
  open: boolean;
  onClose: () => void;
  onCreated: (id: string) => void;
}

/**
 * Dialog "Buat Rundown".
 *
 * Halaman VENDORS dirakit DI SINI, bukan di backend: nama vendor dan kategori
 * milik modul `vendors`, dan konvensi repo ini (knowledge/MODULE_MAP.md)
 * menaruh resolusi nama tampilan di frontend yang store-nya memang sudah
 * termuat. Backend menerimanya sebagai snapshot — itu juga yang membuat buku
 * acara yang sudah dicetak tidak ikut berubah saat master vendor disunting.
 */
export function CreateRundownDialog({ open, onClose, onCreated }: Props) {
  const create = useRundownStore((s) => s.create);
  const usedProjectIds = useRundownStore((s) => s.usedProjectIds);
  const fetchUsedProjectIds = useRundownStore((s) => s.fetchUsedProjectIds);

  const projects = useProjectStore((s) => s.projects);
  const fetchProjects = useProjectStore((s) => s.fetchProjects);
  const vendorEngagements = useProjectStore((s) => s.vendorEngagements);
  const fetchVendorSection = useProjectStore((s) => s.fetchVendorSection);

  const vendors = useVendorStore((s) => s.vendors);
  const fetchVendors = useVendorStore((s) => s.fetchVendors);
  const categories = useVendorCategoryStore((s) => s.categories);
  const fetchCategories = useVendorCategoryStore((s) => s.fetchCategories);
  const staffSummaries = useStaffStore((s) => s.staffSummaries);
  const fetchStaffSummaries = useStaffStore((s) => s.fetchStaffSummaries);

  const [projectId, setProjectId] = useState("");
  const [woPicName, setWoPicName] = useState("");
  const [woPicPhone, setWoPicPhone] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setError("");
    void fetchUsedProjectIds();
    void fetchProjects();
    void fetchVendors();
    void fetchCategories();
    void fetchStaffSummaries();
  }, [open, fetchUsedProjectIds, fetchProjects, fetchVendors, fetchCategories, fetchStaffSummaries]);

  // Begitu project dipilih, tarik daftar vendor project itu supaya halaman
  // VENDORS terisi otomatis.
  useEffect(() => {
    if (!projectId) return;
    void fetchVendorSection(projectId);
  }, [projectId, fetchVendorSection]);

  const selectable = useMemo(() => {
    const used = new Set(usedProjectIds);
    return projects.filter((p) => !used.has(p.id));
  }, [projects, usedProjectIds]);

  const selected = useMemo(
    () => projects.find((p) => p.id === projectId),
    [projects, projectId]
  );

  // Nama PIC diisikan dari PIC project; WO tetap boleh menggantinya.
  useEffect(() => {
    if (!selected) return;
    const pic = staffSummaries.find((s) => s.id === selected.picStaffId);
    setWoPicName((prev) => prev || pic?.name || "");
  }, [selected, staffSummaries]);

  const vendorPrefill: RundownVendor[] = useMemo(() => {
    if (!projectId) return [];
    const categoryName = new Map(categories.map((c) => [c.id, c.name]));
    const vendorName = new Map(vendors.map((v) => [v.id, v.name]));
    const rows = vendorEngagements.map((pv) => ({
      categoryLabel: (categoryName.get(pv.categoryId) ?? "").toUpperCase(),
      vendorName: (vendorName.get(pv.vendorId) ?? "").toUpperCase(),
    }));
    // Dikelompokkan per kategori supaya renderer hanya mencetak judul
    // kategorinya sekali, persis seperti berkas contoh.
    return rows
      .filter((r) => r.vendorName)
      .sort((a, b) => a.categoryLabel.localeCompare(b.categoryLabel));
  }, [projectId, vendorEngagements, categories, vendors]);

  const submit = async () => {
    if (!projectId) {
      setError("Project wajib dipilih");
      return;
    }
    setSaving(true);
    setError("");
    try {
      const detail = await create(
        { projectId, woPicName, woPicPhone, eventTimeLabel: "" },
        vendorPrefill
      );
      onCreated(detail.id);
    } catch (e) {
      setError(getApiErrorMessage(e, "Gagal membuat rundown"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Buat Rundown"
      description="Pilih project. Nama pengantin, tanggal, venue, dan daftar vendor terisi otomatis dari project tersebut."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button onClick={submit} disabled={saving || !projectId}>
            {saving ? "Menyimpan..." : "Buat Rundown"}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <Field label="Project" required hint="Project yang sudah punya rundown tidak ditampilkan.">
          <Select value={projectId} onChange={(e) => setProjectId(e.target.value)} placeholder="Pilih project...">
            {selectable.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        </Field>

        {selected && (
          <div className="rounded-md border border-border bg-surface-muted px-3 py-2.5 text-[13px] text-text-secondary">
            <p>
              <span className="font-medium text-text-primary">
                {selected.brideName} &amp; {selected.groomName}
              </span>
            </p>
            <p>{selected.venue || "Venue belum ditentukan"}</p>
            <p>
              {vendorPrefill.length > 0
                ? `${vendorPrefill.length} vendor akan disalin ke halaman VENDORS`
                : "Belum ada vendor terpasang di project ini"}
            </p>
          </div>
        )}

        <Field label="Nama PIC WO" hint="Tercetak di blok ORGANIZED BY.">
          <Input value={woPicName} onChange={(e) => setWoPicName(e.target.value)} />
        </Field>
        <Field label="Nomor PIC WO">
          <Input value={woPicPhone} onChange={(e) => setWoPicPhone(e.target.value)} placeholder="0856-0000-0000" />
        </Field>

        {error && <p className="text-[13px] text-danger">{error}</p>}
      </div>
    </Modal>
  );
}
