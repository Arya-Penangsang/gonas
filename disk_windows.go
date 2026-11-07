//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// Windows Disk Telemetry via Win32 Syscall
func getDiskStats(path string) (DriveInfo, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	rootPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return DriveInfo{}, err
	}

	r1, _, err := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(rootPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)
	if r1 == 0 {
		return DriveInfo{}, err
	}

	used := totalNumberOfBytes - totalNumberOfFreeBytes
	var percent float64
	if totalNumberOfBytes > 0 {
		percent = (float64(used) / float64(totalNumberOfBytes)) * 100.0
	}

	label := "Local Disk"
	letter := "C:"
	if len(path) >= 2 && path[1] == ':' {
		letter = strings.ToUpper(path[:2])
	}
	switch letter {
	case "N:":
		label = "STORAGE_NAS"
	case "C:":
		label = "Windows (OS)"
	case "D:":
		label = "Perkantoran"
	case "E:":
		label = "Data 2"
	}

	configMu.RLock()
	isRoot := strings.EqualFold(filepath.Clean(path), filepath.Clean(cfg.RootPath))
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
