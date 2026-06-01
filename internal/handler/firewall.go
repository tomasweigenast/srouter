package handler

import (
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

type FirewallHandler struct {
	fw   system.Firewall
	tmpl *template.Template
}

func NewFirewallHandler(i do.Injector) (*FirewallHandler, error) {
	return &FirewallHandler{
		fw:   do.MustInvoke[system.Firewall](i),
		tmpl: web.MustParsePage("firewall"),
	}, nil
}

func (h *FirewallHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Get("/script", h.getScript)
	r.Post("/script", h.saveScript)
	r.Post("/apply", h.apply)
	// Custom rules (UI mode)
	r.Post("/rules", h.addCustomRule)
	r.Delete("/rules/{index}", h.deleteCustomRule)
	return r
}

type firewallPage struct {
	ActivePage   string
	Username     string
	Mode         string // "ui" | "raw"
	Rules        []system.FirewallRule
	CustomRules  []system.FirewallRule
	Files        []system.FirewallFile
	SelectedFile system.FirewallFile
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
		page.Rules, _ = h.fw.GetRulesFromKernel()
		page.CustomRules, _ = h.fw.GetCustomRules()
	} else {
		files, _ := h.fw.GetFirewallFiles()
		page.Files = files
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
			if page.SelectedFile.Name == "" && len(files) > 0 {
				page.SelectedFile = files[0]
			}
		}
	}

	web.Render(w, h.tmpl, page)
}

func (h *FirewallHandler) getScript(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("file")
	file, err := h.fw.GetFirewallFileContent(name)
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
	if err := h.fw.SaveFirewallScript(name, content); err != nil {
		slog.Error("save firewall file", "name", name, "err", err)
		writeJSON(w, false, err.Error())
		return
	}
	slog.Info("firewall file saved", "name", name)
	writeJSON(w, true, "Saved.")
}

func (h *FirewallHandler) apply(w http.ResponseWriter, r *http.Request) {
	if err := h.fw.ApplyFirewall(); err != nil {
		slog.Error("apply firewall", "err", err)
		writeJSON(w, false, "Failed: "+err.Error())
		return
	}
	slog.Info("firewall applied")
	writeJSON(w, true, "Applied successfully.")
}

func (h *FirewallHandler) addCustomRule(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	rule := system.FirewallRule{
		Chain:    r.FormValue("chain"),
		Protocol: r.FormValue("protocol"),
		InIface:  r.FormValue("in_iface"),
		OutIface: r.FormValue("out_iface"),
		SrcIP:    r.FormValue("src_ip"),
		DstIP:    r.FormValue("dst_ip"),
		SrcPort:  r.FormValue("src_port"),
		DstPort:  r.FormValue("dst_port"),
		Action:   r.FormValue("action"),
		Comment:  r.FormValue("comment"),
	}
	if rule.Chain == "" || rule.Action == "" {
		writeJSON(w, false, "Chain and Action are required.")
		return
	}
	if err := h.fw.AddCustomRule(rule); err != nil {
		slog.Error("add custom rule", "err", err)
		writeJSON(w, false, "Failed to add rule.")
		return
	}
	rules, _ := h.fw.GetCustomRules()
	html, _ := web.RenderPartial(h.tmpl, "firewall_custom_rules", rules)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *FirewallHandler) deleteCustomRule(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}
	if err := h.fw.DeleteCustomRule(idx); err != nil {
		slog.Error("delete custom rule", "index", idx, "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	rules, _ := h.fw.GetCustomRules()
	html, _ := web.RenderPartial(h.tmpl, "firewall_custom_rules", rules)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
