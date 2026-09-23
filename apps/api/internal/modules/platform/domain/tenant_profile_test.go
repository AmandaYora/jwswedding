package domain

import (
	"reflect"
	"testing"
)

func completeTenant() Tenant {
	return Tenant{
		BusinessName: "JWS Wedding", OwnerName: "Dimas Prasetio", Phone: "0812-0000-0000",
		Address: "Jl. Melati No. 12", City: "Bandung",
		BankName: "BCA", BankAccountNumber: "1234567890", BankAccountHolderName: "Dimas Prasetio",
	}
}

// TestProfileMissingFields_TenantKosong locks the fixed field order from
// PLAN.md redesain-pdf-invoice-kwitansi-v2 §6.1 — the dialog always lists
// missing fields in this same order regardless of which ones are missing.
func TestProfileMissingFields_TenantKosong(t *testing.T) {
	got := ProfileMissingFields(Tenant{})
	want := []string{
		"Nama Usaha", "Nama Pemilik", "Telepon", "Alamat", "Kota",
		"Nama Bank", "No. Rekening", "Nama Pemilik Rekening",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ProfileMissingFields(kosong) = %v, want %v", got, want)
	}
}

func TestProfileMissingFields_TenantLengkap(t *testing.T) {
	got := ProfileMissingFields(completeTenant())
	if len(got) != 0 {
		t.Errorf("ProfileMissingFields(lengkap) = %v, want empty", got)
	}
	if got == nil {
		t.Error("ProfileMissingFields(lengkap) harus mengembalikan slice kosong, bukan nil")
	}
}

func TestProfileMissingFields_SpasiDianggapKosong(t *testing.T) {
	tenant := completeTenant()
	tenant.Address = "   "
	got := ProfileMissingFields(tenant)
	want := []string{"Alamat"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ProfileMissingFields(address spasi) = %v, want %v", got, want)
	}
}

func TestProfileMissingFields_SatuFieldKurang(t *testing.T) {
	tenant := completeTenant()
	tenant.BankAccountNumber = ""
	got := ProfileMissingFields(tenant)
	want := []string{"No. Rekening"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ProfileMissingFields(no rekening kosong) = %v, want %v", got, want)
	}
}

// TestProfileMissingFields_EmailDanLogoTidakDinilai locks K1 — Email dan logo
// tidak ikut menentukan kelengkapan profil. Tanda tangan sudah tidak ada di
// tingkat tenant sama sekali sejak PLAN tanda-tangan-pengguna (K3), jadi tidak
// ada lagi yang bisa diuji di sini soal itu.
func TestProfileMissingFields_EmailDanLogoTidakDinilai(t *testing.T) {
	tenant := completeTenant()
	tenant.Email = ""
	tenant.LogoStoragePath = nil
	got := ProfileMissingFields(tenant)
	if len(got) != 0 {
		t.Errorf("ProfileMissingFields tidak boleh menilai Email/logo, got %v", got)
	}
}
