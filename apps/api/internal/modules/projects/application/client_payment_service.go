package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/apperror"
)

type ClientPaymentRepository interface {
	ListByProject(ctx context.Context, projectID int64) ([]domain.ClientPayment, error)
	FindByID(ctx context.Context, projectID, id int64) (*domain.ClientPayment, error)
	Create(ctx context.Context, p *domain.ClientPayment) error
	Update(ctx context.Context, projectID, id int64, p domain.ClientPayment) error
	Delete(ctx context.Context, projectID, id int64) error
	// NextReceiptSequence/SetReceiptNumber back EnsureReceiptNumber's lazy
	// Kwitansi numbering (PLAN.md invoice-kwitansi-client §1.5/§1.8).
	NextReceiptSequence(ctx context.Context, tenantID int64, period string) (int, error)
	SetReceiptNumber(ctx context.Context, projectID, id int64, number, period string, seq int) error
}

type ClientPaymentService struct {
	repo     ClientPaymentRepository
	evidence *EvidenceService
	activity *ActivityService
}

func NewClientPaymentService(repo ClientPaymentRepository, evidence *EvidenceService, activity *ActivityService) *ClientPaymentService {
	return &ClientPaymentService{repo: repo, evidence: evidence, activity: activity}
}

func (s *ClientPaymentService) List(ctx context.Context, projectID int64) ([]domain.ClientPayment, error) {
	return s.repo.ListByProject(ctx, projectID)
}

type ClientPaymentInput struct {
	Type            domain.PaymentType
	Amount          int64
	PaymentDate     time.Time
	Method          string
	ReferenceNumber string
	Notes           string
}

// Create records a client payment and logs it under the same
// ActivityPaymentRecorded type PaymentService.Create already uses for
// vendor payments (mirrors ActivityMilestoneUpdated's own reuse across
// Project Milestones and Vendor Milestones) — the two are distinguished
// only by entityType/description, not a new ActivityType value.
func (s *ClientPaymentService) Create(ctx context.Context, projectID int64, actorStaffID int64, input ClientPaymentInput) (*domain.ClientPayment, error) {
	p := &domain.ClientPayment{
		ProjectID: projectID, Type: input.Type, Amount: input.Amount,
		PaymentDate: input.PaymentDate, Method: input.Method, ReferenceNumber: input.ReferenceNumber, Notes: input.Notes,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityPaymentRecorded, actorStaffID, "client_payment", formatID(p.ID), string(p.Type),
		"Pembayaran client dicatat")
	return p, nil
}

// Update is an unconditional full-field overwrite -- see PaymentService.Update.
func (s *ClientPaymentService) Update(ctx context.Context, projectID, id int64, actorStaffID int64, input ClientPaymentInput) (*domain.ClientPayment, error) {
	p, err := s.repo.FindByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperror.NotFound("Pembayaran tidak ditemukan")
	}
	p.Type, p.Amount, p.PaymentDate, p.Method, p.ReferenceNumber, p.Notes =
		input.Type, input.Amount, input.PaymentDate, input.Method, input.ReferenceNumber, input.Notes
	if err := s.repo.Update(ctx, projectID, id, *p); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityPaymentUpdated, actorStaffID, "client_payment", formatID(p.ID), string(p.Type),
		"Pembayaran client diperbarui")
	return p, nil
}

// Delete hard-deletes the payment and, first, its attached evidence -- see
// PaymentService.Delete for the full reasoning (evidence-first ordering,
// Owner-only role check living in the HTTP handler, not here).
func (s *ClientPaymentService) Delete(ctx context.Context, projectID, id int64, actorStaffID int64) error {
	p, err := s.repo.FindByID(ctx, projectID, id)
	if err != nil {
		return err
	}
	if p == nil {
		return apperror.NotFound("Pembayaran tidak ditemukan")
	}
	if err := s.evidence.DeleteForRelated(ctx, domain.RelatedClientPayment, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, projectID, id); err != nil {
		return err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityPaymentDeleted, actorStaffID, "client_payment", formatID(p.ID), string(p.Type),
		"Pembayaran client dihapus")
	return nil
}

// EnsureReceiptNumber lazily assigns a permanent Kwitansi number the first
// time anyone (WO staff or the Client Portal) prints this payment's receipt
// — idempotent (a payment that already has a number is returned unchanged),
// and rejects Refund outright since "Telah terima dari <client>" would be
// semantically backwards for money going the other way (PLAN.md
// invoice-kwitansi-client §1.11). Period comes from PaymentDate, not the
// print date, so a payment made in August but printed in September is still
// numbered KWT/202608/... (§4.3/§4.4).
func (s *ClientPaymentService) EnsureReceiptNumber(ctx context.Context, tenantID, projectID, id int64) (*domain.ClientPayment, error) {
	p, err := s.repo.FindByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperror.NotFound("Pembayaran tidak ditemukan")
	}
	if p.Type == domain.PaymentRefund {
		return nil, apperror.Validation("Kwitansi tidak tersedia untuk pembayaran jenis Refund", nil)
	}
	if p.ReceiptNumber != "" {
		return p, nil
	}

	period := p.PaymentDate.Format("200601")
	seq, err := s.repo.NextReceiptSequence(ctx, tenantID, period)
	if err != nil {
		return nil, err
	}
	number := fmt.Sprintf("KWT/%s/%04d", period, seq)
	if err := s.repo.SetReceiptNumber(ctx, projectID, id, number, period, seq); err != nil {
		if !errors.Is(err, domain.ErrDuplicateReceiptNumber) {
			return nil, err
		}
		// Two concurrent prints raced on the same seq -- re-read: whoever won
		// already stamped a number, so the caller still gets a valid PDF
		// instead of a bare error.
		reread, rereadErr := s.repo.FindByID(ctx, projectID, id)
		if rereadErr != nil {
			return nil, rereadErr
		}
		if reread != nil && reread.ReceiptNumber != "" {
			return reread, nil
		}
		return nil, apperror.Validation("Nomor kwitansi sedang bentrok, silakan coba lagi", nil)
	}

	p.ReceiptNumber, p.ReceiptPeriod, p.ReceiptSeq = number, period, seq
	return p, nil
}
