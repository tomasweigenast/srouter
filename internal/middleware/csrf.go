package middleware

import (
	"net/http"
)

// RequireLocalOrigin rejects state-changing requests (POST, PUT, DELETE, PATCH)
// whose Origin header is present and does not match the request's own Host.
//
// Browsers always include Origin on cross-origin requests. Combined with the
// existing SameSite=Strict session cookie (which blocks credential forwarding
// from other origins), this provides defense-in-depth against CSRF without
// requiring token-based form fields or a new dependency.
//
// Requests without an Origin header (some same-origin form submissions, curl,
// direct API calls from the LAN) pass through unchanged.
func RequireLocalOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
			origin := r.Header.Get("Origin")
			if origin != "" && origin != "null" {
				scheme := "http"
				if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
					scheme = "https"
				}
				if origin != scheme+"://"+r.Host {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
