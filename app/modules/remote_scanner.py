import os
import time
import json
import paramiko
from datetime import datetime
from PySide6.QtCore import QThread, Signal, QObject
from app.core.config import logger

class RemoteScanProgress(QObject):
    progress = Signal(str, int, float) # (current_dir/status, files_scanned, total_size_bytes)
    finished = Signal(dict)          # results dict

class RemoteSSHScanner(QThread):
    def __init__(self, host_config: dict, target_dir: str, rules: list[dict] = None):
        super().__init__()
        self.host = host_config.get("host", "localhost")
        self.port = int(host_config.get("port", 22))
        self.username = host_config.get("username", "root")
        self.auth_type = host_config.get("auth_type", "password")
        self.credentials = host_config.get("credentials", "")
        self.target_dir = target_dir
        self.rules = rules
        self.signals = RemoteScanProgress()
        self.is_running = True
        self.client = None

    def stop(self):
        self.is_running = False
        if self.client:
            try:
                self.client.close()
            except Exception:
                pass

    @staticmethod
    def test_ssh_connection(host: str, port: int, username: str, auth_type: str, credentials: str) -> tuple[bool, str]:
        """Utility to test SSH credentials."""
        client = paramiko.SSHClient()
        client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        try:
            if auth_type == "password":
                client.connect(host, port=port, username=username, password=credentials, timeout=10)
            else: # key
                # Load private key from path
                if not os.path.exists(credentials):
                    return False, f"Private key file not found: {credentials}"
                try:
                    key = paramiko.RSAKey.from_private_key_file(credentials)
                except paramiko.ssh_exception.PasswordRequiredException:
                    return False, "Private key is password encrypted (passphrase required)."
                except Exception:
                    try:
                        key = paramiko.Ed25519Key.from_private_key_file(credentials)
                    except Exception as e:
                        return False, f"Invalid private key format: {e}"
                client.connect(host, port=port, username=username, pkey=key, timeout=10)
            
            client.close()
            return True, f"SSH connection established successfully to {host}!"
        except Exception as e:
            return False, f"SSH connection failed: {str(e)}"

    def format_size(self, size_bytes: int) -> str:
        for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
            if size_bytes < 1024.0:
                return f"{size_bytes:.2f} {unit}"
            size_bytes /= 1024.0
        return f"{size_bytes:.2f} PB"

    def run(self):
        logger.info(f"Starting remote SSH disk scan on {self.host}:{self.port} -> {self.target_dir}")
        self.signals.progress.emit("Connecting via SSH...", 0, 0)
        
        self.client = paramiko.SSHClient()
        self.client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        
        try:
            # 1. Connect
            if self.auth_type == "password":
                self.client.connect(self.host, port=self.port, username=self.username, password=self.credentials, timeout=15)
            else:
                key = None
                try:
                    key = paramiko.RSAKey.from_private_key_file(self.credentials)
                except Exception:
                    key = paramiko.Ed25519Key.from_private_key_file(self.credentials)
                self.client.connect(self.host, port=self.port, username=self.username, pkey=key, timeout=15)

            if not self.is_running:
                return

            # Check for auto-scan or direct target directory
            target_paths = ["/var/log", "/tmp", "/var/tmp", "/var/cache"]
            if self.target_dir and self.target_dir.strip() not in ("", "Автоматичний пошук", "Auto-detect"):
                target_paths = [self.target_dir.strip()]

            # 2. Get remote disk partition totals
            self.signals.progress.emit("Querying remote disk usage stats...", 0, 0)
            df_target = target_paths[0]
            stdin, stdout, stderr = self.client.exec_command(f"df -B1 '{df_target}' | tail -n 1")
            df_out = stdout.read().decode('utf-8').strip()
            
            total_size = 0
            free_size = 0
            try:
                # df output: Filesystem 1B-blocks Used Available Use% MountedOn
                parts = df_out.split()
                if len(parts) >= 4:
                    total_size = int(parts[1])
                    free_size = int(parts[3])
            except Exception:
                pass

            # 3. Find files on target directory
            self.signals.progress.emit("Compiling files inventory...", 0, 0)
            escaped_paths = " ".join([f"'{p}'" for p in target_paths])
            find_cmd = f"find {escaped_paths} -type f -printf '%p|%s|%A@|%T@\\n' 2>/dev/null"
            stdin, stdout, stderr = self.client.exec_command(find_cmd)
            
            # Read stdout line by line
            files_scanned = 0
            scanned_size = 0
            
            large_files = []
            temp_files = []
            log_files = []
            backup_files = []
            cache_files = []
            
            size_groups = {} # size -> list of paths (to find remote duplicates)

            # We parse lines
            # Example format: /var/log/syslog|124500|17823901.12|17823901.12
            for line in stdout:
                if not self.is_running:
                    break
                    
                line = line.strip()
                if not line:
                    continue
                    
                parts = line.split('|')
                if len(parts) < 4:
                    continue
                    
                file_path = parts[0]
                try:
                    size = int(parts[1])
                    atime = float(parts[2])
                    mtime = float(parts[3])
                except Exception:
                    continue

                files_scanned += 1
                scanned_size += size
                
                # Format timestamps
                last_modified = time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(mtime))
                last_access = time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(atime))
                name = os.path.basename(file_path)
                ext = os.path.splitext(name)[1].lower()

                file_info = {
                    "path": file_path,
                    "name": name,
                    "size": size,
                    "size_formatted": self.format_size(size),
                    "ext": ext,
                    "last_access": last_access,
                    "last_modified": last_modified,
                    "last_modified_ts": mtime,
                    "category": "other"
                }

                # Categorization
                path_lower = file_path.lower()
                temp_dirs = {"temp", "tmp", "cache", "logs", "log"}
                
                if ext in ('.tmp', '.temp', '.bak', '.old') or any(td in path_lower for td in temp_dirs):
                    if ext in ('.log', '.txt') and "log" in path_lower:
                        file_info["category"] = "log"
                        log_files.append(file_info)
                    elif "cache" in path_lower:
                        file_info["category"] = "cache"
                        cache_files.append(file_info)
                    else:
                        file_info["category"] = "temp"
                        temp_files.append(file_info)
                elif ext == '.log' or (ext == '.txt' and 'log' in name.lower()):
                    file_info["category"] = "log"
                    log_files.append(file_info)
                elif ext in ('.zip', '.rar', '.tar', '.gz', '.7z', '.bak') and ('backup' in name.lower() or 'bak' in name.lower()):
                    file_info["category"] = "backup"
                    backup_files.append(file_info)
                
                if size > 100 * 1024 * 1024:
                    if file_info["category"] == "other":
                        file_info["category"] = "large"
                    large_files.append(file_info)

                # Track duplicates
                if size > 1024 * 1024: # Only find duplicates for files > 1 MB
                    size_groups.setdefault(size, []).append(file_path)

                # Send progress occasionally
                if files_scanned % 500 == 0:
                    self.signals.progress.emit(f"Scanning remote files: {files_scanned} processed...", files_scanned, scanned_size)

            if not self.is_running:
                logger.info("Remote scan cancelled by user.")
                self.client.close()
                self.signals.finished.emit({"cancelled": True})
                return

            # 4. Remote Duplicates hashing (Execute md5sum for size collisions)
            duplicate_groups = {}
            if self.is_running:
                colliding_paths = []
                # Find size collisions
                for size, paths in size_groups.items():
                    if not self.is_running:
                        break
                    if len(paths) > 1:
                        colliding_paths.extend(paths)
                
                if colliding_paths and self.is_running:
                    self.signals.progress.emit("Hashing remote size collisions...", files_scanned, scanned_size)
                    
                    # Run md5sum in batches to avoid too long shell commands
                    batch_size = 50
                    full_hashes = {}
                    
                    for i in range(0, len(colliding_paths), batch_size):
                        if not self.is_running:
                            break
                        batch = colliding_paths[i:i+batch_size]
                        # escape file names
                        escaped_batch = " ".join([f"'{p}'" for p in batch])
                        md5_cmd = f"md5sum {escaped_batch} 2>/dev/null"
                        
                        _, md5_stdout, _ = self.client.exec_command(md5_cmd)
                        for line in md5_stdout:
                            if not self.is_running:
                                break
                            parts = line.strip().split(None, 1)
                            if len(parts) == 2:
                                h, p = parts[0], parts[1]
                                # remove quotes/escaping if md5sum wraps them
                                p = p.strip("'\"")
                                full_hashes.setdefault(h, []).append(p)
                                
                    # Filter actual duplicates
                    if self.is_running:
                        for h, paths in full_hashes.items():
                            if not self.is_running:
                                break
                            if len(paths) > 1:
                                size = os.path.getsize(paths[0]) if len(paths[0]) < 250 else size_groups.get(size, [0])[0]
                                # Try to extract the correct size
                                first_path = paths[0]
                                # Find size associated with this path
                                actual_size = 0
                                for sz, p_list in size_groups.items():
                                    if first_path in p_list:
                                        actual_size = sz
                                        break
                                        
                                duplicate_groups[h] = [
                                    {
                                        "path": p,
                                        "size": actual_size,
                                        "size_formatted": self.format_size(actual_size)
                                    }
                                    for p in paths
                                ]

            if not self.is_running:
                logger.info("Remote scan cancelled by user.")
                self.client.close()
                self.signals.finished.emit({"cancelled": True})
                return

            # Sort lists
            large_files.sort(key=lambda x: x["size"], reverse=True)
            temp_files.sort(key=lambda x: x["size"], reverse=True)
            log_files.sort(key=lambda x: x["size"], reverse=True)
            backup_files.sort(key=lambda x: x["size"], reverse=True)
            cache_files.sort(key=lambda x: x["size"], reverse=True)

            # DevOps SRE remote docker scan integration
            from app.modules.sre_analyzer import SREAnalyzer
            sre_analyzer = SREAnalyzer()
            sre_data = sre_analyzer.analyze_docker(self.client)

            # Build results dictionary
            results = {
                "total_size": scanned_size,
                "total_size_formatted": self.format_size(scanned_size),
                "files_scanned": files_scanned,
                "large_files": large_files,
                "temp_files": temp_files,
                "log_files": log_files,
                "backup_files": backup_files,
                "cache_files": cache_files,
                "duplicate_groups": duplicate_groups,
                # Extra remote fields
                "remote_host": self.host,
                "remote_partition_capacity": total_size,
                "remote_partition_free": free_size,
                "sre_data": sre_data
            }

            # Process results via Rules Engine in background thread
            try:
                from app.modules.rules_engine import RulesEngine
                engine = RulesEngine(self.rules)
                results = engine.process_files(results)
            except Exception as e:
                logger.error(f"Error running RulesEngine in remote scanner background: {e}")

            self.client.close()
            self.signals.finished.emit(results)

        except Exception as e:
            logger.error(f"Remote scan failed: {e}")
            if self.client:
                try:
                    self.client.close()
                except Exception:
                    pass
            # Emit empty results with error logs
            self.signals.finished.emit({
                "total_size": 0,
                "total_size_formatted": "0 B",
                "files_scanned": 0,
                "large_files": [], "temp_files": [], "log_files": [], "backup_files": [], "cache_files": [],
                "duplicate_groups": {},
                "error": f"SSH Scan Error: {str(e)}"
            })
