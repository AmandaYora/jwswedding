import { useEffect, useState } from "react";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";
import { Input, Textarea, Select, Field } from "@/shared/components/ui/Input";
import { CurrencyInput } from "@/shared/components/ui/CurrencyInput";
import { projectSchema, PROJECT_STATUS_OPTIONS, EVENT_SESSION_PRESETS, type ProjectFormValues } from "@/modules/projects/schemas/project.schema";
import type { Project } from "@/modules/projects/types";
import { useStaffStore } from "@/modules/users/stores/useStaffStore";
import { useAuthStore } from "@/shared/stores/useAuthStore";
import { usePackageTemplateStore } from "@/modules/package-templates/stores/usePackageTemplateStore";

interface ProjectFormModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (values: ProjectFormValues) => void;
  initialProject?: Project;
  // "duplicate" pre-fills every field from initialProject (like edit) but is
  // meant to create a brand-new project — see ADR-0014. Name gets a "(Salinan)"
  // suffix and status resets to Draft as safer defaults; everything else
  // (including dates) is copied verbatim for the user to adjust as needed.
  mode?: "edit" | "duplicate";
  // D15/F7 — once a project has a PO Paket, its contract value is DERIVED
  // (harga paket + penyesuaian) and recomputed backend-side on every
  // composition change. Leaving the field writable here would let someone
  // type a number that silently reverts the next time a block is edited, so
  // it becomes read-only with the reason stated inline.
  contractValueLocked?: boolean;
  // canEditGeneral gates every field except Status Project and Deskripsi —
  // confirmed role rule, PLAN.md mom-25082026-item-sebagian §3c: a
  // Wedding Planner or Sales caller may only change a project's status and
  // description, never its identity/schedule/package/contract value (the
  // backend's guardKonteksUmum rejects those regardless, this just avoids
  // a 403 round-trip). Defaults to true so every other caller of this modal
  // (creating a new project) keeps its current unrestricted behavior.
  canEditGeneral?: boolean;
}

// Jam Acara preset handling (Blok A) — mirrors ProjectVendorFormModal's own
// preset/custom pattern. CUSTOM_SESSION is the sentinel for a hand-typed pair.
const CUSTOM_SESSION = "__custom__";

// sessionPresetKeyFor maps the current start/end pair back to a preset label,
// falling back to CUSTOM_SESSION for any non-preset pair so a hand-typed value
// round-trips instead of snapping to the nearest preset.
function sessionPresetKeyFor(start: string, end: string): string {
  const match = EVENT_SESSION_PRESETS.find((p) => p.start === start && p.end === end);
  return match ? match.label : CUSTOM_SESSION;
}

function toFormValues(project?: Project, defaultStaffId = "", mode?: "edit" | "duplicate"): ProjectFormValues {
  if (!project) {
    return {
      name: "",
      brideName: "",
      groomName: "",
      eventDate: "",
      eventStartTime: "",
      eventEndTime: "",
      pax: 0,
      venue: "",
      prepStartDate: "",
      packageName: "",
      contractValue: 0,
      packageTemplateId: "",
      status: "Draft",
      picStaffId: defaultStaffId,
      picSalesStaffId: "",
      description: "",
    };
  }
  return {
    name: mode === "duplicate" ? `${project.name} (Salinan)` : project.name,
    brideName: project.brideName,
    groomName: project.groomName,
    eventDate: project.eventDate,
    eventStartTime: project.eventStartTime ?? "",
    eventEndTime: project.eventEndTime ?? "",
    pax: project.pax,
    venue: project.venue,
    prepStartDate: project.prepStartDate,
    packageName: project.packageName,
    contractValue: project.contractValue,
    packageTemplateId: "",
    status: mode === "duplicate" ? "Draft" : project.status,
    picStaffId: project.picStaffId,
    picSalesStaffId: project.picSalesStaffId,
    description: project.description,
  };
}

export function ProjectFormModal({
  open,
  onClose,
  onSubmit,
  initialProject,
  mode,
  canEditGeneral = true,
  contractValueLocked = false,
}: ProjectFormModalProps) {
  const staffList = useStaffStore((s) => s.staffSummaries);
  const fetchStaff = useStaffStore((s) => s.fetchStaffSummaries);
  const role = useAuthStore((s) => s.session?.role);
  const currentStaffId = useAuthStore((s) => s.currentStaffId);
  const [values, setValues] = useState<ProjectFormValues>(() => toFormValues(initialProject, "", mode));
  const [errors, setErrors] = useState<Partial<Record<keyof ProjectFormValues, string>>>({});

  // D10 — the template picker only appears on create. On edit it would be
  // meaningless (the composition is already copied and lives on the PO), and
  // on duplicate the composition comes from the source project instead (D19).
  const isCreate = !initialProject;
  const packageTemplates = usePackageTemplateStore((s) => s.templates);
  const fetchPackageTemplates = usePackageTemplateStore((s) => s.fetchTemplates);
  useEffect(() => {
    if (open && isCreate) void fetchPackageTemplates(true);
  }, [open, isCreate, fetchPackageTemplates]);

  // Wedding Planner never reassigns a project's PIC, even their own project's
  // (only Owner/Admin do — see PLAN.md's RBAC section); the backend already
  // rejects a changed picStaffId from this role, so this is a read-only
  // display for them, not a disabled-but-still-submittable control.
  const picLocked = role === "Staff" && Boolean(initialProject) && mode !== "duplicate";
  // Sales always creates a project as their own PIC Sales — the backend
  // forces this server-side regardless of what's submitted, so the field is
  // shown read-only (locked to self) for them rather than a disabled-but-
  // submittable control. Owner/Admin keep a free picker, including on an
  // existing project (mirrors picStaffId's own edit affordance).
  const picSalesLocked = role === "Sales" && !initialProject;

  useEffect(() => {
    void fetchStaff();
  }, [fetchStaff]);

  useEffect(() => {
    if (!initialProject && !values.picStaffId && staffList.length > 0 && role !== "Sales") {
      setValues((prev) => ({ ...prev, picStaffId: staffList[0].id }));
    }
  }, [initialProject, staffList, values.picStaffId, role]);

  useEffect(() => {
    if (!initialProject && picSalesLocked) {
      setValues((prev) => (prev.picSalesStaffId === currentStaffId ? prev : { ...prev, picSalesStaffId: currentStaffId }));
    }
  }, [initialProject, picSalesLocked, currentStaffId]);

  function set<K extends keyof ProjectFormValues>(key: K, value: ProjectFormValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  // Selecting a preset fills both time fields; picking Custom clears them so
  // the manual inputs start blank. Same shape as ProjectVendorFormModal.
  function handleSessionPresetChange(key: string) {
    if (key === CUSTOM_SESSION) {
      setValues((prev) => ({ ...prev, eventStartTime: "", eventEndTime: "" }));
      return;
    }
    const preset = EVENT_SESSION_PRESETS.find((p) => p.label === key);
    if (preset) {
      setValues((prev) => ({ ...prev, eventStartTime: preset.start, eventEndTime: preset.end }));
    }
  }

  const currentSessionPresetKey = sessionPresetKeyFor(values.eventStartTime, values.eventEndTime);

  function handleSubmit() {
    const result = projectSchema.safeParse(values);
    if (!result.success) {
      const fieldErrors: Partial<Record<keyof ProjectFormValues, string>> = {};
      for (const issue of result.error.issues) {
        const key = issue.path[0] as keyof ProjectFormValues;
        fieldErrors[key] = issue.message;
      }
      setErrors(fieldErrors);
      return;
    }
    onSubmit(result.data);
    setValues(toFormValues(undefined, role === "Sales" ? "" : staffList[0]?.id ?? ""));
    setErrors({});
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "duplicate" ? "Duplikat Project" : initialProject ? "Ubah Project" : "Tambah Project Baru"}
      description={
        mode === "duplicate"
          ? "Project baru berdasarkan project ini — timeline dan daftar vendor akan ikut disalin sebagai template. Sesuaikan bagian yang berbeda sebelum menyimpan."
          : "Informasi dasar project pernikahan yang dikelola WO."
      }
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={handleSubmit}>
            {mode === "duplicate" ? "Buat Duplikat" : initialProject ? "Simpan Perubahan" : "Simpan Project"}
          </Button>
        </>
      }
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field label="Nama Project" required hint={errors.name}>
          <Input value={values.name} onChange={(e) => set("name", e.target.value)} placeholder="cth. Aurelia & Bagas Wedding" disabled={!canEditGeneral} />
        </Field>
        <Field label="Status Project" required>
          <Select value={values.status} onChange={(e) => set("status", e.target.value as ProjectFormValues["status"])}>
            {PROJECT_STATUS_OPTIONS.map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </Select>
        </Field>
        <Field label="Nama Mempelai Wanita" required hint={errors.brideName}>
          <Input value={values.brideName} onChange={(e) => set("brideName", e.target.value)} disabled={!canEditGeneral} />
        </Field>
        <Field label="Nama Mempelai Pria" required hint={errors.groomName}>
          <Input value={values.groomName} onChange={(e) => set("groomName", e.target.value)} disabled={!canEditGeneral} />
        </Field>
        <Field label="Tanggal Acara" required hint={errors.eventDate}>
          <Input type="date" value={values.eventDate} onChange={(e) => set("eventDate", e.target.value)} disabled={!canEditGeneral} />
        </Field>
        <Field label="Jam Acara">
          <Select value={currentSessionPresetKey} onChange={(e) => handleSessionPresetChange(e.target.value)} disabled={!canEditGeneral}>
            {EVENT_SESSION_PRESETS.map((p) => (
              <option key={p.label} value={p.label}>{p.label}</option>
            ))}
            <option value={CUSTOM_SESSION}>Custom</option>
          </Select>
        </Field>
        {currentSessionPresetKey === CUSTOM_SESSION && (
          <Field label="Jam Mulai - Selesai (Custom)">
            <div className="flex items-center gap-2">
              <Input type="time" value={values.eventStartTime} onChange={(e) => set("eventStartTime", e.target.value)} disabled={!canEditGeneral} />
              <span className="text-text-secondary">–</span>
              <Input type="time" value={values.eventEndTime} onChange={(e) => set("eventEndTime", e.target.value)} disabled={!canEditGeneral} />
            </div>
          </Field>
        )}
        <Field label="Tanggal Booking" required hint={errors.prepStartDate}>
          <Input type="date" value={values.prepStartDate} onChange={(e) => set("prepStartDate", e.target.value)} disabled={!canEditGeneral} />
        </Field>
        <Field label="Lokasi / Venue" required hint={errors.venue}>
          <Input value={values.venue} onChange={(e) => set("venue", e.target.value)} disabled={!canEditGeneral} />
        </Field>
        <Field label="Jumlah Pax" hint={errors.pax ?? "Tercetak pada kop PO Paket. Kosongkan bila belum ditentukan."}>
          <Input
            type="number"
            min={0}
            value={values.pax || ""}
            onChange={(e) => set("pax", Number(e.target.value || 0))}
            disabled={!canEditGeneral}
          />
        </Field>
        {isCreate && (
          <Field
            label="Template Paket"
            hint="Mengisi komposisi, syarat & ketentuan, dan rencana termin PO sekaligus. Pilih “Tanpa template” untuk mengisinya sendiri nanti."
          >
            <Select
              value={values.packageTemplateId ?? ""}
              placeholder="Tanpa template"
              onChange={(e) => {
                const picked = packageTemplates.find((t) => t.id === e.target.value);
                set("packageTemplateId", e.target.value);
                // Prefill, never lock: the agreed figure is a negotiation
                // result, so the consultant can still type over it and D23
                // makes that typed value win over the template's list price.
                if (picked) {
                  set("packageName", picked.name);
                  set("contractValue", picked.basePrice);
                }
              }}
            >
              {packageTemplates.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name}
                </option>
              ))}
            </Select>
          </Field>
        )}
        <Field label="Paket / Layanan" required hint={errors.packageName}>
          <Input value={values.packageName} onChange={(e) => set("packageName", e.target.value)} disabled={!canEditGeneral} />
        </Field>
        <Field
          label="Nilai Kontrak (Rp)"
          required
          hint={errors.contractValue ?? (contractValueLocked ? "Dihitung dari tab Paket & PO." : undefined)}
        >
          <CurrencyInput
            value={values.contractValue}
            onChange={(n) => set("contractValue", n)}
            disabled={!canEditGeneral || contractValueLocked}
          />
        </Field>
        <Field label="Penanggung Jawab WO (PIC Wedding Planner)" hint={errors.picStaffId}>
          {picLocked ? (
            <Input value={staffList.find((s) => s.id === values.picStaffId)?.name ?? "..."} disabled />
          ) : (
            <Select value={values.picStaffId} onChange={(e) => set("picStaffId", e.target.value)}>
              <option value="">Belum ditugaskan</option>
              {staffList.map((s) => (
                <option key={s.id} value={s.id}>{s.name} — {s.title}</option>
              ))}
            </Select>
          )}
        </Field>
        <Field label="PIC Sales" hint={errors.picSalesStaffId}>
          {picSalesLocked ? (
            <Input value={staffList.find((s) => s.id === values.picSalesStaffId)?.name ?? "..."} disabled />
          ) : (
            <Select value={values.picSalesStaffId} onChange={(e) => set("picSalesStaffId", e.target.value)}>
              <option value="">Belum ditugaskan</option>
              {staffList.map((s) => (
                <option key={s.id} value={s.id}>{s.name} — {s.title}</option>
              ))}
            </Select>
          )}
        </Field>
        <div className="sm:col-span-2">
          <Field label="Deskripsi / Catatan Project">
            <Textarea rows={3} value={values.description} onChange={(e) => set("description", e.target.value)} />
          </Field>
        </div>
      </div>
    </Modal>
  );
}
