package application

import (
	"context"
	"time"

	identitycontracts "jwswedding/internal/modules/identity/contracts"

	"jwswedding/internal/modules/clients/domain"
	"jwswedding/internal/shared/apperror"
	"jwswedding/internal/shared/logger"
	"jwswedding/internal/shared/validator"
)

type ClientContactRepository interface {
	ListByClient(ctx context.Context, tenantID, clientID int64) ([]domain.ClientContact, error)
	CountByClients(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64]int, error)
	FindByID(ctx context.Context, tenantID, id int64) (*domain.ClientContact, error)
	Create(ctx context.Context, c *domain.ClientContact) error
	Update(ctx context.Context, c *domain.ClientContact) error
	SetActive(ctx context.Context, tenantID, id int64, isActive bool) error
	SetCredentialResetAt(ctx context.Context, tenantID, id int64, when time.Time) error
	Delete(ctx context.Context, tenantID, id int64) error
	DeleteForClient(ctx context.Context, tenantID, clientID int64) error
}

// ClientContactService mengelola kontak/akun portal milik satu Client —
// rename langsung dari ClientService yang lama (T1.8): logika kredensial
// dipertahankan utuh, hanya projectID yang diganti clientID.
type ClientContactService struct {
	repo     ClientContactRepository
	clients  ClientRepository
	identity identitycontracts.Contracts
	// syncer dipasang setelah ClientService dibangun (same-module two-phase):
	// tiap perubahan niat-admin memicu hitung ulang login efektif (D4).
	syncer interface {
		SyncCredentialActive(ctx context.Context, tenantID, clientID int64) error
	}
}

func NewClientContactService(repo ClientContactRepository, clients ClientRepository, identity identitycontracts.Contracts) *ClientContactService {
	return &ClientContactService{repo: repo, clients: clients, identity: identity}
}

func (s *ClientContactService) SetSyncer(syncer interface {
	SyncCredentialActive(ctx context.Context, tenantID, clientID int64) error
}) {
	s.syncer = syncer
}

func (s *ClientContactService) ListByClient(ctx context.Context, tenantID, clientID int64) ([]domain.ClientContact, error) {
	if _, err := s.requireClient(ctx, tenantID, clientID); err != nil {
		return nil, err
	}
	return s.repo.ListByClient(ctx, tenantID, clientID)
}

func (s *ClientContactService) Get(ctx context.Context, tenantID, id int64) (*domain.ClientContact, error) {
	c, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, apperror.NotFound("Kontak client tidak ditemukan")
	}
	return c, nil
}

type CreateContactInput struct {
	ClientID     int64
	Role         domain.ClientRole
	RelationNote string
	Name         string
	Phone        string
	Username     string
	Email        string
	Password     string
}

// Create memvalidasi client milik tenant ini lewat repo sendiri (satu modul,
// tanpa kontrak lintas modul) lalu memprovisikan kredensial login — cermin
// dari Create yang lama, termasuk kompensasi hapus-baris bila
// identity.CreateCredential gagal setelahnya.
func (s *ClientContactService) Create(ctx context.Context, tenantID int64, input CreateContactInput) (*domain.ClientContact, error) {
	if err := validator.Username(input.Username); err != nil {
		return nil, err
	}
	if _, err := s.requireClient(ctx, tenantID, input.ClientID); err != nil {
		return nil, err
	}

	c := &domain.ClientContact{
		TenantID: tenantID, ClientID: input.ClientID, Role: input.Role, Username: input.Username, RelationNote: input.RelationNote,
		Name: input.Name, Phone: input.Phone, Email: input.Email, IsActive: true,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}

	if err := s.identity.CreateCredential(ctx, identitycontracts.CreateCredentialInput{
		TenantID: &tenantID, PrincipalType: identitycontracts.PrincipalClient, PrincipalID: formatID(c.ID),
		Username: input.Username, Email: input.Email, Password: input.Password, Role: string(input.Role), DisplayName: input.Name,
	}); err != nil {
		if delErr := s.repo.Delete(ctx, tenantID, c.ID); delErr != nil {
			logger.Error("failed to roll back orphaned client contact %d after credential creation failed: %v", c.ID, delErr)
		}
		return nil, err
	}
	s.syncAfterChange(ctx, tenantID, input.ClientID)
	return c, nil
}

type UpdateContactInput struct {
	Name  string
	Phone string
	Email string
}

func (s *ClientContactService) UpdateContact(ctx context.Context, tenantID, id int64, input UpdateContactInput) (*domain.ClientContact, error) {
	c, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	c.Name = input.Name
	c.Phone = input.Phone
	c.Email = input.Email
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Delete menghapus permanen satu baris kontak — menonaktifkan kredensialnya
// dulu (best-effort) supaya username-nya tidak diam-diam masih bisa dipakai.
func (s *ClientContactService) Delete(ctx context.Context, tenantID, id int64) error {
	c, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if err := s.identity.SetActive(ctx, identitycontracts.PrincipalClient, formatID(c.ID), false); err != nil {
		logger.Error("failed to deactivate credential for client contact %d before delete: %v", c.ID, err)
	}
	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	s.syncAfterChange(ctx, tenantID, c.ClientID)
	return nil
}

// SetActive mencatat "niat admin" lalu menghitung ulang login efektif (D4) —
// kredensial menyala hanya bila is_active DAN client punya ≥1 project.
func (s *ClientContactService) SetActive(ctx context.Context, tenantID, id int64, isActive bool) (*domain.ClientContact, error) {
	c, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetActive(ctx, tenantID, id, isActive); err != nil {
		return nil, err
	}
	c.IsActive = isActive
	s.syncAfterChange(ctx, tenantID, c.ClientID)
	return c, nil
}

func (s *ClientContactService) ResetCredential(ctx context.Context, tenantID, id int64, newPassword string) (*domain.ClientContact, error) {
	c, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.identity.ResetPassword(ctx, identitycontracts.PrincipalClient, formatID(c.ID), newPassword); err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.repo.SetCredentialResetAt(ctx, tenantID, id, now); err != nil {
		return nil, err
	}
	c.LastCredentialResetAt = &now
	return c, nil
}

// ReplaceRepresentative menimpa baris Family Representative yang sama dengan
// detail kontak baru — tanpa histori (perilaku lama yang dipertahankan).
func (s *ClientContactService) ReplaceRepresentative(ctx context.Context, tenantID, id int64, input UpdateContactInput, relationNote string) (*domain.ClientContact, error) {
	c, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if c.Role != domain.RoleFamilyRepresentative {
		return nil, apperror.Forbidden("Hanya Family Representative yang dapat diganti")
	}
	c.Name = input.Name
	c.Phone = input.Phone
	c.Email = input.Email
	c.RelationNote = relationNote
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *ClientContactService) requireClient(ctx context.Context, tenantID, clientID int64) (*domain.Client, error) {
	c, err := s.clients.FindByID(ctx, tenantID, clientID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, apperror.Validation("Client tidak valid", map[string][]string{"clientId": {"Client tidak ditemukan"}})
	}
	return c, nil
}

// syncAfterChange memicu hitung ulang login efektif; best-effort — kegagalan
// dicatat, tidak menggagalkan mutasi kontaknya (idiom yang sama dengan
// ClientCleaner hari ini).
func (s *ClientContactService) syncAfterChange(ctx context.Context, tenantID, clientID int64) {
	if s.syncer == nil {
		return
	}
	if err := s.syncer.SyncCredentialActive(ctx, tenantID, clientID); err != nil {
		logger.Error("gagal sinkron akun portal client %d setelah perubahan kontak: %v", clientID, err)
	}
}
