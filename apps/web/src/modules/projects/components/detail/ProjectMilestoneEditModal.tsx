import { useState } from "react";
import { Plus, FileText } from "lucide-react";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";
import { Badge } from "@/shared/components/ui/Badge";
import { Input, Select, Field } from "@/shared/components/ui/Input";
import { EvidenceViewerModal } from "@/shared/components/ui/EvidenceViewerModal";
import { EVIDENCE_TYPE_OPTIONS } from "@/modules/projects/schemas/evidence.schema";
import type { Evidence, EvidenceType, MilestoneStatus, ProjectMilestone } from "@/modules/projects/types";
import { todayISO } from "@/modules/projects/lib/dates";
import { formatDate } from "@/shared/lib/formatters";

const STATUS_OPTIONS: MilestoneStatus[] = ["Not Started", "In Progress", "Completed", "Blocked", "Cancelled"];

export interface ProjectMilestoneEditFields {
  status: MilestoneStatus;
  targetDate: string;
  completedDate: string;
}

export interface NewMilestoneEvidenceMeta {
  name: string;
  type: EvidenceType;
  documentDate: string;
  description: string;
}

interface ProjectMilestoneEditModalProps {
  open: boolean;
  onClose: () => void;
  milestone: ProjectMilestone;
  projectId: string;
  evidenceList: Evidence[];
  onSave: (fields: ProjectMilestoneEditFields) => void;
  onAddEvidence: (file: File, meta: NewMilestoneEvidenceMeta) => Promise<void>;
}

// Lampiran (Evidence) section mirrors VendorMilestoneEditModal's own —
// PLAN.md mom-25082026-item-sebagian item 6: attachment moves out of the
// Aksi column and into this Edit modal, positioned right after Tanggal
// Selesai (MOM's "attachment untuk target selesai"). One addition over the
// vendor-milestone sibling: each row is clickable and opens
// EvidenceViewerModal, satisfying MOM's "file dapat dilihat ketika
// attachment diklik" — the vendor-milestone list doesn't do this today, but
// nothing here stops that screen from picking up the same affordance later.
export function ProjectMilestoneEditModal({
  open,
  onClose,
  milestone,
  projectId,
  evidenceList,
  onSave,
  onAddEvidence,
}: ProjectMilestoneEditModalProps) {
  const [fields, setFields] = useState<ProjectMilestoneEditFields>({
    status: milestone.status,
    targetDate: milestone.targetDate,
    completedDate: milestone.completedDate ?? "",
  });
  const [showEvidenceForm, setShowEvidenceForm] = useState(false);
  const [evidenceForm, setEvidenceForm] = useState({ name: "", type: "Document" as EvidenceType, documentDate: "", description: "" });
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [viewingEvidence, setViewingEvidence] = useState<Evidence | null>(null);

  function set<K extends keyof ProjectMilestoneEditFields>(key: K, value: ProjectMilestoneEditFields[K]) {
    setFields((prev) => {
      const next = { ...prev, [key]: value };
      if (key === "status" && value === "Completed" && !prev.completedDate) {
        next.completedDate = todayISO();
      }
      return next;
    });
  }

  async function handleAddEvidence() {
    if (!file || !evidenceForm.name || !evidenceForm.documentDate) return;
    setUploading(true);
    setUploadError(null);
    try {
      await onAddEvidence(file, evidenceForm);
      setEvidenceForm({ name: "", type: "Document", documentDate: "", description: "" });
      setFile(null);
      setShowEvidenceForm(false);
    } catch {
      setUploadError("Gagal mengunggah lampiran.");
    } finally {
      setUploading(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={milestone.name}
      description="Perbarui status, jadwal, dan lampiran timeline ini."
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={() => onSave(fields)}>Simpan Perubahan</Button>
        </>
      }
    >
      <div className="flex flex-col gap-6">
        <section>
          <p className="mb-3 text-[12px] font-semibold uppercase tracking-wide text-text-secondary">Status &amp; Jadwal</p>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Status">
              <Select value={fields.status} onChange={(e) => set("status", e.target.value as MilestoneStatus)}>
                {STATUS_OPTIONS.map((s) => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </Select>
            </Field>
            <Field label="Target Tanggal">
              <Input type="date" value={fields.targetDate} onChange={(e) => set("targetDate", e.target.value)} />
            </Field>
            <Field label="Tanggal Selesai">
              <Input type="date" value={fields.completedDate} onChange={(e) => set("completedDate", e.target.value)} />
            </Field>
          </div>
        </section>

        <section>
          <div className="mb-3 flex items-center justify-between">
            <p className="text-[12px] font-semibold uppercase tracking-wide text-text-secondary">Lampiran</p>
            <Button size="sm" variant="secondary" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setShowEvidenceForm((v) => !v)}>
              Tambah Lampiran
            </Button>
          </div>

          {showEvidenceForm && (
            <div className="mb-3 flex flex-col gap-3 rounded-md border border-border bg-surface-muted/60 p-3">
              {uploadError && <p className="text-[12.5px] font-medium text-danger">{uploadError}</p>}
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <Field label="Berkas" required>
                  <input
                    type="file"
                    accept="image/jpeg,image/png,application/pdf"
                    onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                    className="block w-full text-[13px] text-text-secondary file:mr-3 file:rounded-md file:border-0 file:bg-navy-900 file:px-3 file:py-1.5 file:text-[12.5px] file:font-semibold file:text-white"
                  />
                </Field>
                <Field label="Nama Lampiran" required>
                  <Input value={evidenceForm.name} onChange={(e) => setEvidenceForm((p) => ({ ...p, name: e.target.value }))} placeholder="cth. Screenshot Technical Meeting" />
                </Field>
                <Field label="Jenis">
                  <Select value={evidenceForm.type} onChange={(e) => setEvidenceForm((p) => ({ ...p, type: e.target.value as EvidenceType }))}>
                    {EVIDENCE_TYPE_OPTIONS.map((t) => (
                      <option key={t} value={t}>{t}</option>
                    ))}
                  </Select>
                </Field>
                <Field label="Tanggal Dokumen" required>
                  <Input type="date" value={evidenceForm.documentDate} onChange={(e) => setEvidenceForm((p) => ({ ...p, documentDate: e.target.value }))} />
                </Field>
                <Field label="Deskripsi">
                  <Input value={evidenceForm.description} onChange={(e) => setEvidenceForm((p) => ({ ...p, description: e.target.value }))} />
                </Field>
              </div>
              <div className="flex justify-end gap-2">
                <Button size="sm" variant="secondary" onClick={() => setShowEvidenceForm(false)}>Batal</Button>
                <Button size="sm" onClick={() => void handleAddEvidence()} disabled={uploading || !file}>
                  {uploading ? "Mengunggah..." : "Simpan Lampiran"}
                </Button>
              </div>
            </div>
          )}

          {evidenceList.length === 0 ? (
            <p className="rounded-md border border-dashed border-border px-4 py-3 text-[13px] text-text-secondary">
              Belum ada lampiran untuk timeline ini.
            </p>
          ) : (
            <ul className="flex flex-col gap-2">
              {evidenceList.map((e) => (
                <li key={e.id}>
                  <button
                    type="button"
                    onClick={() => setViewingEvidence(e)}
                    className="flex w-full items-center gap-3 rounded-md border border-border px-3 py-2 text-left text-[13px] transition-colors hover:bg-surface-muted"
                  >
                    <FileText className="h-4 w-4 shrink-0 text-text-secondary" />
                    <span className="min-w-0 flex-1 truncate font-medium text-text-primary">{e.name}</span>
                    <Badge tone="neutral">{e.type}</Badge>
                    <span className="shrink-0 text-text-secondary">{formatDate(e.documentDate)}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      {viewingEvidence && (
        <EvidenceViewerModal
          open
          onClose={() => setViewingEvidence(null)}
          projectId={projectId}
          evidence={viewingEvidence}
        />
      )}
    </Modal>
  );
}
