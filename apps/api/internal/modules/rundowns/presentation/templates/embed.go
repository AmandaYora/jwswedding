// Package templates menanam berkas .docx template buku acara ke dalam biner.
//
// Satu template bawaan, bukan unggahan per tenant: aplikasi ini single-tenant
// dan tidak ada permintaan untuk mengganti desainnya per pemakai. Idiom
// go:embed-nya sama dengan apps/api/migrations/embed.go.
//
// Template dibangun ulang dari berkas contoh dengan
// `python tools/rundown-template/build_template.py` — jangan disunting tangan
// di sini, karena suntingan itu akan hilang saat skripnya dijalankan lagi.
package templates

import _ "embed"

//go:embed rundown_template.docx
var RundownDocx []byte
