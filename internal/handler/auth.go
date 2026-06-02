package handler

import (
	"database/sql"
	"html/template"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"
	pam "github.com/msteinert/pam/v2"

	"github.com/tomasweigenast/srouter/internal/config"
	"github.com/tomasweigenast/srouter/internal/logging"
	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/web"
)

var authLogger = logging.GetLogger("auth")

type AuthHandler struct {
	db        *sql.DB
	devMode   bool
	loginTmpl *template.Template
}

func NewAuthHandler(i do.Injector) (*AuthHandler, error) {
	cfg := do.MustInvoke[config.Config](i)
	db  := do.MustInvoke[*sql.DB](i)
	return &AuthHandler{
		db:        db,
		devMode:   cfg.DevMode,
		loginTmpl: web.MustParseStandalone("login"),
	}, nil
}

func (h *AuthHandler) Register(r chi.Router) {
	r.Get("/login", h.showLogin)
	r.Post("/login", h.handleLogin)
	r.Post("/logout", h.handleLogout)
}

type loginPage struct {
	Error string
}

func (h *AuthHandler) showLogin(w http.ResponseWriter, r *http.Request) {
	web.Render(w, h.loginTmpl, loginPage{})
}

func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	if h.devMode {
		if username == "" {
			web.Render(w, h.loginTmpl, loginPage{Error: "Username required."})
			return
		}
		authLogger.Info("[dev] login bypassed", "username", username)
	} else {
		if err := pamAuthenticate(username, password); err != nil {
			authLogger.Warn("login failed", "username", username, "err", err)
			web.Render(w, h.loginTmpl, loginPage{Error: "Invalid username or password."})
			return
		}
	}

	sess, err := session.Create(h.db, username)
	if err != nil {
		authLogger.Error("create session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     session.CookieName,
		Value:    sess.ID,
		Expires:  sess.ExpiresAt,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	authLogger.Info("login", "username", username)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *AuthHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(session.CookieName)
	if err == nil {
		_ = session.Delete(h.db, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:    session.CookieName,
		Value:   "",
		Expires: time.Unix(0, 0),
		Path:    "/",
		MaxAge:  -1,
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func pamAuthenticate(username, password string) error {
	t, err := pam.StartFunc("login", username, func(s pam.Style, msg string) (string, error) {
		if s == pam.PromptEchoOff {
			return password, nil
		}
		return "", nil
	})
	if err != nil {
		return err
	}
	if err := t.Authenticate(0); err != nil {
		return err
	}
	return t.AcctMgmt(0)
}
