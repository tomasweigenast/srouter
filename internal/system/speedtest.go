package system

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type SpeedMeasurement struct {
	Mbps float64
	Err  string
}

// MeasureLatency pings 1.1.1.1 four times and returns average RTT in ms.
func MeasureLatency() SpeedMeasurement {
	out, err := exec.Command("ping", "-c", "4", "-q", "1.1.1.1").Output()
	if err != nil {
		return SpeedMeasurement{Err: fmt.Sprintf("ping failed: %v", err)}
	}
	// Parse "round-trip min/avg/max = 10.1/12.4/15.7 ms"
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "min/avg/max") || strings.Contains(line, "round-trip") {
			for _, field := range strings.Fields(line) {
				if strings.Contains(field, "/") {
					parts := strings.Split(field, "/")
					if len(parts) >= 2 {
						if avg, e := strconv.ParseFloat(parts[1], 64); e == nil {
							return SpeedMeasurement{Mbps: avg}
						}
					}
				}
			}
		}
	}
	return SpeedMeasurement{Err: "could not parse ping output"}
}

// MeasureDownload downloads 25 MB from Cloudflare and returns speed in Mbps.
func MeasureDownload() SpeedMeasurement {
	out, err := exec.Command("curl", "-o", "/dev/null", "-s",
		"-w", "%{speed_download}",
		"https://speed.cloudflare.com/__down?bytes=25000000",
	).Output()
	if err != nil {
		return SpeedMeasurement{Err: fmt.Sprintf("download test failed: %v", err)}
	}
	bps, e := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if e != nil {
		return SpeedMeasurement{Err: "could not parse download speed"}
	}
	return SpeedMeasurement{Mbps: bps * 8 / 1_000_000}
}

// MeasureUpload uploads 10 MB to Cloudflare and returns speed in Mbps.
func MeasureUpload() SpeedMeasurement {
	cmd := exec.Command("sh", "-c",
		`dd if=/dev/urandom bs=1M count=10 2>/dev/null | `+
			`curl -X POST --data-binary @- -o /dev/null -s -w "%{speed_upload}" `+
			`"https://speed.cloudflare.com/__up"`,
	)
	out, err := cmd.Output()
	if err != nil {
		return SpeedMeasurement{Err: fmt.Sprintf("upload test failed: %v", err)}
	}
	bps, e := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if e != nil {
		return SpeedMeasurement{Err: "could not parse upload speed"}
	}
	return SpeedMeasurement{Mbps: bps * 8 / 1_000_000}
}
