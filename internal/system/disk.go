package system

import (
	"fmt"
	"syscall"
)

type DiskInfo struct {
	Path        string
	TotalBytes  uint64
	FreeBytes   uint64
	UsedPercent float64
}

func GetDisks() ([]DiskInfo, error) {
	paths := []string{"/", "/var"}
	// Deduplicate by device ID (st_dev from statfs)
	seen := map[uint64]bool{}
	disks := []DiskInfo{}

	for _, path := range paths {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(path, &stat); err != nil {
			continue
		}
		// Use f_fsid first word as dedup key; fall back to block count if zero
		key := uint64(stat.Bsize) * stat.Blocks
		if seen[key] {
			continue
		}
		seen[key] = true

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		used := total - free
		var pct float64
		if total > 0 {
			pct = float64(used) / float64(total) * 100
		}
		disks = append(disks, DiskInfo{
			Path:        path,
			TotalBytes:  total,
			FreeBytes:   free,
			UsedPercent: pct,
		})
	}

	if len(disks) == 0 {
		return nil, fmt.Errorf("no disk info available")
	}
	return disks, nil
}
