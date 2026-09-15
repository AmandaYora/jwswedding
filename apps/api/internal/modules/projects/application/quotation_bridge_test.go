package application

import (
	"context"
	"errors"
	"testing"

	"jwswedding/internal/shared/apperror"
)

// stubQuotationStatus memenuhi QuotationResolver hanya untuk satu method yang
// diuji di sini; sisanya sengaja panic supaya pemakaian tak terduga langsung
// ketahuan alih-alih diam-diam mengembalikan nol.
type stubQuotationStatus struct {
	status string
	err    error
}

func (s stubQuotationStatus) DeleteQuotation(context.Context, int64, int64) error {
	panic("not implemented")
}
func (s stubQuotationStatus) PONumberForQuotation(context.Context, int64, int64) (string, error) {
	panic("not implemented")
}
func (s stubQuotationStatus) CompositionForQuotation(context.Context, int64, int64) ([][4]string, error) {
	panic("not implemented")
}
func (s stubQuotationStatus) PackageNameForQuotation(context.Context, int64, int64) (string, error) {
	panic("not implemented")
}
func (s stubQuotationStatus) StatusForQuotation(context.Context, int64, int64) (string, error) {
	return s.status, s.err
}

func TestQuotationUnderRevision(t *testing.T) {
	cases := []struct {
		name        string
		quotationID int64
		resolver    QuotationResolver
		want        bool
	}{
		// Revise mengembalikan penawaran ke Draft. Selama belum dikirim ulang,
		// nilai kontrak yang sudah terdorong ke project belum disepakati siapa
		// pun.
		{"draft berarti sedang direvisi", 7, stubQuotationStatus{status: "Draft"}, true},
		{"sudah diterima kembali", 7, stubQuotationStatus{status: "Accepted"}, false},
		{"terkirim, menunggu jawaban", 7, stubQuotationStatus{status: "Sent"}, false},
		// Project pra-penawaran tidak punya penawaran untuk direvisi.
		{"tanpa penawaran", 0, stubQuotationStatus{status: "Draft"}, false},
		{"resolver belum terpasang", 7, nil, false},
		// Menahan penerbitan Tagihan karena satu pembacaan gagal jauh lebih
		// mengganggu daripada satu tanda yang tidak muncul.
		{"pembacaan gagal", 7, stubQuotationStatus{err: errors.New("boom")}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &ProjectService{}
			if tc.resolver != nil {
				svc.quotations = tc.resolver
			}
			if got := svc.QuotationUnderRevision(context.Background(), 1, tc.quotationID); got != tc.want {
				t.Errorf("QuotationUnderRevision() = %v, want %v", got, tc.want)
			}
		})
	}
}

// stubPONumber memenuhi QuotationResolver hanya untuk PONumberForQuotation.
type stubPONumber struct {
	number string
	err    error
}

func (s stubPONumber) DeleteQuotation(context.Context, int64, int64) error { panic("not implemented") }
func (s stubPONumber) PONumberForQuotation(context.Context, int64, int64) (string, error) {
	return s.number, s.err
}
func (s stubPONumber) CompositionForQuotation(context.Context, int64, int64) ([][4]string, error) {
	panic("not implemented")
}
func (s stubPONumber) PackageNameForQuotation(context.Context, int64, int64) (string, error) {
	panic("not implemented")
}
func (s stubPONumber) StatusForQuotation(context.Context, int64, int64) (string, error) {
	panic("not implemented")
}

// Dialog hapus project MENOLAK tampil bila dampaknya gagal dibaca (D14).
// Karena itu penawaran yang barisnya sudah tidak ada tidak boleh terbaca
// sebagai kegagalan: project yatim seperti itu justru yang paling perlu bisa
// dihapus, dan sebelum ini ia terkunci selamanya di layar.
func TestPONumberForQuotation_PenawaranSudahTidakAda_BukanGalat(t *testing.T) {
	svc := &ProjectService{}
	svc.quotations = stubPONumber{err: apperror.NotFound("Penawaran tidak ditemukan")}

	got, err := svc.PONumberForQuotation(context.Background(), 1, 3)
	if err != nil {
		t.Fatalf("PONumberForQuotation() error = %v, want nil -- tidak ada penawaran yang ikut terhapus", err)
	}
	if got != "" {
		t.Errorf("poNumber = %q, want \"\" (tidak ada nomor untuk disebut)", got)
	}
}

// Galat lain harus tetap diteruskan: melaporkan koneksi putus sebagai "tidak
// ada penawaran" membuat dialog hapus berbohong tentang apa yang ikut hilang.
func TestPONumberForQuotation_GalatLain_Diteruskan(t *testing.T) {
	svc := &ProjectService{}
	svc.quotations = stubPONumber{err: errors.New("connection refused")}

	if _, err := svc.PONumberForQuotation(context.Background(), 1, 3); err == nil {
		t.Fatal("PONumberForQuotation() error = nil, want diteruskan")
	}
}

func TestPONumberForQuotation_Normal(t *testing.T) {
	svc := &ProjectService{}
	svc.quotations = stubPONumber{number: "PO/2026/IX/0007"}

	got, err := svc.PONumberForQuotation(context.Background(), 1, 3)
	if err != nil {
		t.Fatalf("PONumberForQuotation() error = %v", err)
	}
	if got != "PO/2026/IX/0007" {
		t.Errorf("poNumber = %q, want PO/2026/IX/0007", got)
	}
}

// Project pra-penawaran tidak pernah menyentuh modul quotations sama sekali
// (stub-nya panic bila disentuh).
func TestPONumberForQuotation_TanpaPenawaran_TidakMemanggilResolver(t *testing.T) {
	svc := &ProjectService{}
	svc.quotations = stubQuotationStatus{}

	got, err := svc.PONumberForQuotation(context.Background(), 1, 0)
	if err != nil || got != "" {
		t.Fatalf("PONumberForQuotation() = %q, %v; want \"\", nil", got, err)
	}
}
