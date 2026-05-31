package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

type BandwidthHandler struct {
	bw   system.BandwidthStream
	tmpl *template.Template
}

func NewBandwidthHandler(i do.Injector) (*BandwidthHandler, error) {
	return &BandwidthHandler{
		bw:   do.MustInvoke[system.BandwidthStream](i),
		tmpl: web.MustParsePage("bandwidth"),
	}, nil
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
	flusher, ok := sseHeaders(w)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch, unsub := h.bw.Subscribe()
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
