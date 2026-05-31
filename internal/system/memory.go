package system

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type MemInfo struct {
	TotalKB     uint64
	AvailableKB uint64
	UsedKB      uint64
	UsedPercent float64
}

func GetMemory() (MemInfo, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemInfo{}, fmt.Errorf("open meminfo: %w", err)
	}
	defer f.Close()

	vals := map[string]uint64{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		v, _ := strconv.ParseUint(parts[1], 10, 64)
		vals[key] = v
	}

	total := vals["MemTotal"]
	avail := vals["MemAvailable"]
	used := total - avail

	var pct float64
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}

	return MemInfo{
		TotalKB:     total,
		AvailableKB: avail,
		UsedKB:      used,
		UsedPercent: pct,
	}, nil
}
