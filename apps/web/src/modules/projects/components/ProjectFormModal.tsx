import { useEffect, useState } from "react";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";
import { Input, Textarea, Select, Field } from "@/shared/components/ui/Input";
import { CurrencyInput } from "@/shared/components/ui/CurrencyInput";
import { projectSchema, PROJECT_STATUS_OPTIONS, EVENT_SESSION_PRESETS, type ProjectFormValues } from "@/modules/projects/schemas/project.schema";
import type { Project } from "@/modules/projects/types";
import { useStaffStore } from "@/modules/users/stores/useStaffStore";
import { useAuthStore } from "@/shared/stores/useAuthStore";

interface ProjectFormModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (values: ProjectFormValues) => void;
  initialProject: Project;
  // canEditGeneral gates every field except Status Project and Deskripsi —
  // confirmed role rule, PLAN.md mom-25082026-item-sebagian §3c: a
  // Wedding Planner or Sales caller may only change a project's status and
  // description, never its identity/schedule/package/contract value (the
  // backend's guardKonteksUmum rejects those regardless, this just avoids
  // a 403 round-trip).
  canEditGeneral?: boolean;
}

// Modal Ubah Project — SATU-SATUNYA mode (D11, T3.9: project baru selalu
// lahir dari penawaran Diterima; duplikat project dihapus, D12). Field
// turunan (mempelai, venue teks, paket, nilai kontrak) read-only (D16, T4.2)
// — diubah dari Penawaran, bukan dari sini.
interface ReadonlyFieldProps {
  label: string;
  value: string;
}

function ReadonlyField({ label, value }: ReadonlyFieldProps) {
  return (
    <Field label={label}>
      <Input value={value} disabled />
    </Field>
  );
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

function toFormValues(project: Project): ProjectFormValues {
  return {
    name: project.name,
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
    status: project.status,
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
  canEditGeneral = true,
}: ProjectFormModalProps) {
  const staffList = useStaffStore((s) => s.staffSummaries);
  const fetchStaff = useStaffStore((s) => s.fetchStaffSummaries);
  const role = useAuthStore((s) => s.session?.role);
  const [values, setValues] = useState<ProjectFormValues>(() => toFormValues(initialProject));
  const [errors, setErrors] = useState<Partial<Record<keyof ProjectFormValues, string>>>({});
  // Nilai turunan terkunci bila project lahir dari penawaran (D15, D16) —
  // diubah dari Penawaran; project lama pra-penawaran tetap bisa diketik.
  const derivedLocked = Boolean(initialProject.quotationId);
  // Nama paket dikunci dengan syarat yang LEBIH SEMPIT daripada nilai kontrak:
  // hanya bila penawarannya benar-benar punya nama untuk didorong. Penawaran
  // lama (sebelum migrasi 000065) tidak punya, jadi tidak ada yang akan
  // menimpa ketikan di sini — dan justru di situlah nilainya masih berupa
  // kategori blok pertama ("CATERING") atau kosong, dan perlu dirapikan
  // sekali. Kunci yang membatasi dirinya sendiri, bukan pintu permanen.
  const packageNameLocked = derivedLocked && initialProject.packageNameFromQuotation;

  // Wedding Planner never reassigns a project's PIC, even their own project's
  // (only Owner/Admin do — see PLAN.md's RBAC section); the backend already
  // rejects a changed picStaffId from this role, so this is a read-only
  // display for them, not a disabled-but-still-submittable control.
  const picLocked = role === "Staff";

  useEffect(() => {
    void fetchStaff();
  }, [fetchStaff]);

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
    setErrors({});
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Ubah Project"
      description="Nilai turunan (mempelai, venue, paket, nilai kontrak) hanya tampil — diubah dari Penawaran."
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={handleSubmit}>
            Simpan Perubahan
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
        {derivedLocked ? (
          <>
            <ReadonlyField label="Mempelai Wanita" value={values.brideName} />
            <ReadonlyField label="Mempelai Pria" value={values.groomName} />
          </>
        ) : (
          <>
            <Field label="Nama Mempelai Wanita" required hint={errors.brideName}>
              <Input value={values.brideName} onChange={(e) => set("brideName", e.target.value)} disabled={!canEditGeneral} />
            </Field>
            <Field label="Nama Mempelai Pria" required hint={errors.groomName}>
              <Input value={values.groomName} onChange={(e) => set("groomName", e.target.value)} disabled={!canEditGeneral} />
            </Field>
          </>
        )}
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
        {derivedLocked ? (
          <ReadonlyField label="Lokasi / Venue" value={values.venue} />
        ) : (
          <Field label="Lokasi / Venue" required hint={errors.venue}>
            <Input value={values.venue} onChange={(e) => set("venue", e.target.value)} disabled={!canEditGeneral} />
          </Field>
        )}
        <Field label="Jumlah Pax" hint={errors.pax ?? "Tercetak pada kop PO Paket. Kosongkan bila belum ditentukan."}>
          <Input
            type="number"
            min={0}
            value={values.pax || ""}
            onChange={(e) => set("pax", Number(e.target.value || 0))}
            disabled={!canEditGeneral}
          />
        </Field>
        {derivedLocked ? (
          <>
            {packageNameLocked ? (
              <ReadonlyField label="Paket / Layanan" value={values.packageName} />
            ) : (
              <Field
                label="Paket / Layanan"
                required
                hint={errors.packageName ?? "Penawaran ini belum punya nama paket, jadi masih diisi di sini."}
              >
                <Input
                  value={values.packageName}
                  onChange={(e) => set("packageName", e.target.value)}
                  disabled={!canEditGeneral}
                />
              </Field>
            )}
            <Field label="Nilai Kontrak (Rp)" hint="Turunan dari total penawaran — diubah dari Penawaran.">
              <CurrencyInput value={values.contractValue} onChange={() => {}} disabled />
            </Field>
          </>
        ) : (
          <>
            <Field label="Paket / Layanan" required hint={errors.packageName}>
              <Input value={values.packageName} onChange={(e) => set("packageName", e.target.value)} disabled={!canEditGeneral} />
            </Field>
            <Field
              label="Nilai Kontrak (Rp)"
              required
              hint={errors.contractValue}
            >
              <CurrencyInput
                value={values.contractValue}
                onChange={(n) => set("contractValue", n)}
                disabled={!canEditGeneral}
              />
            </Field>
          </>
        )}
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
          <Select value={values.picSalesStaffId} onChange={(e) => set("picSalesStaffId", e.target.value)}>
            <option value="">Belum ditugaskan</option>
            {staffList.map((s) => (
              <option key={s.id} value={s.id}>{s.name} — {s.title}</option>
            ))}
          </Select>
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
