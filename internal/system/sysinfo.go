package system

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AppVersion is injected at build time via -ldflags "-X ...system.AppVersion=v1.0.0".
var AppVersion = "dev"

type SystemInfo struct {
	Hostname   string
	Kernel     string
	Uptime     string
	SystemTime string
	NTPSynced  bool
	NTPService string // "openntpd" | "chronyd" | "busybox-ntpd" | "none"
	AppVersion string
}

type RouterPackage struct {
	Name    string
	Version string
}

func GetSystemInfo() (SystemInfo, error) {
	info := SystemInfo{AppVersion: AppVersion}

	// Hostname
	if h, err := os.ReadFile("/etc/hostname"); err == nil {
		info.Hostname = strings.TrimSpace(string(h))
	} else if out, err := exec.Command("hostname").Output(); err == nil {
		info.Hostname = strings.TrimSpace(string(out))
	}

	// Kernel version
	if out, err := exec.Command("uname", "-r").Output(); err == nil {
		info.Kernel = strings.TrimSpace(string(out))
	}

	// Uptime from /proc/uptime
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			if secs, err := strconv.ParseFloat(fields[0], 64); err == nil {
				info.Uptime = formatUptime(int64(secs))
			}
		}
	}

	// System time
	info.SystemTime = time.Now().Format("2006-01-02 15:04:05 MST")

	// NTP status
	info.NTPService, info.NTPSynced = detectNTP()

	return info, nil
}

func GetRouterPackages() ([]RouterPackage, error) {
	// Query specific packages relevant to the router
	targets := []string{
		"dnsmasq", "iptables", "ip6tables", "iproute2",
		"ppp", "pppoe", "openntpd", "chrony",
		"linux-pam", "busybox",
	}

	var pkgs []RouterPackage
	for _, name := range targets {
		out, err := exec.Command("apk", "info", "-e", name).Output()
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(out)) == "" {
			continue
		}
		// Get version
		ver := ""
		if vout, err := exec.Command("apk", "info", "-v", name).Output(); err == nil {
			line := strings.TrimSpace(string(vout))
			// apk info -v returns "name-version" format
			if idx := strings.LastIndex(line, "-"); idx > 0 {
				ver = line[idx+1:]
			}
		}
		pkgs = append(pkgs, RouterPackage{Name: name, Version: ver})
	}
	return pkgs, nil
}

var (
	ntpMu      sync.Mutex
	ntpService string
	ntpSynced  bool
	ntpChecked time.Time
)

func detectNTP() (string, bool) {
	ntpMu.Lock()
	defer ntpMu.Unlock()
	if !ntpChecked.IsZero() && time.Since(ntpChecked) < 5*time.Minute {
		return ntpService, ntpSynced
	}
	services := []string{"openntpd", "chronyd", "ntpd"}
	for _, svc := range services {
		out, err := exec.Command("rc-service", svc, "status").Output()
		if err != nil {
			continue
		}
		if strings.Contains(string(out), "started") {
			ntpService, ntpSynced = svc, true
			ntpChecked = time.Now()
			return ntpService, ntpSynced
		}
	}
	ntpService, ntpSynced = "none", false
	ntpChecked = time.Now()
	return ntpService, ntpSynced
}

func formatUptime(secs int64) string {
	d := secs / 86400
	h := (secs % 86400) / 3600
	m := (secs % 3600) / 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh %dm", d, h, m)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}
