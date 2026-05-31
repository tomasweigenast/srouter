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

type DHCPHandler struct {
	tmpl *template.Template
}

func NewDHCPHandler() *DHCPHandler {
	return &DHCPHandler{tmpl: web.MustParsePage("dhcp")}
}

func (h *DHCPHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.show)
	r.Get("/leases", h.leases)
	r.Post("/reservations", h.addReservation)
	r.Delete("/reservations/{mac}", h.deleteReservation)
	r.Get("/config", h.getConfig)
	r.Post("/config", h.saveConfig)
	return r
}

type dhcpPage struct {
	ActivePage   string
	Username     string
	Leases       []system.Lease
	Reservations []system.Reservation
	Config       system.DHCPConfig
}

func (h *DHCPHandler) show(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	leases, _ := system.GetLeases()
	reservations, _ := system.GetReservations()
	cfg, _ := system.GetDHCPConfig()
	web.Render(w, h.tmpl, dhcpPage{
		ActivePage:   "dhcp",
		Username:     sess.Username,
		Leases:       leases,
		Reservations: reservations,
		Config:       cfg,
	})
}

func (h *DHCPHandler) leases(w http.ResponseWriter, r *http.Request) {
	leases, _ := system.GetLeases()
	html, err := web.RenderPartial(h.tmpl, "dhcp_leases", leases)
	if err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *DHCPHandler) addReservation(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	res := system.Reservation{
		MAC:      r.FormValue("mac"),
		IP:       r.FormValue("ip"),
		Hostname: r.FormValue("hostname"),
	}
	if res.MAC == "" || res.IP == "" {
		http.Error(w, "mac and ip are required", http.StatusBadRequest)
		return
	}
	if err := system.AddReservation(res); err != nil {
		slog.Error("add reservation", "err", err)
		http.Error(w, "failed to add reservation", http.StatusInternalServerError)
		return
	}
	_ = system.ReloadDNSMasq()
	html, _ := web.RenderPartial(h.tmpl, "dhcp_reservation_row", res)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *DHCPHandler) deleteReservation(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")
	if err := system.DeleteReservation(mac); err != nil {
		slog.Error("delete reservation", "mac", mac, "err", err)
		http.Error(w, "failed to delete reservation", http.StatusInternalServerError)
		return
	}
	_ = system.ReloadDNSMasq()
	w.WriteHeader(http.StatusOK)
}

func (h *DHCPHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	cfg, _ := system.GetDHCPConfig()
	html, _ := web.RenderPartial(h.tmpl, "dhcp_config_form", cfg)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *DHCPHandler) saveConfig(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	cfg := system.DHCPConfig{
		RangeStart: r.FormValue("range_start"),
		RangeEnd:   r.FormValue("range_end"),
		LeaseTime:  r.FormValue("lease_time"),
	}
	// SaveDHCPConfig coming in a follow-up; for now just reload
	_ = cfg
	_ = system.ReloadDNSMasq()
	w.WriteHeader(http.StatusOK)
}
