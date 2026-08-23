package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

func tokenFor(t *testing.T, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// nestedAuthedGate wires RequireAuth outer, RequireActiveSubscription
// inner — the mandatory nesting order documented on
// RequireActiveSubscription and wired this way in main.go.
func nestedAuthedGate(allow func(context.Context, int64) (bool, error), whitelist []string) func(http.Handler) http.Handler {
	gate := RequireActiveSubscription(allow, whitelist)
	return func(h http.Handler) http.Handler {
		return RequireAuth(testSecret)(gate(h))
	}
}

func alwaysOK(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func doRequest(t *testing.T, handler http.Handler, method, path, bearerToken string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestGuard_ExpiryLewatMemblokirTulisStaff(t *testing.T) {
	authed := nestedAuthedGate(func(context.Context, int64) (bool, error) { return false, nil }, nil)
	handler := authed(http.HandlerFunc(alwaysOK))
	token := tokenFor(t, Claims{PrincipalType: "staff", TenantID: "1", Role: "Owner"})

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/projects", token)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"subscription_expired"`) {
		t.Errorf("expected errors.code=subscription_expired, got body: %s", rec.Body.String())
	}
}

// TestGuard_StatusAktifTapiExpiryLewat mengunci T3: guard yang benar
// mengevaluasi waktu (di sini disimulasikan sebagai allow=false, persis
// yang akan dikembalikan WritesAllowed ketika expiry sudah lewat walau
// subscription_status masih 'active' di database), bukan kolom status.
func TestGuard_StatusAktifTapiExpiryLewat(t *testing.T) {
	authed := nestedAuthedGate(func(context.Context, int64) (bool, error) { return false, nil }, nil)
	handler := authed(http.HandlerFunc(alwaysOK))
	token := tokenFor(t, Claims{PrincipalType: "staff", TenantID: "1", Role: "Owner"})

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/projects", token)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 even though status would say active, got %d", rec.Code)
	}
}

func TestGuard_BacaSelaluLolos(t *testing.T) {
	authed := nestedAuthedGate(func(context.Context, int64) (bool, error) { return false, nil }, nil)
	handler := authed(http.HandlerFunc(alwaysOK))
	token := tokenFor(t, Claims{PrincipalType: "staff", TenantID: "1", Role: "Owner"})

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		rec := doRequest(t, handler, method, "/api/v1/projects", token)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: expected 200 even with expired subscription, got %d", method, rec.Code)
		}
	}
}

func TestGuard_JalurBayarDiWhitelist(t *testing.T) {
	whitelist := []string{"/api/v1/subscriptions/pay", "/api/v1/subscriptions/pending-charge/cancel"}
	authed := nestedAuthedGate(func(context.Context, int64) (bool, error) { return false, nil }, whitelist)
	handler := authed(http.HandlerFunc(alwaysOK))
	token := tokenFor(t, Claims{PrincipalType: "staff", TenantID: "1", Role: "Owner"})

	for _, path := range whitelist {
		rec := doRequest(t, handler, http.MethodPost, path, token)
		if rec.Code == http.StatusPaymentRequired {
			t.Errorf("%s: expected whitelist to bypass the guard, got 402", path)
		}
	}
}

func TestGuard_PrincipalClientTidakTerpengaruh(t *testing.T) {
	authed := nestedAuthedGate(func(context.Context, int64) (bool, error) { return false, nil }, nil)
	handler := authed(http.HandlerFunc(alwaysOK))
	token := tokenFor(t, Claims{PrincipalType: "client", TenantID: "1"})

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/projects", token)

	if rec.Code == http.StatusPaymentRequired {
		t.Errorf("expected client principal to bypass the guard entirely, got 402")
	}
}

func TestGuard_ExpiryMasihBerlakuMengizinkanTulis(t *testing.T) {
	authed := nestedAuthedGate(func(context.Context, int64) (bool, error) { return true, nil }, nil)
	handler := authed(http.HandlerFunc(alwaysOK))
	token := tokenFor(t, Claims{PrincipalType: "staff", TenantID: "1", Role: "Owner"})

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/projects", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestGuard_ExpiryNullMemblokir(t *testing.T) {
	// A tenant with subscription_expires_at IS NULL (never paid) must also
	// be blocked — the predicate itself owns that decision; the guard just
	// trusts whatever allow() returns.
	authed := nestedAuthedGate(func(context.Context, int64) (bool, error) { return false, nil }, nil)
	handler := authed(http.HandlerFunc(alwaysOK))
	token := tokenFor(t, Claims{PrincipalType: "staff", TenantID: "1", Role: "Owner"})

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/projects", token)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rec.Code)
	}
}

func TestGuard_KlaimTersediaSaatDievaluasi(t *testing.T) {
	// No token at all -> RequireAuth (the outer wrapper) must reject with
	// 401 before the gate ever runs, never a panic reading nil claims.
	authed := nestedAuthedGate(func(context.Context, int64) (bool, error) {
		t.Fatal("allow() must never be called without a valid token")
		return false, nil
	}, nil)
	handler := authed(http.HandlerFunc(alwaysOK))

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/projects", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from RequireAuth, got %d", rec.Code)
	}
}

func TestGuard_AllowErrorMengembalikan500(t *testing.T) {
	authed := nestedAuthedGate(func(context.Context, int64) (bool, error) { return false, errors.New("db down") }, nil)
	handler := authed(http.HandlerFunc(alwaysOK))
	token := tokenFor(t, Claims{PrincipalType: "staff", TenantID: "1", Role: "Owner"})

	rec := doRequest(t, handler, http.MethodPost, "/api/v1/projects", token)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when the predicate itself errors, got %d", rec.Code)
	}
}
