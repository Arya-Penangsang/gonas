//go:build !windows

package main

import (
	"fmt"
	"os"
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

func detectAvailableVolumes() []StorageVolume {
	var detected []StorageVolume
	configMu.RLock()
	rootPath := cfg.RootPath
	configMu.RUnlock()

	cleanRoot := filepath.Clean(rootPath)
	detected = append(detected, StorageVolume{
		ID:          "vol-n",
		Name:        "Storage Utama (GoNAS)",
		DriveLetter: "STORAGE",
		RootPath:    cleanRoot,
		Description: fmt.Sprintf("Folder Utama GoNAS Cloud (%s)", cleanRoot),
	})

	// On Linux, inspect /proc/mounts to automatically find mounted partitions/disks
	if data, err := os.ReadFile("/proc/mounts"); err == nil {
		lines := strings.Split(string(data), "\n")
		seen := make(map[string]bool)
		seen[cleanRoot] = true
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				device := parts[0]
				mountPoint := parts[1]
				fsType := parts[2]

				// Only consider real disk devices (/dev/sd*, /dev/nvme*, /dev/vd*, /dev/mmcblk*)
				if strings.HasPrefix(device, "/dev/sd") || strings.HasPrefix(device, "/dev/nvme") || strings.HasPrefix(device, "/dev/vd") || strings.HasPrefix(device, "/dev/mmcblk") {
					if !seen[mountPoint] && !strings.HasPrefix(mountPoint, "/boot") && !strings.HasPrefix(mountPoint, "/snap") {
						seen[mountPoint] = true
						var stat syscall.Statfs_t
						if err := syscall.Statfs(mountPoint, &stat); err == nil {
							total := uint64(stat.Blocks) * uint64(stat.Bsize)
							if total > 0 {
								name := fmt.Sprintf("Disk %s (%s)", filepath.Base(mountPoint), fsType)
								if mountPoint == "/" {
									name = fmt.Sprintf("Root OS / (%s)", fsType)
								}
								volID := fmt.Sprintf("vol-%s", strings.ReplaceAll(strings.Trim(mountPoint, "/"), "/", "-"))
								if volID == "vol-" {
									volID = "vol-root"
								}
								detected = append(detected, StorageVolume{
									ID:          volID,
									Name:        name,
									DriveLetter: filepath.Base(mountPoint),
									RootPath:    mountPoint,
									Description: fmt.Sprintf("Mount %s pada %s (%s)", device, mountPoint, formatBytes(total)),
								})
							}
						}
					}
				}
			}
		}
	}

	return detected
}
