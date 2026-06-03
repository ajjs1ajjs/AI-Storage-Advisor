import os
import time
import hashlib
from pathlib import Path
from PySide6.QtCore import QThread, Signal, QObject
from app.core.config import logger

class ScanProgress(QObject):
    progress = Signal(str, int, float) # (current_dir, files_scanned, total_size_bytes)
    finished = Signal(dict)          # results dict

class DiskScanner(QThread):
    def __init__(self, start_path: str, rules: list[dict] = None):
        super().__init__()
        self.start_path = os.path.abspath(start_path)
        self.rules = rules
        self.signals = ScanProgress()
        self.is_running = True

    def stop(self):
        self.is_running = False

    def format_size(self, size_bytes: int) -> str:
        for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
            if size_bytes < 1024.0:
                return f"{size_bytes:.2f} {unit}"
            size_bytes /= 1024.0
        return f"{size_bytes:.2f} PB"

    def run(self):
        logger.info(f"Starting disk scan on: {self.start_path}")
        
        files_scanned = 0
        total_size = 0
        
        # Categorized lists
        large_files = []
        temp_files = []
        log_files = []
        backup_files = []
        cache_files = []
        all_files = []
        
        # To group duplicates by size
        size_groups = {}

        temp_dirs = {"temp", "tmp", "cache", "logs", "log", "cache"}
        last_emit_time = 0.0

        for root, dirs, files in os.walk(self.start_path, topdown=True):
            if not self.is_running:
                break
                
            # Filter dirs in-place to prevent scanning recursive junctions/symlinks
            # and common heavy folders
            valid_dirs = []
            for d in dirs:
                d_path = os.path.join(root, d)
                try:
                    if os.path.islink(d_path):
                        continue
                    if d in ("$RECYCLE.BIN", "System Volume Information", ".git", "node_modules", ".venv", "venv", "__pycache__", ".idea", ".vscode"):
                        continue
                    valid_dirs.append(d)
                except Exception:
                    continue
            dirs[:] = valid_dirs

            # Send progress update occasionally (max 10 emissions per second to avoid flooding the GUI thread)
            now = time.time()
            if now - last_emit_time > 0.1:
                self.signals.progress.emit(root, files_scanned, total_size)
                last_emit_time = now
            
            for file in files:
                if not self.is_running:
                    break
                    
                file_path = os.path.join(root, file)
                try:
                    # Skip symlinks to prevent loops
                    if os.path.islink(file_path):
                        continue
                        
                    stat = os.stat(file_path)
                    size = stat.st_size
                    files_scanned += 1
                    total_size += size
                    
                    ext = os.path.splitext(file)[1].lower()
                    last_access = time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(stat.st_atime))
                    last_modified = time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(stat.st_mtime))
                    
                    file_info = {
                        "path": file_path,
                        "name": file,
                        "size": size,
                        "size_formatted": self.format_size(size),
                        "ext": ext,
                        "last_access": last_access,
                        "last_modified": last_modified,
                        "last_modified_ts": stat.st_mtime,
                        "category": "other"
                    }
                    
                    # 1. Categorization
                    path_lower = file_path.lower()
                    
                    # Temporary
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
                    # Logs
                    elif ext == '.log' or (ext == '.txt' and 'log' in file.lower()):
                        file_info["category"] = "log"
                        log_files.append(file_info)
                    # Backups
                    elif ext in ('.zip', '.rar', '.tar', '.gz', '.7z', '.bak') and ('backup' in file.lower() or 'bak' in file.lower()):
                        file_info["category"] = "backup"
                        backup_files.append(file_info)
                    
                    # Large files (> 100 MB)
                    if size > 100 * 1024 * 1024:
                        if file_info["category"] == "other":
                            file_info["category"] = "large"
                        large_files.append(file_info)
                        
                    all_files.append(file_info)

                    # Group for duplicate matching
                    if size > 1024 * 1024: # Only find duplicates for files > 1 MB
                        size_groups.setdefault(size, []).append(file_path)

                except (PermissionError, FileNotFoundError):
                    continue
                except Exception as e:
                    logger.debug(f"Error reading file {file_path}: {e}")
                    continue

        if not self.is_running:
            logger.info("Disk scan cancelled by user.")
            self.signals.finished.emit({"cancelled": True})
            return

        # 2. Fast Duplicate Finding (Size grouping + Hash prefix verification)
        duplicate_groups = {}
        for size, paths in size_groups.items():
            if not self.is_running:
                break
            if len(paths) > 1:
                # Group by prefix hash (first 4KB)
                prefix_groups = {}
                for p in paths:
                    if not self.is_running:
                        break
                    try:
                        with open(p, "rb") as f:
                            prefix = f.read(4096)
                            prefix_hash = hashlib.md5(prefix).hexdigest()
                            prefix_groups.setdefault(prefix_hash, []).append(p)
                    except Exception:
                        continue
                
                # For remaining collisions, compute full hash
                for p_hash, colliding_paths in prefix_groups.items():
                    if not self.is_running:
                        break
                    if len(colliding_paths) > 1:
                        full_groups = {}
                        for p in colliding_paths:
                            if not self.is_running:
                                break
                            try:
                                h = hashlib.md5()
                                with open(p, "rb") as f:
                                    # read in chunks
                                    for chunk in iter(lambda: f.read(65536), b""):
                                        if not self.is_running:
                                            break
                                        h.update(chunk)
                                if not self.is_running:
                                    break
                                full_groups.setdefault(h.hexdigest(), []).append(p)
                            except Exception:
                                continue
                        
                        if not self.is_running:
                            break

                        for f_hash, dup_paths in full_groups.items():
                            if len(dup_paths) > 1:
                                duplicate_groups[f_hash] = [
                                    {
                                        "path": p,
                                        "size": size,
                                        "size_formatted": self.format_size(size)
                                    }
                                    for p in dup_paths
                                ]

        if not self.is_running:
            logger.info("Disk scan cancelled by user.")
            self.signals.finished.emit({"cancelled": True})
            return

        # Sort large files descending
        large_files.sort(key=lambda x: x["size"], reverse=True)
        temp_files.sort(key=lambda x: x["size"], reverse=True)
        log_files.sort(key=lambda x: x["size"], reverse=True)
        backup_files.sort(key=lambda x: x["size"], reverse=True)
        cache_files.sort(key=lambda x: x["size"], reverse=True)

        # Windows SRE system folders inspection
        from app.modules.sre_analyzer import SREAnalyzer
        sre = SREAnalyzer()
        sre_data = sre.analyze_windows_system()

        results = {
            "total_size": total_size,
            "total_size_formatted": self.format_size(total_size),
            "files_scanned": files_scanned,
            "large_files": large_files,
            "temp_files": temp_files,
            "log_files": log_files,
            "backup_files": backup_files,
            "cache_files": cache_files,
            "duplicate_groups": duplicate_groups,
            "sre_data": sre_data
        }

        # Process results via Rules Engine in background thread
        try:
            from app.modules.rules_engine import RulesEngine
            engine = RulesEngine(self.rules)
            results = engine.process_files(results)
        except Exception as e:
            logger.error(f"Error running RulesEngine in background scanner: {e}")
        
        self.signals.finished.emit(results)
        logger.info(f"Disk scan completed. Total files: {files_scanned}, Total Size: {self.format_size(total_size)}")
