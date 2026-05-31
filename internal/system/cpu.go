package system

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type CPUInfo struct {
	LoadAvg1   float64
	LoadAvg5   float64
	LoadAvg15  float64
	Cores      int
	UsedPercent float64 // LoadAvg1 / Cores * 100, capped at 100
}

func GetCPU() (CPUInfo, error) {
	info := CPUInfo{}

	// Load averages from /proc/loadavg
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return info, fmt.Errorf("read loadavg: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return info, fmt.Errorf("unexpected loadavg format")
	}
	info.LoadAvg1, _ = strconv.ParseFloat(fields[0], 64)
	info.LoadAvg5, _ = strconv.ParseFloat(fields[1], 64)
	info.LoadAvg15, _ = strconv.ParseFloat(fields[2], 64)

	// Core count from /proc/cpuinfo
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return info, fmt.Errorf("open cpuinfo: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "processor") {
			info.Cores++
		}
	}

	if info.Cores > 0 {
		info.UsedPercent = min(info.LoadAvg1/float64(info.Cores)*100, 100)
	}
	return info, nil
}
