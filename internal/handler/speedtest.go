package handler

import (
	"encoding/json"
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

var speedtestLogger = logging.GetLogger("speedtest")

type SpeedtestHandler struct {
	tmpl *template.Template
}

func NewSpeedtestHandler(i do.Injector) (*SpeedtestHandler, error) {
	return &SpeedtestHandler{
		tmpl: web.MustParsePage("speedtest"),
	}, nil
}

func (h *SpeedtestHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Get("/run", h.run)
	return r
}

type speedtestPage struct {
	ActivePage string
	Username   string
}

func (h *SpeedtestHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	web.Render(w, h.tmpl, speedtestPage{
		ActivePage: "speedtest",
		Username:   sess.Username,
	})
}

func (h *SpeedtestHandler) run(w http.ResponseWriter, r *http.Request) {
	flusher, ok := sseHeaders(w)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	emit := func(event string, v any) {
		data, err := json.Marshal(v)
		if err != nil {
			speedtestLogger.Error("speedtest marshal", "event", event, "err", err)
			return
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	emit("stage", "latency")
	latency := system.MeasureLatency()
	emit("latency", latency)

	emit("stage", "download")
	download := system.MeasureDownload()
	emit("download", download)

	emit("stage", "upload")
	upload := system.MeasureUpload()
	emit("upload", upload)

	emit("done", nil)
}
