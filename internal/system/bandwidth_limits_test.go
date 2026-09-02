package system

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func newBandwidthTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE device_bandwidth_limits (
			mac             TEXT PRIMARY KEY,
			download_mbps   INTEGER NOT NULL,
			created_at      INTEGER NOT NULL DEFAULT 0,
			updated_at      INTEGER NOT NULL DEFAULT 0
		)
	`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestListBandwidthLimits(t *testing.T) {
	t.Parallel()

	db := newBandwidthTestDB(t)
	defer db.Close()

	if _, err := db.Exec(
		`INSERT INTO device_bandwidth_limits (mac, download_mbps) VALUES (?, ?)`,
		"aa:bb:cc:dd:ee:01", 10,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO device_bandwidth_limits (mac, download_mbps) VALUES (?, ?)`,
		"aa:bb:cc:dd:ee:02", 50,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	limits, err := ListBandwidthLimits(db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(limits) != 2 {
		t.Fatalf("expected 2 limits, got %d", len(limits))
	}
	if limits["aa:bb:cc:dd:ee:01"] != 10 {
		t.Errorf("expected 10, got %d", limits["aa:bb:cc:dd:ee:01"])
	}
	if limits["aa:bb:cc:dd:ee:02"] != 50 {
		t.Errorf("expected 50, got %d", limits["aa:bb:cc:dd:ee:02"])
	}
}

func TestSetAndGetBandwidthLimit(t *testing.T) {
	db := newBandwidthTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	oldScriptPath := bandwidthLimitsScript
	bandwidthLimitsScript = filepath.Join(tmpDir, "80-bandwidth-limits.sh")
	defer func() { bandwidthLimitsScript = oldScriptPath }()

	var lastScript string
	oldTcExec := tcExec
	tcExec = func(name string, arg ...string) ([]byte, error) {
		if name == "sh" && len(arg) >= 2 && arg[0] == "-c" {
			lastScript = arg[1]
		}
		return nil, nil
	}
	defer func() { tcExec = oldTcExec }()

	if err := SetBandwidthLimit(db, "aa:bb:cc:dd:ee:01", 25); err != nil {
		t.Fatalf("set: %v", err)
	}

	mbps, ok, err := GetBandwidthLimit(db, "aa:bb:cc:dd:ee:01")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok {
		t.Fatal("expected limit to exist")
	}
	if mbps != 25 {
		t.Errorf("expected 25, got %d", mbps)
	}

	wantLines := []string{
		"iptables -t mangle -N BWLIMIT",
		"iptables -t mangle -A BWLIMIT -m mac --mac-destination aa:bb:cc:dd:ee:01 -j MARK --set-mark 10",
		"iptables -t mangle -A POSTROUTING -o lan -j BWLIMIT",
		"tc class add dev lan parent 1:1 classid 1:10 htb rate 25mbit ceil 25mbit",
		"tc filter add dev lan parent 1: protocol ip prio 1 handle 10 fw flowid 1:10",
	}
	for _, want := range wantLines {
		if !strings.Contains(lastScript, want) {
			t.Errorf("expected script to contain %q, got:\n%s", want, lastScript)
		}
	}

	written, err := os.ReadFile(bandwidthLimitsScript)
	if err != nil {
		t.Fatalf("read written script: %v", err)
	}
	if !strings.Contains(string(written), "25mbit") {
		t.Errorf("expected written script to contain 25mbit, got:\n%s", string(written))
	}
}

func TestDeleteBandwidthLimit(t *testing.T) {
	db := newBandwidthTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	oldScriptPath := bandwidthLimitsScript
	bandwidthLimitsScript = filepath.Join(tmpDir, "80-bandwidth-limits.sh")
	defer func() { bandwidthLimitsScript = oldScriptPath }()

	oldTcExec := tcExec
	tcExec = func(name string, arg ...string) ([]byte, error) { return nil, nil }
	defer func() { tcExec = oldTcExec }()

	if err := SetBandwidthLimit(db, "aa:bb:cc:dd:ee:01", 10); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := DeleteBandwidthLimit(db, "aa:bb:cc:dd:ee:01"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, ok, err := GetBandwidthLimit(db, "aa:bb:cc:dd:ee:01")
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if ok {
		t.Fatal("expected limit to be deleted")
	}

	written, err := os.ReadFile(bandwidthLimitsScript)
	if err != nil {
		t.Fatalf("read written script: %v", err)
	}
	if strings.Contains(string(written), "10mbit") {
		t.Errorf("expected written script not to contain 10mbit after delete, got:\n%s", string(written))
	}
	if strings.Contains(string(written), "--mac-destination aa:bb:cc:dd:ee:01") {
		t.Errorf("expected written script not to contain mark for deleted MAC, got:\n%s", string(written))
	}
}

func TestGenerateBandwidthScript(t *testing.T) {
	t.Parallel()

	limits := map[string]int{
		"aa:bb:cc:dd:ee:01": 10,
		"aa:bb:cc:dd:ee:02": 50,
	}

	script := generateBandwidthScript("lan", limits)

	wantLines := []string{
		// mangle setup
		"iptables -t mangle -D POSTROUTING -o lan -j BWLIMIT 2>/dev/null",
		"iptables -t mangle -F BWLIMIT 2>/dev/null",
		"iptables -t mangle -X BWLIMIT 2>/dev/null",
		"iptables -t mangle -N BWLIMIT",
		// alphabetical order: ee:01 first → fwmark 10, ee:02 second → fwmark 11
		"iptables -t mangle -A BWLIMIT -m mac --mac-destination aa:bb:cc:dd:ee:01 -j MARK --set-mark 10",
		"iptables -t mangle -A BWLIMIT -m mac --mac-destination aa:bb:cc:dd:ee:02 -j MARK --set-mark 11",
		"iptables -t mangle -A POSTROUTING -o lan -j BWLIMIT",
		// tc setup
		"tc qdisc add dev lan root handle 1: htb default 30",
		"tc class add dev lan parent 1: classid 1:1 htb rate 1000mbit ceil 1000mbit",
		"tc class add dev lan parent 1:1 classid 1:30 htb rate 1000mbit ceil 1000mbit",
		"tc class add dev lan parent 1:1 classid 1:10 htb rate 10mbit ceil 10mbit",
		"tc class add dev lan parent 1:1 classid 1:11 htb rate 50mbit ceil 50mbit",
		"tc filter add dev lan parent 1: protocol ip prio 1 handle 10 fw flowid 1:10",
		"tc filter add dev lan parent 1: protocol ip prio 1 handle 11 fw flowid 1:11",
	}
	for _, want := range wantLines {
		if !strings.Contains(script, want) {
			t.Errorf("expected script to contain %q, got:\n%s", want, script)
		}
	}
}

func TestGenerateBandwidthScriptNoLimits(t *testing.T) {
	t.Parallel()

	script := generateBandwidthScript("lan", map[string]int{})

	// The script must still set up the iptables chain and tc qdisc so that
	// a previous limit is fully wiped, but should not contain any per-device
	// rules.
	if !strings.Contains(script, "iptables -t mangle -N BWLIMIT") {
		t.Errorf("expected script to set up BWLIMIT chain, got:\n%s", script)
	}
	if !strings.Contains(script, "tc class add dev lan parent 1:1 classid 1:30") {
		t.Errorf("expected default class to still exist, got:\n%s", script)
	}
	if strings.Contains(script, "MARK --set-mark") {
		t.Errorf("expected no per-MAC marks, got:\n%s", script)
	}
	if strings.Contains(script, "tc filter add") {
		t.Errorf("expected no per-MAC filters, got:\n%s", script)
	}
}

func TestGenerateBandwidthScriptStableIDs(t *testing.T) {
	t.Parallel()

	// Two calls with the same set of MACs must produce the same fwmark /
	// classID assignments.
	limits := map[string]int{
		"cc:cc:cc:cc:cc:01": 5,
		"aa:aa:aa:aa:aa:01": 25,
		"bb:bb:bb:bb:bb:01": 100,
	}

	first := generateBandwidthScript("lan", limits)
	second := generateBandwidthScript("lan", limits)

	if first != second {
		t.Errorf("expected stable output, got different scripts.\nFirst:\n%s\nSecond:\n%s", first, second)
	}
}
