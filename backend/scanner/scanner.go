package scanner

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
	"golang.org/x/sync/errgroup"

	"aisadvisor/backend/rules"
	"aisadvisor/backend/sre"
)

type FileInfo struct {
	Path           string  `json:"path"`
	Name           string  `json:"name"`
	Size           int64   `json:"size"`
	SizeFormatted  string  `json:"size_formatted"`
	Ext            string  `json:"ext"`
	LastAccess     string  `json:"last_access"`
	LastModified   string  `json:"last_modified"`
	LastModifiedTS int64   `json:"last_modified_ts"`
	Category       string  `json:"category"`
	RuleMatch      *string `json:"rule_match"`
}

type DuplicateFileInfo struct {
	Path          string `json:"path"`
	Size          int64  `json:"size"`
	SizeFormatted string `json:"size_formatted"`
}

type ScanResults struct {
	TotalSize           int64                            `json:"total_size"`
	TotalSizeFormatted  string                           `json:"total_size_formatted"`
	FilesScanned        int                              `json:"files_scanned"`
	LargeFiles          []FileInfo                       `json:"large_files"`
	TempFiles           []FileInfo                       `json:"temp_files"`
	LogFiles            []FileInfo                       `json:"log_files"`
	BackupFiles         []FileInfo                       `json:"backup_files"`
	CacheFiles          []FileInfo                       `json:"cache_files"`
	DuplicateGroups     map[string][]DuplicateFileInfo   `json:"duplicate_groups"`
	SreData             sre.SreReport                    `json:"sre_data"`
	RulesFlaggedCount   int                              `json:"rules_flagged_count"`
	RulesFlaggedSize    int64                            `json:"rules_flagged_size"`
	Cancelled           bool                             `json:"cancelled"`
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

func ScanLocalDisk(ctx context.Context, startPath string, activeRules []rules.Rule, progressCallback func(currentDir string, filesScanned int, totalSize int64)) (ScanResults, error) {
	log.Printf("Starting local scan on: %s", startPath)

	var filesScanned int
	var totalSize int64

	largeFiles := make([]FileInfo, 0)
	tempFiles := make([]FileInfo, 0)
	logFiles := make([]FileInfo, 0)
	backupFiles := make([]FileInfo, 0)
	cacheFiles := make([]FileInfo, 0)

	sizeGroups := make(map[int64][]string)

	tempDirs := []string{"temp", "tmp", "cache", "logs", "log"}
	lastEmitTime := time.Now()

	err := filepath.WalkDir(startPath, func(path string, d os.DirEntry, err error) error {
		// Handle cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			if d == nil {
				if fi, statErr := os.Lstat(path); statErr == nil && fi.IsDir() {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// Progress reporting
		filesScanned++
		info, err := d.Info()
		if err != nil {
			return nil
		}
		size := info.Size()
		totalSize += size

		if time.Since(lastEmitTime) > 100*time.Millisecond {
			progressCallback(filepath.Dir(path), filesScanned, totalSize)
			lastEmitTime = time.Now()
		}

		ext := strings.ToLower(filepath.Ext(path))
		name := d.Name()
		nameLower := strings.ToLower(name)
		pathLower := strings.ToLower(path)

		lastAccess := info.ModTime().Format("2006-01-02 15:04:05")
		lastModified := info.ModTime().Format("2006-01-02 15:04:05")
		lastModifiedTS := info.ModTime().Unix()

		fileInfo := FileInfo{
			Path:           path,
			Name:           name,
			Size:           size,
			SizeFormatted:  FormatSize(size),
			Ext:            ext,
			LastAccess:     lastAccess,
			LastModified:   lastModified,
			LastModifiedTS: lastModifiedTS,
			Category:       "other",
		}

		// Categorization
		isTempDir := false
		for _, td := range tempDirs {
			if strings.Contains(pathLower, td) {
				isTempDir = true
				break
			}
		}

		isTempExt := ext == ".tmp" || ext == ".temp" || ext == ".bak" || ext == ".old"

		if isTempExt || isTempDir {
			if (ext == ".log" || ext == ".txt") && strings.Contains(pathLower, "log") {
				fileInfo.Category = "log"
				logFiles = append(logFiles, fileInfo)
			} else if strings.Contains(pathLower, "cache") {
				fileInfo.Category = "cache"
				cacheFiles = append(cacheFiles, fileInfo)
			} else {
				fileInfo.Category = "temp"
				tempFiles = append(tempFiles, fileInfo)
			}
		} else if ext == ".log" || (ext == ".txt" && strings.Contains(nameLower, "log")) {
			fileInfo.Category = "log"
			logFiles = append(logFiles, fileInfo)
		} else if (ext == ".zip" || ext == ".rar" || ext == ".tar" || ext == ".gz" || ext == ".7z" || ext == ".bak") &&
			(strings.Contains(nameLower, "backup") || strings.Contains(nameLower, "bak")) {
			fileInfo.Category = "backup"
			backupFiles = append(backupFiles, fileInfo)
		}

		if size > 100*1024*1024 {
			if fileInfo.Category == "other" {
				fileInfo.Category = "large"
			}
			largeFiles = append(largeFiles, fileInfo)
		}

		// Group for duplicates (files > 1 MB)
		if size > 1*1024*1024 {
			sizeGroups[size] = append(sizeGroups[size], path)
		}

		return nil
	})

	if err != nil && err != context.Canceled {
		return ScanResults{}, err
	}

	if ctx.Err() == context.Canceled {
		return ScanResults{Cancelled: true}, nil
	}

	// Duplicates matching: parallel processing
	duplicateGroups := make(map[string][]DuplicateFileInfo)

	totalGroups := 0
	for _, paths := range sizeGroups {
		if len(paths) > 1 {
			totalGroups++
		}
	}

	var currentGroup int32 = 0
	var mu sync.Mutex

	g, gCtx := errgroup.WithContext(ctx)
	// Limit concurrency for duplicate hashing to avoid too many open files
	sem := make(chan struct{}, runtime.NumCPU()*2)

	for size, paths := range sizeGroups {
		size := size
		paths := paths
		if len(paths) > 1 {
			g.Go(func() error {
				select {
				case <-gCtx.Done():
					return gCtx.Err()
				default:
				}

				sem <- struct{}{}
				defer func() { <-sem }()

				cg := atomic.AddInt32(&currentGroup, 1)
				if cg%10 == 0 || cg == 1 || int(cg) == totalGroups {
					progressCallback(fmt.Sprintf("Аналіз дублікатів (%d/%d)...", cg, totalGroups), filesScanned, totalSize)
				}

				// Group by prefix hash
				prefixGroups := make(map[string][]string)
				for _, p := range paths {
					h, err := getPrefixHash(p)
					if err != nil {
						continue
					}
					prefixGroups[h] = append(prefixGroups[h], p)
				}

				// Group by full hash
				for _, collidingPaths := range prefixGroups {
					if len(collidingPaths) > 1 {
						fullGroups := make(map[string][]string)
						for _, p := range collidingPaths {
							h, err := getFastHash(gCtx, p)
							if err != nil {
								continue
							}
							fullGroups[h] = append(fullGroups[h], p)
						}

						for h, dupPaths := range fullGroups {
							if len(dupPaths) > 1 {
								dups := make([]DuplicateFileInfo, 0, len(dupPaths))
								for _, p := range dupPaths {
									dups = append(dups, DuplicateFileInfo{
										Path:          p,
										Size:          size,
										SizeFormatted: FormatSize(size),
									})
								}
								mu.Lock()
								duplicateGroups[h] = dups
								mu.Unlock()
							}
						}
					}
				}
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil && err != context.Canceled {
		return ScanResults{}, err
	}

	if ctx.Err() == context.Canceled {
		return ScanResults{Cancelled: true}, nil
	}

	// Calculate SRE data (Windows)
	sreData := sre.AnalyzeWindowsSystem()

	results := ScanResults{
		TotalSize:          totalSize,
		TotalSizeFormatted: FormatSize(totalSize),
		FilesScanned:       filesScanned,
		LargeFiles:         largeFiles,
		TempFiles:          tempFiles,
		LogFiles:           logFiles,
		BackupFiles:        backupFiles,
		CacheFiles:         cacheFiles,
		DuplicateGroups:    duplicateGroups,
		SreData:            sreData,
	}

	// Apply rules engine logic to flag files
	results = rules.ProcessRules(results, activeRules)

	// Sort and limit file lists by size descending to prevent IPC payload size explosion
	// But let's increase limits compared to previous version to show more data
	results.LargeFiles = sortAndLimitFiles(results.LargeFiles, 1000)
	results.TempFiles = sortAndLimitFiles(results.TempFiles, 1000)
	results.LogFiles = sortAndLimitFiles(results.LogFiles, 1000)
	results.BackupFiles = sortAndLimitFiles(results.BackupFiles, 1000)
	results.CacheFiles = sortAndLimitFiles(results.CacheFiles, 1000)
	results.DuplicateGroups = limitDuplicateGroups(results.DuplicateGroups, 500)

	return results, nil
}

func sortAndLimitFiles(files []FileInfo, limit int) []FileInfo {
	sort.Slice(files, func(i, j int) bool {
		return files[i].Size > files[j].Size
	})
	if len(files) > limit {
		return files[:limit]
	}
	return files
}

type dupGroupWasted struct {
	hash  string
	info  []DuplicateFileInfo
	waste int64
}

func limitDuplicateGroups(groups map[string][]DuplicateFileInfo, limit int) map[string][]DuplicateFileInfo {
	if len(groups) <= limit {
		return groups
	}

	wastedList := make([]dupGroupWasted, 0, len(groups))
	for h, list := range groups {
		var waste int64
		if len(list) > 1 {
			waste = int64(len(list)-1) * list[0].Size
		}
		wastedList = append(wastedList, dupGroupWasted{hash: h, info: list, waste: waste})
	}

	sort.Slice(wastedList, func(i, j int) bool {
		return wastedList[i].waste > wastedList[j].waste
	})

	result := make(map[string][]DuplicateFileInfo)
	for i := 0; i < limit && i < len(wastedList); i++ {
		result[wastedList[i].hash] = wastedList[i].info
	}
	return result
}

func getPrefixHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buf := make([]byte, 4096)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	h := xxhash.Sum64(buf[:n])
	return fmt.Sprintf("%016x", h), nil
}

func getFastHash(ctx context.Context, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	size := info.Size()

	h := xxhash.New()

	const portionSize = 1024 * 1024 // 1 MB

	if size <= 2*portionSize {
		// Read entire file
		buf := make([]byte, 65536)
		for {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
			}
			n, err := file.Read(buf)
			if n > 0 {
				h.Write(buf[:n])
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}
		}
	} else {
		// Read first 1 MB
		firstBuf := make([]byte, portionSize)
		_, err = io.ReadFull(file, firstBuf)
		if err != nil && err != io.ErrUnexpectedEOF {
			return "", err
		}
		h.Write(firstBuf)

		// Seek to last 1 MB
		_, err = file.Seek(size-portionSize, io.SeekStart)
		if err != nil {
			return "", err
		}

		lastBuf := make([]byte, portionSize)
		_, err = io.ReadFull(file, lastBuf)
		if err != nil && err != io.ErrUnexpectedEOF {
			return "", err
		}
		h.Write(lastBuf)
	}

	return fmt.Sprintf("%016x", h.Sum64()), nil
}

