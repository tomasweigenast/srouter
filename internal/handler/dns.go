package handler

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

type DNSHandler struct {
	tmpl *template.Template
}

func NewDNSHandler() *DNSHandler {
	return &DNSHandler{tmpl: web.MustParsePage("dns")}
}

func (h *DNSHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Post("/upstream", h.setUpstream)
	r.Post("/entries", h.addEntry)
	r.Delete("/entries/{hostname}", h.deleteEntry)
	r.Post("/test", h.testLookup)
	return r
}

type dnsPage struct {
	ActivePage string
	Username   string
	Upstream   []system.UpstreamServer
	Entries    []system.LocalEntry
}

func (h *DNSHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	upstream, _ := system.GetUpstreamServers()
	entries, _ := system.GetLocalEntries()
	web.Render(w, h.tmpl, dnsPage{
		ActivePage: "dns",
		Username:   sess.Username,
		Upstream:   upstream,
		Entries:    entries,
	})
}

func (h *DNSHandler) setUpstream(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	var servers []system.UpstreamServer
	for _, addr := range r.Form["server"] {
		if addr != "" {
			servers = append(servers, system.UpstreamServer{Address: addr})
		}
	}
	if err := system.SetUpstreamServers(servers); err != nil {
		slog.Error("set upstream servers", "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/dns", http.StatusFound)
}

func (h *DNSHandler) addEntry(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	entry := system.LocalEntry{
		Hostname: r.FormValue("hostname"),
		IP:       r.FormValue("ip"),
	}
	if entry.Hostname == "" || entry.IP == "" {
		http.Error(w, "hostname and ip required", http.StatusBadRequest)
		return
	}
	if err := system.AddLocalEntry(entry); err != nil {
		slog.Error("add local entry", "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	_ = system.ReloadDNSMasq()
	html, _ := web.RenderPartial(h.tmpl, "dns_entry_row", entry)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *DNSHandler) deleteEntry(w http.ResponseWriter, r *http.Request) {
	hostname := chi.URLParam(r, "hostname")
	if err := system.DeleteLocalEntry(hostname); err != nil {
		slog.Error("delete local entry", "hostname", hostname, "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	_ = system.ReloadDNSMasq()
	w.WriteHeader(http.StatusOK)
}

func (h *DNSHandler) testLookup(w http.ResponseWriter, r *http.Request) {
	hostname := r.FormValue("hostname")
	result, err := system.TestLookup(hostname)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<span class="text-red-400">` + err.Error() + `</span>`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<span class="text-emerald-400 mono">` + result + `</span>`))
}
