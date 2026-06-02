package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/logging"
	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

var dohHandlerLogger = logging.GetLogger("doh-handler")

type DoHHandler struct {
	doh   system.DoH
	stats *system.DNSStatsCollector
	tmpl  *template.Template
}

func NewDoHHandler(i do.Injector) (*DoHHandler, error) {
	return &DoHHandler{
		doh:   do.MustInvoke[system.DoH](i),
		stats: do.MustInvoke[*system.DNSStatsCollector](i),
		tmpl:  web.MustParsePage("doh"),
	}, nil
}

func (h *DoHHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Post("/toggle", h.toggle)
	r.Post("/config", h.saveConfig)
	r.Get("/status", h.statusSSE)
	r.Post("/test", h.testLookup)
	return r
}

type dohPage struct {
	ActivePage string
	Username   string
	Status     system.DoHStatus
}

func (h *DoHHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	status, err := h.doh.GetDoHStatus()
	if err != nil {
		dohHandlerLogger.Error("get doh status", "err", err)
	}
	web.Render(w, h.tmpl, dohPage{
		ActivePage: "doh",
		Username:   sess.Username,
		Status:     status,
	})
}

func (h *DoHHandler) toggle(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	enable := r.FormValue("enabled") == "1"

	if enable {
		status, _ := h.doh.GetDoHStatus()
		if !status.AnyInstalled() {
			writeJSON(w, false, "Neither stubby nor dnscrypt-proxy is installed.")
			return
		}

		cfg := system.DoHConfig{
			ProviderID:       r.FormValue("provider"),
			FallbackToPlain:  r.FormValue("fallback") == "1",
			CustomDoTIP:      r.FormValue("custom_dot_ip"),
			CustomDoTTLSName: r.FormValue("custom_dot_tls_name"),
			CustomDoHURL:  r.FormValue("custom_doh_url"),
		}
		if cfg.ProviderID == "" {
			cfg.ProviderID = "cloudflare"
		}
		if cfg.ProviderID == "custom" {
			if status.DoTInstalled && (cfg.CustomDoTIP == "" || cfg.CustomDoTTLSName == "") {
				writeJSON(w, false, "Custom DoT requires Server IP and TLS hostname.")
				return
			}
			if status.DoHInstalled && cfg.CustomDoHURL == "" {
				writeJSON(w, false, "Custom DoH requires a server name.")
				return
			}
		}

		if err := h.doh.EnableEncryptedDNS(cfg); err != nil {
			dohHandlerLogger.Error("enable encrypted dns", "err", err)
			writeJSON(w, false, "Failed to enable encrypted DNS.")
			return
		}
	} else {
		if err := h.doh.DisableEncryptedDNS(); err != nil {
			dohHandlerLogger.Error("disable encrypted dns", "err", err)
			writeJSON(w, false, "Failed to disable encrypted DNS.")
			return
		}
	}

	w.Header().Set("HX-Refresh", "true")
	writeJSON(w, true, "Saved.")
}

func (h *DoHHandler) saveConfig(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	status, err := h.doh.GetDoHStatus()
	if err != nil {
		writeJSON(w, false, "Failed to read current config.")
		return
	}
	cfg := system.DoHConfig{
		Enabled:          status.Config.Enabled,
		ProviderID:       r.FormValue("provider"),
		FallbackToPlain:  r.FormValue("fallback") == "1",
		StashedUpstreams: status.Config.StashedUpstreams,
		CustomDoTIP:      r.FormValue("custom_dot_ip"),
		CustomDoTTLSName: r.FormValue("custom_dot_tls_name"),
		CustomDoHURL:  r.FormValue("custom_doh_url"),
	}
	if cfg.ProviderID == "" {
		cfg.ProviderID = "cloudflare"
	}

	if status.Config.Enabled {
		if err := h.doh.EnableEncryptedDNS(cfg); err != nil {
			dohHandlerLogger.Error("reapply encrypted dns config", "err", err)
			writeJSON(w, false, "Failed to apply new config.")
			return
		}
		w.Header().Set("HX-Refresh", "true")
		writeJSON(w, true, "Config updated & applied.")
		return
	}

	if err := h.doh.SaveDoHConfig(cfg); err != nil {
		dohHandlerLogger.Error("save doh config", "err", err)
		writeJSON(w, false, "Failed to save config.")
		return
	}
	writeJSON(w, true, "Config saved.")
}

type dohStatusPayload struct {
	system.DoHStatus
	DoTQueries   int64
	DoHQueries   int64
	PlainQueries int64
}

func (h *DoHHandler) statusSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := sseHeaders(w)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			status, err := h.doh.GetDoHStatus()
			if err != nil {
				continue
			}
			upstreams := h.stats.Stats().ByUpstream
			payload := dohStatusPayload{
				DoHStatus:    status,
				DoTQueries:   upstreams[system.StubbyListen],
				DoHQueries:   upstreams[system.DnscryptListen],
				PlainQueries: plainQueries(upstreams),
			}
			data, err := json.Marshal(payload)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: dohstatus\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (h *DoHHandler) testLookup(w http.ResponseWriter, r *http.Request) {
	hostname := strings.TrimSpace(r.FormValue("hostname"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if hostname == "" {
		fmt.Fprint(w, `<p class="text-red-400 text-sm">Enter a hostname.</p>`)
		return
	}

	result, err := h.doh.TestEncryptedLookup(hostname)
	if err != nil {
		fmt.Fprintf(w, `<p class="text-red-400 text-sm">%s</p>`, err.Error())
		return
	}

	fmt.Fprint(w, `<div class="space-y-3 text-sm">`)
	fmt.Fprintf(w, `<div><p class="text-xs font-medium text-gray-500 uppercase mb-1">DoT (stubby :5353) — %s</p>`, result.DoTLatency)
	if result.DoTErr != "" {
		fmt.Fprintf(w, `<p class="text-red-400 font-mono text-xs">%s</p>`, result.DoTErr)
	} else {
		fmt.Fprintf(w, `<p class="text-emerald-600 font-mono text-xs">%s</p>`, strings.Join(result.DoTAddrs, ", "))
	}
	fmt.Fprint(w, `</div>`)

	fmt.Fprintf(w, `<div><p class="text-xs font-medium text-gray-500 uppercase mb-1">DoH (dnscrypt-proxy :5454) — %s</p>`, result.DoHLatency)
	if result.DoHErr != "" {
		fmt.Fprintf(w, `<p class="text-red-400 font-mono text-xs">%s</p>`, result.DoHErr)
	} else {
		fmt.Fprintf(w, `<p class="text-emerald-600 font-mono text-xs">%s</p>`, strings.Join(result.DoHAddrs, ", "))
	}
	fmt.Fprint(w, `</div></div>`)
}

// plainQueries counts forwarded queries that went to neither stub address.
func plainQueries(byUpstream map[string]int64) int64 {
	stubs := map[string]bool{system.StubbyListen: true, system.DnscryptListen: true}
	var total int64
	for addr, n := range byUpstream {
		if !stubs[addr] {
			total += n
		}
	}
	return total
}
