package handler

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/config"
	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

// Device is a connected LAN device merged from ARP table + DHCP leases.
type Device struct {
	IP       string
	MAC      string
	Hostname string
	Source   string // "dhcp" | "arp"
}

type dashboardPage struct {
	ActivePage string
	Username   string
	Stats      dashboardData
	Devices    []Device
}

type dashboardData struct {
	CPU    system.CPUInfo
	Memory system.MemInfo
	Disks  []system.DiskInfo
	PPPoE  system.PPPoEStatus
}

type DashboardHandler struct {
	metrics  system.Metrics
	dhcp     system.DHCP
	net      system.Network
	interval time.Duration
	tmpl     *template.Template
}

func NewDashboardHandler(i do.Injector) (*DashboardHandler, error) {
	cfg := do.MustInvoke[config.Config](i)
	return &DashboardHandler{
		metrics:  do.MustInvoke[system.Metrics](i),
		dhcp:     do.MustInvoke[system.DHCP](i),
		net:      do.MustInvoke[system.Network](i),
		interval: time.Duration(cfg.UpdateIntervalMs) * time.Millisecond,
		tmpl:     web.MustParsePage("dashboard"),
	}, nil
}

func (h *DashboardHandler) Register(r chi.Router) {
	r.Get("/dashboard", h.showDashboard)
	r.Get("/events/dashboard", h.sseDashboard)
}

func (h *DashboardHandler) showDashboard(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	data, devices := h.gatherData()
	web.Render(w, h.tmpl, dashboardPage{
		ActivePage: "dashboard",
		Username:   sess.Username,
		Stats:      data,
		Devices:    devices,
	})
}

func (h *DashboardHandler) sseDashboard(w http.ResponseWriter, r *http.Request) {
	flusher, ok := sseHeaders(w)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			slog.Debug("dashboard SSE client disconnected")
			return
		case <-ticker.C:
			data, devices := h.gatherData()

			statsHTML, err := web.RenderSSE(h.tmpl, "dashboard_stats", data)
			if err != nil {
				slog.Error("render dashboard_stats", "err", err)
				continue
			}
			devicesHTML, err := web.RenderSSE(h.tmpl, "dashboard_devices", devices)
			if err != nil {
				slog.Error("render dashboard_devices", "err", err)
				continue
			}

			fmt.Fprintf(w, "event: stats\ndata: %s\n\n", statsHTML)
			fmt.Fprintf(w, "event: devices\ndata: %s\n\n", devicesHTML)
			flusher.Flush()
		}
	}
}

func (h *DashboardHandler) gatherData() (dashboardData, []Device) {
	cpu, _ := h.metrics.GetCPU()
	mem, _ := h.metrics.GetMemory()
	disks, _ := h.metrics.GetDisks()
	pppoe, _ := h.metrics.GetPPPoEStatus()

	leases, _ := h.dhcp.GetLeases()
	arp, _ := h.net.GetARPTable()

	byMAC := map[string]*Device{}
	for _, l := range leases {
		byMAC[l.MAC] = &Device{IP: l.IP, MAC: l.MAC, Hostname: l.Hostname, Source: "dhcp"}
	}
	for _, a := range arp {
		if _, ok := byMAC[a.MAC]; !ok {
			byMAC[a.MAC] = &Device{IP: a.IP, MAC: a.MAC, Source: "arp"}
		}
	}
	devices := make([]Device, 0, len(byMAC))
	for _, d := range byMAC {
		devices = append(devices, *d)
	}

	return dashboardData{CPU: cpu, Memory: mem, Disks: disks, PPPoE: pppoe}, devices
}
