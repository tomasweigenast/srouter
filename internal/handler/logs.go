package handler

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

type LogsHandler struct {
	tmpl        *template.Template
	logBroadcast *system.LogBroadcaster
}

func NewLogsHandler(lb *system.LogBroadcaster) *LogsHandler {
	return &LogsHandler{
		tmpl:        web.MustParsePage("logs"),
		logBroadcast: lb,
	}
}

func (h *LogsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Get("/events", h.sseStream)
	return r
}

type logsPage struct {
	ActivePage string
	Username   string
	Filter     system.LogFilter
}

func (h *LogsHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	filter := system.LogFilter{
		Category: r.URL.Query().Get("category"),
		Search:   r.URL.Query().Get("search"),
	}
	web.Render(w, h.tmpl, logsPage{
		ActivePage: "logs",
		Username:   sess.Username,
		Filter:     filter,
	})
}

func (h *LogsHandler) sseStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	filter := system.LogFilter{
		Category: r.URL.Query().Get("category"),
		Search:   r.URL.Query().Get("search"),
	}

	ch, unsub := h.logBroadcast.Subscribe(filter)
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			slog.Debug("logs SSE client disconnected")
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			lineHTML, err := web.RenderPartial(h.tmpl, "log_line", line)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: logline\ndata: %s\n\n", lineHTML)
			flusher.Flush()
		}
	}
}
