package application

import "strconv"

func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}

// Lebar kolom activity_log yang diisi teks kendali pengguna (migrasi 000008).
const (
	maxActivityLabel       = 255 // entity_label VARCHAR(255)
	maxActivityDescription = 500 // description VARCHAR(500)
)

// truncateTo memangkas teks agar muat di kolom activity_log yang dituju.
// Dipakai setiap call site Record yang meneruskan teks yang dikendalikan
// pengguna: ActivityService.Record MENELAN error tulisnya (hanya mencatat
// log), jadi tanpa pemangkasan ini operasinya tetap berhasil sementara jejak
// aktivitasnya hilang tanpa suara — lihat
// docs/plan/vendor-engagement-500/PLAN.md T2/T3.
//
// Berbasis []rune, bukan byte: satu rune = satu karakter utf8mb4, satuan
// yang sama dengan VARCHAR di MySQL, sehingga karakter multibyte tidak
// terpotong di tengah.
func truncateTo(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
