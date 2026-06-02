package handler

import (
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/logging"
	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

var networkLogger = logging.GetLogger("network")

type NetworkHandler struct {
	net  system.Network
	tmpl *template.Template
}

func NewNetworkHandler(i do.Injector) (*NetworkHandler, error) {
	return &NetworkHandler{
		net:  do.MustInvoke[system.Network](i),
		tmpl: web.MustParsePage("network"),
	}, nil
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
	ifaces, err := h.net.GetInterfaces()
	if err != nil {
		networkLogger.Warn("get interfaces", "err", err)
	}
	arp, err := h.net.GetARPTable()
	if err != nil {
		networkLogger.Warn("get arp table", "err", err)
	}
	routes, err := h.net.GetRoutes()
	if err != nil {
		networkLogger.Warn("get routes", "err", err)
	}
	conntrack, err := h.net.GetConntrackStats()
	if err != nil {
		networkLogger.Warn("get conntrack stats", "err", err)
	}
	return networkPage{
		ActivePage: activePage,
		Username:   username,
		Interfaces: ifaces,
		ARPTable:   arp,
		Routes:     routes,
		Conntrack:  conntrack,
	}
}
