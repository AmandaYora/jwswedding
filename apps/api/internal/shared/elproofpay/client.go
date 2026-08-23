// Package elproofpay is jwswedding's HTTP client for ElProof's external
// payment API — jwswedding is registered there as a `kind=external` App and
// consumes it to pay for its own subscription (see
// knowledge/decisions/ADR-0021-standalone-jws-payment-via-elproof-api.md and
// docs/integrations/PAYMENT_INTEGRATION_GUIDE.md). One instance is shared by
// `platform` (charges) and `billing` (plan catalog), per D12 — the token
// cache MUST be shared: ElProof rate-limits POST /auth/app/token to 10
// attempts/minute/IP.
package elproofpay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"jwswedding/internal/shared/apperror"
)

// tokenRefreshMargin: a cached token is reused as long as more than this
// much of its lifetime remains — refreshing any later than this risks a
// request landing right as the token expires mid-flight.
const tokenRefreshMargin = 60 * time.Second

// plansCacheTTL bounds how stale ListPlans' cached catalog can be — short
// enough that a plan change at ElProof shows up quickly, long enough to
// spare a live round trip on every single page load.
const plansCacheTTL = 60 * time.Second

type Client struct {
	baseURL    string
	appID      string
	appSecret  string
	httpClient *http.Client

	tokenMu        sync.Mutex
	token          string
	tokenExpiresAt time.Time

	plansMu      sync.RWMutex
	plansCache   []Plan
	plansCacheAt time.Time
}

func NewClient(baseURL, appID, appSecret string) *Client {
	return &Client{
		baseURL:    baseURL,
		appID:      appID,
		appSecret:  appSecret,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// ChargeResult mirrors the response shape shared by charge-creation and
// charge-status (PAYMENT_INTEGRATION_GUIDE.md §5/§7) — provider-agnostic on
// jwswedding's side, exactly like payment/contracts.ChargeResult was for
// ElProof's own internal Apps.
type ChargeResult struct {
	OrderRef    string
	ProviderRef string
	Channel     string
	QRImageURL  string
	PayCode     string
	CheckoutURL string
	Amount      int64
	FeeAmount   int64
	ExpiresAt   time.Time
	Status      string
}

// Plan is one row of ElProof's subscription-plan catalog for this appId
// (PLAN.md §5.2). Features is the plan's feature list, shown on
// SubscriptionPage's plan cards; may be empty.
type Plan struct {
	ID             int64
	Name           string
	DurationMonths int
	Price          int64
	Active         bool
	Features       []string
}

type tokenResponse struct {
	Success bool `json:"success"`
	Data    struct {
		AccessToken string `json:"accessToken"`
		ExpiresIn   int64  `json:"expiresIn"`
	} `json:"data"`
}

type chargeResponse struct {
	Success bool `json:"success"`
	Data    struct {
		OrderRef    string `json:"orderRef"`
		ProviderRef string `json:"providerRef"`
		Channel     string `json:"channel"`
		QRImageURL  string `json:"qrImageUrl"`
		PayCode     string `json:"payCode"`
		CheckoutURL string `json:"checkoutUrl"`
		Amount      int64  `json:"amount"`
		FeeAmount   int64  `json:"feeAmount"`
		ExpiresAt   string `json:"expiresAt"`
		Status      string `json:"status"`
	} `json:"data"`
}

type planListResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		ID             int64    `json:"id"`
		Name           string   `json:"name"`
		DurationMonths int      `json:"durationMonths"`
		Price          int64    `json:"price"`
		Active         bool     `json:"active"`
		Features       []string `json:"features"`
	} `json:"data"`
}

type errorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Errors  *struct {
		Code string `json:"code"`
	} `json:"errors"`
}

// Token returns a valid access token, re-exchanging appId+secret only once
// the cached one has less than tokenRefreshMargin left — see the package
// doc's rate-limit note. Shared by every method below.
func (c *Client) Token(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.token != "" && time.Until(c.tokenExpiresAt) > tokenRefreshMargin {
		return c.token, nil
	}
	return c.exchangeTokenLocked(ctx)
}

// invalidateToken drops the cached token so the next Token() call re-exchanges
// it — used when a downstream call gets 401 despite a "fresh" cached token
// (PAYMENT_INTEGRATION_GUIDE.md §9 Penting #2: expiry is not always exactly
// what expiresIn promised).
func (c *Client) invalidateToken() {
	c.tokenMu.Lock()
	c.token = ""
	c.tokenExpiresAt = time.Time{}
	c.tokenMu.Unlock()
}

func (c *Client) exchangeTokenLocked(ctx context.Context) (string, error) {
	body, err := json.Marshal(map[string]string{"appId": c.appID, "secret": c.appSecret})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/app/token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("elproofpay: gagal menghubungi ElProof untuk token: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", mapErrorResponse(resp.StatusCode, raw)
	}

	var parsed tokenResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("elproofpay: respons token tidak terbaca: %w", err)
	}

	c.token = parsed.Data.AccessToken
	c.tokenExpiresAt = time.Now().Add(time.Duration(parsed.Data.ExpiresIn) * time.Second)
	return c.token, nil
}

// do executes one authenticated request, retrying exactly once with a freshly
// exchanged token if the first attempt comes back 401 — ElProof's token
// expiry (~1h) is otherwise indistinguishable up front from a token that just
// happens to be near the cache's own refresh margin.
func (c *Client) do(ctx context.Context, method, path string, body any) (int, []byte, error) {
	attempt := func() (int, []byte, error) {
		token, err := c.Token(ctx)
		if err != nil {
			return 0, nil, err
		}

		var reader io.Reader
		if body != nil {
			encoded, err := json.Marshal(body)
			if err != nil {
				return 0, nil, err
			}
			reader = bytes.NewReader(encoded)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return 0, nil, fmt.Errorf("elproofpay: gagal menghubungi ElProof: %w", err)
		}
		defer resp.Body.Close()

		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return 0, nil, err
		}
		return resp.StatusCode, raw, nil
	}

	status, raw, err := attempt()
	if err != nil {
		return 0, nil, err
	}
	if status == http.StatusUnauthorized {
		c.invalidateToken()
		status, raw, err = attempt()
		if err != nil {
			return 0, nil, err
		}
	}
	return status, raw, nil
}

// CreateSubscriptionCharge creates a charge for jwswedding's own ElProof
// subscription via the dedicated subscription-charge endpoint — the amount
// is derived by ElProof from planID's own catalog price, never sent by the
// caller (unlike the generic /external/payments/charges endpoint). Customer
// identity is forwarded to ElProof's checkout page (Tripay).
func (c *Client) CreateSubscriptionCharge(ctx context.Context, orderRef string, planID int64, customerName, customerEmail, customerPhone string) (*ChargeResult, error) {
	status, raw, err := c.do(ctx, http.MethodPost, "/external/subscriptions/charges", map[string]any{
		"orderRef":      orderRef,
		"planId":        planID,
		"customerName":  customerName,
		"customerEmail": customerEmail,
		"customerPhone": customerPhone,
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusCreated {
		return nil, mapErrorResponse(status, raw)
	}
	return parseChargeResponse(raw)
}

// ChargeStatus checks a charge's live status at ElProof, scoped to this App's
// own orderRef (a foreign orderRef comes back 404, mapped to
// apperror.NotFound).
func (c *Client) ChargeStatus(ctx context.Context, orderRef string) (*ChargeResult, error) {
	status, raw, err := c.do(ctx, http.MethodGet, "/external/payments/charges/"+orderRef+"/status", nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, mapErrorResponse(status, raw)
	}
	return parseChargeResponse(raw)
}

func parseChargeResponse(raw []byte) (*ChargeResult, error) {
	var parsed chargeResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("elproofpay: respons charge tidak terbaca: %w", err)
	}
	result := &ChargeResult{
		OrderRef:    parsed.Data.OrderRef,
		ProviderRef: parsed.Data.ProviderRef,
		Channel:     parsed.Data.Channel,
		QRImageURL:  parsed.Data.QRImageURL,
		PayCode:     parsed.Data.PayCode,
		CheckoutURL: parsed.Data.CheckoutURL,
		Amount:      parsed.Data.Amount,
		FeeAmount:   parsed.Data.FeeAmount,
		Status:      parsed.Data.Status,
	}
	if parsed.Data.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, parsed.Data.ExpiresAt); err == nil {
			result.ExpiresAt = t
		}
	}
	return result, nil
}

// ListPlans returns ElProof's subscription-plan catalog scoped to this App's
// appId (PLAN.md §5.2), cached for plansCacheTTL — read-through, like
// payment's own channel cache.
func (c *Client) ListPlans(ctx context.Context) ([]Plan, error) {
	c.plansMu.RLock()
	fresh := !c.plansCacheAt.IsZero() && time.Since(c.plansCacheAt) < plansCacheTTL
	cached := c.plansCache
	c.plansMu.RUnlock()
	if fresh {
		return cached, nil
	}

	status, raw, err := c.do(ctx, http.MethodGet, "/external/subscription-plans", nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, mapErrorResponse(status, raw)
	}

	var parsed planListResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("elproofpay: respons daftar paket tidak terbaca: %w", err)
	}
	plans := make([]Plan, 0, len(parsed.Data))
	for _, p := range parsed.Data {
		plans = append(plans, Plan{ID: p.ID, Name: p.Name, DurationMonths: p.DurationMonths, Price: p.Price, Active: p.Active, Features: p.Features})
	}

	c.plansMu.Lock()
	c.plansCache = plans
	c.plansCacheAt = time.Now()
	c.plansMu.Unlock()

	return plans, nil
}

// VerifyWebhookSignature checks the X-Webhook-Signature header ElProof sends
// with its webhook relay (PAYMENT_INTEGRATION_GUIDE.md §8) — HMAC-SHA256 over
// the raw body, keyed with this App's own secret, compared in constant time.
func (c *Client) VerifyWebhookSignature(rawBody []byte, sigHex string) bool {
	mac := hmac.New(sha256.New, []byte(c.appSecret))
	mac.Write(rawBody)
	expected := mac.Sum(nil)

	decoded, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, decoded)
}

// mapErrorResponse maps ElProof's error envelope to the same apperror kinds
// the rest of this codebase already uses, per PAYMENT_INTEGRATION_GUIDE.md
// §9 — including its Penting #2 exception: a 401 from one of the three
// /external/payments/* endpoints (or the plans endpoint, same token layer)
// has no `errors` field at all, so status code alone decides for 401/403/404/409;
// `errors.code` is only consulted to distinguish 400 vs 422 within bad_request.
func mapErrorResponse(status int, raw []byte) error {
	var parsed errorResponse
	_ = json.Unmarshal(raw, &parsed)
	message := parsed.Message
	if message == "" {
		message = fmt.Sprintf("elproofpay: ElProof merespons status %d", status)
	}

	switch status {
	case http.StatusUnauthorized:
		return apperror.Unauthorized(message)
	case http.StatusForbidden:
		return apperror.Forbidden(message)
	case http.StatusNotFound:
		return apperror.NotFound(message)
	case http.StatusConflict:
		return apperror.Conflict(message)
	case http.StatusTooManyRequests:
		return apperror.RateLimited(message)
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return apperror.Validation(message, nil)
	default:
		return apperror.Internal(message)
	}
}
