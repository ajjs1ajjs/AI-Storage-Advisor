package sre

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type FolderInfo struct {
	Path          string `json:"path"`
	Size          int64  `json:"size"`
	Count         int    `json:"count"`
	SizeFormatted string `json:"size_formatted"`
}

type ContainerInfo struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Image                string `json:"image"`
	WriteSize            int64  `json:"write_size"`
	WriteSizeFormatted   string `json:"write_size_formatted"`
	VirtualSize          int64  `json:"virtual_size"`
	VirtualSizeFormatted string `json:"virtual_size_formatted"`
}

type VolumeInfo struct {
	Name          string `json:"name"`
	Size          int64  `json:"size"`
	SizeFormatted string `json:"size_formatted"`
}

type SreReport struct {
	DockerActive  bool                  `json:"docker_active"`
	Containers    []ContainerInfo       `json:"containers"`
	Volumes       []VolumeInfo          `json:"volumes"`
	WindowsActive bool                  `json:"windows_active"`
	Folders       map[string]FolderInfo `json:"folders"`
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

func ParseDockerSize(sizeStr string) (int64, int64) {
	var writeBytes int64
	var virtBytes int64

	// Regex for "21MB (virtual 1.5GB)" or similar
	re := regexp.MustCompile(`([0-9.]+)\s*([a-zA-Z]+)\s*\(virtual\s*([0-9.]+)\s*([a-zA-Z]+)\)`)
	matches := re.FindStringSubmatch(sizeStr)

	units := map[string]int64{
		"B": 1, "KB": 1024, "MB": 1024 * 1024, "GB": 1024 * 1024 * 1024, "TB": 1024 * 1024 * 1024 * 1024,
		"b": 1, "kb": 1024, "mb": 1024 * 1024, "gb": 1024 * 1024 * 1024,
	}

	if len(matches) == 5 {
		wValStr, wUnit, vValStr, vUnit := matches[1], matches[2], matches[3], matches[4]

		wVal, _ := strconv.ParseFloat(wValStr, 64)
		vVal, _ := strconv.ParseFloat(vValStr, 64)

		writeBytes = int64(wVal * float64(units[wUnit]))
		virtBytes = int64(vVal * float64(units[vUnit]))
	} else {
		// Fallback: search for just simple value unit like "21MB"
		re2 := regexp.MustCompile(`([0-9.]+)\s*([a-zA-Z]+)`)
		matches2 := re2.FindStringSubmatch(sizeStr)
		if len(matches2) == 3 {
			valStr, unit := matches2[1], matches2[2]
			val, _ := strconv.ParseFloat(valStr, 64)
			writeBytes = int64(val * float64(units[unit]))
		}
	}

	return writeBytes, virtBytes
}

func AnalyzeWindowsSystem() SreReport {
	isWin := runtime.GOOS == "windows"
	report := SreReport{
		WindowsActive: isWin,
		Folders:       make(map[string]FolderInfo),
	}

	if !isWin {
		return report
	}

	winDirs := map[string]string{
		"minidumps":  `C:\Windows\Minidump`,
		"iis_logs":   `C:\inetpub\logs\LogFiles`,
		"event_logs": `C:\Windows\System32\Winevt\Logs`,
		"win_temp":   `C:\Windows\Temp`,
	}

	for key, path := range winDirs {
		fInfo := FolderInfo{
			Path:          path,
			SizeFormatted: "Not Found",
		}

		if _, err := os.Stat(path); err == nil {
			var size int64
			var count int
			errWalk := filepath.Walk(path, func(p string, info os.FileInfo, errWalk error) error {
				if errWalk != nil {
					return nil
				}
				if !info.IsDir() {
					size += info.Size()
					count++
				}
				return nil
			})
			if errWalk == nil {
				fInfo.Size = size
				fInfo.Count = count
				fInfo.SizeFormatted = FormatSize(size)
			} else {
				fInfo.SizeFormatted = "Access Denied"
			}
		}

		report.Folders[key] = fInfo
	}

	// Probe local Docker as well
	dockerReport := AnalyzeLocalDocker()
	report.DockerActive = dockerReport.DockerActive
	report.Containers = dockerReport.Containers
	report.Volumes = dockerReport.Volumes

	return report
}

func AnalyzeLocalDocker() SreReport {
	report := SreReport{
		DockerActive: false,
		Containers:   make([]ContainerInfo, 0),
		Volumes:      make([]VolumeInfo, 0),
	}

	// Probe if docker is installed/running
	_, err := exec.LookPath("docker")
	if err != nil {
		return report
	}

	// Use a 2-second timeout context to prevent indefinite hangs if the Docker daemon is unresponsive/frozen
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--size", "--format", "{{.ID}}|{{.Names}}|{{.Image}}|{{.Size}}")
	out, err := cmd.Output()
	if err != nil {
		return report
	}

	report.DockerActive = true
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		cid, name, image, sizeRaw := parts[0], parts[1], parts[2], parts[3]
		wBytes, vBytes := ParseDockerSize(sizeRaw)

		report.Containers = append(report.Containers, ContainerInfo{
			ID:                   cid,
			Name:                 name,
			Image:                image,
			WriteSize:            wBytes,
			WriteSizeFormatted:   FormatSize(wBytes),
			VirtualSize:          vBytes,
			VirtualSizeFormatted: FormatSize(vBytes),
		})
	}

	// Get volumes with a 2-second timeout context as well
	volCtx, volCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer volCancel()

	volCmd := exec.CommandContext(volCtx, "docker", "system", "df", "-v")
	volOut, err := volCmd.Output()
	if err == nil {
		volLines := strings.Split(string(volOut), "\n")
		volStarted := false
		volNameRe := regexp.MustCompile(`([0-9.]+)([a-zA-Z]+)`)

		for _, line := range volLines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "VOLUME NAME") {
				volStarted = true
				continue
			}
			if volStarted && line != "" {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					vName := parts[0]
					vSizeRaw := parts[2]

					var vBytes int64
					m := volNameRe.FindStringSubmatch(vSizeRaw)
					if len(m) == 3 {
						val, _ := strconv.ParseFloat(m[1], 64)
						unit := strings.ToUpper(m[2])

						units := map[string]int64{
							"B": 1, "KB": 1024, "MB": 1024 * 1024, "GB": 1024 * 1024 * 1024,
						}
						vBytes = int64(val * float64(units[unit]))
					}

					report.Volumes = append(report.Volumes, VolumeInfo{
						Name:          vName,
						Size:          vBytes,
						SizeFormatted: FormatSize(vBytes),
					})
				}
			}
		}
	}

	return report
}

func CalculateHealthScore(totalSize int64, daysRemaining int, duplicateWaste int64, logSize int64, tempSize int64, sreData SreReport) (int, []string) {
	score := 100
	warnings := make([]string, 0)

	// 1. Deduct by total disk fill rate
	if daysRemaining != -1 {
		if daysRemaining < 30 {
			score -= 30
			warnings = append(warnings, fmt.Sprintf("Critical: Storage exhaustion projected in %d days.", daysRemaining))
		} else if daysRemaining < 90 {
			score -= 15
			warnings = append(warnings, fmt.Sprintf("Warning: Storage exhaustion projected in %d days.", daysRemaining))
		}
	}

	// 2. Deduct by duplicate waste
	if duplicateWaste > 10*1024*1024*1024 { // 10 GB
		score -= 15
		warnings = append(warnings, fmt.Sprintf("High Waste: Duplicates are wasting %s disk space.", FormatSize(duplicateWaste)))
	} else if duplicateWaste > 1*1024*1024*1024 { // 1 GB
		score -= 8
		warnings = append(warnings, fmt.Sprintf("Waste: Duplicates are wasting %s disk space.", FormatSize(duplicateWaste)))
	}

	// 3. Deduct by unrotated log files
	if logSize > 5*1024*1024*1024 { // 5 GB
		score -= 15
		warnings = append(warnings, fmt.Sprintf("Warning: Unrotated log files are consuming %s.", FormatSize(logSize)))
	} else if logSize > 500*1024*1024 { // 500 MB
		score -= 5
		warnings = append(warnings, fmt.Sprintf("Log consumption: Log files are consuming %s.", FormatSize(logSize)))
	}

	// 4. Deduct by temporary files
	if tempSize > 5*1024*1024*1024 { // 5 GB
		score -= 10
		warnings = append(warnings, fmt.Sprintf("Warning: Temporary files are occupying %s.", FormatSize(tempSize)))
	}

	// 5. Deduct by SRE data
	if sreData.DockerActive {
		var largeCount int
		for _, c := range sreData.Containers {
			if c.WriteSize > 1*1024*1024*1024 { // > 1 GB
				largeCount++
			}
		}
		if largeCount > 0 {
			score -= 10
			warnings = append(warnings, fmt.Sprintf("DevOps Warning: %d Docker container(s) have write-layers > 1 GB (potential unrotated app logs).", largeCount))
		}
	}

	if sreData.WindowsActive {
		if dumpInfo, exists := sreData.Folders["minidumps"]; exists {
			if dumpInfo.Size > 500*1024*1024 { // 500 MB
				score -= 10
				warnings = append(warnings, fmt.Sprintf("Windows Warning: Crash memory dumps folder is consuming %s.", dumpInfo.SizeFormatted))
			}
		}
	}

	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	return score, warnings
}
