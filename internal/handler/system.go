package handler

import (
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

type SystemHandler struct {
	db   *sql.DB
	tmpl *template.Template
}

func NewSystemHandler(i do.Injector) (*SystemHandler, error) {
	return &SystemHandler{
		db:   do.MustInvoke[*sql.DB](i),
		tmpl: web.MustParsePage("system"),
	}, nil
}

func (h *SystemHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Post("/reboot", h.reboot)
	r.Post("/schedule", h.schedule)
	r.Delete("/schedule", h.cancelSchedule)
	return r
}

type systemPage struct {
	ActivePage    string
	Username      string
	Scheduled     bool
	ScheduledTime string
}

func (h *SystemHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	t, ok, _ := system.GetScheduledReboot(h.db)
	page := systemPage{
		ActivePage: "system",
		Username:   sess.Username,
		Scheduled:  ok,
	}
	if ok {
		page.ScheduledTime = t.Format("2006-01-02T15:04")
	}
	web.Render(w, h.tmpl, page)
}

func (h *SystemHandler) reboot(w http.ResponseWriter, r *http.Request) {
	slog.Info("manual reboot requested")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<span class="text-yellow-400 text-sm">Rebooting…</span>`)
	go system.ExecuteReboot()
}

func (h *SystemHandler) schedule(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	raw := r.FormValue("reboot_at")
	t, err := time.ParseInLocation("2006-01-02T15:04", raw, time.Local)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Invalid date/time.</span>`)
		return
	}
	if t.Before(time.Now()) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Time must be in the future.</span>`)
		return
	}
	if err := system.ScheduleReboot(h.db, t); err != nil {
		slog.Error("schedule reboot", "err", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Failed to save.</span>`)
		return
	}
	html, _ := web.RenderPartial(h.tmpl, "schedule_status", systemPage{
		Scheduled:     true,
		ScheduledTime: t.Format("2006-01-02T15:04"),
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
