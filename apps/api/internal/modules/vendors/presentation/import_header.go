package presentation

import (
	"fmt"
	"strings"
)

// validateImportHeaderRow memastikan baris header berkas yang diunggah masih
// sesuai urutan kolom yang diharapkan parser.
//
// Kenapa ini ada: parser Import membaca sel BERDASARKAN INDEKS
// (vendorTemplateHeaders/venueTemplateHeaders adalah satu-satunya sumber
// urutan), dan baris header dulu dilewati begitu saja dengan alasan
// "position and meaning are fixed". Alasan itu berlaku selama daftar kolom
// tidak pernah berubah — dan ia berubah: kolom "Kategori" disisipkan ke
// template Venue (PLAN revisi-vendor-venue-portal), menggeser setiap kolom
// sesudahnya.
//
// Akibatnya, berkas 13-kolom yang sudah terlanjur diunduh user sebelum rilis
// itu sebagian besar akan ditolak per baris — tapi TIDAK semuanya. Bila Harga
// Sewa, Charge, Kapasitas, dan Fasilitas sama-sama kosong, tak satu pun
// validasi terpicu dan barisnya tersimpan dengan data tergeser diam-diam:
// Sosial Media masuk ke Fasilitas, Catatan masuk ke Sosial Media. Kegagalan
// senyap seperti itu jauh lebih mahal daripada penolakan yang jelas.
//
// Toleransi yang disengaja, supaya berkas yang SAH tetap diterima:
//   - berkas dari Template() membawa penanda wajib " *" pada sebagian header
//     (lihat templateHeaderLabel) — penanda itu dilucuti sebelum dibandingkan;
//   - berkas dari Export() membawa kolom ekstra "Status" di ujung — kolom
//     berlebih di belakang diabaikan, hanya kekurangan yang ditolak;
//   - beda huruf besar/kecil dan spasi tepi tidak dipermasalahkan.
func validateImportHeaderRow(cells []string, expected []string) error {
	if len(cells) == 0 || strings.TrimSpace(strings.Join(cells, "")) == "" {
		return fmt.Errorf("baris header tidak ditemukan — gunakan berkas hasil unduh Template")
	}
	// Dibandingkan kolom per kolom LEBIH DULU, bukan jumlahnya lebih dulu:
	// pesan "13 kolom, seharusnya 14" tidak memberi tahu user apa yang harus
	// diperbaiki, sedangkan "kolom ke-8 seharusnya Kategori, bukan Harga
	// Sewa" langsung menunjuk kolom yang tergeser.
	for i, want := range expected {
		if i >= len(cells) {
			return fmt.Errorf(
				"kolom ke-%d (%q) tidak ada di berkas — unduh ulang Template lalu isi kembali",
				i+1, want)
		}
		if got := normalizeImportHeader(cells[i]); !strings.EqualFold(got, want) {
			return fmt.Errorf(
				"kolom ke-%d seharusnya %q, bukan %q — unduh ulang Template lalu isi kembali",
				i+1, want, strings.TrimSpace(cells[i]))
		}
	}
	return nil
}

// normalizeImportHeader melucuti spasi tepi dan penanda wajib " *" yang
// ditambahkan templateHeaderLabel, sehingga header dari Template dan dari
// Export sama-sama cocok dengan nama kolom aslinya.
func normalizeImportHeader(cell string) string {
	trimmed := strings.TrimSpace(cell)
	trimmed = strings.TrimSuffix(trimmed, "*")
	return strings.TrimSpace(trimmed)
}
