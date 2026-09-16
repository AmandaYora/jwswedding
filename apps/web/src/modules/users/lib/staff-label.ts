import type { StaffRole, StaffSummary } from "@/modules/users/types";

// staffOptionLabel merender satu opsi dropdown pemilih staff sebagai
// "Nama user (Jabatan)" (PLAN wording-role-dan-filter-sales-wp, D2/D5).
// `title` adalah field yang di menu Pengguna berlabel "Jabatan" (teks bebas).
// Jatuh kembali ke nama saja bila `title` kosong — staff yang lahir sebelum
// "Jabatan" diwajibkan tidak boleh tampil sebagai "Budi ()".
export function staffOptionLabel(s: { name: string; title: string }): string {
  const title = s.title.trim();
  if (!title) return s.name;
  return `${s.name} (${title})`;
}

// staffOptionsForRole menyaring daftar staff ke satu role untuk dropdown
// penugasan maupun filter daftar (PLAN wording-role-dan-filter-sales-wp, D6).
// `keepId` memaksa satu staff tetap muncul walau role-nya tidak cocok —
// dipakai form penugasan agar nilai tersimpan tidak hilang dari dropdown lalu
// ter-reassign diam-diam saat form disimpan tanpa menyentuh field tersebut.
// Filter daftar TIDAK memakai keepId: filter hanya membaca, tidak ada nilai
// tersimpan yang bisa rusak.
export function staffOptionsForRole(
  list: StaffSummary[],
  role: StaffRole,
  keepId?: string,
): StaffSummary[] {
  return list.filter((s) => s.role === role || (keepId != null && keepId !== "" && s.id === keepId));
}
