//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32Dll           = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW   = kernel32Dll.NewProc("GetDiskFreeSpaceExW")
	getLogicalDrivesW     = kernel32Dll.NewProc("GetLogicalDrives")
	getDriveTypeW         = kernel32Dll.NewProc("GetDriveTypeW")
	getVolumeInformationW = kernel32Dll.NewProc("GetVolumeInformationW")
)

const (
	DRIVE_UNKNOWN     = 0
	DRIVE_NO_ROOT_DIR = 1
	DRIVE_REMOVABLE   = 2
	DRIVE_FIXED       = 3
	DRIVE_REMOTE      = 4
	DRIVE_CDROM       = 5
	DRIVE_RAMDISK     = 6
)

// getVolumeLabel gets the volume label of a drive root path (e.g. "C:\\")
func getVolumeLabel(rootPath string) (string, uint32) {
	rootPtr, err := syscall.UTF16PtrFromString(rootPath)
	if err != nil {
		return "", DRIVE_UNKNOWN
	}

	driveTypeRet, _, _ := getDriveTypeW.Call(uintptr(unsafe.Pointer(rootPtr)))
	driveType := uint32(driveTypeRet)

	var volName [256]uint16
	var fsName [256]uint16
	r, _, _ := getVolumeInformationW.Call(
		uintptr(unsafe.Pointer(rootPtr)),
		uintptr(unsafe.Pointer(&volName[0])),
		uintptr(len(volName)),
		0, 0, 0,
		uintptr(unsafe.Pointer(&fsName[0])),
		uintptr(len(fsName)),
	)

	label := ""
	if r != 0 {
		label = syscall.UTF16ToString(volName[:])
	}
	return strings.TrimSpace(label), driveType
}

// Windows Disk Telemetry via Win32 Syscall
func getDiskStats(path string) (DriveInfo, error) {
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	rootPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return DriveInfo{}, err
	}

	r1, _, err := getDiskFreeSpaceExW.Call(
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

	letter := "C:"
	cleanPath := filepath.Clean(path)
	if len(cleanPath) >= 2 && cleanPath[1] == ':' {
		letter = strings.ToUpper(cleanPath[:2])
	}

	label, _ := getVolumeLabel(letter + "\\")
	if label == "" {
		if letter == "C:" {
			label = "Local Disk (OS)"
		} else {
			label = fmt.Sprintf("Local Disk (%s)", letter)
		}
	} else {
		label = fmt.Sprintf("%s (%s)", label, letter)
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

// detectAvailableVolumes scans all physical/logical drives on the Windows PC
func detectAvailableVolumes() []StorageVolume {
	var detected []StorageVolume

	// 1. Primary volume: GoNAS configured Storage Root
	configMu.RLock()
	rootPath := cfg.RootPath
	configMu.RUnlock()

	cleanRoot := filepath.Clean(rootPath)
	detected = append(detected, StorageVolume{
		ID:          "vol-n", // keep vol-n for frontend default compatibility
		Name:        "Storage Utama (GoNAS)",
		DriveLetter: "STORAGE",
		RootPath:    cleanRoot,
		Description: fmt.Sprintf("Folder Utama GoNAS Cloud (%s)", cleanRoot),
	})

	// 2. Scan all drive letters from bitmask (A: through Z:)
	r, _, _ := getLogicalDrivesW.Call()
	mask := uint32(r)

	for i := 0; i < 26; i++ {
		if (mask & (1 << i)) != 0 {
			letter := fmt.Sprintf("%c:", 'A'+i)
			root := letter + "\\"

			label, dType := getVolumeLabel(root)

			// Skip optical drives (CD/DVD-ROM)
			if dType == DRIVE_CDROM {
				continue
			}

			// Verify drive has accessible space
			stat, err := getDiskStats(root)
			if err != nil || stat.TotalBytes == 0 {
				continue
			}

			driveName := label
			if driveName == "" {
				if letter == "C:" {
					driveName = fmt.Sprintf("Local Disk (%s)", letter)
				} else {
					driveName = fmt.Sprintf("Drive (%s)", letter)
				}
			} else {
				driveName = fmt.Sprintf("%s (%s)", driveName, letter)
			}

			volID := fmt.Sprintf("vol-%c", rune('a'+i))

			detected = append(detected, StorageVolume{
				ID:          volID,
				Name:        driveName,
				DriveLetter: letter,
				RootPath:    root,
				Description: fmt.Sprintf("Penyimpanan Fisik %s - Kapasitas: %s", letter, stat.TotalDisplay),
			})
		}
	}

	return detected
}
