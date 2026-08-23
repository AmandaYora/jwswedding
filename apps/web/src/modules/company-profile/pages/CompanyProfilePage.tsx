import { useEffect, useState } from "react";
import { Building2, Signature } from "lucide-react";
import { Card, CardHeader, CardContent } from "@/shared/components/ui/Card";
import { Button } from "@/shared/components/ui/Button";
import { Input, Textarea, Field } from "@/shared/components/ui/Input";
import { useCompanyProfileStore } from "@/modules/company-profile/stores/useCompanyProfileStore";
import { companyProfileSchema, type CompanyProfileFormValues } from "@/modules/company-profile/schemas/company-profile.schema";
import { getApiErrorMessage } from "@/shared/lib/api-error";

const EMPTY_VALUES: CompanyProfileFormValues = {
  businessName: "", ownerName: "", email: "", phone: "", city: "",
  address: "", bankName: "", bankAccountNumber: "", bankAccountHolderName: "",
};

// The one Owner-only "Profil Usaha" page (PLAN.md invoice-kwitansi-client
// §1.7/§4.8) — closes the gap where the Owner previously had no way at all
// to edit their own tenant's profile (Platform Console's admin CRUD is gone
// entirely). Single-column form, pola visual SubscriptionPage.tsx.
export default function CompanyProfilePage() {
  const profile = useCompanyProfileStore((s) => s.profile);
  const logoUrl = useCompanyProfileStore((s) => s.logoUrl);
  const signatureUrl = useCompanyProfileStore((s) => s.signatureUrl);
  const fetchProfile = useCompanyProfileStore((s) => s.fetchProfile);
  const updateProfile = useCompanyProfileStore((s) => s.updateProfile);
  const uploadLogo = useCompanyProfileStore((s) => s.uploadLogo);
  const uploadSignature = useCompanyProfileStore((s) => s.uploadSignature);

  const [values, setValues] = useState<CompanyProfileFormValues>(EMPTY_VALUES);
  const [errors, setErrors] = useState<Partial<Record<keyof CompanyProfileFormValues, string>>>({});
  const [submitting, setSubmitting] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [logoError, setLogoError] = useState<string | null>(null);
  const [uploadingLogo, setUploadingLogo] = useState(false);
  const [signatureError, setSignatureError] = useState<string | null>(null);
  const [uploadingSignature, setUploadingSignature] = useState(false);

  useEffect(() => {
    void fetchProfile();
  }, [fetchProfile]);

  // Re-prefill whenever the store's profile changes (initial load, and
  // after a successful save) -- never while the Owner is mid-edit, since
  // this effect only fires on `profile` identity changes, not on `values`.
  useEffect(() => {
    if (profile) {
      setValues({
        businessName: profile.businessName, ownerName: profile.ownerName, email: profile.email,
        phone: profile.phone, city: profile.city, address: profile.address, bankName: profile.bankName,
        bankAccountNumber: profile.bankAccountNumber, bankAccountHolderName: profile.bankAccountHolderName,
      });
    }
  }, [profile]);

  function set<K extends keyof CompanyProfileFormValues>(key: K, value: CompanyProfileFormValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSubmit() {
    const result = companyProfileSchema.safeParse(values);
    if (!result.success) {
      const fieldErrors: Partial<Record<keyof CompanyProfileFormValues, string>> = {};
      for (const issue of result.error.issues) {
        fieldErrors[issue.path[0] as keyof CompanyProfileFormValues] = issue.message;
      }
      setErrors(fieldErrors);
      return;
    }
    setSubmitting(true);
    setSaveError(null);
    setSaveSuccess(false);
    try {
      await updateProfile(result.data);
      setErrors({});
      setSaveSuccess(true);
    } catch (err) {
      setSaveError(getApiErrorMessage(err, "Gagal menyimpan profil usaha"));
    } finally {
      setSubmitting(false);
    }
  }

  async function handleLogoChange(file: File | undefined) {
    if (!file) return;
    setLogoError(null);
    setUploadingLogo(true);
    try {
      await uploadLogo(file);
    } catch (err) {
      setLogoError(getApiErrorMessage(err, "Gagal mengunggah logo"));
    } finally {
      setUploadingLogo(false);
    }
  }

  // Mirrors handleLogoChange exactly (PLAN.md redesain-pdf-invoice-kwitansi
  // §D4/§D7).
  async function handleSignatureChange(file: File | undefined) {
    if (!file) return;
    setSignatureError(null);
    setUploadingSignature(true);
    try {
      await uploadSignature(file);
    } catch (err) {
      setSignatureError(getApiErrorMessage(err, "Gagal mengunggah tanda tangan"));
    } finally {
      setUploadingSignature(false);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex items-center gap-2">
        <Building2 className="h-5 w-5 text-navy-900" />
        <div>
          <h1 className="text-xl font-bold text-text-primary">Profil Usaha</h1>
          <p className="mt-1 text-[13px] text-text-secondary">
            Data ini tampil pada kop surat Invoice dan Kwitansi yang diterbitkan ke client.
          </p>
        </div>
      </div>

      <Card className="max-w-2xl">
        <CardHeader title="Logo Usaha" subtitle="Logo ini muncul persis seperti ini pada kop surat PDF." />
        <CardContent>
          {logoError && (
            <p className="mb-3 rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{logoError}</p>
          )}
          <div className="flex items-center gap-4">
            <div className="flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-md border border-border bg-surface-muted">
              {logoUrl ? (
                <img src={logoUrl} alt="Logo usaha" className="h-full w-full object-contain" />
              ) : (
                <Building2 className="h-6 w-6 text-text-secondary" />
              )}
            </div>
            <div className="flex flex-col gap-1.5">
              <input
                type="file"
                accept="image/jpeg,image/png,image/webp"
                disabled={uploadingLogo}
                onChange={(e) => void handleLogoChange(e.target.files?.[0])}
                className="block text-[13px] text-text-secondary file:mr-3 file:rounded-md file:border-0 file:bg-navy-900 file:px-3 file:py-1.5 file:text-[12.5px] file:font-semibold file:text-white disabled:opacity-60"
              />
              <span className="text-xs text-text-secondary">JPEG, PNG, atau WEBP. Tersimpan otomatis begitu file dipilih.</span>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="max-w-2xl">
        <CardHeader title="Tanda Tangan" subtitle="Tanda tangan ini muncul otomatis pada Kwitansi yang diterbitkan." />
        <CardContent>
          {signatureError && (
            <p className="mb-3 rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{signatureError}</p>
          )}
          <div className="flex items-center gap-4">
            <div className="flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-md border border-border bg-surface-muted">
              {signatureUrl ? (
                <img src={signatureUrl} alt="Tanda tangan" className="h-full w-full object-contain" />
              ) : (
                <Signature className="h-6 w-6 text-text-secondary" />
              )}
            </div>
            <div className="flex flex-col gap-1.5">
              <input
                type="file"
                accept="image/png"
                disabled={uploadingSignature}
                onChange={(e) => void handleSignatureChange(e.target.files?.[0])}
                className="block text-[13px] text-text-secondary file:mr-3 file:rounded-md file:border-0 file:bg-navy-900 file:px-3 file:py-1.5 file:text-[12.5px] file:font-semibold file:text-white disabled:opacity-60"
              />
              <span className="text-xs text-text-secondary">
                PNG saja, sebaiknya berlatar transparan. Tersimpan otomatis begitu file dipilih.
              </span>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="max-w-2xl">
        <CardHeader title="Data Usaha" />
        <CardContent className="flex flex-col gap-4">
          {saveError && (
            <p className="rounded-md border border-danger/30 bg-danger-soft px-3.5 py-2.5 text-[13px] font-medium text-danger">{saveError}</p>
          )}
          {saveSuccess && (
            <p className="rounded-md border border-success/30 bg-success-soft px-3.5 py-2.5 text-[13px] font-medium text-success">
              Profil usaha berhasil disimpan.
            </p>
          )}
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Nama Usaha" required hint={errors.businessName}>
              <Input value={values.businessName} onChange={(e) => set("businessName", e.target.value)} />
            </Field>
            <Field label="Nama Pemilik" required hint={errors.ownerName}>
              <Input value={values.ownerName} onChange={(e) => set("ownerName", e.target.value)} />
            </Field>
            <Field label="Email">
              <Input type="email" value={values.email} onChange={(e) => set("email", e.target.value)} />
            </Field>
            <Field label="Telepon">
              <Input value={values.phone} onChange={(e) => set("phone", e.target.value)} />
            </Field>
            <Field label="Kota">
              <Input value={values.city} onChange={(e) => set("city", e.target.value)} />
            </Field>
            <div className="sm:col-span-2">
              <Field label="Alamat">
                <Textarea rows={2} value={values.address} onChange={(e) => set("address", e.target.value)} />
              </Field>
            </div>
          </div>

          <div className="border-t border-border-light pt-4">
            <p className="mb-3 text-[13px] font-semibold text-text-primary">Rekening Tujuan Pembayaran</p>
            <p className="mb-3 text-[12.5px] text-text-secondary">Tampil pada Invoice sebagai rekening tujuan transfer client.</p>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <Field label="Nama Bank">
                <Input value={values.bankName} onChange={(e) => set("bankName", e.target.value)} />
              </Field>
              <Field label="No. Rekening">
                <Input value={values.bankAccountNumber} onChange={(e) => set("bankAccountNumber", e.target.value)} />
              </Field>
              <Field label="Atas Nama">
                <Input value={values.bankAccountHolderName} onChange={(e) => set("bankAccountHolderName", e.target.value)} />
              </Field>
            </div>
          </div>

          <div className="flex justify-end pt-2">
            <Button onClick={() => void handleSubmit()} disabled={submitting}>
              {submitting ? "Menyimpan..." : "Simpan Profil Usaha"}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
