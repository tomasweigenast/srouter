package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/logging"
	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

var dhcpLogger = logging.GetLogger("dhcp")

type DHCPHandler struct {
	dhcp system.DHCP
	tmpl *template.Template
}

func NewDHCPHandler(i do.Injector) (*DHCPHandler, error) {
	return &DHCPHandler{
		dhcp: do.MustInvoke[system.DHCP](i),
		tmpl: web.MustParsePage("dhcp"),
	}, nil
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
	leases, _ := h.dhcp.GetLeases()
	reservations, _ := h.dhcp.GetReservations()
	cfg, _ := h.dhcp.GetDHCPConfig()
	web.Render(w, h.tmpl, dhcpPage{
		ActivePage:   "dhcp",
		Username:     sess.Username,
		Leases:       leases,
		Reservations: reservations,
		Config:       cfg,
	})
}

func (h *DHCPHandler) leases(w http.ResponseWriter, r *http.Request) {
	leases, _ := h.dhcp.GetLeases()
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
		MAC:      strings.ToLower(strings.TrimSpace(r.FormValue("mac"))),
		IP:       strings.TrimSpace(r.FormValue("ip")),
		Hostname: strings.TrimSpace(r.FormValue("hostname")),
	}
	if res.MAC == "" || res.IP == "" {
		writeJSON(w, false, "MAC address and IP are required.")
		return
	}

	// Duplicate check
	existing, _ := h.dhcp.GetReservations()
	for _, e := range existing {
		if strings.EqualFold(e.MAC, res.MAC) {
			writeJSON(w, false, fmt.Sprintf("MAC %s is already reserved (%s).", res.MAC, e.IP))
			return
		}
		if e.IP == res.IP {
			writeJSON(w, false, fmt.Sprintf("IP %s is already reserved for %s.", res.IP, e.MAC))
			return
		}
	}

	if err := h.dhcp.AddReservation(res); err != nil {
		dhcpLogger.Error("add reservation", "err", err)
		writeJSON(w, false, "Failed to add reservation.")
		return
	}
	_ = h.dhcp.ReloadDNSMasq()
	html, _ := web.RenderPartial(h.tmpl, "dhcp_reservation_row", res)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (h *DHCPHandler) deleteReservation(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")
	if err := h.dhcp.DeleteReservation(mac); err != nil {
		dhcpLogger.Error("delete reservation", "mac", mac, "err", err)
		http.Error(w, "failed to delete reservation", http.StatusInternalServerError)
		return
	}
	_ = h.dhcp.ReloadDNSMasq()
	w.WriteHeader(http.StatusOK)
}

func (h *DHCPHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	cfg, _ := h.dhcp.GetDHCPConfig()
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
	if err := h.dhcp.SaveDHCPConfig(cfg); err != nil {
		dhcpLogger.Error("save dhcp config", "err", err)
		writeJSON(w, false, "Failed to save config.")
		return
	}
	_ = h.dhcp.ReloadDNSMasq()
	writeJSON(w, true, "Saved & reloaded.")
}
