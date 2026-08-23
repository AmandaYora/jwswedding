package infrastructure

import (
	"context"

	"jwswedding/internal/modules/platform/application"
	"jwswedding/internal/shared/elproofpay"
)

// ElProofChargeClient implements application.ChargeCreator by delegating to
// the shared elproofpay.Client (D12) — a thin adapter, exactly like
// payment's own gateway wrapper, so `platform`'s application layer never
// imports elproofpay's concrete types directly (T4).
type ElProofChargeClient struct {
	client *elproofpay.Client
}

func NewElProofChargeClient(client *elproofpay.Client) *ElProofChargeClient {
	return &ElProofChargeClient{client: client}
}

func (c *ElProofChargeClient) CreateSubscriptionCharge(ctx context.Context, orderRef string, planID int64, customerName, customerEmail, customerPhone string) (*application.ChargeResult, error) {
	result, err := c.client.CreateSubscriptionCharge(ctx, orderRef, planID, customerName, customerEmail, customerPhone)
	if err != nil {
		return nil, err
	}
	return toChargeResult(result), nil
}

func (c *ElProofChargeClient) ChargeStatus(ctx context.Context, orderRef string) (*application.ChargeResult, error) {
	result, err := c.client.ChargeStatus(ctx, orderRef)
	if err != nil {
		return nil, err
	}
	return toChargeResult(result), nil
}

func toChargeResult(c *elproofpay.ChargeResult) *application.ChargeResult {
	return &application.ChargeResult{
		OrderRef: c.OrderRef, ProviderRef: c.ProviderRef, Channel: c.Channel,
		QRImageURL: c.QRImageURL, PayCode: c.PayCode, CheckoutURL: c.CheckoutURL,
		Amount: c.Amount, FeeAmount: c.FeeAmount, ExpiresAt: c.ExpiresAt, Status: c.Status,
	}
}
