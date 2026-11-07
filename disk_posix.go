//go:build !windows

package main

import (
	"path/filepath"
	"strings"
	"syscall"
)

// POSIX Disk Telemetry (Linux, macOS, BSD, Docker)
func getDiskStats(path string) (DriveInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return DriveInfo{}, err
	}

	totalNumberOfBytes := uint64(stat.Blocks) * uint64(stat.Bsize)
	totalNumberOfFreeBytes := uint64(stat.Bavail) * uint64(stat.Bsize) // available to non-root
	used := totalNumberOfBytes - (uint64(stat.Bfree) * uint64(stat.Bsize))
	if totalNumberOfBytes < (uint64(stat.Bfree) * uint64(stat.Bsize)) {
		used = 0
	}

	var percent float64
	if totalNumberOfBytes > 0 {
		percent = (float64(used) / float64(totalNumberOfBytes)) * 100.0
	}

	label := "Volume Storage"
	letter := "ROOT"
	if len(path) >= 2 && path[1] == ':' {
		letter = strings.ToUpper(path[:2])
	} else if path != "" {
		letter = filepath.Base(path)
		if letter == "." || letter == "/" {
			letter = "ROOT"
		}
	}

	configMu.RLock()
	isRoot := filepath.Clean(path) == filepath.Clean(cfg.RootPath)
	configMu.RUnlock()

	return DriveInfo{
		Letter:       letter,
		Label:        label,
		TotalBytes:   totalNumberOfBytes,
		FreeBytes:    totalNumberOfFreeBytes,
		UsedBytes:    used,
		UsedPercent:  percent,
		TotalDisplay: formatBytes(totalNumberOfBytes),
		UsedDisplay:  formatBytes(used),
		FreeDisplay:  formatBytes(totalNumberOfFreeBytes),
		IsNASRoot:    isRoot,
	}, nil
}
