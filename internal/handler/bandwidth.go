package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

type BandwidthHandler struct {
	tmpl        *template.Template
	broadcaster *system.Broadcaster
}

func NewBandwidthHandler(b *system.Broadcaster) *BandwidthHandler {
	return &BandwidthHandler{
		tmpl:        web.MustParsePage("bandwidth"),
		broadcaster: b,
	}
}

func (h *BandwidthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Get("/events", h.sseStream)
	return r
}

type bandwidthPage struct {
	ActivePage string
	Username   string
}

func (h *BandwidthHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	web.Render(w, h.tmpl, bandwidthPage{
		ActivePage: "bandwidth",
		Username:   sess.Username,
	})
}

func (h *BandwidthHandler) sseStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch, unsub := h.broadcaster.Subscribe()
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			slog.Debug("bandwidth SSE client disconnected")
			return
		case sample, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(sample)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: bwdata\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}
