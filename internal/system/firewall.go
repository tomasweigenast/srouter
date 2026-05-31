package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	firewallScript  = "/etc/firewall.sh"
	firewallDDir    = "/etc/firewall.d"
	portForwardFile = "/etc/firewall.d/50-portforward.sh"
)

type FirewallRule struct {
	Chain    string
	Protocol string
	SrcIP    string
	DstIP    string
	DstPort  string
	Action   string
}

type FirewallFile struct {
	Name    string // basename, e.g. "50-portforward.sh"
	Path    string // full path
	Content string
}

type PortForwardRule struct {
	Name     string
	Protocol string
	ExtPort  string
	IntIP    string
	IntPort  string
}

// GetFirewallScript returns the content of the orchestrator script /etc/firewall.sh.
func GetFirewallScript() (string, error) {
	b, err := os.ReadFile(firewallScript)
	if err != nil {
		return "", fmt.Errorf("read firewall script: %w", err)
	}
	return string(b), nil
}

// GetFirewallFiles returns all .sh files in /etc/firewall.d/ with their contents, sorted by name.
func GetFirewallFiles() ([]FirewallFile, error) {
	entries, err := os.ReadDir(firewallDDir)
	if os.IsNotExist(err) {
		return []FirewallFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", firewallDDir, err)
	}

	var files []FirewallFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		path := filepath.Join(firewallDDir, e.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			content = []byte{}
		}
		files = append(files, FirewallFile{
			Name:    e.Name(),
			Path:    path,
			Content: string(content),
		})
	}
	return files, nil
}

// GetFirewallFileContent reads a single file by basename from /etc/firewall.d/.
// Rejects names with path separators to prevent traversal.
func GetFirewallFileContent(name string) (FirewallFile, error) {
	if err := validateFirewallFileName(name); err != nil {
		return FirewallFile{}, err
	}
	path := filepath.Join(firewallDDir, name)
	b, err := os.ReadFile(path)
	if err != nil {
		return FirewallFile{}, fmt.Errorf("read %s: %w", path, err)
	}
	return FirewallFile{Name: name, Path: path, Content: string(b)}, nil
}

// SaveFirewallScript writes content to a file in /etc/firewall.d/ identified by basename.
func SaveFirewallScript(name, content string) error {
	if err := validateFirewallFileName(name); err != nil {
		return err
	}
	path := filepath.Join(firewallDDir, name)
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// ApplyFirewall executes the orchestrator script which loads all firewall.d/ files.
func ApplyFirewall() error {
	out, err := exec.Command(firewallScript).CombinedOutput()
	if err != nil {
		return fmt.Errorf("apply firewall: %w\n%s", err, out)
	}
	return nil
}

func GetRulesFromKernel() ([]FirewallRule, error) {
	chains := []string{"INPUT", "FORWARD", "OUTPUT"}
	var rules []FirewallRule

	for _, chain := range chains {
		out, err := exec.Command("iptables", "-L", chain, "-n", "--line-numbers").Output()
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		scanner.Scan() // skip "Chain X" line
		scanner.Scan() // skip column headers
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			r := FirewallRule{Chain: chain, Action: fields[1], Protocol: fields[2]}
			if fields[3] != "--" {
				r.SrcIP = fields[3]
			}
			if len(fields) > 4 && fields[4] != "--" {
				r.DstIP = fields[4]
			}
			for i := 5; i < len(fields); i++ {
				if strings.HasPrefix(fields[i], "dpt:") {
					r.DstPort = strings.TrimPrefix(fields[i], "dpt:")
				}
			}
			rules = append(rules, r)
		}
	}
	return rules, nil
}

// GetPortForwardRules reads structured PF comments from /etc/firewall.d/50-portforward.sh.
// Convention: # PF: name|proto|extport|intip|intport
func GetPortForwardRules() ([]PortForwardRule, error) {
	f, err := os.Open(portForwardFile)
	if os.IsNotExist(err) {
		return []PortForwardRule{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open port forward file: %w", err)
	}
	defer f.Close()

	var rules []PortForwardRule
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "# PF: ") {
			continue
		}
		val := strings.TrimPrefix(line, "# PF: ")
		parts := strings.Split(val, "|")
		if len(parts) < 5 {
			continue
		}
		rules = append(rules, PortForwardRule{
			Name:     parts[0],
			Protocol: parts[1],
			ExtPort:  parts[2],
			IntIP:    parts[3],
			IntPort:  parts[4],
		})
	}
	return rules, nil
}

func AddPortForwardRule(r PortForwardRule) error {
	f, err := os.OpenFile(portForwardFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("open port forward file: %w", err)
	}
	defer f.Close()

	block := fmt.Sprintf(
		"\n# PF: %s|%s|%s|%s|%s\n"+
			"iptables -t nat -A PREROUTING -i ppp0 -p %s --dport %s -j DNAT --to-destination %s:%s\n"+
			"iptables -A FORWARD -i ppp0 -o lan -p %s -d %s --dport %s -m state --state NEW -j ACCEPT\n",
		r.Name, r.Protocol, r.ExtPort, r.IntIP, r.IntPort,
		r.Protocol, r.ExtPort, r.IntIP, r.IntPort,
		r.Protocol, r.IntIP, r.IntPort,
	)
	_, err = fmt.Fprint(f, block)
	return err
}

func DeletePortForwardRule(name string) error {
	f, err := os.Open(portForwardFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open port forward file: %w", err)
	}

	var (
		lines []string
		skip  bool
	)
	marker := fmt.Sprintf("# PF: %s|", name)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), marker) {
			skip = true
			continue
		}
		if skip && strings.Contains(line, "iptables") {
			// skip the two iptables lines following the PF comment
			skip = false
			continue
		}
		if skip {
			skip = false
		}
		lines = append(lines, line)
	}
	f.Close()

	return os.WriteFile(portForwardFile, []byte(strings.Join(lines, "\n")+"\n"), 0755)
}

func validateFirewallFileName(name string) error {
	if name == "" || strings.ContainsAny(name, "/\\") || !strings.HasSuffix(name, ".sh") {
		return fmt.Errorf("invalid firewall file name: %q", name)
	}
	// Ensure resolved path stays inside firewall.d/
	resolved := filepath.Join(firewallDDir, name)
	if !strings.HasPrefix(resolved, firewallDDir+"/") {
		return fmt.Errorf("invalid firewall file name: %q", name)
	}
	return nil
}
