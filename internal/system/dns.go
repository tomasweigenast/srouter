package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type UpstreamServer struct {
	Address string
}

type LocalEntry struct {
	Hostname string
	IP       string
}

func GetUpstreamServers() ([]UpstreamServer, error) {
	f, err := os.Open(dnsmasqConf)
	if err != nil {
		return nil, fmt.Errorf("open dnsmasq.conf: %w", err)
	}
	defer f.Close()

	var servers []UpstreamServer
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "server=") {
			addr := strings.TrimPrefix(line, "server=")
			servers = append(servers, UpstreamServer{Address: addr})
		}
	}
	return servers, nil
}

func SetUpstreamServers(servers []UpstreamServer) error {
	f, err := os.Open(dnsmasqConf)
	if err != nil {
		return fmt.Errorf("open dnsmasq.conf: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(strings.TrimSpace(line), "server=") {
			lines = append(lines, line)
		}
	}
	f.Close()

	for _, s := range servers {
		lines = append(lines, "server="+s.Address)
	}

	if err := os.WriteFile(dnsmasqConf, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("write dnsmasq.conf: %w", err)
	}
	return ReloadDNSMasq()
}

func GetLocalEntries() ([]LocalEntry, error) {
	f, err := os.Open(dnsmasqConf)
	if err != nil {
		return nil, fmt.Errorf("open dnsmasq.conf: %w", err)
	}
	defer f.Close()

	var entries []LocalEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "address=/") {
			// address=/hostname/ip
			val := strings.TrimPrefix(line, "address=/")
			parts := strings.SplitN(val, "/", 2)
			if len(parts) == 2 {
				entries = append(entries, LocalEntry{Hostname: parts[0], IP: parts[1]})
			}
		}
	}
	return entries, nil
}

func AddLocalEntry(e LocalEntry) error {
	f, err := os.OpenFile(dnsmasqConf, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open dnsmasq.conf: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "address=/%s/%s\n", e.Hostname, e.IP)
	return err
}

func DeleteLocalEntry(hostname string) error {
	f, err := os.Open(dnsmasqConf)
	if err != nil {
		return fmt.Errorf("open dnsmasq.conf: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		prefix := fmt.Sprintf("address=/%s/", hostname)
		if !strings.HasPrefix(strings.TrimSpace(line), prefix) {
			lines = append(lines, line)
		}
	}
	f.Close()

	return os.WriteFile(dnsmasqConf, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// TestLookup resolves hostname against a configured upstream DNS server directly,
// bypassing the local dnsmasq cache so the test verifies upstream reachability.
func TestLookup(hostname string) (string, string, error) {
	upstream := "8.8.8.8"
	if servers, err := GetUpstreamServers(); err == nil && len(servers) > 0 {
		upstream = servers[0].Address
	}

	out, err := exec.Command("dig", "+short", hostname, "@"+upstream).Output()
	if err != nil {
		// Fallback to nslookup
		out2, err2 := exec.Command("nslookup", hostname, upstream).Output()
		if err2 != nil {
			return "", upstream, fmt.Errorf("lookup failed: %w", err)
		}
		return strings.TrimSpace(string(out2)), upstream, nil
	}
	return strings.TrimSpace(string(out)), upstream, nil
}
