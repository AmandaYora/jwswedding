package application

import (
	"context"
	"strings"
	"testing"

	"jwswedding/internal/modules/rundowns/domain"
	"jwswedding/internal/shared/apperror"
)

// rundownForTest melahirkan satu rundown dan mengembalikan service, repo-nya,
// serta id rundown itu — bentuk awal yang sama untuk seluruh tes di berkas ini.
func rundownForTest(t *testing.T) (*RundownService, *fakeRepo, int64) {
	t.Helper()
	svc, repo, _, _ := newService()
	view, err := svc.CreateFromProject(context.Background(), 1, CreateInput{ProjectID: 42})
	if err != nil {
		t.Fatalf("CreateFromProject: %v", err)
	}
	repo.writeTenantIDs = nil
	return svc, repo, view.Rundown.ID
}

// Panjang kolom dijaga di server, bukan cuma di skema Zod editor. Tanpa ini
// payload yang lebih panjang sampai ke MySQL dan ditolak sebagai galat 1406,
// yang terbaca pengguna sebagai 500 — padahal keadaannya sangat bisa
// dijelaskan.
func TestReplaceSection_RejectsOverlongValues(t *testing.T) {
	cases := []struct {
		name    string
		section domain.SectionKey
		payload SectionPayload
		field   string
	}{
		{
			name:    "cover.woPicName lebih dari 100",
			section: domain.SectionKeyCover,
			payload: SectionPayload{Cover: CoverPayload{WOPICName: strings.Repeat("a", 101)}},
			field:   "cover.woPicName",
		},
		{
			name:    "vendors.vendorName lebih dari 150",
			section: domain.SectionKeyVendors,
			payload: SectionPayload{Vendors: []domain.Vendor{{VendorName: strings.Repeat("a", 151)}}},
			field:   "vendors[0].vendorName",
		},
		{
			name:    "tableClothNote lebih dari 255",
			section: domain.SectionKeyDataLainnya,
			payload: SectionPayload{DataLainnya: DataLainnyaPayload{
				TableClothNote: strings.Repeat("a", 256)}},
			field: "dataLainnya.tableClothNote",
		},
		{
			name:    "items.timeLabel lebih dari 50",
			section: domain.SectionKeyAcaraAkad,
			payload: SectionPayload{Items: []domain.Item{{TimeLabel: strings.Repeat("a", 51)}}},
			field:   "items[0].timeLabel",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, repo, id := rundownForTest(t)
			_, err := svc.ReplaceSection(context.Background(), 1, id, c.section, c.payload)
			appErr, ok := apperror.As(err)
			if !ok || appErr.Kind != apperror.KindValidation {
				t.Fatalf("mau galat validasi, dapat %v", err)
			}
			if _, named := appErr.Fields[c.field]; !named {
				t.Errorf("galat tidak menyebut field %q: %v", c.field, appErr.Fields)
			}
			if len(repo.sections) != 0 {
				t.Error("payload yang ditolak seharusnya tidak sampai ke repository")
			}
		})
	}
}

// Batasnya karakter, bukan byte: VARCHAR(150) di MySQL berarti 150 karakter,
// dan satu huruf beraksen memakan dua byte. Menghitung byte akan menolak nama
// yang sebenarnya sah.
func TestReplaceSection_LengthIsCountedInCharactersNotBytes(t *testing.T) {
	svc, _, id := rundownForTest(t)
	name := strings.Repeat("é", 150) // 150 karakter, 300 byte
	_, err := svc.ReplaceSection(context.Background(), 1, id, domain.SectionKeyVendors,
		SectionPayload{Vendors: []domain.Vendor{{VendorName: name}}})
	if err != nil {
		t.Fatalf("150 karakter seharusnya lolos batas VARCHAR(150): %v", err)
	}
}

// Gerbang tenant harus sampai ke repository, bukan berhenti di service:
// `WHERE id = ?` tanpa tenant membuat satu pemanggil baru yang lupa memuat
// rundown-nya lebih dulu bisa menulis ke tenant lain tanpa hambatan apa pun.
func TestReplaceSection_PassesTenantIDToRepository(t *testing.T) {
	svc, repo, id := rundownForTest(t)
	if _, err := svc.ReplaceSection(context.Background(), 1, id,
		domain.SectionKeyFotoTamu,
		SectionPayload{PhotoGroups: []domain.PhotoGroup{{GroupName: "Keluarga Inti"}}}); err != nil {
		t.Fatalf("ReplaceSection: %v", err)
	}
	if len(repo.writeTenantIDs) == 0 {
		t.Fatal("repository tidak menerima satu pun penulisan")
	}
	for _, got := range repo.writeTenantIDs {
		if got != 1 {
			t.Errorf("repository menerima tenantID %d, mau 1", got)
		}
	}
}
