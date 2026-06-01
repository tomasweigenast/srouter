package handler

import (
	"database/sql"
	"fmt"
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
	ScheduledTime string // HH:MM
}

func (h *SystemHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	tod, ok, _ := system.GetScheduledReboot(h.db)
	web.Render(w, h.tmpl, systemPage{
		ActivePage:    "system",
		Username:      sess.Username,
		Scheduled:     ok,
		ScheduledTime: tod,
	})
}

func (h *SystemHandler) reboot(w http.ResponseWriter, r *http.Request) {
	slog.Info("manual reboot requested")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<span class="text-yellow-400 text-sm">Rebooting…</span>`)
	go system.ExecuteReboot()
}

func (h *SystemHandler) schedule(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	tod := r.FormValue("reboot_at")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if !timeRE.MatchString(tod) {
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Invalid time — use HH:MM (24h).</span>`)
		return
	}
	if err := system.ScheduleDailyReboot(h.db, tod); err != nil {
		slog.Error("schedule daily reboot", "err", err)
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Failed to save.</span>`)
		return
	}
	html, _ := web.RenderPartial(h.tmpl, "schedule_status", systemPage{
		Scheduled:     true,
		ScheduledTime: tod,
	})
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
