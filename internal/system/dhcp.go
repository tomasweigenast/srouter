package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Lease struct {
	Expiry   time.Time
	MAC      string
	IP       string
	Hostname string
}

type Reservation struct {
	MAC      string
	IP       string
	Hostname string
}

type DHCPConfig struct {
	RangeStart string
	RangeEnd   string
	LeaseTime  string
}

const (
	leasesFile      = "/var/lib/misc/dnsmasq.leases"
	reservationsFile = "/etc/dnsmasq.d/reservas.conf"
	dnsmasqConf     = "/etc/dnsmasq.conf"
)

func GetLeases() ([]Lease, error) {
	f, err := os.Open(leasesFile)
	if os.IsNotExist(err) {
		return []Lease{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open leases: %w", err)
	}
	defer f.Close()

	var leases []Lease
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		expiry, _ := strconv.ParseInt(fields[0], 10, 64)
		hostname := fields[3]
		if hostname == "*" {
			hostname = ""
		}
		leases = append(leases, Lease{
			Expiry:   time.Unix(expiry, 0),
			MAC:      fields[1],
			IP:       fields[2],
			Hostname: hostname,
		})
	}
	return leases, nil
}

func GetReservations() ([]Reservation, error) {
	f, err := os.Open(reservationsFile)
	if os.IsNotExist(err) {
		return []Reservation{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open reservations: %w", err)
	}
	defer f.Close()

	var reservations []Reservation
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "dhcp-host=")
		parts := strings.SplitN(line, ",", 3)
		if len(parts) < 2 {
			continue
		}
		r := Reservation{MAC: parts[0], IP: parts[1]}
		if len(parts) == 3 {
			r.Hostname = parts[2]
		}
		reservations = append(reservations, r)
	}
	return reservations, nil
}

func AddReservation(r Reservation) error {
	f, err := os.OpenFile(reservationsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open reservations: %w", err)
	}
	defer f.Close()
	line := fmt.Sprintf("dhcp-host=%s,%s,%s\n", r.MAC, r.IP, r.Hostname)
	if _, err := fmt.Fprint(f, line); err != nil {
		return fmt.Errorf("write reservation: %w", err)
	}
	return nil
}

func DeleteReservation(mac string) error {
	f, err := os.Open(reservationsFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open reservations: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(strings.ToLower(line), strings.ToLower(mac)) {
			lines = append(lines, line)
		}
	}
	f.Close()

	return os.WriteFile(reservationsFile, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func GetDHCPConfig() (DHCPConfig, error) {
	f, err := os.Open(dnsmasqConf)
	if err != nil {
		return DHCPConfig{}, fmt.Errorf("open dnsmasq.conf: %w", err)
	}
	defer f.Close()

	var cfg DHCPConfig
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "dhcp-range=") {
			val := strings.TrimPrefix(line, "dhcp-range=")
			parts := strings.Split(val, ",")
			if len(parts) >= 2 {
				cfg.RangeStart = parts[0]
				cfg.RangeEnd = parts[1]
			}
			if len(parts) >= 3 {
				cfg.LeaseTime = parts[2]
			}
		}
	}
	return cfg, nil
}

func SaveDHCPConfig(cfg DHCPConfig) error {
	f, err := os.Open(dnsmasqConf)
	if err != nil {
		return fmt.Errorf("open dnsmasq.conf: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(strings.TrimSpace(line), "dhcp-range=") {
			lines = append(lines, line)
		}
	}
	f.Close()

	leaseTime := cfg.LeaseTime
	if leaseTime == "" {
		leaseTime = "24h"
	}
	lines = append(lines, fmt.Sprintf("dhcp-range=%s,%s,%s", cfg.RangeStart, cfg.RangeEnd, leaseTime))
	return os.WriteFile(dnsmasqConf, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func ReloadDNSMasq() error {
	// Send SIGHUP directly to avoid OpenRC dependency chain (rc-service dnsmasq
	// reload would try to stop services that depend on dnsmasq, including srouter).
	out, err := exec.Command("pidof", "dnsmasq").Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return fmt.Errorf("dnsmasq not running")
	}
	pid := strings.TrimSpace(strings.Fields(string(out))[0])
	if err := exec.Command("kill", "-HUP", pid).Run(); err != nil {
		return fmt.Errorf("reload dnsmasq: %w", err)
	}
	return nil
}
