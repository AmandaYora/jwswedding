package elproofpay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"jwswedding/internal/shared/apperror"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func TestClient_TokenDicacheSampaiKedaluwarsa(t *testing.T) {
	var tokenCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/auth/app/token":
			atomic.AddInt32(&tokenCalls, 1)
			writeJSON(w, http.StatusOK, map[string]any{
				"success": true, "message": "ok",
				"data": map[string]any{"accessToken": "tok-1", "tokenType": "Bearer", "expiresIn": 3600},
			})
		default:
			writeJSON(w, http.StatusOK, map[string]any{
				"success": true, "message": "ok",
				"data": map[string]any{"orderRef": "X", "status": "unpaid"},
			})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "app_1", "secret")
	for i := 0; i < 6; i++ {
		if _, err := client.ChargeStatus(context.Background(), "X"); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}

	if got := atomic.LoadInt32(&tokenCalls); got != 1 {
		t.Fatalf("expected exactly 1 POST /auth/app/token across 6 calls, got %d", got)
	}
}

func TestClient_401MemicuTukarTokenUlangSekali(t *testing.T) {
	var tokenCalls int32
	firstToken := "tok-stale"
	secondToken := "tok-fresh"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/app/token" {
			n := atomic.AddInt32(&tokenCalls, 1)
			token := firstToken
			if n > 1 {
				token = secondToken
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"success": true, "message": "ok",
				"data": map[string]any{"accessToken": token, "tokenType": "Bearer", "expiresIn": 3600},
			})
			return
		}
		auth := r.Header.Get("Authorization")
		if auth == "Bearer "+firstToken {
			// No `errors` field at all on this path — PAYMENT_INTEGRATION_GUIDE.md
			// §9 Penting #2.
			writeJSON(w, http.StatusUnauthorized, map[string]any{"success": false, "message": "Token tidak valid atau sudah kedaluwarsa"})
			return
		}
		if auth == "Bearer "+secondToken {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": true, "message": "ok",
				"data": map[string]any{"orderRef": "X", "status": "unpaid"},
			})
			return
		}
		t.Errorf("unexpected Authorization header: %q", auth)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "app_1", "secret")
	result, err := client.ChargeStatus(context.Background(), "X")
	if err != nil {
		t.Fatalf("expected success after one retry, got error: %v", err)
	}
	if result.OrderRef != "X" {
		t.Errorf("expected orderRef X, got %q", result.OrderRef)
	}
	if got := atomic.LoadInt32(&tokenCalls); got != 2 {
		t.Fatalf("expected exactly 2 token exchanges (initial + one retry, no infinite loop), got %d", got)
	}
}

func TestClient_409DipetakanJadiKonflik(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/app/token" {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": true, "message": "ok",
				"data": map[string]any{"accessToken": "tok", "tokenType": "Bearer", "expiresIn": 3600},
			})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]any{
			"success": false, "message": `order_ref "X" sudah pernah dipakai`, "errors": map[string]string{"code": "conflict"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "app_1", "secret")
	_, err := client.CreateCharge(context.Background(), "X", 100000)
	if err == nil {
		t.Fatal("expected an error for a 409 response")
	}
	appErr, ok := apperror.As(err)
	if !ok {
		t.Fatalf("expected an *apperror.AppError, got %T: %v", err, err)
	}
	if appErr.Kind != apperror.KindConflict {
		t.Errorf("expected KindConflict, got %v", appErr.Kind)
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	client := NewClient("https://example.invalid", "app_1", "the-secret")
	body := []byte(`{"orderRef":"INV-1","paid":true,"amount":150000}`)

	mac := hmac.New(sha256.New, []byte("the-secret"))
	mac.Write(body)
	validSig := hex.EncodeToString(mac.Sum(nil))

	if !client.VerifyWebhookSignature(body, validSig) {
		t.Error("expected valid signature to verify")
	}

	tampered := []byte(`{"orderRef":"INV-1","paid":false,"amount":150000}`)
	if client.VerifyWebhookSignature(tampered, validSig) {
		t.Error("expected a 1-byte body change to invalidate the signature")
	}

	if client.VerifyWebhookSignature(body, "") {
		t.Error("expected an empty signature header to fail")
	}

	if client.VerifyWebhookSignature(body, "not-hex-!!") {
		t.Error("expected a malformed (non-hex) signature to fail without panicking")
	}
}

func TestClient_ListPlans_ScopedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/app/token" {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": true, "message": "ok",
				"data": map[string]any{"accessToken": "tok", "tokenType": "Bearer", "expiresIn": 3600},
			})
			return
		}
		if r.URL.Path != "/external/subscription-plans" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true, "message": "ok",
			"data": []map[string]any{
				{"id": 3, "name": "Paket Tahunan", "durationMonths": 12, "price": 2_400_000, "active": true},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "app_1", "secret")
	plans, err := client.ListPlans(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plans) != 1 || plans[0].ID != 3 || plans[0].DurationMonths != 12 || plans[0].Price != 2_400_000 {
		t.Errorf("unexpected plans: %+v", plans)
	}
}

func TestClient_NotFoundDipetakanJadiNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/app/token" {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": true, "message": "ok",
				"data": map[string]any{"accessToken": "tok", "tokenType": "Bearer", "expiresIn": 3600},
			})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]any{
			"success": false, "message": "order_ref tidak ditemukan", "errors": map[string]string{"code": "not_found"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "app_1", "secret")
	_, err := client.ChargeStatus(context.Background(), fmt.Sprintf("ghost-%d", 1))
	appErr, ok := apperror.As(err)
	if !ok || appErr.Kind != apperror.KindNotFound {
		t.Fatalf("expected KindNotFound, got %v", err)
	}
}
