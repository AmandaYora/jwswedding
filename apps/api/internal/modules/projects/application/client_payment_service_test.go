package application

import (
	"context"
	"errors"
	"testing"

	"jwswedding/internal/modules/projects/domain"
)

// fakeClientPaymentRepo is a minimal in-memory stand-in for
// ClientPaymentRepository — good enough to exercise TotalReceived's control
// flow (PLAN.md redesain-pdf-invoice-kwitansi-v2 §6, T6) without a real
// database. Every method beyond ListByProject is left unimplemented since
// TotalReceived is the only one under test here.
type fakeClientPaymentRepo struct {
	payments []domain.ClientPayment
	err      error
}

func (f *fakeClientPaymentRepo) ListByProject(ctx context.Context, projectID int64) ([]domain.ClientPayment, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.payments, nil
}
func (f *fakeClientPaymentRepo) FindByID(ctx context.Context, projectID, id int64) (*domain.ClientPayment, error) {
	panic("not implemented")
}
func (f *fakeClientPaymentRepo) Create(ctx context.Context, p *domain.ClientPayment) error {
	panic("not implemented")
}
func (f *fakeClientPaymentRepo) Update(ctx context.Context, projectID, id int64, p domain.ClientPayment) error {
	panic("not implemented")
}
func (f *fakeClientPaymentRepo) Delete(ctx context.Context, projectID, id int64) error {
	panic("not implemented")
}
func (f *fakeClientPaymentRepo) NextReceiptSequence(ctx context.Context, tenantID int64, period string) (int, error) {
	panic("not implemented")
}
func (f *fakeClientPaymentRepo) SetReceiptNumber(ctx context.Context, projectID, id int64, number, period string, seq int) error {
	panic("not implemented")
}

func TestTotalReceived_DaftarKosong(t *testing.T) {
	svc := &ClientPaymentService{repo: &fakeClientPaymentRepo{}}
	got, err := svc.TotalReceived(context.Background(), 1)
	if err != nil {
		t.Fatalf("TotalReceived: %v", err)
	}
	if got != 0 {
		t.Errorf("TotalReceived(kosong) = %d, want 0", got)
	}
}

func TestTotalReceived_MenjumlahkanDPdanTermin(t *testing.T) {
	repo := &fakeClientPaymentRepo{payments: []domain.ClientPayment{
		{Type: domain.PaymentDP, Amount: 5_000_000},
		{Type: domain.PaymentTermin, Amount: 10_000_000},
	}}
	svc := &ClientPaymentService{repo: repo}
	got, err := svc.TotalReceived(context.Background(), 1)
	if err != nil {
		t.Fatalf("TotalReceived: %v", err)
	}
	if got != 15_000_000 {
		t.Errorf("TotalReceived(DP+Termin) = %d, want 15000000", got)
	}
}

func TestTotalReceived_RefundDikurangkan(t *testing.T) {
	repo := &fakeClientPaymentRepo{payments: []domain.ClientPayment{
		{Type: domain.PaymentDP, Amount: 5_000_000},
		{Type: domain.PaymentRefund, Amount: 2_000_000},
	}}
	svc := &ClientPaymentService{repo: repo}
	got, err := svc.TotalReceived(context.Background(), 1)
	if err != nil {
		t.Fatalf("TotalReceived: %v", err)
	}
	if got != 3_000_000 {
		t.Errorf("TotalReceived(DP-Refund) = %d, want 3000000", got)
	}
}

func TestTotalReceived_RepositoryError(t *testing.T) {
	wantErr := errors.New("boom")
	svc := &ClientPaymentService{repo: &fakeClientPaymentRepo{err: wantErr}}
	_, err := svc.TotalReceived(context.Background(), 1)
	if !errors.Is(err, wantErr) {
		t.Errorf("TotalReceived error = %v, want %v", err, wantErr)
	}
}
