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

type FirewallHandler struct {
	tmpl *template.Template
}

func NewFirewallHandler() *FirewallHandler {
	return &FirewallHandler{tmpl: web.MustParsePage("firewall")}
}

func (h *FirewallHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Get("/script", h.getScript)
	r.Post("/script", h.saveScript)
	r.Post("/apply", h.apply)
	return r
}

type firewallPage struct {
	ActivePage      string
	Username        string
	Mode            string // "ui" | "raw"
	Rules           []system.FirewallRule
	Files           []system.FirewallFile
	SelectedFile    system.FirewallFile
}

func (h *FirewallHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "ui"
	}

	page := firewallPage{
		ActivePage: "firewall",
		Username:   sess.Username,
		Mode:       mode,
	}

	if mode == "ui" {
		page.Rules, _ = system.GetRulesFromKernel()
	} else {
		files, _ := system.GetFirewallFiles()
		page.Files = files
		// Select the file from query param, default to first
		selectedName := r.URL.Query().Get("file")
		if selectedName == "" && len(files) > 0 {
			page.SelectedFile = files[0]
		} else {
			for _, f := range files {
				if f.Name == selectedName {
					page.SelectedFile = f
					break
				}
			}
			// If not found, default to first
			if page.SelectedFile.Name == "" && len(files) > 0 {
				page.SelectedFile = files[0]
			}
		}
	}

	web.Render(w, h.tmpl, page)
}

func (h *FirewallHandler) getScript(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("file")
	file, err := system.GetFirewallFileContent(name)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusBadRequest)
		return
	}
	html, _ := web.RenderPartial(h.tmpl, "firewall_raw_editor", file)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *FirewallHandler) saveScript(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("filename")
	content := r.FormValue("content")
	if err := system.SaveFirewallScript(name, content); err != nil {
		slog.Error("save firewall file", "name", name, "err", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<span class="text-red-400 text-sm">` + err.Error() + `</span>`))
		return
	}
	slog.Info("firewall file saved", "name", name)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<span class="text-emerald-400 text-sm">Saved.</span>`))
}

func (h *FirewallHandler) apply(w http.ResponseWriter, r *http.Request) {
	if err := system.ApplyFirewall(); err != nil {
		slog.Error("apply firewall", "err", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<span class="text-red-400 text-sm">Failed: ` + err.Error() + `</span>`))
		return
	}
	slog.Info("firewall applied")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<span class="text-emerald-400 text-sm">Firewall applied successfully.</span>`))
}
