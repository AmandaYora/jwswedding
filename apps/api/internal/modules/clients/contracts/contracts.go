// Package contracts is the ONLY package other modules may import from
// clients (PLAN penawaran-client-master, T1.9).
//
// Setelah Fase 1, `clients` tidak memegang informasi project apa pun —
// ProjectIDForClient mati bersama client_contacts.project_id. Modul ini
// menjawab dari tabel miliknya sendiri; relasi client<->project ditanyakan
// lewat projects (ProjectDirectory), bukan sebaliknya.
package contracts

import (
	"context"

	"jwswedding/internal/modules/clients/application"
	"jwswedding/internal/modules/clients/domain"
	"jwswedding/internal/shared/apperror"
)

// ClientActivator adalah port yang dipakai `projects` (D4): tiap kelahiran /
// penghapusan project menghitung ulang login efektif akun portal client itu.
type ClientActivator interface {
	SyncCredentialActive(ctx context.Context, tenantID, clientID int64) error
}

// ClientDirectory adalah port baca yang dipakai `quotations` (kop PO, daftar
// Penawaran, dan TTD Penawaran): nama pasangan + telepon dari client_id,
// tanpa join lintas modul. Batch memakai map[int64][2]string ({bride, groom})
// — tipe primitif supaya tidak ada impor tipe lintas modul dua arah. Aturan
// yang sama berlaku untuk method specimen: peran dan sumber sebagai string,
// gambar sebagai bytes — jangan bocorkan tipe domain.
type ClientDirectory interface {
	CoupleNames(ctx context.Context, tenantID, clientID int64) (bride, groom string, err error)
	CoupleNamesBatch(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64][2]string, error)
	PhoneForClient(ctx context.Context, tenantID, clientID int64) (string, error)
	// SaveSpecimen menimpa specimen milik client (D6b/D12). role dan source
	// string ("Bride"/"Groom"/"Family Representative", "draw"/"upload").
	SaveSpecimen(ctx context.Context, tenantID, clientID int64, role, signerName string, img []byte, mimeType, source string) error
	// SpecimenImage mengembalikan bytes gambar specimen untuk dipakai ulang.
	SpecimenImage(ctx context.Context, tenantID, clientID int64) ([]byte, error)
	// SpecimenMeta mengembalikan keterangan specimen sebagai
	// {role, signerName, source, updatedAt} + ada/tidak.
	SpecimenMeta(ctx context.Context, tenantID, clientID int64) ([4]string, bool, error)
	// SignerOptions mengembalikan opsi Atas Nama sebagai pasangan
	// {role, name} — dari clients, bukan client_contacts (T1).
	SignerOptions(ctx context.Context, tenantID, clientID int64) ([][2]string, error)
}

type Contracts interface {
	ClientActivator
	ClientDirectory
	// ClientIDForContact memetakan token portal (id kontak) ke master
	// client-nya — dipakai resolver akses portal di `projects` (T1.10).
	// Error = tolak akses; akun nonaktif ditolak di sini (D4).
	ClientIDForContact(ctx context.Context, tenantID, contactID int64) (int64, error)
}

type impl struct {
	clients    *application.ClientService
	signatures *application.ClientSignatureService
}

func New(clients *application.ClientService, signatures *application.ClientSignatureService) Contracts {
	return &impl{clients: clients, signatures: signatures}
}

func (c *impl) SyncCredentialActive(ctx context.Context, tenantID, clientID int64) error {
	return c.clients.SyncCredentialActive(ctx, tenantID, clientID)
}

func (c *impl) CoupleNames(ctx context.Context, tenantID, clientID int64) (string, string, error) {
	return c.clients.CoupleNames(ctx, tenantID, clientID)
}

func (c *impl) CoupleNamesBatch(ctx context.Context, tenantID int64, clientIDs []int64) (map[int64][2]string, error) {
	return c.clients.CoupleNamesBatch(ctx, tenantID, clientIDs)
}

func (c *impl) PhoneForClient(ctx context.Context, tenantID, clientID int64) (string, error) {
	return c.clients.PhoneForClient(ctx, tenantID, clientID)
}

func (c *impl) SaveSpecimen(ctx context.Context, tenantID, clientID int64, role, signerName string, img []byte, mimeType, source string) error {
	_, err := c.signatures.SaveSpecimen(ctx, tenantID, clientID,
		domain.ClientRole(role), signerName, img, mimeType, domain.ClientSignatureSource(source))
	return err
}

func (c *impl) SpecimenImage(ctx context.Context, tenantID, clientID int64) ([]byte, error) {
	return c.signatures.SpecimenImage(ctx, tenantID, clientID)
}

func (c *impl) SpecimenMeta(ctx context.Context, tenantID, clientID int64) ([4]string, bool, error) {
	spec, err := c.signatures.Specimen(ctx, tenantID, clientID)
	if err != nil {
		return [4]string{}, false, err
	}
	if spec == nil {
		return [4]string{}, false, nil
	}
	return [4]string{string(spec.Role), spec.SignerName, string(spec.Source), spec.UpdatedAt.Format("2006-01-02")}, true, nil
}

func (c *impl) SignerOptions(ctx context.Context, tenantID, clientID int64) ([][2]string, error) {
	options, err := c.signatures.SignerOptions(ctx, tenantID, clientID)
	if err != nil {
		return nil, err
	}
	out := make([][2]string, 0, len(options))
	for _, o := range options {
		out = append(out, [2]string{string(o.Role), o.Name})
	}
	return out, nil
}

func (c *impl) ClientIDForContact(ctx context.Context, tenantID, contactID int64) (int64, error) {
	id, err := c.clients.ClientIDForContact(ctx, tenantID, contactID)
	if err != nil {
		if appErr, ok := apperror.As(err); ok && appErr.Kind == apperror.KindNotFound {
			return 0, apperror.Forbidden("Akses ditolak")
		}
		return 0, err
	}
	return id, nil
}
