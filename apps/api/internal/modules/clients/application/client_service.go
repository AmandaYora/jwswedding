package application

import (
	"context"

	identitycontracts "jwswedding/internal/modules/identity/contracts"
	projectscontracts "jwswedding/internal/modules/projects/contracts"
	quotationscontracts "jwswedding/internal/modules/quotations/contracts"

	"jwswedding/internal/modules/clients/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/logger"
	"jwswedding/internal/shared/pagination"
)

type ClientRepository interface {
	FindByID(ctx context.Context, tenantID, id int64) (*domain.Client, error)
	// FindByIDs memuat sekumpulan client dalam satu query — dipakai
	// CoupleNamesBatch supaya daftar Penawaran tidak jadi N+1 (§11).
	FindByIDs(ctx context.Context, tenantID int64, ids []int64) ([]domain.Client, error)
	ListPaginated(ctx context.Context, tenantID int64, params pagination.Params, search string) ([]domain.Client, int64, error)
	Create(ctx context.Context, c *domain.Client) error
	Update(ctx context.Context, c *domain.Client) error
	Delete(ctx context.Context, tenantID, id int64) error
}

// ClientService mengelola master pasangan (T1.8) — Client baru. Bertindak
// sebagai SyncCredentialActive (D4), direktori nama/telepon untuk modul
// quotations, dan orkestrasi hapus-berjenjang (T3.5).
type ClientService struct {
	repo       ClientRepository
	contacts   *ClientContactService
	projects   projectscontracts.Contracts
	quotations quotationscontracts.Contracts
	identity   identitycontracts.Contracts
}

func NewClientService(repo ClientRepository, contacts *ClientContactService, projects projectscontracts.Contracts, quotations quotationscontracts.Contracts, identity identitycontracts.Contracts) *ClientService {
	return &ClientService{repo: repo, contacts: contacts, projects: projects, quotations: quotations, identity: identity}
}

type CreateClientInput struct {
	BrideName string
	GroomName string
	Phone     string
	Email     string
	Notes     string
}

func (s *ClientService) Create(ctx context.Context, tenantID int64, input CreateClientInput) (*domain.Client, error) {
	if input.BrideName == "" || input.GroomName == "" {
		return nil, apperror.Validation("Data pasangan belum lengkap", map[string][]string{
			"brideName": {"Nama mempelai wanita wajib diisi"},
			"groomName": {"Nama mempelai pria wajib diisi"},
		})
	}
	c := &domain.Client{
		TenantID: tenantID, BrideName: input.BrideName, GroomName: input.GroomName,
		Phone: input.Phone, Email: input.Email, Notes: input.Notes,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

type UpdateClientInput struct {
	BrideName string
	GroomName string
	Phone     string
	Email     string
	Notes     string
}

func (s *ClientService) Update(ctx context.Context, tenantID, id int64, input UpdateClientInput) (*domain.Client, error) {
	c, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if input.BrideName == "" || input.GroomName == "" {
		return nil, apperror.Validation("Data pasangan belum lengkap", map[string][]string{
			"brideName": {"Nama mempelai wanita wajib diisi"},
			"groomName": {"Nama mempelai pria wajib diisi"},
		})
	}
	c.BrideName = input.BrideName
	c.GroomName = input.GroomName
	c.Phone = input.Phone
	c.Email = input.Email
	c.Notes = input.Notes
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *ClientService) Get(ctx context.Context, tenantID, id int64) (*domain.Client, error) {
	c, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, apperror.NotFound("Client tidak ditemukan")
	}
	return c, nil
}

// ClientListItem adalah satu baris daftar Client: pasangan + hitungannya.
// ContactCount dari subquery agregat satu halaman (§11); ProjectCount dari
// satu panggilan batch ke projects (tanpa join lintas modul).
type ClientListItem struct {
	Client       domain.Client
	ContactCount int
	ProjectCount int
}

func (s *ClientService) ListPaginated(ctx context.Context, tenantID int64, params pagination.Params, search string) ([]ClientListItem, int64, error) {
	list, total, err := s.repo.ListPaginated(ctx, tenantID, params, search)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]int64, 0, len(list))
	for _, c := range list {
		ids = append(ids, c.ID)
	}
	contactCounts, err := s.contacts.repo.CountByClients(ctx, tenantID, ids)
	if err != nil {
		return nil, 0, err
	}
	projectCounts, err := s.projects.ProjectCountsForClients(ctx, tenantID, ids)
	if err != nil {
		return nil, 0, err
	}
	items := make([]ClientListItem, 0, len(list))
	for _, c := range list {
		items = append(items, ClientListItem{
			Client: c, ContactCount: contactCounts[c.ID], ProjectCount: projectCounts[c.ID],
		})
	}
	return items, total, nil
}

// VerifyClientAccess membatasi Staff (Wedding Planner) dan Sales pada client
// yang punya >=1 project PIC mereka — Owner/Admin lewat tanpa syarat (aturan
// yang sama dengan VerifyProjectReadAccess yang lama, digeser ke master).
//
// Pengecualian untuk PROSPEK: client yang belum punya project sama sekali
// terbuka untuk Sales. Tanpa ini alur utama fitur ini mati — Sales membuat
// client calon, lalu langsung 403 saat membuka detailnya, karena project baru
// lahir setelah penawaran Diterima. Wedding Planner tidak ikut dikecualikan:
// prospek belum jadi pekerjaan siapa pun sampai ada project.
func (s *ClientService) VerifyClientAccess(ctx context.Context, tenantID, clientID, staffID int64, role string) error {
	switch role {
	case "Staff", "Sales":
		projectIDs, err := s.projects.ProjectIDsForClient(ctx, tenantID, clientID)
		if err != nil {
			return err
		}
		if len(projectIDs) == 0 && role == "Sales" {
			return nil
		}
		for _, pid := range projectIDs {
			var picID int64
			var err error
			if role == "Staff" {
				picID, err = s.projects.ProjectPICStaffID(ctx, tenantID, pid)
			} else {
				picID, err = s.projects.ProjectPICSalesStaffID(ctx, tenantID, pid)
			}
			if err != nil {
				continue
			}
			if picID == staffID {
				return nil
			}
		}
		return apperror.Forbidden("Anda tidak memiliki akses ke client ini")
	}
	return nil
}

// SyncCredentialActive menghitung ulang login efektif seluruh kontak milik
// satu client (D4): credential.IsActive = is_active DAN (client punya ≥1
// project). Dipanggil dari Create/SetActive kontak, dari luar saat project
// lahir (best-effort), dan setelah project dihapus.
func (s *ClientService) SyncCredentialActive(ctx context.Context, tenantID, clientID int64) error {
	hasProject, err := s.projects.ClientHasProject(ctx, tenantID, clientID)
	if err != nil {
		return err
	}
	list, err := s.contacts.repo.ListByClient(ctx, tenantID, clientID)
	if err != nil {
		return err
	}
	for _, c := range list {
		effective := c.IsActive && hasProject
		if err := s.identity.SetActive(ctx, identitycontracts.PrincipalClient, formatID(c.ID), effective); err != nil {
			logger.Error("gagal sinkron kredensial kontak %d (client %d): %v", c.ID, clientID, err)
		}
	}
	return nil
}

// ClientIDForContact memetakan token portal (id kontak) ke master client-nya
// — langkah pertama resolusi akses portal (T1.10).
func (s *ClientService) ClientIDForContact(ctx context.Context, tenantID, contactID int64) (int64, error) {
	c, err := s.contacts.Get(ctx, tenantID, contactID)
	if err != nil {
		return 0, err
	}
	if !c.IsActive {
		return 0, apperror.Forbidden("Akun client ini sudah dinonaktifkan")
	}
	return c.ClientID, nil
}

// CoupleNames / CoupleNamesBatch / PhoneForClient adalah direktori baca untuk
// modul quotations (kop PO, daftar Penawaran) — satu baris atau satu halaman
// dalam satu panggilan, tidak pernah join lintas modul (§11).
func (s *ClientService) CoupleNames(ctx context.Context, tenantID, clientID int64) (bride, groom string, err error) {
	c, err := s.Get(ctx, tenantID, clientID)
	if err != nil {
		return "", "", err
	}
	return c.BrideName, c.GroomName, nil
}

func (s *ClientService) CoupleNamesBatch(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64][2]string, error) {
	out := make(map[int64][2]string, len(clientIDs))
	if len(clientIDs) == 0 {
		return out, nil
	}
	list, err := s.repo.FindByIDs(ctx, tenantID, clientIDs)
	if err != nil {
		return nil, err
	}
	for _, c := range list {
		out[c.ID] = [2]string{c.BrideName, c.GroomName}
	}
	return out, nil
}

func (s *ClientService) PhoneForClient(ctx context.Context, tenantID, clientID int64) (string, error) {
	c, err := s.Get(ctx, tenantID, clientID)
	if err != nil {
		return "", err
	}
	if c.Phone != "" {
		return c.Phone, nil
	}
	list, err := s.contacts.repo.ListByClient(ctx, tenantID, clientID)
	if err != nil {
		return "", err
	}
	for _, contact := range list {
		if contact.Phone != "" {
			return contact.Phone, nil
		}
	}
	return "", nil
}

// ClientDeleteImpact adalah bahan dialog konfirmasi hapus Client (D14, T3.5):
// digabung dari tiga sumber yang masing-masing menjawab dari datanya sendiri.
type ClientDeleteImpact struct {
	Client           domain.Client
	ContactCount     int
	ProjectCount     int
	ProjectNames     []string
	PaidInvoiceCount int
	PaidInvoiceTotal int64
	QuotationCount   int
	QuotationNumbers []string
}

func (s *ClientService) DeleteImpact(ctx context.Context, tenantID, id int64) (ClientDeleteImpact, error) {
	var out ClientDeleteImpact
	c, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return out, err
	}
	out.Client = *c

	list, err := s.contacts.repo.ListByClient(ctx, tenantID, id)
	if err != nil {
		return out, err
	}
	out.ContactCount = len(list)

	pImpact, err := s.projects.ImpactForClient(ctx, tenantID, id)
	if err != nil {
		return out, err
	}
	out.ProjectCount = pImpact.ProjectCount
	out.ProjectNames = pImpact.ProjectNames
	out.PaidInvoiceCount = pImpact.PaidInvoiceCount
	out.PaidInvoiceTotal = pImpact.PaidInvoiceTotal

	qImpact, err := s.quotations.ImpactForClient(ctx, tenantID, id)
	if err != nil {
		return out, err
	}
	out.QuotationCount = qImpact.Count
	out.QuotationNumbers = qImpact.Numbers
	return out, nil
}

// Delete menghapus Client berjenjang (D14, T3.5) — selalu tersedia, tanpa
// pengecualian yang memblokir. Urutan dari yang merujuk ke yang dirujuk:
// project (beserta isinya + penawaran Diterima-nya) → penawaran yang belum
// jadi project → kontak (+akun portal) → baris client. Lintas modul
// sekuensial (bukan satu transaksi); kegagalan di tengah dicatat, tidak
// menggagalkan yang sudah terjadi.
func (s *ClientService) Delete(ctx context.Context, tenantID, id int64) error {
	if _, err := s.Get(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.projects.DeleteProjectsForClient(ctx, tenantID, id); err != nil {
		logger.Error("gagal menghapus project milik client %d: %v", id, err)
	}
	if err := s.quotations.DeleteForClient(ctx, tenantID, id); err != nil {
		logger.Error("gagal menghapus penawaran milik client %d: %v", id, err)
	}
	list, err := s.contacts.repo.ListByClient(ctx, tenantID, id)
	if err != nil {
		return err
	}
	for _, c := range list {
		if err := s.identity.SetActive(ctx, identitycontracts.PrincipalClient, formatID(c.ID), false); err != nil {
			logger.Error("gagal menonaktifkan kredensial kontak %d saat menghapus client %d: %v", c.ID, id, err)
		}
	}
	if err := s.contacts.repo.DeleteForClient(ctx, tenantID, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, tenantID, id)
}
