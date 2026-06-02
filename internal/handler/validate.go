package handler

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/tomasweigenast/srouter/internal/system"
)

var (
	macRE      = regexp.MustCompile(`^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$`)
	ifaceRE    = regexp.MustCompile(`^[a-z][a-z0-9_.@-]{0,14}$`)
	pfNameRE   = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)
	hostnameRE = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?$`)
)

var (
	validChains    = map[string]bool{"INPUT": true, "FORWARD": true, "OUTPUT": true}
	validActions   = map[string]bool{"ACCEPT": true, "DROP": true, "REJECT": true, "LOG": true}
	validProtocols = map[string]bool{"tcp": true, "udp": true, "icmp": true, "all": true}
	validPFProtos  = map[string]bool{"tcp": true, "udp": true}
)

func validateFirewallRule(r system.FirewallRule) error {
	if !validChains[r.Chain] {
		return fmt.Errorf("invalid chain %q: must be INPUT, FORWARD, or OUTPUT", r.Chain)
	}
	if !validActions[r.Action] {
		return fmt.Errorf("invalid action %q: must be ACCEPT, DROP, REJECT, or LOG", r.Action)
	}
	if r.Protocol != "" && !validProtocols[r.Protocol] {
		return fmt.Errorf("invalid protocol %q: must be tcp, udp, icmp, or all", r.Protocol)
	}
	if r.SrcIP != "" {
		if err := validateIPOrCIDR(r.SrcIP); err != nil {
			return fmt.Errorf("invalid source IP: %w", err)
		}
	}
	if r.DstIP != "" {
		if err := validateIPOrCIDR(r.DstIP); err != nil {
			return fmt.Errorf("invalid destination IP: %w", err)
		}
	}
	if r.SrcPort != "" {
		if err := validatePort(r.SrcPort); err != nil {
			return fmt.Errorf("invalid source port: %w", err)
		}
	}
	if r.DstPort != "" {
		if err := validatePort(r.DstPort); err != nil {
			return fmt.Errorf("invalid destination port: %w", err)
		}
	}
	if r.InIface != "" && !ifaceRE.MatchString(r.InIface) {
		return fmt.Errorf("invalid input interface %q", r.InIface)
	}
	if r.OutIface != "" && !ifaceRE.MatchString(r.OutIface) {
		return fmt.Errorf("invalid output interface %q", r.OutIface)
	}
	if strings.ContainsAny(r.Comment, "\n\r") {
		return fmt.Errorf("comment must not contain newlines")
	}
	return nil
}

func validatePortForwardRule(r system.PortForwardRule) error {
	if !pfNameRE.MatchString(r.Name) {
		return fmt.Errorf("invalid name %q: use only letters, digits, hyphens, and underscores (1-50 chars)", r.Name)
	}
	if !validPFProtos[r.Protocol] {
		return fmt.Errorf("invalid protocol %q: must be tcp or udp", r.Protocol)
	}
	if err := validatePort(r.ExtPort); err != nil {
		return fmt.Errorf("invalid external port: %w", err)
	}
	if err := validatePort(r.IntPort); err != nil {
		return fmt.Errorf("invalid internal port: %w", err)
	}
	ip := net.ParseIP(r.IntIP)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("invalid internal IPv4 address %q", r.IntIP)
	}
	return nil
}

func validateMAC(mac string) error {
	if !macRE.MatchString(mac) {
		return fmt.Errorf("invalid MAC address %q: expected XX:XX:XX:XX:XX:XX", mac)
	}
	return nil
}

// validateDNSServer validates a dnsmasq upstream server address (IP or hostname, optional #port suffix).
func validateDNSServer(addr string) error {
	host := addr
	if h, portStr, ok := strings.Cut(addr, "#"); ok {
		host = h
		n, err := strconv.Atoi(portStr)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("invalid port in %q", addr)
		}
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	if len(host) == 0 || len(host) > 253 || !hostnameRE.MatchString(host) {
		return fmt.Errorf("%q is not a valid IP address or hostname", host)
	}
	return nil
}

func validateIPOrCIDR(s string) error {
	if net.ParseIP(s) != nil {
		return nil
	}
	if _, _, err := net.ParseCIDR(s); err == nil {
		return nil
	}
	return fmt.Errorf("%q is not a valid IP address or CIDR", s)
}

// validatePort accepts a single port ("80") or a range ("80:443").
func validatePort(port string) error {
	parts := strings.SplitN(port, ":", 2)
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("port %q must be a number between 1 and 65535", p)
		}
	}
	return nil
}
