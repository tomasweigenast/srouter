package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Interface struct {
	Name    string
	MAC     string
	IPv4    string
	State   string
	MTU     int
	RxBytes uint64
	TxBytes uint64
}

type ARPEntry struct {
	IP    string
	MAC   string
	Iface string
}

type Route struct {
	Destination string
	Gateway     string
	Iface       string
	Flags       string
	Metric      int
}

type ConntrackStats struct {
	Current int
	Max     int
}

func GetInterfaces() ([]Interface, error) {
	// Read /proc/net/dev for byte counters
	rxTx := map[string][2]uint64{}
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("open /proc/net/dev: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header 1
	scanner.Scan() // skip header 2
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 10 {
			continue
		}
		name := strings.TrimSuffix(parts[0], ":")
		rx, _ := strconv.ParseUint(parts[1], 10, 64)
		tx, _ := strconv.ParseUint(parts[9], 10, 64)
		rxTx[name] = [2]uint64{rx, tx}
	}

	// Enumerate interfaces via /sys/class/net
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil, fmt.Errorf("read /sys/class/net: %w", err)
	}

	ifaces := []Interface{}
	for _, e := range entries {
		name := e.Name()
		iface := Interface{Name: name}

		readSys := func(sub string) string {
			b, _ := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/%s", name, sub))
			return strings.TrimSpace(string(b))
		}

		iface.MAC = readSys("address")
		iface.State = readSys("operstate")
		if mtu, err := strconv.Atoi(readSys("mtu")); err == nil {
			iface.MTU = mtu
		}

		if counters, ok := rxTx[name]; ok {
			iface.RxBytes = counters[0]
			iface.TxBytes = counters[1]
		}

		// Get IPv4 via ip addr
		out, err := exec.Command("ip", "-4", "addr", "show", name).Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "inet ") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						iface.IPv4 = parts[1]
					}
					break
				}
			}
		}

		ifaces = append(ifaces, iface)
	}

	return ifaces, nil
}

func GetARPTable() ([]ARPEntry, error) {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil, fmt.Errorf("open /proc/net/arp: %w", err)
	}
	defer f.Close()

	entries := []ARPEntry{}
	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 {
			continue
		}
		// Skip incomplete entries (0x0 flag)
		if fields[2] == "0x0" {
			continue
		}
		entries = append(entries, ARPEntry{
			IP:    fields[0],
			MAC:   fields[3],
			Iface: fields[5],
		})
	}
	return entries, nil
}

func GetRoutes() ([]Route, error) {
	out, err := exec.Command("ip", "route", "show").Output()
	if err != nil {
		return nil, fmt.Errorf("ip route show: %w", err)
	}

	routes := []Route{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		r := Route{}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		r.Destination = fields[0]
		for i := 1; i < len(fields); i++ {
			switch fields[i] {
			case "via":
				if i+1 < len(fields) {
					r.Gateway = fields[i+1]
				}
			case "dev":
				if i+1 < len(fields) {
					r.Iface = fields[i+1]
				}
			case "metric":
				if i+1 < len(fields) {
					r.Metric, _ = strconv.Atoi(fields[i+1])
				}
			}
		}
		routes = append(routes, r)
	}
	return routes, nil
}

func CheckInternetConnectivity() (bool, error) {
	err := exec.Command("ping", "-c", "1", "-W", "2", "8.8.8.8").Run()
	return err == nil, nil
}

func GetConntrackStats() (ConntrackStats, error) {
	readInt := func(path string) int {
		b, err := os.ReadFile(path)
		if err != nil {
			return 0
		}
		v, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		return v
	}
	return ConntrackStats{
		Current: readInt("/proc/sys/net/netfilter/nf_conntrack_count"),
		Max:     readInt("/proc/sys/net/netfilter/nf_conntrack_max"),
	}, nil
}
