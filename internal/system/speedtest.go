package system

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type SpeedMeasurement struct {
	Mbps   float64
	Err    string
}

// MeasureLatency pings 8.8.8.8 four times and returns the average RTT in milliseconds.
func MeasureLatency() SpeedMeasurement {
	out, err := exec.Command("ping", "-c", "4", "-q", "8.8.8.8").Output()
	if err != nil {
		return SpeedMeasurement{Err: fmt.Sprintf("ping failed: %v", err)}
	}
	// Parse "round-trip min/avg/max = 10.123/12.456/15.789 ms"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "min/avg/max") || strings.Contains(line, "round-trip") {
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.Contains(p, "/") {
					segments := strings.Split(p, "/")
					if len(segments) >= 2 {
						avg, e := strconv.ParseFloat(segments[1], 64)
						if e == nil {
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

// MeasureUpload uploads 10 MB of random data to Cloudflare and returns speed in Mbps.
func MeasureUpload() SpeedMeasurement {
	dd := exec.Command("dd", "if=/dev/urandom", "bs=1M", "count=10")
	curl := exec.Command("curl", "-X", "POST", "--data-binary", "@-",
		"-o", "/dev/null", "-s",
		"-w", "%{speed_upload}",
		"https://speed.cloudflare.com/__up",
	)

	r, w, err := os.Pipe()
	if err != nil {
		return SpeedMeasurement{Err: fmt.Sprintf("pipe failed: %v", err)}
	}
	dd.Stdout = w
	curl.Stdin = r

	var curlOut bytes.Buffer
	curl.Stdout = &curlOut

	if err := dd.Start(); err != nil {
		w.Close()
		r.Close()
		return SpeedMeasurement{Err: fmt.Sprintf("dd start failed: %v", err)}
	}
	if err := curl.Start(); err != nil {
		w.Close()
		r.Close()
		return SpeedMeasurement{Err: fmt.Sprintf("curl start failed: %v", err)}
	}

	dd.Wait()
	w.Close()
	curl.Wait()
	r.Close()

	bps, e := strconv.ParseFloat(strings.TrimSpace(curlOut.String()), 64)
	if e != nil {
		return SpeedMeasurement{Err: "could not parse upload speed"}
	}
	return SpeedMeasurement{Mbps: bps * 8 / 1_000_000}
}
