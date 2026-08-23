package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"jwswedding/internal/modules/projects/domain"
	"jwswedding/internal/shared/apperror"
)

type ClientInvoiceRepository interface {
	ListByProject(ctx context.Context, projectID int64) ([]domain.ClientInvoice, error)
	FindByID(ctx context.Context, projectID, id int64) (*domain.ClientInvoice, error)
	// FindByClientPaymentID supports 2 callers: reverse-looking-up the
	// Invoice number when printing a Kwitansi, and guarding delete/edit of
	// the ClientPayment it's tracking (see ClientInvoiceService.FindByPaymentID).
	FindByClientPaymentID(ctx context.Context, projectID, clientPaymentID int64) (*domain.ClientInvoice, error)
	Create(ctx context.Context, inv *domain.ClientInvoice) error
	Update(ctx context.Context, projectID, id int64, inv domain.ClientInvoice) error
	Delete(ctx context.Context, projectID, id int64) error
	NextInvoiceSequence(ctx context.Context, tenantID int64, period string) (int, error)
}

type ClientInvoiceService struct {
	repo     ClientInvoiceRepository
	payments *ClientPaymentService
	activity *ActivityService
}

// NewClientInvoiceService depends on ClientPaymentService (MarkPaid creates a
// ClientPayment, UnmarkPaid deletes one) — one-way only, so there's no
// construction cycle between the two services.
func NewClientInvoiceService(repo ClientInvoiceRepository, payments *ClientPaymentService, activity *ActivityService) *ClientInvoiceService {
	return &ClientInvoiceService{repo: repo, payments: payments, activity: activity}
}

func (s *ClientInvoiceService) List(ctx context.Context, projectID int64) ([]domain.ClientInvoice, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *ClientInvoiceService) Get(ctx context.Context, projectID, id int64) (*domain.ClientInvoice, error) {
	inv, err := s.repo.FindByID(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, apperror.NotFound("Tagihan tidak ditemukan")
	}
	return inv, nil
}

// FindByPaymentID resolves the Invoice (if any) that produced a given
// ClientPayment via MarkPaid — nil, nil when the payment was recorded
// manually, never through an Invoice.
func (s *ClientInvoiceService) FindByPaymentID(ctx context.Context, projectID, clientPaymentID int64) (*domain.ClientInvoice, error) {
	return s.repo.FindByClientPaymentID(ctx, projectID, clientPaymentID)
}

type ClientInvoiceInput struct {
	Type        domain.PaymentType
	Description string
	Amount      int64
	DueDate     time.Time
}

// Create issues a new Tagihan in Draft status, numbered INV/{YYYYMM}/{seq}
// scoped per tenant+period (PLAN.md invoice-kwitansi-client §1.8/§4.4). On a
// UNIQUE-key collision (two concurrent creates computing the same seq) it
// recomputes the sequence and retries exactly once before giving up.
func (s *ClientInvoiceService) Create(ctx context.Context, tenantID, projectID, actorStaffID int64, input ClientInvoiceInput) (*domain.ClientInvoice, error) {
	if input.Type == domain.PaymentRefund {
		return nil, apperror.Validation("Jenis Refund tidak berlaku untuk Tagihan", map[string][]string{
			"type": {"Tagihan tidak dapat berjenis Refund"},
		})
	}

	period := time.Now().Format("200601")
	for attempt := 0; attempt < 2; attempt++ {
		seq, err := s.repo.NextInvoiceSequence(ctx, tenantID, period)
		if err != nil {
			return nil, err
		}
		inv := &domain.ClientInvoice{
			ProjectID: projectID, InvoiceNumber: fmt.Sprintf("INV/%s/%04d", period, seq),
			NumberPeriod: period, NumberSeq: seq, Type: input.Type, Description: input.Description,
			Amount: input.Amount, DueDate: input.DueDate, Status: domain.InvoiceDraft, CreatedByStaffID: actorStaffID,
		}
		err = s.repo.Create(ctx, inv)
		if err == nil {
			s.activity.Record(ctx, &projectID, domain.ActivityInvoiceCreated, actorStaffID, "client_invoice", formatID(inv.ID), inv.InvoiceNumber,
				"Tagihan dibuat: "+inv.InvoiceNumber)
			return inv, nil
		}
		if !errors.Is(err, domain.ErrDuplicateInvoiceNumber) {
			return nil, err
		}
	}
	return nil, apperror.Validation("Nomor invoice sedang bentrok, silakan coba lagi", nil)
}

// Update overwrites type/description/amount/dueDate — rejected once the
// Invoice is Lunas (its nominal must match the ClientPayment MarkPaid
// created; use "Batalkan Pelunasan" first). status is never set through this
// path: "Lunas" only ever happens via MarkPaid (which also fills
// ClientPaymentID), so callers pass the desired status separately and this
// method rejects InvoicePaid outright.
func (s *ClientInvoiceService) Update(ctx context.Context, projectID, id, actorStaffID int64, input ClientInvoiceInput, status domain.InvoiceStatus) (*domain.ClientInvoice, error) {
	inv, err := s.Get(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	if inv.Status == domain.InvoicePaid {
		return nil, apperror.Validation("Tagihan yang sudah Lunas tidak dapat diubah — batalkan pelunasannya terlebih dahulu", nil)
	}
	if status == domain.InvoicePaid {
		return nil, apperror.Validation("Status Lunas hanya dapat diset lewat aksi Tandai Lunas", map[string][]string{
			"status": {"Gunakan tombol \"Tandai Lunas\""},
		})
	}
	if input.Type == domain.PaymentRefund {
		return nil, apperror.Validation("Jenis Refund tidak berlaku untuk Tagihan", map[string][]string{
			"type": {"Tagihan tidak dapat berjenis Refund"},
		})
	}

	inv.Type, inv.Description, inv.Amount, inv.DueDate, inv.Status = input.Type, input.Description, input.Amount, input.DueDate, status
	if err := s.repo.Update(ctx, projectID, id, *inv); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityInvoiceUpdated, actorStaffID, "client_invoice", formatID(inv.ID), inv.InvoiceNumber,
		"Tagihan diperbarui: "+inv.InvoiceNumber)
	return inv, nil
}

// MarkPaid creates the linked ClientPayment and flips status to Lunas.
// paymentInput.Type/Amount from the caller are IGNORED — forced to the
// Invoice's own Type/Amount, since the Invoice is the source of truth for
// what was billed (PLAN.md §4.2). The returned ClientInvoice carries
// ClientPaymentID so the frontend can attach a transfer-proof evidence
// against it (§4.7).
func (s *ClientInvoiceService) MarkPaid(ctx context.Context, projectID, id, actorStaffID int64, paymentInput ClientPaymentInput) (*domain.ClientInvoice, error) {
	inv, err := s.Get(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	if inv.Status == domain.InvoicePaid || inv.Status == domain.InvoiceCancelled {
		return nil, apperror.Validation("Tagihan berstatus "+string(inv.Status)+" tidak dapat ditandai Lunas lagi", nil)
	}

	paymentInput.Type = inv.Type
	paymentInput.Amount = inv.Amount
	payment, err := s.payments.Create(ctx, projectID, actorStaffID, paymentInput)
	if err != nil {
		return nil, err
	}

	inv.ClientPaymentID = payment.ID
	inv.Status = domain.InvoicePaid
	if err := s.repo.Update(ctx, projectID, id, *inv); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityInvoiceMarkedPaid, actorStaffID, "client_invoice", formatID(inv.ID), inv.InvoiceNumber,
		"Tagihan ditandai Lunas: "+inv.InvoiceNumber)
	return inv, nil
}

// UnmarkPaid reverses MarkPaid ("Batalkan Pelunasan", PLAN.md §1.9): deletes
// the linked ClientPayment (and, via ClientPaymentService.Delete, its
// evidence) and returns the Invoice to Terkirim. Idempotent against the
// payment already being gone (e.g. a retried request) so the Invoice is
// never stuck at Lunas pointing at nothing.
func (s *ClientInvoiceService) UnmarkPaid(ctx context.Context, projectID, id, actorStaffID int64) (*domain.ClientInvoice, error) {
	inv, err := s.Get(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	if inv.Status != domain.InvoicePaid {
		return nil, apperror.Validation("Hanya Tagihan berstatus Lunas yang dapat dibatalkan pelunasannya", nil)
	}

	if inv.ClientPaymentID != 0 {
		if err := s.payments.Delete(ctx, projectID, inv.ClientPaymentID, actorStaffID); err != nil {
			if appErr, ok := apperror.As(err); !ok || appErr.Kind != apperror.KindNotFound {
				return nil, err
			}
			// Payment already gone -- proceed anyway so this stays idempotent.
		}
	}

	inv.ClientPaymentID = 0
	inv.Status = domain.InvoiceSent
	if err := s.repo.Update(ctx, projectID, id, *inv); err != nil {
		return nil, err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityInvoiceUnmarkedPaid, actorStaffID, "client_invoice", formatID(inv.ID), inv.InvoiceNumber,
		"Pelunasan Tagihan dibatalkan: "+inv.InvoiceNumber)
	return inv, nil
}

// Delete hard-deletes a Tagihan — rejected while Lunas (use "Batalkan
// Pelunasan" first, which is always available now, so this never deadlocks).
func (s *ClientInvoiceService) Delete(ctx context.Context, projectID, id, actorStaffID int64) error {
	inv, err := s.Get(ctx, projectID, id)
	if err != nil {
		return err
	}
	if inv.Status == domain.InvoicePaid {
		return apperror.Validation("Batalkan pelunasannya terlebih dahulu sebelum menghapus Tagihan ini", nil)
	}
	if err := s.repo.Delete(ctx, projectID, id); err != nil {
		return err
	}
	s.activity.Record(ctx, &projectID, domain.ActivityInvoiceDeleted, actorStaffID, "client_invoice", formatID(inv.ID), inv.InvoiceNumber,
		"Tagihan dihapus: "+inv.InvoiceNumber)
	return nil
}
