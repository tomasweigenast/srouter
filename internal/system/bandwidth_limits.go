package system

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DevMode disables actual tc/iptables execution and /etc/firewall.d script
// writing so development can happen on macOS without a router. Set once in
// cmd/main.go.
var DevMode bool

// AllowedBandwidthMbps is the preset list shown in the UI and accepted by
// validation.
var AllowedBandwidthMbps = []int{1, 5, 10, 25, 50, 100, 250}

var bandwidthLimitsScript = "/etc/firewall.d/80-bandwidth-limits.sh"

// classIDStart is the first HTB class ID / fwmark used for limited devices.
// 0-9 are reserved for the parent (1:1) and default (1:30) classes.
const classIDStart = 10

// classIDMax caps the class ID / fwmark. tc class IDs are 16-bit so 65535
// is the hard limit; we cap much lower to keep the rule count readable.
const classIDMax = 245

// ListBandwidthLimits returns a map of MAC -> download Mbps for all limited
// devices.
func ListBandwidthLimits(db *sql.DB) (map[string]int, error) {
	rows, err := db.Query(`SELECT mac, download_mbps FROM device_bandwidth_limits ORDER BY mac`)
	if err != nil {
		return nil, fmt.Errorf("list bandwidth limits: %w", err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var mac string
		var mbps int
		if err := rows.Scan(&mac, &mbps); err != nil {
			return nil, fmt.Errorf("scan bandwidth limit: %w", err)
		}
		out[mac] = mbps
	}
	return out, rows.Err()
}

// GetBandwidthLimit returns the download Mbps for a MAC and whether it exists.
func GetBandwidthLimit(db *sql.DB, mac string) (int, bool, error) {
	var mbps int
	err := db.QueryRow(`SELECT download_mbps FROM device_bandwidth_limits WHERE mac = ?`, mac).Scan(&mbps)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("get bandwidth limit: %w", err)
	}
	return mbps, true, nil
}

// SetBandwidthLimit saves a download limit for a MAC and re-applies the
// iptables + tc config.
func SetBandwidthLimit(db *sql.DB, mac string, mbps int) error {
	now := time.Now().Unix()
	if _, err := db.Exec(
		`INSERT INTO device_bandwidth_limits (mac, download_mbps, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(mac) DO UPDATE SET
		   download_mbps = excluded.download_mbps,
		   updated_at = excluded.updated_at`,
		mac, mbps, now,
	); err != nil {
		return fmt.Errorf("set bandwidth limit: %w", err)
	}
	if err := ApplyAllBandwidthLimits(db, "lan"); err != nil {
		return fmt.Errorf("set bandwidth limit apply: %w", err)
	}
	return nil
}

// DeleteBandwidthLimit removes a download limit for a MAC and re-applies the
// iptables + tc config.
func DeleteBandwidthLimit(db *sql.DB, mac string) error {
	if _, err := db.Exec(`DELETE FROM device_bandwidth_limits WHERE mac = ?`, mac); err != nil {
		return fmt.Errorf("delete bandwidth limit: %w", err)
	}
	if err := ApplyAllBandwidthLimits(db, "lan"); err != nil {
		return fmt.Errorf("delete bandwidth limit apply: %w", err)
	}
	return nil
}

// ApplyAllBandwidthLimits regenerates the iptables marks + HTB qdisc for the
// given limits and rewrites the boot-persistent firewall.d script. In
// DevMode it skips iptables/tc execution and script writing so development
// does not require root or a real router.
func ApplyAllBandwidthLimits(db *sql.DB, iface string) error {
	limits, err := ListBandwidthLimits(db)
	if err != nil {
		return err
	}

	script := generateBandwidthScript(iface, limits)

	if DevMode {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(bandwidthLimitsScript), 0755); err != nil {
		return fmt.Errorf("mkdir firewall.d: %w", err)
	}
	if err := os.WriteFile(bandwidthLimitsScript, []byte(script), 0750); err != nil {
		return fmt.Errorf("write bandwidth limits script: %w", err)
	}

	if _, err := tcExec("sh", "-c", script); err != nil {
		return fmt.Errorf("apply bandwidth rules: %w", err)
	}
	return nil
}

// generateBandwidthScript builds the shell script that:
//  1. Marks every packet in the mangle table going to a limited MAC's
//     destination, using a dedicated BWLIMIT chain jumped into from
//     POSTROUTING.
//  2. Installs an HTB qdisc on iface with one class per limited device and
//     a fwmark-based filter for each class.
//
// MACs are sorted alphabetically so a given MAC always gets the same
// fwmark/classID while its row exists, regardless of insertion order.
func generateBandwidthScript(iface string, limits map[string]int) string {
	macs := make([]string, 0, len(limits))
	for mac := range limits {
		macs = append(macs, mac)
	}
	sort.Strings(macs)

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("# Managed by srouter — do not edit manually\n")
	fmt.Fprintf(&sb, "[ -d /sys/class/net/%s ] || exit 0\n", iface)

	// ── iptables mangle: dedicated chain, flushed and rebuilt each run ──
	sb.WriteString("iptables -t mangle -D POSTROUTING -o " + iface + " -j BWLIMIT 2>/dev/null\n")
	sb.WriteString("iptables -t mangle -F BWLIMIT 2>/dev/null\n")
	sb.WriteString("iptables -t mangle -X BWLIMIT 2>/dev/null\n")
	sb.WriteString("iptables -t mangle -N BWLIMIT\n")
	classID := classIDStart
	for _, mac := range macs {
		if classID > classIDMax {
			break
		}
		fmt.Fprintf(&sb,
			"iptables -t mangle -A BWLIMIT -m mac --mac-destination %s -j MARK --set-mark %d\n",
			mac, classID)
		classID++
	}
	fmt.Fprintf(&sb, "iptables -t mangle -A POSTROUTING -o %s -j BWLIMIT\n", iface)

	// ── tc: HTB qdisc + one class + one fwmark filter per limited device ──
	fmt.Fprintf(&sb, "tc qdisc del dev %s root 2>/dev/null || true\n", iface)
	fmt.Fprintf(&sb, "tc qdisc add dev %s root handle 1: htb default 30\n", iface)
	fmt.Fprintf(&sb, "tc class add dev %s parent 1: classid 1:1 htb rate 1000mbit ceil 1000mbit\n", iface)
	fmt.Fprintf(&sb, "tc class add dev %s parent 1:1 classid 1:30 htb rate 1000mbit ceil 1000mbit\n", iface)
	classID = classIDStart
	for _, mac := range macs {
		if classID > classIDMax {
			break
		}
		mbps := limits[mac]
		fmt.Fprintf(&sb,
			"tc class add dev %s parent 1:1 classid 1:%d htb rate %dmbit ceil %dmbit\n",
			iface, classID, mbps, mbps)
		fmt.Fprintf(&sb,
			"tc filter add dev %s parent 1: protocol ip prio 1 handle %d fw flowid 1:%d\n",
			iface, classID, classID)
		classID++
	}

	return sb.String()
}

// tcExec is swappable for tests.
var tcExec = func(name string, arg ...string) ([]byte, error) {
	return exec.Command(name, arg...).CombinedOutput()
}
