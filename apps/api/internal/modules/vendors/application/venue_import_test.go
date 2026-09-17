package application

import (
	"context"
	"strings"
	"testing"

	"jwswedding/internal/modules/vendors/domain"
	"jwswedding/internal/shared/pagination"
)

// fakeVenueRepo adalah VenueRepository dalam memori untuk mengunci alur
// ImportVenues + kategori (PLAN revisi-vendor-venue-portal §6): CreateBatch
// sengaja tidak mengisi ID (seperti MySQL), Update menimpa penuh termasuk
// kategori, SetCategoriesBatch mencatat tulisannya.
type fakeVenueRepo struct {
	rows       map[int64]*domain.Venue
	categories map[int64][]string
	nextID     int64
	batchCalls int
}

func newFakeVenueRepo() *fakeVenueRepo {
	return &fakeVenueRepo{rows: map[int64]*domain.Venue{}, categories: map[int64][]string{}, nextID: 1}
}

func (f *fakeVenueRepo) List(_ context.Context, _ int64) ([]domain.Venue, error) {
	var out []domain.Venue
	for _, v := range f.rows {
		cp := *v
		cp.Categories = append([]string{}, f.categories[v.ID]...)
		out = append(out, cp)
	}
	return out, nil
}

func (f *fakeVenueRepo) ListPaginated(_ context.Context, _ int64, _ VenueListFilter, _ pagination.Params) ([]domain.Venue, int64, error) {
	panic("not implemented")
}

func (f *fakeVenueRepo) ListFiltered(_ context.Context, _ int64, _ VenueListFilter) ([]domain.Venue, error) {
	panic("not implemented")
}

func (f *fakeVenueRepo) FindByID(_ context.Context, _, id int64) (*domain.Venue, error) {
	if v, ok := f.rows[id]; ok {
		cp := *v
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeVenueRepo) Create(_ context.Context, venue *domain.Venue) error {
	venue.ID = f.nextID
	f.nextID++
	cp := *venue
	f.rows[venue.ID] = &cp
	f.categories[venue.ID] = append([]string{}, venue.Categories...)
	return nil
}

func (f *fakeVenueRepo) CreateBatch(_ context.Context, venues []domain.Venue) error {
	f.batchCalls++
	for _, v := range venues {
		v.ID = f.nextID
		f.nextID++
		cp := v
		f.rows[v.ID] = &cp
		// Kategori TIDAK ditulis di sini — seperti MySQL yang tidak membaca
		// ID kembali; jalurnya lewat SetCategoriesBatch.
	}
	return nil
}

func (f *fakeVenueRepo) Update(_ context.Context, venue *domain.Venue) error {
	cp := *venue
	f.rows[venue.ID] = &cp
	f.categories[venue.ID] = append([]string{}, venue.Categories...)
	return nil
}

func (f *fakeVenueRepo) SetActive(_ context.Context, _, _ int64, _ bool) error {
	panic("not implemented")
}

func (f *fakeVenueRepo) UpdateAttachment(_ context.Context, _, _ int64, _, _ *string) error {
	panic("not implemented")
}

func (f *fakeVenueRepo) SetCategoriesBatch(_ context.Context, _ int64, venueCategories map[int64][]string) error {
	for id, cats := range venueCategories {
		f.categories[id] = append([]string{}, cats...)
		if v, ok := f.rows[id]; ok {
			v.Categories = append([]string{}, cats...)
		}
	}
	return nil
}

func (f *fakeVenueRepo) Delete(_ context.Context, _, _ int64) error {
	panic("not implemented")
}

func venueRow(name, city string, cats ...string) VenueImportRow {
	return VenueImportRow{
		Name: name, PICName: "PIC " + name, PhonePIC: "08123456789",
		City: city, Categories: cats,
	}
}

// TestImportVenues_Categories mengunci penulisan kategori lewat import (PLAN
// revisi-vendor-venue-portal §4.3/C2 + §6).
func TestImportVenues_Categories(t *testing.T) {
	ctx := context.Background()

	t.Run("kategori tak dikenal jadi galat baris, bukan gagal total", func(t *testing.T) {
		repo := newFakeVenueRepo()
		svc := NewVenueService(repo, nil, nil)
		result, err := svc.ImportVenues(ctx, 1, []VenueImportRow{
			venueRow("Gedung A", "Kota Bogor", "Function Hall"),
			venueRow("Gedung B", "Kota Bogor", "Gedung Ngawur"),
		})
		if err != nil {
			t.Fatalf("ImportVenues: %v", err)
		}
		if result.InsertedCount != 1 {
			t.Errorf("InsertedCount = %d, seharusnya 1", result.InsertedCount)
		}
		if len(result.Errors) != 1 || result.Errors[0].Row != 3 {
			t.Errorf("Errors = %+v, seharusnya satu galat baris 3", result.Errors)
		}
		if !strings.Contains(result.Errors[0].Message, "Gedung Ngawur") {
			t.Errorf("pesan galat seharusnya menyebut nilai tak dikenal, dapat %q", result.Errors[0].Message)
		}
	})

	t.Run("kategori jamak dipisah koma dan tersimpan", func(t *testing.T) {
		repo := newFakeVenueRepo()
		svc := NewVenueService(repo, nil, nil)
		result, err := svc.ImportVenues(ctx, 1, []VenueImportRow{
			venueRow("Gedung A", "Kota Bogor", "Restaurant", "Hotel Bintang 5"),
		})
		if err != nil {
			t.Fatalf("ImportVenues: %v", err)
		}
		if result.InsertedCount != 1 || len(result.Errors) != 0 {
			t.Fatalf("result = %+v, err = %v", result, err)
		}
		var stored *domain.Venue
		for _, v := range repo.rows {
			stored = v
		}
		if stored == nil {
			t.Fatal("tidak ada venue tersimpan")
		}
		got := repo.categories[stored.ID]
		if len(got) != 2 || got[0] != "Hotel Bintang 5" || got[1] != "Restaurant" {
			t.Errorf("kategori tersimpan = %v, seharusnya [Hotel Bintang 5 Restaurant] (urutan kanonik)", got)
		}
	})

	t.Run("sel kosong mengosongkan kategori", func(t *testing.T) {
		repo := newFakeVenueRepo()
		existing := &domain.Venue{TenantID: 1, ID: 1, Name: "Gedung A", City: strptr("Kota Bogor"), IsActive: true}
		repo.rows[1] = existing
		repo.categories[1] = []string{"Function Hall", "Restaurant", "Hotel Bintang 5"}
		repo.nextID = 2
		svc := NewVenueService(repo, nil, nil)
		result, err := svc.ImportVenues(ctx, 1, []VenueImportRow{
			venueRow("Gedung A", "Kota Bogor"),
		})
		if err != nil {
			t.Fatalf("ImportVenues: %v", err)
		}
		if result.UpdatedCount != 1 || len(result.Errors) != 0 {
			t.Fatalf("result = %+v, err = %v", result, err)
		}
		if len(repo.categories[1]) != 0 {
			t.Errorf("kategori seharusnya dikosongkan (D-5), tersisa %v", repo.categories[1])
		}
	})

	t.Run("dua baris baru sama tetap satu venue dan satu set kategori", func(t *testing.T) {
		repo := newFakeVenueRepo()
		svc := NewVenueService(repo, nil, nil)
		result, err := svc.ImportVenues(ctx, 1, []VenueImportRow{
			venueRow("Gedung A", "Kota Bogor", "Function Hall"),
			venueRow("gedung a", "kota bogor", "Restaurant"),
		})
		if err != nil {
			t.Fatalf("ImportVenues: %v", err)
		}
		if result.InsertedCount != 1 || len(result.Errors) != 0 {
			t.Fatalf("result = %+v, err = %v", result, err)
		}
		if len(repo.rows) != 1 {
			t.Fatalf("venue tersimpan = %d, seharusnya 1", len(repo.rows))
		}
		for id := range repo.rows {
			if got := repo.categories[id]; len(got) != 1 || got[0] != "Restaurant" {
				t.Errorf("kategori = %v, seharusnya [Restaurant] (baris terakhir menang)", got)
			}
		}
	})
}

func strptr(s string) *string { return &s }
