package domain

import "time"

type ClientRole string

const (
	RoleBride                ClientRole = "Bride"
	RoleGroom                ClientRole = "Groom"
	RoleFamilyRepresentative ClientRole = "Family Representative"
)

// ClientContact adalah akun kontak portal per client (dulu `Client` per
// project — D1 me-rename-nya). Id baris TIDAK berubah saat rename (D3):
// akun portal adalah Credential di modul identity dengan PrincipalID = id
// baris ini, jadi id stabil = kredensial tak tersentuh sama sekali.
//
// IsActive berarti "niat admin" (D4), bukan status login efektif — login
// efektif = IsActive DAN client punya ≥1 project, dihitung ulang oleh
// ClientService.SyncCredentialActive.
type ClientContact struct {
	ID                    int64
	TenantID              int64
	ClientID              int64
	Role                  ClientRole
	Username              string
	RelationNote          string
	Name                  string
	Phone                 string
	Email                 string
	IsActive              bool
	LastCredentialResetAt *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
