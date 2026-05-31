package handler

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

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

// DashboardHandler handles the main dashboard page and its SSE feed.
type DashboardHandler struct {
	tmpl *template.Template
}

func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{
		tmpl: web.MustParsePage("dashboard"),
	}
}

func (h *DashboardHandler) Register(r chi.Router) {
	r.Get("/dashboard", h.showDashboard)
	r.Get("/events/dashboard", h.sseDashboard)
}

func (h *DashboardHandler) showDashboard(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	data, devices := gatherDashboardData()
	web.Render(w, h.tmpl, dashboardPage{
		ActivePage: "dashboard",
		Username:   sess.Username,
		Stats:      data,
		Devices:    devices,
	})
}

func (h *DashboardHandler) sseDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			slog.Debug("dashboard SSE client disconnected")
			return
		case <-ticker.C:
			data, devices := gatherDashboardData()

			statsHTML, err := web.RenderPartial(h.tmpl, "dashboard_stats", data)
			if err != nil {
				slog.Error("render dashboard_stats", "err", err)
				continue
			}
			devicesHTML, err := web.RenderPartial(h.tmpl, "dashboard_devices", devices)
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

func gatherDashboardData() (dashboardData, []Device) {
	cpu, _ := system.GetCPU()
	mem, _ := system.GetMemory()
	disks, _ := system.GetDisks()
	pppoe, _ := system.GetPPPoEStatus()

	data := dashboardData{CPU: cpu, Memory: mem, Disks: disks, PPPoE: pppoe}
	devices := mergeDevices()
	return data, devices
}

func mergeDevices() []Device {
	leases, _ := system.GetLeases()
	arp, _ := system.GetARPTable()

	byMAC := map[string]*Device{}

	for _, l := range leases {
		d := &Device{IP: l.IP, MAC: l.MAC, Hostname: l.Hostname, Source: "dhcp"}
		byMAC[l.MAC] = d
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
	return devices
}
