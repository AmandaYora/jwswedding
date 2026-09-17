package presentation

import "testing"

// Template venue SEBELUM kolom "Kategori" disisipkan — inilah berkas yang
// sudah terlanjur ada di komputer user, dan yang dulu menyelinap masuk dengan
// data tergeser.
var venueTemplateHeadersLegacy = []string{
	"Nama Venue", "Nama PIC", "No Tlp PIC", "No Tlp Venue", "Email",
	"Alamat", "Kota", "Harga Sewa", "Charge", "Kapasitas", "Fasilitas", "Sosial Media", "Catatan",
}

func TestValidateImportHeaderRow_Venue(t *testing.T) {
	cases := []struct {
		name    string
		cells   []string
		wantErr bool
	}{
		{
			name:  "template terbaru apa adanya",
			cells: append([]string{}, venueTemplateHeaders...),
		},
		{
			// Dibangun lewat templateHeaderLabel + venueRequiredImportHeaders,
			// persis panggilan yang dipakai Template() — bukan disalin manual,
			// supaya uji ini ikut gagal kalau daftar kolom wajib berubah.
			name:  "baris header seperti yang ditulis Template()",
			cells: templateHeaderRowFor(venueTemplateHeaders, venueRequiredImportHeaders),
		},
		{
			name:  "berkas hasil Export: kolom Status ekstra di ujung",
			cells: append(append([]string{}, venueTemplateHeaders...), "Status"),
		},
		{
			name: "beda huruf besar kecil dan spasi tepi",
			cells: []string{
				"  nama venue  ", "NAMA PIC", "No Tlp PIC", "No Tlp Venue", "Email",
				"Alamat", "kota", "KATEGORI", "Harga Sewa", "Charge", "Kapasitas",
				"Fasilitas", "Sosial Media", "Catatan",
			},
		},
		{
			name:    "TEMPLATE LAMA tanpa kolom Kategori -- harus ditolak",
			cells:   append([]string{}, venueTemplateHeadersLegacy...),
			wantErr: true,
		},
		{
			name:    "kolom tertukar urutannya",
			cells:   []string{"Nama PIC", "Nama Venue", "No Tlp PIC", "No Tlp Venue", "Email", "Alamat", "Kota", "Kategori", "Harga Sewa", "Charge", "Kapasitas", "Fasilitas", "Sosial Media", "Catatan"},
			wantErr: true,
		},
		{
			name:    "baris header kosong",
			cells:   []string{"", "  ", ""},
			wantErr: true,
		},
		{
			name:    "tidak ada sel sama sekali",
			cells:   nil,
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateImportHeaderRow(c.cells, venueTemplateHeaders)
			if c.wantErr && err == nil {
				t.Fatalf("seharusnya ditolak, tapi diterima")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("seharusnya diterima, tapi ditolak: %v", err)
			}
		})
	}
}

// Template lama venue punya "Harga Sewa" di indeks 7, tempat "Kategori" berada
// sekarang. Pesan galatnya harus menyebut kolom yang tidak cocok itu, bukan
// sekadar "format salah" -- itu yang memberi tahu user apa yang harus mereka
// lakukan.
func TestValidateImportHeaderRow_PesanMenyebutKolomYangMeleset(t *testing.T) {
	err := validateImportHeaderRow(venueTemplateHeadersLegacy, venueTemplateHeaders)
	if err == nil {
		t.Fatal("template lama seharusnya ditolak")
	}
	msg := err.Error()
	for _, want := range []string{"Kategori", "Harga Sewa", "Template"} {
		if !contains(msg, want) {
			t.Errorf("pesan %q seharusnya menyebut %q", msg, want)
		}
	}
}

// templateHeaderRowFor merakit baris header persis seperti Template():
// nama kolom apa adanya, dengan penanda " *" pada kolom wajib.
func templateHeaderRowFor(headers, required []string) []string {
	requiredSet := make(map[string]struct{}, len(required))
	for _, h := range required {
		requiredSet[h] = struct{}{}
	}
	row := make([]string, 0, len(headers))
	for _, h := range headers {
		_, isRequired := requiredSet[h]
		row = append(row, templateHeaderLabel(h, isRequired))
	}
	return row
}

func TestValidateImportHeaderRow_Vendor(t *testing.T) {
	if err := validateImportHeaderRow(append([]string{}, vendorTemplateHeaders...), vendorTemplateHeaders); err != nil {
		t.Fatalf("template vendor terbaru seharusnya diterima: %v", err)
	}
	// Baris header Template() milik Vendor, dirakit dari sumber yang sama.
	if err := validateImportHeaderRow(
		templateHeaderRowFor(vendorTemplateHeaders, vendorRequiredImportHeaders),
		vendorTemplateHeaders); err != nil {
		t.Fatalf("baris header Template() vendor seharusnya diterima: %v", err)
	}
	if err := validateImportHeaderRow(append(append([]string{}, vendorTemplateHeaders...), "Status"), vendorTemplateHeaders); err != nil {
		t.Fatalf("hasil Export vendor seharusnya diterima: %v", err)
	}
	// Berkas venue diunggah ke Import vendor -- kolom pertamanya saja sudah beda.
	if err := validateImportHeaderRow(append([]string{}, venueTemplateHeaders...), vendorTemplateHeaders); err == nil {
		t.Error("berkas venue seharusnya ditolak oleh Import vendor")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
