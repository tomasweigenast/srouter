package system

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type PPPoEStatus struct {
	Connected  bool
	PublicIP   string
	Iface      string
	UptimeSecs int64
}

func GetPPPoEStatus() (PPPoEStatus, error) {
	iface := "ppp0"
	status := PPPoEStatus{Iface: iface}

	// Check if ppp0 exists and is UP
	statePath := fmt.Sprintf("/sys/class/net/%s/operstate", iface)
	state, err := os.ReadFile(statePath)
	if err != nil {
		// Interface doesn't exist — not connected
		return status, nil
	}
	status.Connected = strings.TrimSpace(string(state)) == "up"
	if !status.Connected {
		return status, nil
	}

	// Get IP via `ip addr show ppp0`
	out, err := exec.Command("ip", "addr", "show", iface).Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "inet ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					status.PublicIP = strings.Split(parts[1], "/")[0]
				}
				break
			}
		}
	}

	// Estimate uptime from carrier_changes — approximate with ifindex file mtime
	ifindexPath := fmt.Sprintf("/sys/class/net/%s/ifindex", iface)
	if fi, err := os.Stat(ifindexPath); err == nil {
		status.UptimeSecs = int64(time.Since(fi.ModTime()).Seconds())
	}

	return status, nil
}

func ReconnectPPPoE() error {
	if err := exec.Command("rc-service", "pppoe", "restart").Run(); err != nil {
		return fmt.Errorf("restart pppoe: %w", err)
	}
	return nil
}
