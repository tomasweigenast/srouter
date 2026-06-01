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
	customRulesFile = "/etc/firewall.d/99-custom.sh"
)

type FirewallRule struct {
	Chain    string
	Protocol string
	InIface  string // -i interface
	OutIface string // -o interface
	SrcIP    string
	DstIP    string
	SrcPort  string
	DstPort  string
	Action   string
	Comment  string // stored as a # comment above the iptables line
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

// GetCustomRules parses iptables commands from 99-custom.sh into FirewallRule slice.
// Each line must match: iptables -A CHAIN ... -j ACTION
func GetCustomRules() ([]FirewallRule, error) {
	data, err := os.ReadFile(customRulesFile)
	if os.IsNotExist(err) {
		return []FirewallRule{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read custom rules: %w", err)
	}
	var rules []FirewallRule
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "iptables") {
			continue
		}
		r := parseIPTablesLine(line)
		if r.Chain != "" {
			rules = append(rules, r)
		}
	}
	return rules, nil
}

func AddCustomRule(r FirewallRule) error {
	cmd := buildIPTablesCommand(r)
	if cmd == "" {
		return fmt.Errorf("invalid rule")
	}
	// Ensure file exists with shebang
	if _, err := os.Stat(customRulesFile); os.IsNotExist(err) {
		if err := os.WriteFile(customRulesFile, []byte("#!/bin/sh\n"), 0755); err != nil {
			return fmt.Errorf("create custom rules file: %w", err)
		}
	}
	f, err := os.OpenFile(customRulesFile, os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("open custom rules: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", buildIPTablesCommandWithComment(r))
	return err
}

func DeleteCustomRule(index int) error {
	data, err := os.ReadFile(customRulesFile)
	if err != nil {
		return fmt.Errorf("read custom rules: %w", err)
	}
	var keep []string
	ruleIdx := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "iptables") {
			if ruleIdx != index {
				keep = append(keep, line)
			}
			ruleIdx++
		} else {
			keep = append(keep, line)
		}
	}
	return os.WriteFile(customRulesFile, []byte(strings.Join(keep, "\n")+"\n"), 0755)
}

func parseIPTablesLine(line string) FirewallRule {
	r := FirewallRule{}
	fields := strings.Fields(line)
	for i, f := range fields {
		next := func() string {
			if i+1 < len(fields) {
				return fields[i+1]
			}
			return ""
		}
		switch f {
		case "-A", "-I":
			r.Chain = next()
		case "-p":
			r.Protocol = next()
		case "-i":
			r.InIface = next()
		case "-o":
			r.OutIface = next()
		case "-s":
			r.SrcIP = next()
		case "-d":
			r.DstIP = next()
		case "--sport":
			r.SrcPort = next()
		case "--dport":
			r.DstPort = next()
		case "-j":
			r.Action = next()
		}
	}
	return r
}

func buildIPTablesCommand(r FirewallRule) string {
	if r.Chain == "" || r.Action == "" {
		return ""
	}
	cmd := fmt.Sprintf("iptables -A %s", r.Chain)
	if r.Protocol != "" && r.Protocol != "all" {
		cmd += " -p " + r.Protocol
	}
	if r.InIface != "" {
		cmd += " -i " + r.InIface
	}
	if r.OutIface != "" {
		cmd += " -o " + r.OutIface
	}
	if r.SrcIP != "" {
		cmd += " -s " + r.SrcIP
	}
	if r.DstIP != "" {
		cmd += " -d " + r.DstIP
	}
	if r.SrcPort != "" {
		cmd += " --sport " + r.SrcPort
	}
	if r.DstPort != "" {
		cmd += " --dport " + r.DstPort
	}
	cmd += " -j " + r.Action
	return cmd
}

func buildIPTablesCommandWithComment(r FirewallRule) string {
	cmd := buildIPTablesCommand(r)
	if cmd == "" {
		return ""
	}
	if r.Comment != "" {
		return fmt.Sprintf("# %s\n%s", r.Comment, cmd)
	}
	return cmd
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
