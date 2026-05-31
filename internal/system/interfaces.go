package system

// Metrics provides router hardware and connectivity status.
type Metrics interface {
	GetCPU() (CPUInfo, error)
	GetMemory() (MemInfo, error)
	GetDisks() ([]DiskInfo, error)
	GetPPPoEStatus() (PPPoEStatus, error)
	ReconnectPPPoE() error
}

// DHCP provides DHCP lease and reservation management via dnsmasq.
type DHCP interface {
	GetLeases() ([]Lease, error)
	GetReservations() ([]Reservation, error)
	AddReservation(Reservation) error
	DeleteReservation(mac string) error
	GetDHCPConfig() (DHCPConfig, error)
	SaveDHCPConfig(DHCPConfig) error
	ReloadDNSMasq() error
}

// DNS provides DNS upstream and local entry management via dnsmasq.
type DNS interface {
	GetUpstreamServers() ([]UpstreamServer, error)
	SetUpstreamServers([]UpstreamServer) error
	GetLocalEntries() ([]LocalEntry, error)
	AddLocalEntry(LocalEntry) error
	DeleteLocalEntry(hostname string) error
	TestLookup(hostname string) (string, error)
	Reload() error
}

// Network provides read-only network interface, ARP, routing and conntrack data.
type Network interface {
	GetInterfaces() ([]Interface, error)
	GetARPTable() ([]ARPEntry, error)
	GetRoutes() ([]Route, error)
	GetConntrackStats() (ConntrackStats, error)
}

// Firewall provides firewall script management and port forwarding CRUD.
type Firewall interface {
	GetFirewallFiles() ([]FirewallFile, error)
	GetFirewallFileContent(name string) (FirewallFile, error)
	SaveFirewallScript(name, content string) error
	ApplyFirewall() error
	GetRulesFromKernel() ([]FirewallRule, error)
	GetPortForwardRules() ([]PortForwardRule, error)
	AddPortForwardRule(PortForwardRule) error
	DeletePortForwardRule(name string) error
}

// WoL sends Wake-on-LAN magic packets to LAN devices.
type WoL interface {
	SendMagicPacket(mac string) error
}

// LogStream tails a log source and fans out filtered lines to subscribers.
// LogBroadcaster satisfies this interface.
type LogStream interface {
	Subscribe(filter LogFilter) (<-chan LogLine, func())
	Stop()
}

// BandwidthStream samples network interface traffic and fans out to subscribers.
// Broadcaster satisfies this interface.
type BandwidthStream interface {
	Subscribe() (<-chan BandwidthSample, func())
	Stop()
}
