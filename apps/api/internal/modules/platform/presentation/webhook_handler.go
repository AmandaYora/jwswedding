package presentation

import (
	"encoding/json"
	"io"
	"net/http"

	"jwswedding/internal/modules/platform/application"
	"jwswedding/internal/shared/elproofpay"
	"jwswedding/internal/shared/logger"
	"jwswedding/internal/shared/response"
)

// WebhookHandler receives ElProof's fire-and-forget payment webhook relay
// (PAYMENT_INTEGRATION_GUIDE.md §8) at POST /webhooks/elproof-payment —
// registered in RegisterPublicRoutes, never wrapped in `authed`: trust comes
// entirely from the X-Webhook-Signature header, verified against this App's
// own secret.
type WebhookHandler struct {
	verifier *elproofpay.Client
	tenants  *application.TenantService
}

func NewWebhookHandler(verifier *elproofpay.Client, tenants *application.TenantService) *WebhookHandler {
	return &WebhookHandler{verifier: verifier, tenants: tenants}
}

type webhookPayload struct {
	OrderRef string `json:"orderRef"`
	Paid     bool   `json:"paid"`
	Amount   int64  `json:"amount"`
	PaidAt   string `json:"paidAt,omitempty"`
}

// Receive reads the raw body BEFORE parsing JSON — the signature is
// computed over the exact bytes ElProof sent, not a re-marshaled copy,
// which can differ in key order/whitespace (PAYMENT_INTEGRATION_GUIDE.md
// §8). An invalid signature is rejected before any parsing happens at all.
func (h *WebhookHandler) Receive(w http.ResponseWriter, r *http.Request) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Gagal membaca body permintaan", nil)
		return
	}

	signature := r.Header.Get("X-Webhook-Signature")
	if !h.verifier.VerifyWebhookSignature(rawBody, signature) {
		response.Error(w, http.StatusUnauthorized, "Signature webhook tidak valid", nil)
		return
	}

	var payload webhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		response.Error(w, http.StatusBadRequest, "Body permintaan tidak valid", nil)
		return
	}

	if err := h.tenants.ApplyWebhookEvent(r.Context(), payload.OrderRef, application.WebhookEvent{Paid: payload.Paid}); err != nil {
		logger.Error("platform: gagal memproses webhook order_ref %q: %v", payload.OrderRef, err)
		response.Error(w, http.StatusInternalServerError, "Gagal memproses webhook", nil)
		return
	}

	response.OK(w, "ok", nil)
}
