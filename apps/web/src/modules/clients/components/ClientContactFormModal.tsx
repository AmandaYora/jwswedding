import { useState } from "react";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";
import { Input, Field } from "@/shared/components/ui/Input";
import {
  clientContactSchema,
  clientCreateSchema,
  representativeSchema,
  type ClientContactFormValues,
  type ClientCreateFormValues,
  type RepresentativeFormValues,
} from "@/modules/clients/schemas/client.schema";
import type { ClientRole } from "@/modules/clients/types";

interface ClientContactFormModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (values: ClientContactFormValues | ClientCreateFormValues | RepresentativeFormValues) => void;
  initialValues: { name: string; phone: string; email: string };
  username?: string;
  title?: string;
  description?: string;
  /** "create" menampilkan username/password (+ relasi opsional); "replace"
   * menampilkan relasi wajib; default "edit" hanya kontak. */
  mode?: "edit" | "create" | "replace";
  role?: ClientRole;
}

export function ClientContactFormModal({
  open,
  onClose,
  onSubmit,
  initialValues,
  username,
  title = "Ubah Kontak Client",
  description = "Perbarui nama dan informasi kontak client.",
  mode = "edit",
  role,
}: ClientContactFormModalProps) {
  const [values, setValues] = useState({
    ...initialValues,
    relationNote: "",
    username: "",
    password: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});

  function set(key: string, value: string) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  function handleSubmit() {
    if (mode === "create") {
      const result = clientCreateSchema.safeParse(values);
      if (!result.success) return collectErrors(result.error.issues);
      onSubmit(result.data);
    } else if (mode === "replace") {
      const result = representativeSchema.safeParse(values);
      if (!result.success) return collectErrors(result.error.issues);
      onSubmit(result.data);
    } else {
      const result = clientContactSchema.safeParse(values);
      if (!result.success) return collectErrors(result.error.issues);
      onSubmit(result.data);
    }
    setErrors({});
  }

  function collectErrors(issues: { path: (string | number)[]; message: string }[]) {
    const fieldErrors: Record<string, string> = {};
    for (const issue of issues) {
      const key = String(issue.path[0] ?? "");
      if (key) fieldErrors[key] = issue.message;
    }
    setErrors(fieldErrors);
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      description={description}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Batal</Button>
          <Button onClick={handleSubmit}>Simpan Perubahan</Button>
        </>
      }
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {role && (
          <div className="sm:col-span-2">
            <Field label="Peran">
              <Input value={role} disabled />
            </Field>
          </div>
        )}
        <div className="sm:col-span-2">
          <Field label="Nama" required hint={errors.name}>
            <Input value={values.name} onChange={(e) => set("name", e.target.value)} placeholder="Nama lengkap" />
          </Field>
        </div>
        <Field label="Nomor Telepon" required hint={errors.phone}>
          <Input value={values.phone} onChange={(e) => set("phone", e.target.value)} placeholder="08xx-xxxx-xxxx" />
        </Field>
        <Field label="Email" required hint={errors.email}>
          <Input type="email" value={values.email} onChange={(e) => set("email", e.target.value)} placeholder="nama@email.com" />
        </Field>
        {(mode === "create" || mode === "replace") && (
          <div className="sm:col-span-2">
            <Field
              label={mode === "create" ? "Keterangan Hubungan (opsional)" : "Keterangan Hubungan"}
              required={mode === "replace"}
              hint={errors.relationNote}
            >
              <Input
                value={values.relationNote}
                onChange={(e) => set("relationNote", e.target.value)}
                placeholder="cth. Sepupu mempelai wanita"
              />
            </Field>
          </div>
        )}
        {mode === "create" && (
          <>
            <Field label="Username Login" required hint={errors.username}>
              <Input value={values.username} onChange={(e) => set("username", e.target.value)} placeholder="username unik" />
            </Field>
            <Field label="Password" required hint={errors.password}>
              <Input type="password" value={values.password} onChange={(e) => set("password", e.target.value)} placeholder="Minimal 6 karakter" />
            </Field>
          </>
        )}
        {username !== undefined && mode !== "create" && (
          <div className="sm:col-span-2">
            <Field label="Username">
              <Input value={username} disabled />
            </Field>
          </div>
        )}
      </div>
    </Modal>
  );
}
