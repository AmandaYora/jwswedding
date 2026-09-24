import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";
import { Field, Input, Select } from "@/shared/components/ui/Input";
import { useRundownStore } from "@/modules/rundowns/stores/useRundownStore";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import { useStaffStore } from "@/modules/users/stores/useStaffStore";
import { useProjectVendorPrefill } from "@/modules/rundowns/hooks/useProjectVendorPrefill";
import type { RundownTemplate } from "@/modules/rundowns/types";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import { formatDate } from "@/shared/lib/formatters";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

interface Props {
  open: boolean;
  onClose: () => void;
  onCreated: (id: string) => void;
}

/** Ringkasan isi template untuk dialog, mis. "12 peran, 18 acara akad". */
export function templateSummary(t: RundownTemplate): string[] {
  const parts: [number, string][] = [
    [t.roles.length, "peran"],
    [t.committees.length, "panitia"],
    [t.makeupRooms.length, "ruangan makeup"],
    [t.itemsAkad.length, "acara akad"],
    [t.itemsResepsi.length, "acara resepsi"],
    [t.layoutNotes.length, "catatan layout"],
  ];
  return parts.filter(([n]) => n > 0).map(([n, label]) => `${n} ${label}`);
}

type TemplateState = { status: "loading" } | { status: "error" } | { status: "ready"; template: RundownTemplate };

/**
 * Dialog "Buat Rundown". Vendor project dirakit oleh useProjectVendorPrefill
 * (lihat catatannya soal snapshot dan project yang tertukar).
 */
export function CreateRundownDialog({ open, onClose, onCreated }: Props) {
  const create = useRundownStore((s) => s.create);
  const usedProjectIds = useRundownStore((s) => s.usedProjectIds);
  const fetchUsedProjectIds = useRundownStore((s) => s.fetchUsedProjectIds);
  const fetchTemplate = useRundownStore((s) => s.fetchTemplate);

  const projects = useProjectStore((s) => s.projects);
  const fetchProjects = useProjectStore((s) => s.fetchProjects);
  const staffSummaries = useStaffStore((s) => s.staffSummaries);
  const fetchStaffSummaries = useStaffStore((s) => s.fetchStaffSummaries);

  const [projectId, setProjectId] = useState("");
  const [woPicName, setWoPicName] = useState("");
  const [woPicPhone, setWoPicPhone] = useState("");
  const [useTemplate, setUseTemplate] = useState(true);
  const [templateState, setTemplateState] = useState<TemplateState>({ status: "loading" });
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const vendorPrefill = useProjectVendorPrefill(open ? projectId : "");

  useEffect(() => {
    if (!open) return;
    setError("");
    setProjectId("");
    setWoPicName("");
    setWoPicPhone("");
    void fetchUsedProjectIds();
    void fetchProjects();
    void fetchStaffSummaries();
    setTemplateState({ status: "loading" });
    fetchTemplate()
      .then((template) => {
        setTemplateState({ status: "ready", template });
        setUseTemplate(templateSummary(template).length > 0);
      })
      .catch(() => {
        setTemplateState({ status: "error" });
        setUseTemplate(false);
      });
  }, [open, fetchUsedProjectIds, fetchProjects, fetchStaffSummaries, fetchTemplate]);

  // Project arsip tidak lagi dikerjakan; project yang sudah punya rundown
  // cukup dibuka dari daftar.
  const selectable = useMemo(() => {
    const used = new Set(usedProjectIds);
    return projects.filter((p) => !p.isArchived && !used.has(p.id));
  }, [projects, usedProjectIds]);

  const selected = useMemo(() => projects.find((p) => p.id === projectId), [projects, projectId]);

  // Nama PIC diisikan dari PIC project; WO tetap boleh menggantinya.
  useEffect(() => {
    if (!selected) return;
    const pic = staffSummaries.find((s) => s.id === selected.picStaffId);
    setWoPicName((prev) => prev || pic?.name || "");
  }, [selected, staffSummaries]);

  const summary = templateState.status === "ready" ? templateSummary(templateState.template) : [];
  const templateEmpty = templateState.status === "ready" && summary.length === 0;

  const submit = async () => {
    if (!projectId) {
      setError("Pilih project terlebih dahulu.");
      return;
    }
    setSaving(true);
    setError("");
    try {
      const detail = await create(
        { projectId, woPicName, woPicPhone, eventTimeLabel: "" },
        vendorPrefill.rows,
        useTemplate && summary.length > 0
      );
      onCreated(detail.id);
    } catch (e) {
      setError(getApiErrorMessage(e, "Gagal membuat rundown"));
    } finally {
      setSaving(false);
    }
  };

  const waitingVendors = !!projectId && vendorPrefill.loading;

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
          <Button onClick={submit} disabled={saving || !projectId || waitingVendors}>
            {saving ? "Membuat..." : waitingVendors ? "Memuat vendor..." : "Buat Rundown"}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <Field label="Project" required hint="Project arsip dan project yang sudah punya rundown tidak ditampilkan.">
          <Select value={projectId} onChange={(e) => setProjectId(e.target.value)} placeholder="Cari project...">
            {selectable.map((p) => (
              <option key={p.id} value={p.id}>
                {p.eventDate ? `${p.name} — ${formatDate(p.eventDate)}` : p.name}
              </option>
            ))}
          </Select>
        </Field>

        {selected && (
          <div className="rounded-md border border-border bg-surface-muted px-3 py-2.5 text-[13px] text-text-secondary">
            <p className="font-medium text-text-primary">
              {selected.brideName} &amp; {selected.groomName}
            </p>
            <p>{selected.venue || "Venue belum ditentukan"}</p>
            <p>
              {vendorPrefill.error
                ? vendorPrefill.error
                : waitingVendors
                  ? "Memuat vendor project..."
                  : vendorPrefill.rows.length > 0
                    ? `${vendorPrefill.rows.length} vendor akan disalin ke halaman Vendor`
                    : "Belum ada vendor terpasang di project ini"}
            </p>
          </div>
        )}

        <div className="rounded-md border border-border px-3 py-2.5">
          <label className="flex cursor-pointer items-start gap-3">
            <input
              type="checkbox"
              className="mt-0.5 h-4 w-4 rounded border-border accent-navy-900"
              checked={useTemplate && summary.length > 0}
              disabled={summary.length === 0}
              onChange={(e) => setUseTemplate(e.target.checked)}
            />
            <span className="flex flex-col gap-0.5 text-[13px]">
              <span className="font-medium text-text-primary">Isi dengan Template Rundown</span>
              <span className="text-text-secondary">
                {templateState.status === "loading" && "Memuat template..."}
                {templateState.status === "error" && "Template tidak dapat dimuat. Rundown dibuat tanpa template."}
                {templateState.status === "ready" && summary.length > 0 && summary.join(", ")}
                {templateEmpty && (
                  <>
                    Template masih kosong.{" "}
                    <Link to={ROUTE_PATHS.rundownTemplate()} className="font-medium text-text-primary underline">
                      Susun template
                    </Link>
                  </>
                )}
              </span>
            </span>
          </label>
        </div>

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
