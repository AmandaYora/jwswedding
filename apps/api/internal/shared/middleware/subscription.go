package middleware

import (
	"context"
	"net/http"

	"jwswedding/internal/shared/logger"
	"jwswedding/internal/shared/response"
)

// RequireActiveSubscription is a technical utility (like RequireAuth) — it
// has no domain logic of its own, only a predicate injected by the caller
// (main.go, wired to platform/contracts.Contracts.WritesAllowed) — see
// .claude/rules/backend-modular-monolith.md: shared/ never holds domain
// logic itself.
//
// Deny-by-default (D5): every non-GET request from a `staff` principal is
// rejected unless allow() says otherwise or the path is whitelisted — a new
// route is automatically protected, not automatically exempt. Must be
// nested INSIDE RequireAuth (RequireAuth wraps this, not the reverse) so
// Claims already exist in the request context by the time this runs — see
// main.go's wiring.
func RequireActiveSubscription(allow func(context.Context, int64) (bool, error), whitelist []string) func(http.Handler) http.Handler {
	whitelisted := make(map[string]bool, len(whitelist))
	for _, path := range whitelist {
		whitelisted[path] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			claims, ok := FromContext(r.Context())
			if !ok || claims.PrincipalType != "staff" {
				// D13: this guard only ever applies to the `staff` principal —
				// Client Portal is already read-only globally elsewhere, and a
				// missing claim here means RequireAuth (which must wrap this)
				// already rejected the request before this handler ever ran.
				next.ServeHTTP(w, r)
				return
			}

			if whitelisted[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			tenantID, ok := claims.TenantIDInt()
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			allowed, err := allow(r.Context(), tenantID)
			if err != nil {
				logger.Error("subscription guard: gagal memeriksa status langganan tenant %d: %v", tenantID, err)
				response.Error(w, http.StatusInternalServerError, "Gagal memeriksa status langganan", nil)
				return
			}
			if !allowed {
				response.Error(w, http.StatusPaymentRequired, "Langganan sudah kedaluwarsa, selesaikan pembayaran untuk melanjutkan", map[string]string{"code": "subscription_expired"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
