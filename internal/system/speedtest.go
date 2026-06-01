package system

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type SpeedMeasurement struct {
	Mbps float64
	Err  string
}

// MeasureLatency measures average TCP round-trip time to 8.8.8.8:53 (4 probes).
func MeasureLatency() SpeedMeasurement {
	const probes = 4
	var total time.Duration
	for i := 0; i < probes; i++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", "8.8.8.8:53", 5*time.Second)
		if err != nil {
			return SpeedMeasurement{Err: fmt.Sprintf("latency probe failed: %v", err)}
		}
		total += time.Since(start)
		conn.Close()
		time.Sleep(100 * time.Millisecond)
	}
	return SpeedMeasurement{Mbps: float64(total.Milliseconds()) / probes}
}

// MeasureDownload downloads 25 MB from Cloudflare and returns speed in Mbps.
func MeasureDownload() SpeedMeasurement {
	client := &http.Client{Timeout: 90 * time.Second}
	start := time.Now()
	resp, err := client.Get("https://speed.cloudflare.com/__down?bytes=25000000")
	if err != nil {
		return SpeedMeasurement{Err: fmt.Sprintf("download failed: %v", err)}
	}
	defer resp.Body.Close()
	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return SpeedMeasurement{Err: fmt.Sprintf("download read error: %v", err)}
	}
	elapsed := time.Since(start).Seconds()
	return SpeedMeasurement{Mbps: float64(n) * 8 / elapsed / 1_000_000}
}

// MeasureUpload uploads 10 MB to Cloudflare and returns speed in Mbps.
func MeasureUpload() SpeedMeasurement {
	const size = 10 * 1024 * 1024
	data := make([]byte, size)
	rand.Read(data)

	client := &http.Client{Timeout: 90 * time.Second}
	start := time.Now()
	resp, err := client.Post(
		"https://speed.cloudflare.com/__up",
		"application/octet-stream",
		bytes.NewReader(data),
	)
	if err != nil {
		return SpeedMeasurement{Err: fmt.Sprintf("upload failed: %v", err)}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	elapsed := time.Since(start).Seconds()
	return SpeedMeasurement{Mbps: float64(size) * 8 / elapsed / 1_000_000}
}
