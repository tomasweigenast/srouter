package middleware

import (
	"database/sql"
	"net/http"

	"github.com/tomasweigenast/srouter/internal/session"
)

func RequireAuth(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(session.CookieName)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			sess, ok, err := session.Get(db, cookie.Value)
			if err != nil || !ok {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r.WithContext(session.NewContext(r.Context(), sess)))
		})
	}
}
