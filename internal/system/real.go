package system

// RealSystem implements all system interfaces using the actual Linux filesystem,
// /proc, /sys, and router config files. Use in production on Alpine Linux.
type RealSystem struct{}

// Metrics
func (RealSystem) GetCPU() (CPUInfo, error)           { return GetCPU() }
func (RealSystem) GetMemory() (MemInfo, error)         { return GetMemory() }
func (RealSystem) GetDisks() ([]DiskInfo, error)       { return GetDisks() }
func (RealSystem) GetPPPoEStatus() (PPPoEStatus, error) { return GetPPPoEStatus() }
func (RealSystem) ReconnectPPPoE() error               { return ReconnectPPPoE() }

// DHCP
func (RealSystem) GetLeases() ([]Lease, error)                { return GetLeases() }
func (RealSystem) GetReservations() ([]Reservation, error)    { return GetReservations() }
func (RealSystem) AddReservation(r Reservation) error         { return AddReservation(r) }
func (RealSystem) DeleteReservation(mac string) error         { return DeleteReservation(mac) }
func (RealSystem) GetDHCPConfig() (DHCPConfig, error)         { return GetDHCPConfig() }
func (RealSystem) SaveDHCPConfig(cfg DHCPConfig) error        { return SaveDHCPConfig(cfg) }
func (RealSystem) ReloadDNSMasq() error                       { return ReloadDNSMasq() }

// DNS
func (RealSystem) GetUpstreamServers() ([]UpstreamServer, error)      { return GetUpstreamServers() }
func (RealSystem) SetUpstreamServers(s []UpstreamServer) error        { return SetUpstreamServers(s) }
func (RealSystem) GetLocalEntries() ([]LocalEntry, error)             { return GetLocalEntries() }
func (RealSystem) AddLocalEntry(e LocalEntry) error                   { return AddLocalEntry(e) }
func (RealSystem) DeleteLocalEntry(hostname string) error             { return DeleteLocalEntry(hostname) }
func (RealSystem) TestLookup(hostname string) (string, error)         { return TestLookup(hostname) }
func (RealSystem) Reload() error                                       { return ReloadDNSMasq() }

// Network
func (RealSystem) GetInterfaces() ([]Interface, error)           { return GetInterfaces() }
func (RealSystem) GetARPTable() ([]ARPEntry, error)              { return GetARPTable() }
func (RealSystem) GetRoutes() ([]Route, error)                   { return GetRoutes() }
func (RealSystem) GetConntrackStats() (ConntrackStats, error)    { return GetConntrackStats() }

// Firewall
func (RealSystem) GetFirewallFiles() ([]FirewallFile, error)                   { return GetFirewallFiles() }
func (RealSystem) GetFirewallFileContent(n string) (FirewallFile, error)       { return GetFirewallFileContent(n) }
func (RealSystem) SaveFirewallScript(name, content string) error               { return SaveFirewallScript(name, content) }
func (RealSystem) ApplyFirewall() error                                        { return ApplyFirewall() }
func (RealSystem) GetRulesFromKernel() ([]FirewallRule, error)                 { return GetRulesFromKernel() }
func (RealSystem) GetPortForwardRules() ([]PortForwardRule, error)             { return GetPortForwardRules() }
func (RealSystem) AddPortForwardRule(r PortForwardRule) error                  { return AddPortForwardRule(r) }
func (RealSystem) DeletePortForwardRule(name string) error                     { return DeletePortForwardRule(name) }
func (RealSystem) GetCustomRules() ([]FirewallRule, error)                     { return GetCustomRules() }
func (RealSystem) AddCustomRule(r FirewallRule) error                          { return AddCustomRule(r) }
func (RealSystem) DeleteCustomRule(index int) error                            { return DeleteCustomRule(index) }

// WoL
func (RealSystem) SendMagicPacket(mac string) error { return SendMagicPacket(mac) }
