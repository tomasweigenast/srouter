package handler

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

type PortForwardHandler struct {
	fw   system.Firewall
	tmpl *template.Template
}

func NewPortForwardHandler(i do.Injector) (*PortForwardHandler, error) {
	return &PortForwardHandler{
		fw:   do.MustInvoke[system.Firewall](i),
		tmpl: web.MustParsePage("portforward"),
	}, nil
}

func (h *PortForwardHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Post("/", h.add)
	r.Delete("/{name}", h.delete)
	r.Post("/apply", h.apply)
	return r
}

type portForwardPage struct {
	ActivePage string
	Username   string
	Rules      []system.PortForwardRule
}

func (h *PortForwardHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	rules, _ := h.fw.GetPortForwardRules()
	web.Render(w, h.tmpl, portForwardPage{
		ActivePage: "portforward",
		Username:   sess.Username,
		Rules:      rules,
	})
}

func (h *PortForwardHandler) add(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	rule := system.PortForwardRule{
		Name:     r.FormValue("name"),
		Protocol: r.FormValue("protocol"),
		ExtPort:  r.FormValue("ext_port"),
		IntIP:    r.FormValue("int_ip"),
		IntPort:  r.FormValue("int_port"),
	}
	if rule.Name == "" || rule.ExtPort == "" || rule.IntIP == "" {
		http.Error(w, "name, ext_port, and int_ip are required", http.StatusBadRequest)
		return
	}
	if rule.IntPort == "" {
		rule.IntPort = rule.ExtPort
	}
	if err := h.fw.AddPortForwardRule(rule); err != nil {
		slog.Error("add port forward rule", "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	html, _ := web.RenderPartial(h.tmpl, "portforward_rows", []system.PortForwardRule{rule})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *PortForwardHandler) delete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.fw.DeletePortForwardRule(name); err != nil {
		slog.Error("delete port forward rule", "name", name, "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *PortForwardHandler) apply(w http.ResponseWriter, r *http.Request) {
	if err := h.fw.ApplyFirewall(); err != nil {
		slog.Error("apply firewall (portforward)", "err", err)
		http.Error(w, "failed to apply firewall", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<span class="text-emerald-400 text-sm">Applied.</span>`))
}
