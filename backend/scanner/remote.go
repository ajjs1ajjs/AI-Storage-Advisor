package scanner

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aisadvisor/backend/db"
	"aisadvisor/backend/rules"
	"aisadvisor/backend/sre"

	"golang.org/x/crypto/ssh"
)

// KnownHostsStore manages SSH host key verification using TOFU (Trust On First Use)
// Keys are stored in the settings table as 'ssh_known_key_<host_fingerprint>'
var knownHostsCache = make(map[string]ssh.PublicKey)

func getHostKeyStoreKey(host string, key ssh.PublicKey) string {
	h := hmac.New(sha256.New, []byte("ssh-known-host"))
	h.Write([]byte(host))
	h.Write(key.Marshal())
	return "ssh_known_key_" + base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func storeHostKey(host string, key ssh.PublicKey) {
	storeKey := getHostKeyStoreKey(host, key)
	marshaled := key.Marshal()
	encoded := base64.StdEncoding.EncodeToString(marshaled)
	_, err := db.DB.Exec(
		"INSERT INTO settings (profile_id, setting_key, setting_value) VALUES (1, ?, ?) "+
			"ON CONFLICT(profile_id, setting_key) DO UPDATE SET setting_value = excluded.setting_value",
		storeKey, encoded,
	)
	if err != nil {
		return
	}
	knownHostsCache[storeKey] = key
}

func verifyHostKey(host string, remote net.Addr, key ssh.PublicKey) error {
	storeKey := getHostKeyStoreKey(host, key)
	if cached, ok := knownHostsCache[storeKey]; ok {
		if !hmac.Equal(cached.Marshal(), key.Marshal()) {
			return fmt.Errorf("host key mismatch for %s: possible MITM attack", host)
		}
		return nil
	}

	// Check DB for stored key
	var encoded string
	err := db.DB.QueryRow("SELECT setting_value FROM settings WHERE profile_id = 1 AND setting_key = ?", storeKey).Scan(&encoded)
	if err == sql.ErrNoRows {
		// First connection — store key (TOFU)
		storeHostKey(host, key)
		return nil
	}
	if err != nil {
		// DB error — allow but log
		storeHostKey(host, key)
		return nil
	}

	marshaled, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("failed to decode stored host key for %s: %w", host, err)
	}

	storedKey, err := ssh.ParsePublicKey(marshaled)
	if err != nil {
		return fmt.Errorf("failed to parse stored host key for %s: %w", host, err)
	}

	if !hmac.Equal(storedKey.Marshal(), key.Marshal()) {
		return fmt.Errorf("host key mismatch for %s: possible MITM attack", host)
	}

	knownHostsCache[storeKey] = key
	return nil
}

func ConnectSSH(host string, port int, username string, authType string, credentials string, keyPassphrase string) (*ssh.Client, error) {
	var authMethod ssh.AuthMethod

	if authType == "password" {
		authMethod = ssh.Password(credentials)
	} else {
		// Key authentication
		keyBytes, err := os.ReadFile(credentials)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}

		var signer ssh.Signer
		if keyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(keyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to parse private key (if it is encrypted, passphrase is required): %w", err)
		}
		authMethod = ssh.PublicKeys(signer)
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: verifyHostKey,
		Timeout:         15 * time.Second,
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh connection failed: %w", err)
	}

	return client, nil
}

func TestSSHConnection(host string, port int, username string, authType string, credentials string, keyPassphrase string) (bool, string) {
	client, err := ConnectSSH(host, port, username, authType, credentials, keyPassphrase)
	if err != nil {
		return false, err.Error()
	}
	client.Close()
	return true, fmt.Sprintf("SSH connection established successfully to %s!", host)
}

func RunSSHCommand(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	out, err := session.Output(cmd)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellQuote(value string) string {
	return ShellQuote(value)
}

func ScanRemoteSSH(ctx context.Context, hostConfig map[string]interface{}, targetDir string, activeRules []rules.Rule, progressCallback func(status string, filesScanned int, totalSize int64)) (ScanResults, error) {
	host := hostConfig["host"].(string)
	port := int(hostConfig["port"].(float64)) // JSON parses numbers as float64
	username := hostConfig["username"].(string)
	authType := hostConfig["auth_type"].(string)
	credentials := hostConfig["credentials"].(string)
	keyPassphrase := hostConfig["key_passphrase"].(string)

	progressCallback("Connecting via SSH...", 0, 0)
	client, err := ConnectSSH(host, port, username, authType, credentials, keyPassphrase)
	if err != nil {
		return ScanResults{Cancelled: false}, fmt.Errorf("SSH Connection Error: %w", err)
	}
	defer client.Close()

	select {
	case <-ctx.Done():
		return ScanResults{Cancelled: true}, nil
	default:
	}

	// 1. Get remote capacity using df
	progressCallback("Querying remote disk usage stats...", 0, 0)
	dfTarget := "/var/log"
	if targetDir != "" && targetDir != "Автоматичний пошук" && targetDir != "Auto-detect" {
		dfTarget = targetDir
	}

	dfOut, err := RunSSHCommand(client, fmt.Sprintf("df -B1 %s | tail -n 1", shellQuote(dfTarget)))
	if err == nil {
		fields := strings.Fields(dfOut)
		if len(fields) >= 4 {
			_, _ = strconv.ParseInt(fields[1], 10, 64)
			_, _ = strconv.ParseInt(fields[3], 10, 64)
		}
	}

	// 2. Compile remote files inventory
	progressCallback("Compiling files inventory...", 0, 0)
	targetPaths := []string{"/var/log", "/tmp", "/var/tmp", "/var/cache"}
	if targetDir != "" && targetDir != "Автоматичний пошук" && targetDir != "Auto-detect" {
		targetPaths = []string{targetDir}
	}

	escapedPaths := make([]string, len(targetPaths))
	for i, p := range targetPaths {
		escapedPaths[i] = shellQuote(p)
	}

	findCmd := fmt.Sprintf("find %s -type f -printf '%%p|%%s|%%A@|%%T@\\n' 2>/dev/null", strings.Join(escapedPaths, " "))

	session, err := client.NewSession()
	if err != nil {
		return ScanResults{}, err
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return ScanResults{}, err
	}

	if err := session.Start(findCmd); err != nil {
		return ScanResults{}, err
	}

	largeFiles := make([]FileInfo, 0)
	tempFiles := make([]FileInfo, 0)
	logFiles := make([]FileInfo, 0)
	backupFiles := make([]FileInfo, 0)
	cacheFiles := make([]FileInfo, 0)

	sizeGroups := make(map[int64][]string)
	var filesScanned int
	var scannedSize int64

	scanner := bufio.NewScanner(stdout)
	tempDirs := []string{"temp", "tmp", "cache", "logs", "log"}

	// Read remote output line by line
	go func() {
		<-ctx.Done()
		session.Signal(ssh.SIGINT) // Abort remote command on cancel
	}()

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ScanResults{Cancelled: true}, nil
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		filePath := parts[0]
		size, errSize := strconv.ParseInt(parts[1], 10, 64)
		atimeF, errAtime := strconv.ParseFloat(parts[2], 64)
		mtimeF, errMtime := strconv.ParseFloat(parts[3], 64)

		if errSize != nil || errAtime != nil || errMtime != nil {
			continue
		}

		filesScanned++
		scannedSize += size

		if filesScanned%500 == 0 {
			progressCallback(fmt.Sprintf("Scanning remote files: %d processed...", filesScanned), filesScanned, scannedSize)
		}

		name := path.Base(filePath)
		nameLower := strings.ToLower(name)
		pathLower := strings.ToLower(filePath)
		ext := strings.ToLower(path.Ext(name))

		lastAccess := time.Unix(int64(atimeF), 0).Format("2006-01-02 15:04:05")
		lastModified := time.Unix(int64(mtimeF), 0).Format("2006-01-02 15:04:05")

		fileInfo := FileInfo{
			Path:           filePath,
			Name:           name,
			Size:           size,
			SizeFormatted:  FormatSize(size),
			Ext:            ext,
			LastAccess:     lastAccess,
			LastModified:   lastModified,
			LastModifiedTS: int64(mtimeF),
			Category:       "other",
		}

		// Categorize
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

		if size > 1*1024*1024 {
			sizeGroups[size] = append(sizeGroups[size], filePath)
		}
	}

	session.Wait()

	if ctx.Err() == context.Canceled {
		return ScanResults{Cancelled: true}, nil
	}

	// 3. Remote Duplicates MD5 hashing
	duplicateGroups := make(map[string][]DuplicateFileInfo)
	collidingPaths := make([]string, 0)
	collidingSizes := make(map[string]int64)

	for size, paths := range sizeGroups {
		if len(paths) > 1 {
			collidingPaths = append(collidingPaths, paths...)
			for _, p := range paths {
				collidingSizes[p] = size
			}
		}
	}

	if len(collidingPaths) > 0 {
		progressCallback("Hashing remote size collisions...", filesScanned, scannedSize)
		batchSize := 50
		fullHashes := make(map[string][]string)

		for i := 0; i < len(collidingPaths); i += batchSize {
			select {
			case <-ctx.Done():
				return ScanResults{Cancelled: true}, nil
			default:
			}

			end := i + batchSize
			if end > len(collidingPaths) {
				end = len(collidingPaths)
			}
			batch := collidingPaths[i:end]

			escapedBatch := make([]string, len(batch))
			for j, p := range batch {
				escapedBatch[j] = shellQuote(p)
			}

			md5Cmd := fmt.Sprintf("md5sum %s 2>/dev/null", strings.Join(escapedBatch, " "))
			md5Out, err := RunSSHCommand(client, md5Cmd)
			if err == nil {
				scannerMd5 := bufio.NewScanner(strings.NewReader(md5Out))
				for scannerMd5.Scan() {
					line := strings.TrimSpace(scannerMd5.Text())
					if h, p, ok := parseMD5SumLine(line); ok {
						fullHashes[h] = append(fullHashes[h], p)
					}
				}
			}
		}

		for h, paths := range fullHashes {
			if len(paths) > 1 {
				dups := make([]DuplicateFileInfo, 0)
				for _, p := range paths {
					size := collidingSizes[p]
					dups = append(dups, DuplicateFileInfo{
						Path:          p,
						Size:          size,
						SizeFormatted: FormatSize(size),
					})
				}
				duplicateGroups[h] = dups
			}
		}
	}

	// 4. Remote DevOps SRE Docker analysis
	sreData := AnalyzeRemoteDocker(client)
	sreData.PackageCaches = AnalyzeRemotePackageCaches(client)

	results := ScanResults{
		TotalSize:          scannedSize,
		TotalSizeFormatted: FormatSize(scannedSize),
		FilesScanned:       filesScanned,
		LargeFiles:         largeFiles,
		TempFiles:          tempFiles,
		LogFiles:           logFiles,
		BackupFiles:        backupFiles,
		CacheFiles:         cacheFiles,
		DuplicateGroups:    duplicateGroups,
		SreData:            sreData,
	}

	// Apply rules engine
	results = rules.ProcessRules(results, activeRules)

	return results, nil
}

func parseMD5SumLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if len(line) < 34 {
		return "", "", false
	}

	hash := line[:32]
	rest := strings.TrimSpace(line[32:])
	if rest == "" {
		return "", "", false
	}
	if strings.HasPrefix(rest, "*") {
		rest = strings.TrimPrefix(rest, "*")
	}

	return hash, strings.Trim(rest, "'\""), true
}

func AnalyzeRemoteDocker(client *ssh.Client) sre.SreReport {
	report := sre.SreReport{
		DockerActive:  false,
		Containers:    make([]sre.ContainerInfo, 0),
		Volumes:       make([]sre.VolumeInfo, 0),
		WindowsActive: false,
		Folders:       make(map[string]sre.FolderInfo),
	}

	// Check if docker CLI is available on remote
	_, err := RunSSHCommand(client, "which docker 2>/dev/null")
	if err != nil {
		return report
	}

	cmd := `docker ps -a --size --format "{{.ID}}|{{.Names}}|{{.Image}}|{{.Size}}" 2>/dev/null`
	out, err := RunSSHCommand(client, cmd)
	if err != nil {
		return report
	}

	report.DockerActive = true
	lines := strings.Split(out, "\n")
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
		wBytes, vBytes := sre.ParseDockerSize(sizeRaw)

		report.Containers = append(report.Containers, sre.ContainerInfo{
			ID:                   cid,
			Name:                 name,
			Image:                image,
			WriteSize:            wBytes,
			WriteSizeFormatted:   sre.FormatSize(wBytes),
			VirtualSize:          vBytes,
			VirtualSizeFormatted: sre.FormatSize(vBytes),
		})
	}

	volCmd := "docker system df -v 2>/dev/null"
	volOut, err := RunSSHCommand(client, volCmd)
	if err == nil {
		volLines := strings.Split(volOut, "\n")
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

					report.Volumes = append(report.Volumes, sre.VolumeInfo{
						Name:          vName,
						Size:          vBytes,
						SizeFormatted: sre.FormatSize(vBytes),
					})
				}
			}
		}
	}

	return report
}

func AnalyzeRemotePackageCaches(client *ssh.Client) []sre.PackageCacheInfo {
	caches := make([]sre.PackageCacheInfo, 0)
	cmd := "du -sb /var/cache/apt/archives /var/cache/dnf /var/cache/yum ~/.npm ~/.cache/pip /var/cache/pacman/pkg /var/cache/zypp/packages ~/.cargo/registry/cache ~/.cache/go-build 2>/dev/null"
	out, err := RunSSHCommand(client, cmd)
	if err != nil {
		return caches
	}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sizeVal, errSize := strconv.ParseInt(fields[0], 10, 64)
		if errSize != nil {
			continue
		}
		p := fields[1]

		name := ""
		cleanCmd := ""
		if strings.Contains(p, "apt") {
			name = "apt (Debian/Ubuntu Cache)"
			cleanCmd = "sudo apt-get clean || apt-get clean"
		} else if strings.Contains(p, "dnf") || strings.Contains(p, "yum") {
			name = "yum/dnf (RHEL/CentOS Cache)"
			cleanCmd = "sudo dnf clean all || sudo yum clean all"
		} else if strings.Contains(p, "pacman") {
			name = "pacman (Arch Cache)"
			cleanCmd = "sudo pacman -Scc --noconfirm || pacman -Scc --noconfirm"
		} else if strings.Contains(p, "zypp") {
			name = "zypper (openSUSE Cache)"
			cleanCmd = "sudo zypper clean -a || zypper clean -a"
		} else if strings.Contains(p, ".npm") {
			name = "npm (Node.js Cache)"
			cleanCmd = "npm cache clean --force"
		} else if strings.Contains(p, "pip") {
			name = "pip (Python Cache)"
			cleanCmd = "pip cache purge"
		} else if strings.Contains(p, "cargo") {
			name = "cargo (Rust Cache)"
			cleanCmd = "rm -rf ~/.cargo/registry/cache/*"
		} else if strings.Contains(p, "go-build") {
			name = "go mod (Go Cache)"
			cleanCmd = "go clean -cache -modcache"
		}

		if name != "" && sizeVal > 0 {
			caches = append(caches, sre.PackageCacheInfo{
				Name:          name,
				Path:          p,
				Size:          sizeVal,
				SizeFormatted: sre.FormatSize(sizeVal),
				CleanCmd:      cleanCmd,
			})
		}
	}
	return caches
}
