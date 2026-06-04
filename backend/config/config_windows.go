//go:build windows

package config

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func isNetworkDrive(path string) bool {
	// Check for UNC path prefix
	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, `\\`) || strings.HasPrefix(cleanPath, `//`) {
		return true
	}

	vol := filepath.VolumeName(cleanPath)
	if vol == "" {
		return false
	}
	if !strings.HasSuffix(vol, `\`) {
		vol += `\`
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDriveType := kernel32.NewProc("GetDriveTypeW")

	ptr, err := syscall.UTF16PtrFromString(vol)
	if err != nil {
		return false
	}

	r, _, _ := getDriveType.Call(uintptr(unsafe.Pointer(ptr)))
	return r == 4 // DRIVE_REMOTE
}
