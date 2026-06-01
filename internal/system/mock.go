package system

import (
	"log/slog"
	"math/rand/v2"
	"time"
)

// MockSystem implements all system interfaces with realistic fake data.
// Use when SROUTER_DEV_MODE=true to develop on macOS without a router.
type MockSystem struct{}

// ── Metrics ──────────────────────────────────────────────────────────────

func (MockSystem) GetCPU() (CPUInfo, error) {
	load1 := 0.42 + rand.Float64()*0.6 - 0.2  // 0.22 – 0.82
	load5 := load1*0.9 + rand.Float64()*0.05
	load15 := load5*0.95 + rand.Float64()*0.03
	cores := 2
	pct := min(load1/float64(cores)*100, 100)
	return CPUInfo{LoadAvg1: load1, LoadAvg5: load5, LoadAvg15: load15, Cores: cores, UsedPercent: pct}, nil
}

func (MockSystem) GetMemory() (MemInfo, error) {
	total := uint64(493568)
	used := uint64(90000 + rand.IntN(80000)) // 90–170 MB in KB
	avail := total - used
	pct := float64(used) / float64(total) * 100
	return MemInfo{TotalKB: total, AvailableKB: avail, UsedKB: used, UsedPercent: pct}, nil
}

func (MockSystem) GetDisks() ([]DiskInfo, error) {
	used := 888000000 + uint64(rand.IntN(10000000))
	total := uint64(10737418240)
	free := total - used
	return []DiskInfo{
		{Path: "/", TotalBytes: total, FreeBytes: free, UsedPercent: float64(used) / float64(total) * 100},
	}, nil
}

func (MockSystem) GetPPPoEStatus() (PPPoEStatus, error) {
	return PPPoEStatus{Connected: true, PublicIP: "200.123.54.72", Iface: "ppp0", UptimeSecs: 14523 + int64(time.Now().Unix()%3600)}, nil
}

func (MockSystem) ReconnectPPPoE() error {
	slog.Info("[mock] ReconnectPPPoE")
	return nil
}

func (MockSystem) CheckInternetConnectivity() (bool, error) { return true, nil }
func (MockSystem) ForceCheckInternet() (bool, error)        { return true, nil }

func (MockSystem) GetSystemInfo() (SystemInfo, error) {
	return SystemInfo{
		Hostname:   "router",
		Kernel:     "6.15.0-0-edge",
		Uptime:     "2d 4h 12m",
		SystemTime: "2026-05-31 12:00:00 ART",
		NTPSynced:  true,
		NTPService: "openntpd",
		AppVersion: AppVersion,
	}, nil
}

func (MockSystem) GetRouterPackages() ([]RouterPackage, error) {
	return []RouterPackage{
		{Name: "dnsmasq", Version: "2.91-r0"},
		{Name: "iptables", Version: "1.8.10-r3"},
		{Name: "iproute2", Version: "6.9.0-r0"},
		{Name: "ppp", Version: "2.5.0-r0"},
		{Name: "openntpd", Version: "6.8p1-r7"},
		{Name: "busybox", Version: "1.37.0-r9"},
	}, nil
}

// ── DHCP ─────────────────────────────────────────────────────────────────

func (MockSystem) GetLeases() ([]Lease, error) {
	return []Lease{
		{IP: "192.168.0.101", MAC: "aa:bb:cc:dd:ee:01", Hostname: "my-laptop", Expiry: time.Now().Add(12 * time.Hour)},
		{IP: "192.168.0.102", MAC: "aa:bb:cc:dd:ee:02", Hostname: "my-phone", Expiry: time.Now().Add(8 * time.Hour)},
		{IP: "192.168.0.103", MAC: "aa:bb:cc:dd:ee:03", Hostname: "", Expiry: time.Now().Add(3 * time.Hour)},
	}, nil
}

func (MockSystem) GetReservations() ([]Reservation, error) {
	return []Reservation{
		{MAC: "aa:bb:cc:dd:ee:10", IP: "192.168.0.10", Hostname: "servidor-minecraft"},
		{MAC: "aa:bb:cc:dd:ee:05", IP: "192.168.0.5", Hostname: "access-point"},
	}, nil
}

func (MockSystem) AddReservation(r Reservation) error {
	slog.Info("[mock] AddReservation", "mac", r.MAC, "ip", r.IP, "hostname", r.Hostname)
	return nil
}

func (MockSystem) DeleteReservation(mac string) error {
	slog.Info("[mock] DeleteReservation", "mac", mac)
	return nil
}

func (MockSystem) GetDHCPConfig() (DHCPConfig, error) {
	return DHCPConfig{RangeStart: "192.168.0.100", RangeEnd: "192.168.0.200", LeaseTime: "24h"}, nil
}

func (MockSystem) SaveDHCPConfig(cfg DHCPConfig) error {
	slog.Info("[mock] SaveDHCPConfig", "start", cfg.RangeStart, "end", cfg.RangeEnd)
	return nil
}

func (MockSystem) ReloadDNSMasq() error {
	slog.Info("[mock] ReloadDNSMasq")
	return nil
}

// ── DNS ──────────────────────────────────────────────────────────────────

func (MockSystem) GetUpstreamServers() ([]UpstreamServer, error) {
	return []UpstreamServer{{Address: "1.1.1.1"}, {Address: "8.8.8.8"}, {Address: "1.0.0.1"}}, nil
}

func (MockSystem) SetUpstreamServers(servers []UpstreamServer) error {
	slog.Info("[mock] SetUpstreamServers", "count", len(servers))
	return nil
}

func (MockSystem) GetLocalEntries() ([]LocalEntry, error) {
	return []LocalEntry{
		{Hostname: "minecraft.local", IP: "192.168.0.10"},
		{Hostname: "router.local", IP: "192.168.0.1"},
	}, nil
}

func (MockSystem) AddLocalEntry(e LocalEntry) error {
	slog.Info("[mock] AddLocalEntry", "hostname", e.Hostname, "ip", e.IP)
	return nil
}

func (MockSystem) DeleteLocalEntry(hostname string) error {
	slog.Info("[mock] DeleteLocalEntry", "hostname", hostname)
	return nil
}

func (MockSystem) TestLookup(hostname string) (string, string, error) {
	return "93.184.216.34\n2600:1406:3a00::6812:263e (mock)", "1.1.1.1", nil
}

func (MockSystem) Reload() error {
	slog.Info("[mock] Reload DNS")
	return nil
}

// ── Network ──────────────────────────────────────────────────────────────

func (MockSystem) GetInterfaces() ([]Interface, error) {
	return []Interface{
		{Name: "lan", MAC: "bc:24:11:12:23:d9", IPv4: "192.168.0.1/24", State: "up", MTU: 1500, RxBytes: 1234567890, TxBytes: 987654321},
		{Name: "ppp0", MAC: "", IPv4: "200.123.54.72/32", State: "up", MTU: 1492, RxBytes: 234567890, TxBytes: 123456789},
		{Name: "lo", MAC: "00:00:00:00:00:00", IPv4: "127.0.0.1/8", State: "unknown", MTU: 65536, RxBytes: 12345, TxBytes: 12345},
	}, nil
}

func (MockSystem) GetARPTable() ([]ARPEntry, error) {
	return []ARPEntry{
		{IP: "192.168.0.101", MAC: "aa:bb:cc:dd:ee:01", Iface: "lan"},
		{IP: "192.168.0.102", MAC: "aa:bb:cc:dd:ee:02", Iface: "lan"},
		{IP: "192.168.0.10", MAC: "aa:bb:cc:dd:ee:10", Iface: "lan"},
	}, nil
}

func (MockSystem) GetRoutes() ([]Route, error) {
	return []Route{
		{Destination: "default", Gateway: "100.100.0.1", Iface: "ppp0"},
		{Destination: "192.168.0.0/24", Iface: "lan"},
		{Destination: "100.100.0.1", Iface: "ppp0"},
	}, nil
}

func (MockSystem) GetConntrackStats() (ConntrackStats, error) {
	return ConntrackStats{Current: 42, Max: 65536}, nil
}

// ── Firewall ─────────────────────────────────────────────────────────────

func (MockSystem) GetFirewallFiles() ([]FirewallFile, error) {
	files := []FirewallFile{
		{Name: "00-flush.sh", Path: "/etc/firewall.d/00-flush.sh", Content: "#!/bin/sh\niptables -F\niptables -X\n"},
		{Name: "10-politicas.sh", Path: "/etc/firewall.d/10-politicas.sh", Content: "#!/bin/sh\niptables -P INPUT DROP\niptables -P FORWARD DROP\niptables -P OUTPUT ACCEPT\n"},
		{Name: "50-portforward.sh", Path: "/etc/firewall.d/50-portforward.sh", Content: "#!/bin/sh\n# PF: minecraft|tcp|25565|192.168.0.10|25565\niptables -t nat -A PREROUTING -i ppp0 -p tcp --dport 25565 -j DNAT --to-destination 192.168.0.10:25565\n"},
	}
	return files, nil
}

func (MockSystem) GetFirewallFileContent(name string) (FirewallFile, error) {
	files, _ := MockSystem{}.GetFirewallFiles()
	for _, f := range files {
		if f.Name == name {
			return f, nil
		}
	}
	return FirewallFile{Name: name, Path: "/etc/firewall.d/" + name, Content: "#!/bin/sh\n"}, nil
}

func (MockSystem) SaveFirewallScript(name, content string) error {
	slog.Info("[mock] SaveFirewallScript", "name", name, "bytes", len(content))
	return nil
}

func (MockSystem) ApplyFirewall() error {
	slog.Info("[mock] ApplyFirewall")
	return nil
}

func (MockSystem) GetRulesFromKernel() ([]FirewallRule, error) {
	return []FirewallRule{
		{Chain: "INPUT", Action: "ACCEPT", Protocol: "all"},
		{Chain: "INPUT", Action: "DROP", Protocol: "all", SrcIP: "0.0.0.0/0"},
		{Chain: "FORWARD", Action: "ACCEPT", Protocol: "tcp", DstIP: "192.168.0.10", DstPort: "25565"},
		{Chain: "FORWARD", Action: "ACCEPT", Protocol: "all"},
	}, nil
}

func (MockSystem) GetPortForwardRules() ([]PortForwardRule, error) {
	return []PortForwardRule{
		{Name: "minecraft", Protocol: "tcp", ExtPort: "25565", IntIP: "192.168.0.10", IntPort: "25565"},
		{Name: "voicechat", Protocol: "udp", ExtPort: "24454", IntIP: "192.168.0.10", IntPort: "24454"},
	}, nil
}

func (MockSystem) AddPortForwardRule(r PortForwardRule) error {
	slog.Info("[mock] AddPortForwardRule", "name", r.Name, "proto", r.Protocol, "port", r.ExtPort)
	return nil
}

func (MockSystem) DeletePortForwardRule(name string) error {
	slog.Info("[mock] DeletePortForwardRule", "name", name)
	return nil
}

var mockCustomRules = []FirewallRule{
	{Chain: "INPUT", Protocol: "tcp", DstPort: "8080", Action: "ACCEPT"},
}

func (MockSystem) GetCustomRules() ([]FirewallRule, error) {
	out := make([]FirewallRule, len(mockCustomRules))
	copy(out, mockCustomRules)
	return out, nil
}
func (MockSystem) AddCustomRule(r FirewallRule) error {
	slog.Info("[mock] AddCustomRule", "chain", r.Chain, "action", r.Action)
	mockCustomRules = append(mockCustomRules, r)
	return nil
}
func (MockSystem) DeleteCustomRule(index int) error {
	slog.Info("[mock] DeleteCustomRule", "index", index)
	if index >= 0 && index < len(mockCustomRules) {
		mockCustomRules = append(mockCustomRules[:index], mockCustomRules[index+1:]...)
	}
	return nil
}

// ── WoL ──────────────────────────────────────────────────────────────────

func (MockSystem) SendMagicPacket(mac string) error {
	slog.Info("[mock] SendMagicPacket", "mac", mac)
	return nil
}

// NewMockDNSStatsCollector returns a collector seeded with plausible fake data.
func NewMockDNSStatsCollector() *DNSStatsCollector {
	c := &DNSStatsCollector{
		byUpstream: map[string]int64{"1.1.1.1": 312, "8.8.8.8": 87},
		byDomain: map[string]int64{
			"google.com": 142, "youtube.com": 98, "spotify.com": 76,
			"github.com": 54, "cloudflare.com": 43, "reddit.com": 31,
		},
		queries:   842,
		cacheHits: 399,
		forwarded: 399,
		stopCh:    make(chan struct{}),
	}
	return c
}

// ── Mock SSE streams ──────────────────────────────────────────────────────

// MockLogStream emits realistic fake log lines every 2 seconds.
type MockLogStream struct {
	stopCh chan struct{}
}

func NewMockLogStream() *MockLogStream {
	return &MockLogStream{stopCh: make(chan struct{})}
}

func (m *MockLogStream) Subscribe(filter LogFilter) (<-chan LogLine, func()) {
	ch := make(chan LogLine, 8)
	stop := make(chan struct{})

	rawLines := []string{
		"Jun  1 12:00:01 router dnsmasq-dhcp: DHCPDISCOVER(lan) aa:bb:cc:dd:ee:01",
		"Jun  1 12:00:05 router kernel: [FW-INPUT-DROP: ] IN=ppp0 OUT= SRC=1.2.3.4 PROTO=TCP DPT=22",
		"Jun  1 12:00:10 router dnsmasq[1234]: cached google.com is 142.250.64.14",
		"Jun  1 12:00:15 router pppd[999]: LCP: timeout sending Config-Requests",
		"Jun  1 12:00:20 router dnsmasq-dhcp: DHCPACK(lan) 192.168.0.101 aa:bb:cc:dd:ee:01 my-laptop",
		"Jun  1 12:00:25 router kernel: [MC-CONNECT: ] IN=ppp0 SRC=203.0.113.45 DST=200.123.54.72 PROTO=TCP DPT=25565",
		"Jun  1 12:00:30 router dnsmasq[1234]: query[A] youtube.com from 192.168.0.101",
	}

	go func() {
		idx := 0
		for {
			select {
			case <-stop:
				close(ch)
				return
			case <-time.After(2 * time.Second):
				line := ParseLogLine(rawLines[idx%len(rawLines)])
				if matchesFilter(line, filter) {
					select {
					case ch <- line:
					default:
					}
				}
				idx++
			}
		}
	}()

	return ch, func() { close(stop) }
}

func (m *MockLogStream) Stop() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
}

// MockBandwidthStream emits fake rx/tx samples simulating realistic traffic:
// smooth baseline with occasional download/upload bursts.
type MockBandwidthStream struct{}

func (MockBandwidthStream) Subscribe() (<-chan BandwidthSample, func()) {
	ch := make(chan BandwidthSample, 8)
	stop := make(chan struct{})

	go func() {
		// Smoothed current values per interface
		state := map[string][2]float64{
			"ppp0": {20000, 5000},
			"lan":  {50000, 30000},
		}
		// Burst targets: occasionally spike to simulate a download or upload
		burstTick := 0
		for {
			select {
			case <-stop:
				close(ch)
				return
			case <-time.After(500 * time.Millisecond):
				burstTick++
				ts := time.Now().UnixMilli()

				// Every ~8 ticks (4s) chance of a burst
				var burstRx, burstTx float64
				if burstTick%8 == 0 {
					burstRx = rand.Float64() * 800000 // up to ~800 KB/s download spike
					burstTx = rand.Float64() * 150000
				}

				for iface, s := range state {
					// Exponential smoothing toward a slowly drifting base + burst
					baseRx := 15000 + rand.Float64()*30000 + burstRx
					baseTx := 3000 + rand.Float64()*15000 + burstTx
					// α=0.3: fast enough to be visible, slow enough to look smooth
					newRx := s[0]*0.7 + baseRx*0.3
					newTx := s[1]*0.7 + baseTx*0.3
					state[iface] = [2]float64{newRx, newTx}

					select {
					case ch <- BandwidthSample{
						Timestamp: ts,
						Iface:     iface,
						RxBps:     newRx,
						TxBps:     newTx,
					}:
					default:
					}
				}
			}
		}
	}()

	return ch, func() { close(stop) }
}

func (MockBandwidthStream) Stop() {}
