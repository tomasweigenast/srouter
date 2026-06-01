package handler

import (
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"

	"github.com/tomasweigenast/srouter/internal/config"
	"github.com/tomasweigenast/srouter/internal/session"
	"github.com/tomasweigenast/srouter/internal/system"
	"github.com/tomasweigenast/srouter/web"
)

// Device is a currently-connected LAN device from ARP, enriched with DHCP hostname, optional label, and block state.
type Device struct {
	IP       string
	MAC      string
	Hostname string
	Label    string
	Blocked  bool
}

type dashboardPage struct {
	ActivePage string
	Username   string
	Stats      dashboardData
	Devices    []Device
	SysInfo    system.SystemInfo
	Packages   []system.RouterPackage
}

type dashboardData struct {
	CPU          system.CPUInfo
	Memory       system.MemInfo
	Disks        []system.DiskInfo
	PPPoE        system.PPPoEStatus
	SysInfo      system.SystemInfo
	InternetOK   bool
}

type DashboardHandler struct {
	metrics  system.Metrics
	dhcp     system.DHCP
	net      system.Network
	db       *sql.DB
	interval time.Duration
	tmpl     *template.Template
}

func NewDashboardHandler(i do.Injector) (*DashboardHandler, error) {
	cfg := do.MustInvoke[config.Config](i)
	return &DashboardHandler{
		metrics:  do.MustInvoke[system.Metrics](i),
		dhcp:     do.MustInvoke[system.DHCP](i),
		net:      do.MustInvoke[system.Network](i),
		db:       do.MustInvoke[*sql.DB](i),
		interval: time.Duration(cfg.UpdateIntervalMs) * time.Millisecond,
		tmpl:     web.MustParsePage("dashboard"),
	}, nil
}

func (h *DashboardHandler) Register(r chi.Router) {
	r.Get("/dashboard", h.showDashboard)
	r.Get("/events/dashboard", h.sseDashboard)
	r.Post("/dashboard/labels", h.setLabel)
	r.Delete("/dashboard/labels/{mac}", h.deleteLabel)
	r.Post("/dashboard/blocks", h.blockDevice)
	r.Delete("/dashboard/blocks/{mac}", h.unblockDevice)
}

func (h *DashboardHandler) showDashboard(w http.ResponseWriter, r *http.Request) {
	sess, _ := session.FromContext(r.Context())
	data, devices := h.gatherData()
	pkgs, _ := h.metrics.GetRouterPackages()
	web.Render(w, h.tmpl, dashboardPage{
		ActivePage: "dashboard",
		Username:   sess.Username,
		Stats:      data,
		Devices:    devices,
		SysInfo:    data.SysInfo,
		Packages:   pkgs,
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

			sysinfoHTML, err := web.RenderSSE(h.tmpl, "dashboard_sysinfo", data.SysInfo)
			if err != nil {
				slog.Error("render dashboard_sysinfo", "err", err)
				continue
			}

			fmt.Fprintf(w, "event: stats\ndata: %s\n\n", statsHTML)
			fmt.Fprintf(w, "event: devices\ndata: %s\n\n", devicesHTML)
			fmt.Fprintf(w, "event: sysinfo\ndata: %s\n\n", sysinfoHTML)
			flusher.Flush()
		}
	}
}

func (h *DashboardHandler) setLabel(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	mac := r.FormValue("mac")
	label := r.FormValue("label")
	if mac == "" || label == "" {
		http.Error(w, "mac and label required", http.StatusBadRequest)
		return
	}
	if err := system.SetDeviceLabel(h.db, mac, label); err != nil {
		slog.Error("set device label", "mac", mac, "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	// Return an updated device row — look up the device in current ARP+DHCP
	_, devices := h.gatherData()
	for _, d := range devices {
		if d.MAC == mac {
			html, _ := web.RenderPartial(h.tmpl, "device_row", d)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(html))
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (h *DashboardHandler) deleteLabel(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")
	if err := system.DeleteDeviceLabel(h.db, mac); err != nil {
		slog.Error("delete device label", "mac", mac, "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	_, devices := h.gatherData()
	for _, d := range devices {
		if d.MAC == mac {
			html, _ := web.RenderPartial(h.tmpl, "device_row", d)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(html))
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (h *DashboardHandler) blockDevice(w http.ResponseWriter, r *http.Request) {
	mac := r.FormValue("mac")
	if mac == "" {
		http.Error(w, "mac required", http.StatusBadRequest)
		return
	}
	if err := system.BlockDevice(h.db, mac); err != nil {
		slog.Error("block device", "mac", mac, "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	if err := system.RebuildBlocksScript(h.db); err != nil {
		slog.Error("rebuild blocks script", "err", err)
	}
	_, devices := h.gatherData()
	for _, d := range devices {
		if d.MAC == mac {
			html, _ := web.RenderPartial(h.tmpl, "device_row", d)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(html))
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (h *DashboardHandler) unblockDevice(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")
	if err := system.UnblockDevice(h.db, mac); err != nil {
		slog.Error("unblock device", "mac", mac, "err", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	if err := system.RebuildBlocksScript(h.db); err != nil {
		slog.Error("rebuild blocks script", "err", err)
	}
	_, devices := h.gatherData()
	for _, d := range devices {
		if d.MAC == mac {
			html, _ := web.RenderPartial(h.tmpl, "device_row", d)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(html))
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (h *DashboardHandler) gatherData() (dashboardData, []Device) {
	cpu, _ := h.metrics.GetCPU()
	mem, _ := h.metrics.GetMemory()
	disks, _ := h.metrics.GetDisks()
	pppoe, _ := h.metrics.GetPPPoEStatus()
	sysInfo, _ := h.metrics.GetSystemInfo()
	internetOK, _ := h.metrics.CheckInternetConnectivity()

	leases, _ := h.dhcp.GetLeases()
	arp, _ := h.net.GetARPTable()
	labels, _ := system.ListDeviceLabels(h.db)
	blocked, _ := system.ListBlockedMACs(h.db)

	// Index DHCP leases by MAC for hostname lookup
	hostnameByMAC := map[string]string{}
	for _, l := range leases {
		if l.Hostname != "" {
			hostnameByMAC[l.MAC] = l.Hostname
		}
	}

	// ARP table = currently connected devices; enrich with hostname, label, and block state
	devices := make([]Device, 0, len(arp))
	for _, a := range arp {
		_, isBlocked := blocked[a.MAC]
		devices = append(devices, Device{
			IP:       a.IP,
			MAC:      a.MAC,
			Hostname: hostnameByMAC[a.MAC],
			Label:    labels[a.MAC],
			Blocked:  isBlocked,
		})
	}

	// Sort by IP ascending
	sort.Slice(devices, func(i, j int) bool {
		return ipLess(devices[i].IP, devices[j].IP)
	})

	return dashboardData{
		CPU:        cpu,
		Memory:     mem,
		Disks:      disks,
		PPPoE:      pppoe,
		SysInfo:    sysInfo,
		InternetOK: internetOK,
	}, devices
}

func ipLess(a, b string) bool {
	ia := net.ParseIP(a)
	ib := net.ParseIP(b)
	if ia == nil || ib == nil {
		return a < b
	}
	ia = ia.To4()
	ib = ib.To4()
	if ia == nil || ib == nil {
		return a < b
	}
	for i := range ia {
		if ia[i] != ib[i] {
			return ia[i] < ib[i]
		}
	}
	return false
}
