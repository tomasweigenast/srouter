package handler

import (
	"context"
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

var timeRE = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

type SystemHandler struct {
	db      *sql.DB
	updater *system.UpdateChecker
	tmpl    *template.Template
}

func NewSystemHandler(i do.Injector) (*SystemHandler, error) {
	return &SystemHandler{
		db:      do.MustInvoke[*sql.DB](i),
		updater: do.MustInvoke[*system.UpdateChecker](i),
		tmpl:    web.MustParsePage("system"),
	}, nil
}

func (h *SystemHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Post("/reboot", h.reboot)
	r.Post("/schedule", h.schedule)
	r.Delete("/schedule", h.cancelSchedule)
	r.Post("/update/check", h.checkUpdate)
	r.Post("/update/install", h.installUpdate)
	return r
}

type systemPage struct {
	ActivePage    string
	Username      string
	Scheduled     bool
	ScheduledTime string // HH:MM
	AppVersion    string
	Update        system.UpdateStatus
}

func (h *SystemHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	tod, ok, _ := system.GetScheduledReboot(h.db)
	web.Render(w, h.tmpl, systemPage{
		ActivePage:    "system",
		Username:      sess.Username,
		Scheduled:     ok,
		ScheduledTime: tod,
		AppVersion:    system.AppVersion,
		Update:        h.updater.Status(),
	})
}

func (h *SystemHandler) reboot(w http.ResponseWriter, r *http.Request) {
	slog.Info("manual reboot requested")
	writeJSON(w, true, "Rebooting…")
	go system.ExecuteReboot()
}

func (h *SystemHandler) schedule(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	tod := r.FormValue("reboot_at")
	if !timeRE.MatchString(tod) {
		writeJSON(w, false, "Invalid time — use HH:MM (24h).")
		return
	}
	if err := system.ScheduleDailyReboot(h.db, tod); err != nil {
		slog.Error("schedule daily reboot", "err", err)
		writeJSON(w, false, "Failed to save.")
		return
	}
	html, _ := web.RenderPartial(h.tmpl, "schedule_status", systemPage{
		Scheduled:     true,
		ScheduledTime: tod,
	})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *SystemHandler) cancelSchedule(w http.ResponseWriter, r *http.Request) {
	if err := system.CancelScheduledReboot(h.db); err != nil {
		slog.Error("cancel scheduled reboot", "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	html, _ := web.RenderPartial(h.tmpl, "schedule_status", systemPage{Scheduled: false})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *SystemHandler) checkUpdate(w http.ResponseWriter, r *http.Request) {
	status := h.updater.Check(r.Context())
	html, err := web.RenderPartial(h.tmpl, "update_status", systemPage{
		AppVersion: system.AppVersion,
		Update:     status,
	})
	if err != nil {
		slog.Error("render update_status partial", "err", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *SystemHandler) installUpdate(w http.ResponseWriter, r *http.Request) {
	slog.Info("update install requested")
	go func() {
		if err := h.updater.Install(context.Background()); err != nil {
			slog.Error("install update", "err", err)
		}
	}()
	writeJSON(w, true, "Installing update, service will restart shortly…")
}
