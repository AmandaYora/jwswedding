import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Pencil, RefreshCw, UserCog, Plus, Trash2 } from "lucide-react";
import { Card, CardHeader, CardContent } from "@/shared/components/ui/Card";
import { Badge } from "@/shared/components/ui/Badge";
import { Button } from "@/shared/components/ui/Button";
import { ConfirmDialog } from "@/shared/components/ui/ConfirmDialog";
import { Modal } from "@/shared/components/ui/Modal";
import { Input, Field } from "@/shared/components/ui/Input";
import { useClientStore } from "@/modules/clients/stores/useClientStore";
import { useProjectStore } from "@/modules/projects/stores/useProjectStore";
import {
  clientContactSchema,
  clientCreateSchema,
  representativeSchema,
  type ClientContactFormValues,
  type ClientCreateFormValues,
  type RepresentativeFormValues,
} from "@/modules/clients/schemas/client.schema";
import { suggestUsername, suggestPassword } from "@/modules/clients/lib/credential-suggestion";
import type { ClientContact, ClientRole } from "@/modules/clients/types";
import { ClientRoleBadge } from "@/modules/clients/components/ClientRoleBadge";
import { getApiErrorMessage } from "@/shared/lib/api-error";
import { formatDate } from "@/shared/lib/formatters";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

const ROLE_ORDER: ClientRole[] = ["Bride", "Groom", "Family Representative"];

const ROLE_LABEL: Record<ClientRole, string> = {
  Bride: "Pengantin Wanita",
  Groom: "Pengantin Pria",
  "Family Representative": "Wedding Representative Keluarga",
};

type ModalMode = "contact" | "replace" | "reset";

interface ModalTarget {
  contactId: string;
  mode: ModalMode;
}

// Tab Client di detail project: master pasangan pemilik project + kontak/akun
// portalnya. Kontak dikelola di sini maupun di menu Client (store yang sama);
// hapus CLIENT-nya sendiri hanya dari menu Client (berjenjang, D14).
export function ProjectClientsSection() {
  const project = useProjectStore((s) => s.currentProject);
  const currentClient = useClientStore((s) => s.currentClient);
  const contacts = useClientStore((s) => s.contacts);
  const fetchClient = useClientStore((s) => s.fetchClient);
  const createContact = useClientStore((s) => s.createContact);
  const updateContact = useClientStore((s) => s.updateContact);
  const toggleContactActive = useClientStore((s) => s.toggleContactActive);
  const deleteContact = useClientStore((s) => s.deleteContact);
  const resetContactCredential = useClientStore((s) => s.resetContactCredential);
  const replaceRepresentative = useClientStore((s) => s.replaceRepresentative);

  const [modalTarget, setModalTarget] = useState<ModalTarget | null>(null);
  const [createRole, setCreateRole] = useState<ClientRole | null>(null);
  const [deletingContactId, setDeletingContactId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const clientId = project?.clientId && project.clientId !== "0" ? project.clientId : "";

  useEffect(() => {
    if (clientId) void fetchClient(clientId);
  }, [clientId, fetchClient]);

  const activeTarget = modalTarget ? contacts.find((c) => c.id === modalTarget.contactId) ?? null : null;

  async function handleToggleActive(id: string) {
    if (!clientId) return;
    setActionError(null);
    try {
      await toggleContactActive(clientId, id);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal mengubah status kontak"));
    }
  }

  async function handleContactSubmit(values: ClientContactFormValues) {
    if (!modalTarget || !clientId) return;
    setActionError(null);
    try {
      await updateContact(clientId, modalTarget.contactId, values);
      setModalTarget(null);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal memperbarui kontak"));
    }
  }

  async function handleDelete(id: string) {
    if (!clientId) return;
    setActionError(null);
    try {
      await deleteContact(clientId, id);
      setDeletingContactId(null);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal menghapus kontak"));
    }
  }

  async function handleReplaceSubmit(values: RepresentativeFormValues) {
    if (!modalTarget || !clientId) return;
    setActionError(null);
    try {
      await replaceRepresentative(clientId, modalTarget.contactId, values);
      setModalTarget(null);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal mengganti wedding representative"));
    }
  }

  async function handleResetSubmit(password: string) {
    if (!modalTarget || !clientId) return;
    setActionError(null);
    try {
      await resetContactCredential(clientId, modalTarget.contactId, password);
      setModalTarget(null);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal mereset kredensial"));
    }
  }

  async function handleCreateSubmit(values: ClientCreateFormValues) {
    if (!createRole || !clientId) return;
    setActionError(null);
    try {
      await createContact(clientId, createRole, values);
      setCreateRole(null);
    } catch (err) {
      setActionError(getApiErrorMessage(err, "Gagal menambahkan kontak"));
    }
  }

  function defaultNameFor(role: ClientRole): string {
    if (role === "Bride") return project?.brideName ?? "";
    if (role === "Groom") return project?.groomName ?? "";
    return "";
  }

  if (!clientId) {
    return (
      <Card>
        <CardContent className="py-10 text-center text-[13px] text-text-secondary">
          Project lama ini belum tertaut ke master Client.
        </CardContent>
      </Card>
    );
  }

  return (
    <div id="client">
      <Card>
        <CardHeader
          title="Client Project"
          subtitle={
            currentClient ? (
              <>
                {currentClient.displayName} ·{" "}
                <Link to={ROUTE_PATHS.clientDetail(currentClient.id)} className="font-semibold text-navy-700 hover:underline">
                  Kelola di menu Client →
                </Link>
              </>
            ) : (
              "Memuat data pasangan..."
            )
          }
        />
        <CardContent className="flex flex-col gap-3">
          {actionError && (
            <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{actionError}</p>
          )}
          {ROLE_ORDER.map((role) => {
            const contact = contacts.find((c) => c.role === role);
            if (!contact) {
              return (
                <div
                  key={role}
                  className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-dashed border-border px-4 py-3 text-[13px] text-text-secondary"
                >
                  <span>Belum ada data {ROLE_LABEL[role]}.</span>
                  <Button size="sm" variant="secondary" icon={<Plus className="h-3.5 w-3.5" />} onClick={() => setCreateRole(role)}>
                    Tambah {ROLE_LABEL[role]}
                  </Button>
                </div>
              );
            }
            return (
              <div key={contact.id} className="rounded-md border border-border p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="flex flex-wrap items-center gap-2 text-[11.5px] font-semibold uppercase tracking-wide text-text-secondary">
                      {ROLE_LABEL[role]} <ClientRoleBadge role={contact.role} />
                    </p>
                    {contact.relationNote && <p className="mt-1 text-[13px] text-text-secondary">{contact.relationNote}</p>}
                    <p className="mt-1.5 text-sm font-semibold text-text-primary">{contact.name}</p>
                    <p className="text-[13px] text-text-secondary">
                      {contact.phone} &middot; {contact.email}
                    </p>
                  </div>
                  <Badge tone={contact.isActive ? "success" : "neutral"}>{contact.isActive ? "Aktif" : "Nonaktif"}</Badge>
                </div>

                <div className="mt-3 flex flex-wrap items-center gap-2 text-[12.5px] text-text-secondary">
                  <span>
                    {contact.lastCredentialResetAt
                      ? `Direset terakhir: ${formatDate(contact.lastCredentialResetAt)}`
                      : "Belum pernah direset"}
                  </span>
                </div>

                <div className="mt-3 flex flex-wrap gap-2">
                  <Button
                    size="sm"
                    variant="secondary"
                    icon={<Pencil className="h-3.5 w-3.5" />}
                    onClick={() => setModalTarget({ contactId: contact.id, mode: "contact" })}
                  >
                    Ubah Kontak
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    icon={<RefreshCw className="h-3.5 w-3.5" />}
                    onClick={() => setModalTarget({ contactId: contact.id, mode: "reset" })}
                  >
                    Reset Credential
                  </Button>
                  <Button
                    size="sm"
                    variant={contact.isActive ? "danger" : "secondary"}
                    onClick={() => void handleToggleActive(contact.id)}
                  >
                    {contact.isActive ? "Nonaktifkan" : "Aktifkan"}
                  </Button>
                  {role === "Family Representative" && (
                    <Button
                      size="sm"
                      variant="secondary"
                      icon={<UserCog className="h-3.5 w-3.5" />}
                      onClick={() => setModalTarget({ contactId: contact.id, mode: "replace" })}
                    >
                      Ganti Wedding Representative
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="danger"
                    icon={<Trash2 className="h-3.5 w-3.5" />}
                    onClick={() => setDeletingContactId(contact.id)}
                  >
                    Hapus Kontak
                  </Button>
                </div>

                <ConfirmDialog
                  open={deletingContactId === contact.id}
                  onClose={() => setDeletingContactId(null)}
                  onConfirm={() => void handleDelete(contact.id)}
                  title={`Hapus ${ROLE_LABEL[role]}`}
                  message={
                    <>
                      Yakin ingin menghapus <strong>{contact.name}</strong> sebagai {ROLE_LABEL[role]}?
                    </>
                  }
                  details="Akun login yang terkait ikut dinonaktifkan."
                  confirmLabel="Ya, Hapus"
                />
              </div>
            );
          })}
        </CardContent>
      </Card>

      {activeTarget && modalTarget?.mode === "contact" && (
        <EditContactModal2
          key={activeTarget.id}
          contact={activeTarget}
          onClose={() => setModalTarget(null)}
          onSubmit={(values) => void handleContactSubmit(values)}
        />
      )}

      {activeTarget && modalTarget?.mode === "replace" && (
        <ReplaceRepresentativeModal2
          key={activeTarget.id}
          contact={activeTarget}
          onClose={() => setModalTarget(null)}
          onSubmit={(values) => void handleReplaceSubmit(values)}
        />
      )}

      {activeTarget && modalTarget?.mode === "reset" && (
        <ResetCredentialModal2
          key={activeTarget.id}
          contact={activeTarget}
          onClose={() => setModalTarget(null)}
          onSubmit={(password) => void handleResetSubmit(password)}
        />
      )}

      {createRole && (
        <CreateContactModal2
          role={createRole}
          defaultName={defaultNameFor(createRole)}
          eventDate={project?.eventDate ?? ""}
          onClose={() => setCreateRole(null)}
          onSubmit={(values) => void handleCreateSubmit(values)}
        />
      )}
    </div>
  );
}

function EditContactModal2({
  contact,
  onClose,
  onSubmit,
}: {
  contact: ClientContact;
  onClose: () => void;
  onSubmit: (values: ClientContactFormValues) => void;
}) {
  const [values, setValues] = useState<ClientContactFormValues>({ name: contact.name, phone: contact.phone, email: contact.email });
  const [errors, setErrors] = useState<Partial<Record<keyof ClientContactFormValues, string>>>({});

  function set<K extends keyof ClientContactFormValues>(key: K, value: ClientContactFormValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  function handleSubmit() {
    const result = clientContactSchema.safeParse(values);
    if (!result.success) {
      const fieldErrors: Partial<Record<keyof ClientContactFormValues, string>> = {};
      for (const issue of result.error.issues) {
        fieldErrors[issue.path[0] as keyof ClientContactFormValues] = issue.message;
      }
      setErrors(fieldErrors);
      return;
    }
    onSubmit(result.data);
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Ubah Kontak"
      description={`Perbarui informasi kontak untuk ${contact.name}.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={handleSubmit}>Simpan Perubahan</Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <Field label="Nama" required hint={errors.name}>
          <Input value={values.name} onChange={(e) => set("name", e.target.value)} />
        </Field>
        <Field label="No. Telepon" required hint={errors.phone}>
          <Input value={values.phone} onChange={(e) => set("phone", e.target.value)} />
        </Field>
        <Field label="Email" required hint={errors.email}>
          <Input type="email" value={values.email} onChange={(e) => set("email", e.target.value)} />
        </Field>
        <Field label="Username">
          <Input value={contact.username} disabled />
        </Field>
      </div>
    </Modal>
  );
}

function ReplaceRepresentativeModal2({
  contact,
  onClose,
  onSubmit,
}: {
  contact: ClientContact;
  onClose: () => void;
  onSubmit: (values: RepresentativeFormValues) => void;
}) {
  const [values, setValues] = useState<RepresentativeFormValues>({ name: "", phone: "", email: "", relationNote: "" });
  const [errors, setErrors] = useState<Partial<Record<keyof RepresentativeFormValues, string>>>({});

  function set<K extends keyof RepresentativeFormValues>(key: K, value: RepresentativeFormValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  function handleSubmit() {
    const result = representativeSchema.safeParse(values);
    if (!result.success) {
      const fieldErrors: Partial<Record<keyof RepresentativeFormValues, string>> = {};
      for (const issue of result.error.issues) {
        fieldErrors[issue.path[0] as keyof RepresentativeFormValues] = issue.message;
      }
      setErrors(fieldErrors);
      return;
    }
    onSubmit(result.data);
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Ganti Wedding Representative"
      description={`Data representative keluarga saat ini (${contact.name}) akan digantikan oleh data baru. Kredensial login yang sudah ada tetap dipakai.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={handleSubmit}>Simpan Representative Baru</Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <Field label="Nama Representative Baru" required hint={errors.name}>
          <Input value={values.name} onChange={(e) => set("name", e.target.value)} placeholder="Nama lengkap" />
        </Field>
        <Field label="Hubungan dengan Pengantin" required hint={errors.relationNote}>
          <Input
            value={values.relationNote}
            onChange={(e) => set("relationNote", e.target.value)}
            placeholder="cth. Ayah kandung mempelai wanita"
          />
        </Field>
        <Field label="No. Telepon" required hint={errors.phone}>
          <Input value={values.phone} onChange={(e) => set("phone", e.target.value)} />
        </Field>
        <Field label="Email" required hint={errors.email}>
          <Input type="email" value={values.email} onChange={(e) => set("email", e.target.value)} />
        </Field>
        <Field label="Username">
          <Input value={contact.username} disabled />
        </Field>
      </div>
    </Modal>
  );
}

function ResetCredentialModal2({
  contact,
  onClose,
  onSubmit,
}: {
  contact: ClientContact;
  onClose: () => void;
  onSubmit: (password: string) => void;
}) {
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
    <Modal
      open
      onClose={onClose}
      title="Reset Credential"
      description={`Atur password login baru untuk ${contact.name}.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={handleSubmit}>Reset Password</Button>
        </>
      }
    >
      <Field label="Password Baru" required hint={error ?? undefined}>
        <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Minimal 6 karakter" />
      </Field>
    </Modal>
  );
}

function CreateContactModal2({
  role,
  defaultName,
  eventDate,
  onClose,
  onSubmit,
}: {
  role: ClientRole;
  defaultName: string;
  eventDate: string;
  onClose: () => void;
  onSubmit: (values: ClientCreateFormValues) => void;
}) {
  const [values, setValues] = useState<ClientCreateFormValues>(() => ({
    name: defaultName,
    phone: "",
    email: "",
    relationNote: "",
    username: suggestUsername(defaultName),
    password: suggestPassword(eventDate),
  }));
  const [errors, setErrors] = useState<Partial<Record<keyof ClientCreateFormValues, string>>>({});
  const [usernameTouched, setUsernameTouched] = useState(false);

  function set<K extends keyof ClientCreateFormValues>(key: K, value: ClientCreateFormValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  function setName(value: string) {
    setValues((prev) => ({
      ...prev,
      name: value,
      username: usernameTouched ? prev.username : suggestUsername(value),
    }));
  }

  function setUsername(value: string) {
    setUsernameTouched(true);
    set("username", value);
  }

  function handleSubmit() {
    const result = clientCreateSchema.safeParse(values);
    if (!result.success) {
      const fieldErrors: Partial<Record<keyof ClientCreateFormValues, string>> = {};
      for (const issue of result.error.issues) {
        fieldErrors[issue.path[0] as keyof ClientCreateFormValues] = issue.message;
      }
      setErrors(fieldErrors);
      return;
    }
    onSubmit(result.data);
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={`Tambah ${ROLE_LABEL[role]}`}
      description="Membuat kontak baru sekaligus akun login untuk Client Portal."
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={handleSubmit}>Simpan Kontak</Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <Field label="Nama" required hint={errors.name}>
          <Input value={values.name} onChange={(e) => setName(e.target.value)} placeholder="Nama lengkap" />
        </Field>
        {role === "Family Representative" && (
          <Field label="Hubungan dengan Pengantin" hint={errors.relationNote}>
            <Input
              value={values.relationNote}
              onChange={(e) => set("relationNote", e.target.value)}
              placeholder="cth. Ayah kandung mempelai wanita"
            />
          </Field>
        )}
        <Field label="No. Telepon" required hint={errors.phone}>
          <Input value={values.phone} onChange={(e) => set("phone", e.target.value)} />
        </Field>
        <Field label="Email" required hint={errors.email}>
          <Input type="email" value={values.email} onChange={(e) => set("email", e.target.value)} />
        </Field>
        <Field label="Username" required hint={errors.username ?? "Terisi otomatis dari Nama — bisa diubah bila sudah dipakai"}>
          <Input value={values.username} onChange={(e) => setUsername(e.target.value)} placeholder="cth. jws_budi" />
        </Field>
        <Field label="Password Login" required hint={errors.password ?? "Terisi otomatis dari tanggal acara — sampaikan ke client"}>
          <Input type="text" value={values.password} onChange={(e) => set("password", e.target.value)} placeholder="Minimal 6 karakter" />
        </Field>
      </div>
    </Modal>
  );
}
