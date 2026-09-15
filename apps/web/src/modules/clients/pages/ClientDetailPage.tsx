import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Pencil, KeyRound, UserCheck, UserX, Repeat, Trash2, Plus } from "lucide-react";
import { Badge } from "@/shared/components/ui/Badge";
import { Avatar } from "@/shared/components/ui/Avatar";
import { Input, Textarea, Field, Select } from "@/shared/components/ui/Input";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";
import { ConfirmDialog } from "@/shared/components/ui/ConfirmDialog";
import { Card, CardContent, CardHeader } from "@/shared/components/ui/Card";
import { IconActionButton } from "@/shared/components/ui/IconActionButton";
import { EmptyState } from "@/shared/components/feedback/EmptyState";
import { ClientRoleBadge } from "@/modules/clients/components/ClientRoleBadge";
import { ClientContactFormModal } from "@/modules/clients/components/ClientContactFormModal";
import type { ClientContactFormValues, ClientCreateFormValues, ClientMasterFormValues, RepresentativeFormValues } from "@/modules/clients/schemas/client.schema";
import { CLIENT_ROLE_OPTIONS } from "@/modules/clients/schemas/client.schema";
import { useClientStore } from "@/modules/clients/stores/useClientStore";
import type { ClientContact, ClientDeleteImpact, ClientRole } from "@/modules/clients/types";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import { formatCurrency, formatDate } from "@/shared/lib/formatters";
import { httpClient } from "@/shared/services/http-client";
import { API } from "@/shared/services/api-endpoints";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

type ContactModalMode = "edit" | "replace" | "reset" | "create";

export default function ClientDetailPage() {
  const { clientId } = useParams<{ clientId: string }>();
  const navigate = useNavigate();
  const currentClient = useClientStore((s) => s.currentClient);
  const contacts = useClientStore((s) => s.contacts);
  const fetchClient = useClientStore((s) => s.fetchClient);
  const updateClient = useClientStore((s) => s.updateClient);
  const fetchDeleteImpact = useClientStore((s) => s.fetchDeleteImpact);
  const deleteClient = useClientStore((s) => s.deleteClient);
  const createContact = useClientStore((s) => s.createContact);
  const updateContact = useClientStore((s) => s.updateContact);
  const toggleContactActive = useClientStore((s) => s.toggleContactActive);
  const deleteContact = useClientStore((s) => s.deleteContact);
  const resetContactCredential = useClientStore((s) => s.resetContactCredential);
  const replaceRepresentative = useClientStore((s) => s.replaceRepresentative);
  const clientSignature = useClientStore((s) => s.clientSignature);
  const fetchClientSignature = useClientStore((s) => s.fetchClientSignature);
  const deleteClientSignature = useClientStore((s) => s.deleteClientSignature);

  const [editMaster, setEditMaster] = useState<ClientMasterFormValues | null>(null);
  const [contactModal, setContactModal] = useState<{ mode: ContactModalMode; contact: ClientContact | null; role: ClientRole } | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [confirmDeleteSignature, setConfirmDeleteSignature] = useState(false);
  const [signaturePreview, setSignaturePreview] = useState<string | null>(null);
  const [impact, setImpact] = useState<ClientDeleteImpact | null>(null);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    if (clientId) {
      setLoadError(null);
      fetchClient(clientId).catch((err) => setLoadError(getApiErrorMessage(err, "Gagal memuat client")));
      fetchClientSignature(clientId).catch(() => {
        // Tanpa specimen = kartu kosong, bukan galat halaman.
      });
    }
  }, [clientId, fetchClient, fetchClientSignature]);

  // Pratinjau gambar specimen — maksimum satu (D12).
  useEffect(() => {
    if (!clientId || !clientSignature) {
      setSignaturePreview(null);
      return;
    }
    let cancelled = false;
    let objectUrl: string | null = null;
    void httpClient
      .get(API.clients.signatureImage(clientId), { responseType: "blob" })
      .then((res) => {
        if (cancelled) return;
        objectUrl = URL.createObjectURL(res.data as Blob);
        setSignaturePreview(objectUrl);
      })
      .catch(() => {
        if (!cancelled) setSignaturePreview(null);
      });
    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
      setSignaturePreview(null);
    };
  }, [clientId, clientSignature]);

  if (!clientId) return <p className="py-10 text-center text-[13px] text-text-secondary">Client tidak ditemukan.</p>;
  if (loadError) {
    return (
      <div className="flex flex-col gap-5">
        <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{loadError}</p>
      </div>
    );
  }
  if (!currentClient) return <p className="py-10 text-center text-[13px] text-text-secondary">Memuat client...</p>;

  async function handleMasterSubmit() {
    if (!editMaster) return;
    setBusy(true);
    setActionError(null);
    try {
      await updateClient(clientId!, editMaster);
      setEditMaster(null);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal menyimpan perubahan client"));
    } finally {
      setBusy(false);
    }
  }

  async function handleContactSubmit(values: ClientContactFormValues | ClientCreateFormValues | RepresentativeFormValues) {
    if (!contactModal) return;
    setActionError(null);
    try {
      if (contactModal.mode === "create") {
        await createContact(clientId!, contactModal.role, values as ClientCreateFormValues);
      } else if (contactModal.contact && contactModal.mode === "replace") {
        await replaceRepresentative(clientId!, contactModal.contact.id, values as RepresentativeFormValues);
      } else if (contactModal.contact) {
        await updateContact(clientId!, contactModal.contact.id, values as ClientContactFormValues);
      }
      setContactModal(null);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal menyimpan kontak"));
    }
  }

  async function handleResetSubmit(password: string) {
    if (!contactModal?.contact) return;
    setActionError(null);
    try {
      await resetContactCredential(clientId!, contactModal.contact.id, password);
      setContactModal(null);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal mereset kredensial"));
    }
  }

  async function openDeleteConfirm() {
    setActionError(null);
    setBusy(true);
    try {
      // D14: dialog menolak tampil kalau dampaknya gagal dibaca.
      const data = await fetchDeleteImpact(clientId!);
      setImpact(data);
      setConfirmDelete(true);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal membaca dampak penghapusan"));
    } finally {
      setBusy(false);
    }
  }

  async function handleDelete() {
    setBusy(true);
    try {
      await deleteClient(clientId!);
      navigate(ROUTE_PATHS.clients);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal menghapus client"));
      setBusy(false);
    }
  }

  async function handleDeleteSignature() {
    setBusy(true);
    setActionError(null);
    try {
      await deleteClientSignature(clientId!);
      setSignaturePreview(null);
      setConfirmDeleteSignature(false);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal menghapus TTD tersimpan"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-bold text-text-primary">{currentClient.displayName}</h1>
        <p className="mt-1 text-[13px] text-text-secondary">
          {currentClient.phone || "Tanpa telepon"} · {currentClient.email || "Tanpa email"} · {currentClient.projectCount} project
        </p>
      </div>

      {actionError && (
        <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{actionError}</p>
      )}

      <Card>
        <CardHeader
          title="Data Pasangan"
          action={
            <Button
              size="sm"
              variant="secondary"
              icon={<Pencil className="h-3.5 w-3.5" />}
              onClick={() =>
                setEditMaster({
                  brideName: currentClient.brideName,
                  groomName: currentClient.groomName,
                  phone: currentClient.phone,
                  email: currentClient.email,
                  notes: currentClient.notes,
                })
              }
            >
              Ubah
            </Button>
          }
        />
        <CardContent className="grid gap-2 text-[13px] sm:grid-cols-2">
          <InfoRow label="Mempelai Wanita" value={currentClient.brideName} />
          <InfoRow label="Mempelai Pria" value={currentClient.groomName} />
          <InfoRow label="Telepon" value={currentClient.phone || "—"} />
          <InfoRow label="Email" value={currentClient.email || "—"} />
          {currentClient.notes && (
            <p className="rounded-md bg-surface-muted px-3 py-2 text-text-secondary sm:col-span-2">{currentClient.notes}</p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader title="Tanda Tangan" subtitle="Specimen untuk dipakai ulang saat menerima penawaran — maksimum satu." />
        <CardContent className="flex flex-col gap-2">
          {!clientSignature ? (
            <EmptyState
              title="Belum ada TTD tersimpan"
              description="Specimen tercatat otomatis setiap kali client menandatangani penawaran — lewat link, unggahan, maupun pemakaian ulang."
            />
          ) : (
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border-light px-4 py-3">
              <div className="flex min-w-0 items-center gap-3">
                {signaturePreview ? (
                  <img
                    src={signaturePreview}
                    alt={`TTD milik ${clientSignature.signerName}`}
                    className="h-14 max-w-40 rounded border border-border-light bg-white object-contain px-2 py-1"
                  />
                ) : (
                  <span className="text-[12.5px] text-text-secondary">Memuat pratinjau…</span>
                )}
                <div className="min-w-0">
                  <p className="font-semibold text-text-primary">{clientSignature.signerName}</p>
                  <p className="text-[12.5px] text-text-secondary">
                    {clientSignature.role} · {formatDate(clientSignature.updatedAt)} ·{" "}
                    {clientSignature.source === "draw" ? "Digambar klien (link)" : "Diunggah pengelola"}
                  </p>
                </div>
              </div>
              <IconActionButton icon={Trash2} label="Hapus TTD tersimpan" tone="danger" onClick={() => setConfirmDeleteSignature(true)} />
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader title="Kontak & Akun Portal" subtitle="Masing-masing bisa punya akun login sendiri." />
        <CardContent className="flex flex-col gap-2">
          {contacts.length === 0 ? (
            <EmptyState title="Belum ada kontak" description="Tambah Bride, Groom, atau Family Representative beserta akunnya." />
          ) : (
            contacts.map((c) => (
              <ContactRow
                key={c.id}
                contact={c}
                onEdit={() => setContactModal({ mode: "edit", contact: c, role: c.role })}
                onReplace={() => setContactModal({ mode: "replace", contact: c, role: c.role })}
                onReset={() => setContactModal({ mode: "reset", contact: c, role: c.role })}
                onToggleActive={() => void toggleContactActive(clientId!, c.id).catch((err) => setActionError(getApiErrorMessage(err, "Gagal mengubah status")))}
                onDelete={() => void deleteContact(clientId!, c.id).catch((err) => setActionError(getApiErrorMessage(err, "Gagal menghapus kontak")))}
              />
            ))
          )}
          <div className="flex flex-wrap gap-2 pt-2">
            {CLIENT_ROLE_OPTIONS.filter((r) => !contacts.some((c) => c.role === r && r !== "Family Representative")).map((r) => (
              <Button key={r} size="sm" variant="secondary" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setContactModal({ mode: "create", contact: null, role: r })}>
                Tambah {r === "Bride" ? "Pengantin Wanita" : r === "Groom" ? "Pengantin Pria" : "Representative"}
              </Button>
            ))}
            <Button size="sm" variant="ghost" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setContactModal({ mode: "create", contact: null, role: "Family Representative" })}>
              Tambah Representative Lagi
            </Button>
          </div>
        </CardContent>
      </Card>

      <div>
        <Button variant="danger" icon={<Trash2 className="h-4 w-4" />} disabled={busy} onClick={() => void openDeleteConfirm()}>
          Hapus Client Permanen
        </Button>
      </div>

      {editMaster && (
        <Modal open onClose={() => setEditMaster(null)} title="Ubah Data Pasangan">
          <div className="flex flex-col gap-3">
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="Mempelai Wanita" required>
                <Input value={editMaster.brideName} onChange={(e) => setEditMaster({ ...editMaster, brideName: e.target.value })} />
              </Field>
              <Field label="Mempelai Pria" required>
                <Input value={editMaster.groomName} onChange={(e) => setEditMaster({ ...editMaster, groomName: e.target.value })} />
              </Field>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="Telepon">
                <Input value={editMaster.phone} onChange={(e) => setEditMaster({ ...editMaster, phone: e.target.value })} />
              </Field>
              <Field label="Email">
                <Input value={editMaster.email} onChange={(e) => setEditMaster({ ...editMaster, email: e.target.value })} />
              </Field>
            </div>
            <Field label="Catatan">
              <Textarea rows={3} value={editMaster.notes} onChange={(e) => setEditMaster({ ...editMaster, notes: e.target.value })} />
            </Field>
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={() => setEditMaster(null)}>Batal</Button>
              <Button disabled={busy} onClick={() => void handleMasterSubmit()}>{busy ? "Menyimpan..." : "Simpan"}</Button>
            </div>
          </div>
        </Modal>
      )}

      {contactModal && (contactModal.mode === "edit" || contactModal.mode === "replace" || contactModal.mode === "create") && (
        <ClientContactFormModal
          key={`${contactModal.mode}-${contactModal.contact?.id ?? "new"}`}
          open
          onClose={() => setContactModal(null)}
          onSubmit={(values) => void handleContactSubmit(values)}
          initialValues={
            contactModal.mode === "replace"
              ? { name: "", phone: "", email: "" }
              : contactModal.contact
                ? { name: contactModal.contact.name, phone: contactModal.contact.phone, email: contactModal.contact.email }
                : { name: "", phone: "", email: "" }
          }
          username={contactModal.contact?.username}
          mode={contactModal.mode}
          role={contactModal.role}
          title={
            contactModal.mode === "create"
              ? "Tambah Kontak & Akun Portal"
              : contactModal.mode === "replace"
                ? "Ganti Wedding Representative"
                : "Ubah Kontak"
          }
          description={
            contactModal.mode === "create"
              ? "Kontak baru langsung mendapat akun login. Akun aktif begitu client punya project."
              : contactModal.mode === "replace"
                ? "Perbarui data wedding representative. Kredensial login yang sudah ada tetap dipakai."
                : "Perbarui nama dan informasi kontak."
          }
        />
      )}

      {contactModal?.mode === "reset" && contactModal.contact && (
        <ResetCredentialModal
          name={contactModal.contact.name}
          onClose={() => setContactModal(null)}
          onSubmit={(password) => void handleResetSubmit(password)}
        />
      )}

      <ConfirmDialog
        open={confirmDeleteSignature}
        onClose={() => setConfirmDeleteSignature(false)}
        onConfirm={() => void handleDeleteSignature()}
        title="Hapus TTD Tersimpan"
        message={
          <>
            Yakin ingin menghapus TTD milik <strong>{clientSignature?.signerName}</strong>?
          </>
        }
        details="TTD ini tidak bisa dipakai ulang lagi. PO yang sudah ditandatangani TIDAK terpengaruh — setiap dokumen memegang salinannya sendiri."
        confirmLabel="Ya, Hapus"
        busyLabel="Menghapus..."
        busy={busy}
      />

      <ConfirmDialog
        open={confirmDelete}
        onClose={() => {
          setConfirmDelete(false);
          setImpact(null);
        }}
        onConfirm={() => void handleDelete()}
        title="Hapus Client Permanen"
        message={
          <>
            Yakin ingin menghapus <strong>{currentClient.displayName}</strong> secara permanen?
          </>
        }
        details={
          <>
            <p>
              Client ini beserta <strong>{impact?.contactCount ?? 0} kontak/akun portal</strong> ikut terhapus.
            </p>
            {(impact?.projectCount ?? 0) > 0 && (
              <p className="mt-1.5">
                <strong>{impact?.projectCount} project</strong> ikut terhapus: {(impact?.projectNames ?? []).join(", ")}. Tagihan
                terbayar: <strong>{impact?.paidInvoiceCount}</strong> ({formatCurrency(impact?.paidInvoiceTotal ?? 0)}).
              </p>
            )}
            {(impact?.quotationCount ?? 0) > 0 && (
              <p className="mt-1.5">
                <strong>{impact?.quotationCount} penawaran</strong> yang belum jadi project ikut terhapus:{" "}
                {(impact?.quotationNumbers ?? []).join(", ")}.
              </p>
            )}
            <p className="mt-1.5">Tindakan ini permanen dan tidak dapat dipulihkan.</p>
          </>
        }
        confirmLabel="Ya, Hapus Permanen"
        busyLabel="Menghapus..."
        busy={busy}
      />
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[11.5px] font-medium uppercase tracking-wide text-text-secondary">{label}</p>
      <p className="mt-0.5 text-[13.5px] font-medium text-text-primary">{value}</p>
    </div>
  );
}

function ContactRow({
  contact: c,
  onEdit,
  onReplace,
  onReset,
  onToggleActive,
  onDelete,
}: {
  contact: ClientContact;
  onEdit: () => void;
  onReplace: () => void;
  onReset: () => void;
  onToggleActive: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border-light px-4 py-3">
      <div className="flex min-w-0 items-center gap-3">
        <Avatar name={c.name} />
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold text-text-primary">{c.name}</span>
            <ClientRoleBadge role={c.role} />
          </div>
          <p className="truncate text-[12.5px] text-text-secondary">{c.phone} · {c.email}</p>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <div className="text-right">
          {c.isActive ? <Badge tone="success">Aktif</Badge> : <Badge tone="neutral">Nonaktif</Badge>}
          <p className="mt-1 text-[11.5px] text-text-secondary">
            {c.lastCredentialResetAt ? `Reset: ${formatDate(c.lastCredentialResetAt)}` : "Belum pernah direset"}
          </p>
        </div>
        <div className="flex items-center gap-1.5">
          <IconActionButton icon={Pencil} label="Ubah Kontak" tone="neutral" onClick={onEdit} />
          <IconActionButton icon={KeyRound} label="Reset Credential" tone="info" onClick={onReset} />
          {c.isActive ? (
            <IconActionButton icon={UserX} label="Nonaktifkan" tone="danger" onClick={onToggleActive} />
          ) : (
            <IconActionButton icon={UserCheck} label="Aktifkan" tone="success" onClick={onToggleActive} />
          )}
          {c.role === "Family Representative" && (
            <IconActionButton icon={Repeat} label="Ganti Representative" tone="navy" onClick={onReplace} />
          )}
          <IconActionButton icon={Trash2} label="Hapus Kontak" tone="danger" onClick={onDelete} />
        </div>
      </div>
    </div>
  );
}

function ResetCredentialModal({ name, onClose, onSubmit }: { name: string; onClose: () => void; onSubmit: (password: string) => void }) {
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);

  function handleSubmit() {
    if (password.length < 6) {
      setError("Password minimal 6 karakter");
      return;
    }
    onSubmit(password);
  }

  return (
    <Modal open onClose={onClose} title="Reset Credential" description={`Atur password login baru untuk ${name}.`}>
      <Field label="Password Baru" required hint={error ?? undefined}>
        <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Minimal 6 karakter" />
      </Field>
      <div className="mt-3 flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose}>Batal</Button>
        <Button onClick={handleSubmit}>Reset Password</Button>
      </div>
    </Modal>
  );
}

export function ClientRoleSelect({ value, onChange }: { value: ClientRole; onChange: (r: ClientRole) => void }) {
  return (
    <Select value={value} onChange={(e) => onChange(e.target.value as ClientRole)}>
      {CLIENT_ROLE_OPTIONS.map((r) => (
        <option key={r} value={r}>{r}</option>
      ))}
    </Select>
  );
}
