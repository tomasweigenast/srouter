package handler

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/logging"
	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

var logsHandlerLogger = logging.GetLogger("logs-handler")

type LogsHandler struct {
	logs system.LogStream
	tmpl *template.Template
}

func NewLogsHandler(i do.Injector) (*LogsHandler, error) {
	return &LogsHandler{
		logs: do.MustInvoke[system.LogStream](i),
		tmpl: web.MustParsePage("logs"),
	}, nil
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
	flusher, ok := sseHeaders(w)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	filter := system.LogFilter{
		Category: q.Get("category"),
		// Search is applied client-side so existing lines can be filtered too
	}
	parsedFirewall := q.Get("parsed") == "1" && filter.Category == "firewall"

	ch, unsub := h.logs.Subscribe(filter)
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			logsHandlerLogger.Debug("logs SSE client disconnected")
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			var (
				lineHTML string
				err      error
			)
			if parsedFirewall {
				if fw, ok := system.ParseFirewallLog(line); ok {
					lineHTML, err = web.RenderSSE(h.tmpl, "firewall_log_row", fw)
				} else {
					lineHTML, err = web.RenderSSE(h.tmpl, "log_line", line)
				}
			} else {
				lineHTML, err = web.RenderSSE(h.tmpl, "log_line", line)
			}
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: logline\ndata: %s\n\n", lineHTML)
			flusher.Flush()
		}
	}
}
