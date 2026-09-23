// Package contracts is the ONLY package other modules may import from staff.
package contracts

import (
	"context"
	"strconv"

	"jwswedding/internal/modules/staff/application"
	"jwswedding/internal/shared/apperror"
)

type CreateOwnerInput struct {
	TenantID int64
	Name     string
	Email    string
	Phone    string
	Username string
}

type CreateOwnerResult struct {
	StaffID string // stringified — identity.principal_id is a primitive VARCHAR
}

// Summary is a staff member's public-safe {id, name} projection — used by
// `projects` to embed a project's PIC name directly in its own response
// (PLAN.md's Client Portal restructure), so a client principal never needs
// to reach the staff-only `/staff/summary` endpoint just to see who their
// WO contact is.
type Summary struct {
	ID   int64
	Name string
}

// Signer adalah pengesah satu dokumen: nama + jabatan yang dicetak di bawah
// garis tanda tangan, beserta gambar TTD-nya (PLAN tanda-tangan-pengguna).
//
// SignatureImage boleh nil walau Signer-nya ada — pengguna yang belum mengisi
// TTD tetap tercetak namanya, dengan slot gambar dibiarkan kosong untuk tanda
// tangan basah (K4).
type Signer struct {
	ID                   int64
	Name                 string
	Title                string
	SignatureImage       []byte
	SignatureContentType string
}

type Contracts interface {
	CreateOwner(ctx context.Context, input CreateOwnerInput) (CreateOwnerResult, error)
	// GetSummary resolves one staff member by ID. Returns (nil, nil) — not an
	// error — when the ID doesn't resolve (e.g. the staff row was
	// hard-deleted, PLAN.md's Staff hard delete), so a stale pic_staff_id
	// degrades to "no name available" rather than breaking the whole
	// response it's embedded in.
	GetSummary(ctx context.Context, tenantID, staffID int64) (*Summary, error)
	// GetSigner resolves the staff member who authorised a document, for the
	// signature block of Penawaran/PO, Invoice, and Kwitansi PDFs.
	//
	// Returns (nil, nil) — not an error — when staffID doesn't resolve: the 0
	// sentinel on pre-existing rows, a hard-deleted staff row, or an ID from
	// another tenant. The PDF then prints the block with neither name nor
	// image, leaving room for a wet signature (PLAN tanda-tangan-pengguna K6).
	//
	// Deliberately does NOT filter on is_active: a deactivated staff member
	// still legitimately authorised the documents they issued.
	GetSigner(ctx context.Context, tenantID, staffID int64) (*Signer, error)
}

type impl struct {
	service    *application.StaffService
	signatures *application.StaffSignatureService
}

func New(service *application.StaffService, signatures *application.StaffSignatureService) Contracts {
	return &impl{service: service, signatures: signatures}
}

func (c *impl) CreateOwner(ctx context.Context, input CreateOwnerInput) (CreateOwnerResult, error) {
	member, err := c.service.CreateOwner(ctx, input.TenantID, input.Name, input.Email, input.Phone, input.Username)
	if err != nil {
		return CreateOwnerResult{}, err
	}
	return CreateOwnerResult{StaffID: strconv.FormatInt(member.ID, 10)}, nil
}

func (c *impl) GetSummary(ctx context.Context, tenantID, staffID int64) (*Summary, error) {
	member, err := c.service.Get(ctx, tenantID, staffID)
	if err != nil {
		if appErr, ok := apperror.As(err); ok && appErr.Kind == apperror.KindNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &Summary{ID: member.ID, Name: member.Name}, nil
}

// GetSigner merangkai dua service: StaffService.Get untuk nama + jabatan
// (mengikuti GetSummary di atas persis), dan StaffSignatureService.SignatureImage
// untuk gambarnya. Yang kedua tidak pernah menggagalkan yang pertama — TTD yang
// belum ada atau objek yang gagal dibaca hanya membuat SignatureImage nil.
func (c *impl) GetSigner(ctx context.Context, tenantID, staffID int64) (*Signer, error) {
	member, err := c.service.Get(ctx, tenantID, staffID)
	if err != nil {
		if appErr, ok := apperror.As(err); ok && appErr.Kind == apperror.KindNotFound {
			return nil, nil
		}
		return nil, err
	}
	signer := &Signer{ID: member.ID, Name: member.Name, Title: member.Title}
	img, contentType, ok, err := c.signatures.SignatureImage(ctx, tenantID, staffID)
	if err != nil {
		return nil, err
	}
	if ok {
		signer.SignatureImage = img
		signer.SignatureContentType = contentType
	}
	return signer, nil
}
