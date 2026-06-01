package handler

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

type DNSHandler struct {
	dns  system.DNS
	tmpl *template.Template
}

func NewDNSHandler(i do.Injector) (*DNSHandler, error) {
	return &DNSHandler{
		dns:  do.MustInvoke[system.DNS](i),
		tmpl: web.MustParsePage("dns"),
	}, nil
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
	upstream, _ := h.dns.GetUpstreamServers()
	entries, _ := h.dns.GetLocalEntries()
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
			servers = append(servers, system.UpstreamServer{Address: strings.TrimSpace(addr)})
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.dns.SetUpstreamServers(servers); err != nil {
		slog.Error("set upstream servers", "err", err)
		fmt.Fprintf(w, `<span class="text-red-500 text-sm">Failed to save.</span>`)
		return
	}
	fmt.Fprintf(w, `<span class="text-emerald-600 text-sm">Saved &amp; reloaded.</span>`)
}

func (h *DNSHandler) addEntry(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	entry := system.LocalEntry{
		Hostname: strings.TrimSpace(r.FormValue("hostname")),
		IP:       strings.TrimSpace(r.FormValue("ip")),
	}
	if entry.Hostname == "" || entry.IP == "" {
		writeHTMXInlineError(w, "Hostname and IP are required.")
		return
	}

	// Duplicate hostname check
	existing, _ := h.dns.GetLocalEntries()
	for _, e := range existing {
		if strings.EqualFold(e.Hostname, entry.Hostname) {
			writeHTMXInlineError(w, fmt.Sprintf("Hostname %q already exists.", entry.Hostname))
			return
		}
	}

	if err := h.dns.AddLocalEntry(entry); err != nil {
		slog.Error("add local entry", "err", err)
		writeHTMXInlineError(w, "Failed to add entry.")
		return
	}
	_ = h.dns.Reload()

	// On success, retarget to the entries table
	w.Header().Set("HX-Retarget", "#dns-entries")
	w.Header().Set("HX-Reswap", "beforeend")
	html, _ := web.RenderPartial(h.tmpl, "dns_entry_row", entry)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *DNSHandler) deleteEntry(w http.ResponseWriter, r *http.Request) {
	hostname := chi.URLParam(r, "hostname")
	if err := h.dns.DeleteLocalEntry(hostname); err != nil {
		slog.Error("delete local entry", "hostname", hostname, "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	_ = h.dns.Reload()
	w.WriteHeader(http.StatusOK)
}

func (h *DNSHandler) testLookup(w http.ResponseWriter, r *http.Request) {
	hostname := r.FormValue("hostname")
	result, upstream, err := h.dns.TestLookup(hostname)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		fmt.Fprintf(w, `<span class="text-red-400">%s</span>`, err.Error())
		return
	}
	fmt.Fprintf(w,
		`<pre class="text-emerald-600 mono text-xs whitespace-pre-wrap">%s</pre>`+
			`<p class="text-xs text-gray-500 mt-1">Queried upstream: %s (local cache bypassed)</p>`,
		result, upstream,
	)
}
