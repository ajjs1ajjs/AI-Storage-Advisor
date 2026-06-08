package cleanup

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"aisadvisor/backend/db"
)

// Windows Shell File Operations Constants
const (
	foDelete          = 3
	fofAllowUndo      = 0x0040
	fofNoconfirmation = 0x0010
	fofNoerrorui      = 0x0400
	fofSilent         = 0x0004
)

type shFileOpStructW struct {
	hwnd                  syscall.Handle
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

type DryRunResult struct {
	TotalCount         int                      `json:"total_count"`
	TotalSize          int64                    `json:"total_size"`
	TotalSizeFormatted string                   `json:"total_size_formatted"`
	WritableFiles      []map[string]interface{} `json:"writable_files"`
	RestrictedFiles    []map[string]interface{} `json:"restricted_files"`
	CanProceed         bool                     `json:"can_proceed"`
}

func FormatSize(sizeBytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	val := float64(sizeBytes)
	for _, unit := range units {
		if val < 1024.0 {
			return fmt.Sprintf("%.2f %s", val, unit)
		}
		val /= 1024.0
	}
	return fmt.Sprintf("%.2f PB", val)
}

func DryRun(filePaths []string) DryRunResult {
	var totalCount int
	var totalSize int64
	writableFiles := make([]map[string]interface{}, 0)
	restrictedFiles := make([]map[string]interface{}, 0)

	for _, p := range filePaths {
		pAbs, err := filepath.Abs(p)
		if err != nil {
			pAbs = p
		}

		info, err := os.Stat(pAbs)
		if err != nil {
			// File does not exist or access denied
			restrictedFiles = append(restrictedFiles, map[string]interface{}{
				"path": pAbs,
				"size": int64(0),
			})
			continue
		}

		totalCount++
		size := info.Size()

		// Check if file is writeable
		file, errOpen := os.OpenFile(pAbs, os.O_WRONLY, 0666)
		if errOpen == nil {
			file.Close()
			writableFiles = append(writableFiles, map[string]interface{}{
				"path": pAbs,
				"size": size,
			})
			totalSize += size
		} else {
			restrictedFiles = append(restrictedFiles, map[string]interface{}{
				"path": pAbs,
				"size": size,
			})
		}
	}

	return DryRunResult{
		TotalCount:         totalCount,
		TotalSize:          totalSize,
		TotalSizeFormatted: FormatSize(totalSize),
		WritableFiles:      writableFiles,
		RestrictedFiles:    restrictedFiles,
		CanProceed:         len(writableFiles) > 0,
	}
}

func RecycleFileWindows(filePath string) error {
	pFrom, err := syscall.UTF16FromString(filePath + "\x00")
	if err != nil {
		return err
	}

	shell32 := syscall.NewLazyDLL("shell32.dll")
	shFileOperation := shell32.NewProc("SHFileOperationW")

	op := shFileOpStructW{
		hwnd:   0,
		wFunc:  foDelete,
		pFrom:  &pFrom[0],
		pTo:    nil,
		fFlags: fofAllowUndo | fofNoconfirmation | fofNoerrorui | fofSilent,
	}

	ret, _, _ := shFileOperation.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("SHFileOperationW failed with return code %d", ret)
	}

	return nil
}

func SafeDeleteFile(profileID int, filePath string, useRecycleBin bool) (int64, error) {
	pAbs, err := filepath.Abs(filePath)
	if err != nil {
		pAbs = filePath
	}

	info, err := os.Stat(pAbs)
	if err != nil {
		return 0, err
	}
	size := info.Size()

	if useRecycleBin && runtime.GOOS == "windows" {
		err = RecycleFileWindows(pAbs)
	} else {
		err = os.Remove(pAbs)
	}

	if err != nil {
		if db.DB != nil {
			_, dbErr := db.DB.Exec(
				"INSERT INTO cleanup_history (profile_id, cleaned_path, size_freed, status, error_message) VALUES (?, ?, 0, 'failed', ?)",
				profileID, pAbs, err.Error(),
			)
			if dbErr != nil {
				log.Printf("Warning: Failed to log cleanup failure to DB: %v", dbErr)
			}
		}
		return 0, err
	}

	if db.DB != nil {
		_, dbErr := db.DB.Exec(
			"INSERT INTO cleanup_history (profile_id, cleaned_path, size_freed, status) VALUES (?, ?, ?, 'success')",
			profileID, pAbs, size,
		)
		if dbErr != nil {
			log.Printf("Warning: Failed to log cleanup success to DB: %v", dbErr)
		}
	}

	return size, nil
}
