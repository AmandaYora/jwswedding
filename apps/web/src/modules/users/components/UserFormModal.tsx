import { useEffect, useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { Modal } from "@/shared/components/ui/Modal";
import { Button } from "@/shared/components/ui/Button";
import { Input, Select, Field } from "@/shared/components/ui/Input";
import {
  userSchema,
  userCreateSchema,
  STAFF_ROLE_OPTIONS,
  type UserFormValues,
  type UserCreateFormValues,
} from "@/modules/users/schemas/user.schema";
import { ROLE_LABELS, type StaffMember } from "@/modules/users/types";
import { SignatureField, type SignaturePayload } from "@/modules/users/components/SignatureField";
import { useStaffStore } from "@/modules/users/stores/useStaffStore";
import { useTenantBrandingStore } from "@/shared/stores/useTenantBrandingStore";
import { APP_NAME } from "@/shared/constants/brand";

interface FormState extends UserFormValues {
  username: string;
  password: string;
  confirmPassword: string;
}

function toFormState(user?: StaffMember): FormState {
  if (!user) {
    return { name: "", title: "", role: "Staff", email: "", phone: "", username: "", password: "", confirmPassword: "" };
  }
  return {
    name: user.name,
    title: user.title,
    role: user.role,
    email: user.email,
    phone: user.phone,
    username: user.username,
    password: "",
    confirmPassword: "",
  };
}

interface UserFormModalProps {
  open: boolean;
  onClose: () => void;
  initialUser?: StaffMember;
  // signature = TTD baru yang digambar/diunggah di modal ini, null bila tidak
  // diisi. Dikirim TERPISAH dari values karena ia aset biner, bukan field form
  // ber-Zod — dan pada alur Tambah baru bisa dikirim setelah baris pengguna
  // punya id (PLAN tanda-tangan-pengguna A6).
  onSubmitCreate: (values: UserCreateFormValues, signature: SignaturePayload | null) => void;
  onSubmitEdit: (values: UserFormValues, signature: SignaturePayload | null) => void;
  // Menghapus TTD berlaku SEKETIKA (bukan menunggu Simpan), jadi induk perlu
  // tahu supaya badge "Ada/Belum" di daftar tidak tertinggal basi ketika
  // modalnya ditutup lewat Batal.
  onSignatureDeleted?: () => void;
}

export function UserFormModal({ open, onClose, initialUser, onSubmitCreate, onSubmitEdit, onSignatureDeleted }: UserFormModalProps) {
  const businessName = useTenantBrandingStore((s) => s.businessName) ?? APP_NAME;
  const isEditing = Boolean(initialUser);
  const [values, setValues] = useState<FormState>(() => toFormState(initialUser));
  const [errors, setErrors] = useState<Partial<Record<keyof FormState, string>>>({});
  const [showPassword, setShowPassword] = useState(false);
  const [signature, setSignature] = useState<SignaturePayload | null>(null);
  const [existingSignatureUrl, setExistingSignatureUrl] = useState<string | null>(null);
  const fetchStaffSignatureImageUrl = useStaffStore((s) => s.fetchStaffSignatureImageUrl);
  const deleteStaffSignature = useStaffStore((s) => s.deleteStaffSignature);

  // Pratinjau TTD tersimpan hanya diambil saat memang ada (hasSignature) dan
  // modalnya terbuka — daftar Pengguna sendiri tidak pernah menarik gambar.
  // Object URL-nya di-revoke saat modal ditutup supaya tidak menumpuk blob.
  useEffect(() => {
    if (!open || !initialUser?.hasSignature) {
      setExistingSignatureUrl(null);
      return;
    }
    let revoked = false;
    let url: string | null = null;
    void fetchStaffSignatureImageUrl(initialUser.id).then((objectUrl) => {
      if (revoked) {
        if (objectUrl) URL.revokeObjectURL(objectUrl);
        return;
      }
      url = objectUrl;
      setExistingSignatureUrl(objectUrl);
    });
    return () => {
      revoked = true;
      if (url) URL.revokeObjectURL(url);
    };
  }, [open, initialUser?.id, initialUser?.hasSignature, fetchStaffSignatureImageUrl]);

  async function handleDeleteExistingSignature() {
    if (!initialUser) return;
    await deleteStaffSignature(initialUser.id);
    setExistingSignatureUrl((prev) => {
      if (prev) URL.revokeObjectURL(prev);
      return null;
    });
    onSignatureDeleted?.();
  }

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  function handleSubmit() {
    if (isEditing) {
      const result = userSchema.safeParse(values);
      if (!result.success) {
        const fieldErrors: Partial<Record<keyof FormState, string>> = {};
        for (const issue of result.error.issues) {
          fieldErrors[issue.path[0] as keyof FormState] = issue.message;
        }
        setErrors(fieldErrors);
        return;
      }
      onSubmitEdit(result.data, signature);
      setErrors({});
      return;
    }

    const result = userCreateSchema.safeParse(values);
    if (!result.success) {
      const fieldErrors: Partial<Record<keyof FormState, string>> = {};
      for (const issue of result.error.issues) {
        fieldErrors[issue.path[0] as keyof FormState] = issue.message;
      }
      setErrors(fieldErrors);
      return;
    }
    onSubmitCreate(result.data, signature);
    setErrors({});
  }

  function handleClose() {
    setValues(toFormState(initialUser));
    setErrors({});
    setSignature(null);
    onClose();
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title={isEditing ? "Ubah Pengguna" : "Tambah Pengguna Baru"}
      description={`Kelola akun staff internal yang dapat mengakses ${businessName}.`}
      footer={
        <>
          <Button variant="secondary" onClick={handleClose}>Batal</Button>
          <Button onClick={handleSubmit}>{isEditing ? "Simpan Perubahan" : "Simpan Pengguna"}</Button>
        </>
      }
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field label="Nama Lengkap" required hint={errors.name}>
          <Input value={values.name} onChange={(e) => set("name", e.target.value)} placeholder="cth. Anisa Putri" />
        </Field>
        <Field label="Role" required>
          {initialUser?.role === "Owner" ? (
            <Input value="Owner" disabled />
          ) : (
            <Select value={values.role} onChange={(e) => set("role", e.target.value as UserFormValues["role"])}>
              {STAFF_ROLE_OPTIONS.map((r) => (
                <option key={r} value={r}>{ROLE_LABELS[r]}</option>
              ))}
            </Select>
          )}
        </Field>
        <Field label="Jabatan" required hint={errors.title}>
          <Input value={values.title} onChange={(e) => set("title", e.target.value)} placeholder="cth. Lead Planner" />
        </Field>
        <Field label="Nomor Telepon" required hint={errors.phone}>
          <Input value={values.phone} onChange={(e) => set("phone", e.target.value)} placeholder="0812-xxxx-xxxx" />
        </Field>
        <div className="sm:col-span-2">
          <Field label="Email" required hint={errors.email}>
            <Input type="email" value={values.email} onChange={(e) => set("email", e.target.value)} placeholder="nama@jwswedding.id" />
          </Field>
        </div>
        <div className="sm:col-span-2">
          <Field label="Username" required={!isEditing} hint={errors.username}>
            <Input
              value={values.username}
              disabled={isEditing}
              onChange={(e) => set("username", e.target.value)}
              placeholder="cth. anisa.putri"
            />
          </Field>
        </div>

        {!isEditing && (
          <>
            <Field label="Password" required hint={errors.password}>
              <div className="relative">
                <Input
                  type={showPassword ? "text" : "password"}
                  value={values.password}
                  onChange={(e) => set("password", e.target.value)}
                  placeholder="Minimal 8 karakter"
                  className="pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((v) => !v)}
                  aria-label={showPassword ? "Sembunyikan password" : "Tampilkan password"}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-text-secondary hover:text-text-primary"
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </Field>
            <Field label="Konfirmasi Password" required hint={errors.confirmPassword}>
              <Input
                type={showPassword ? "text" : "password"}
                value={values.confirmPassword}
                onChange={(e) => set("confirmPassword", e.target.value)}
                placeholder="Ulangi password"
              />
            </Field>
          </>
        )}
        <div className="sm:col-span-2">
          <SignatureField
            existingImageUrl={existingSignatureUrl}
            onChange={setSignature}
            onClear={initialUser ? () => void handleDeleteExistingSignature() : undefined}
          />
        </div>
      </div>
    </Modal>
  );
}
