package handler

import (
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

type NetworkHandler struct {
	tmpl *template.Template
}

func NewNetworkHandler() *NetworkHandler {
	return &NetworkHandler{tmpl: web.MustParsePage("network")}
}

func (h *NetworkHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Get("/refresh", h.refresh)
	return r
}

type networkPage struct {
	ActivePage  string
	Username    string
	Interfaces  []system.Interface
	ARPTable    []system.ARPEntry
	Routes      []system.Route
	Conntrack   system.ConntrackStats
}

func (h *NetworkHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	web.Render(w, h.tmpl, h.buildPage("network", sess.Username))
}

func (h *NetworkHandler) refresh(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	html, _ := web.RenderPartial(h.tmpl, "network_content", h.buildPage("network", sess.Username))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *NetworkHandler) buildPage(activePage, username string) networkPage {
	ifaces, _ := system.GetInterfaces()
	arp, _ := system.GetARPTable()
	routes, _ := system.GetRoutes()
	conntrack, _ := system.GetConntrackStats()
	return networkPage{
		ActivePage: activePage,
		Username:   username,
		Interfaces: ifaces,
		ARPTable:   arp,
		Routes:     routes,
		Conntrack:  conntrack,
	}
}
