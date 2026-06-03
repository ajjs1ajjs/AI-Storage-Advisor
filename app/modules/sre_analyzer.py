import os
import re
from app.core.config import logger

class SREAnalyzer:
    def __init__(self):
        pass

    def format_size(self, size_bytes: int) -> str:
        for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
            if size_bytes < 1024.0:
                return f"{size_bytes:.2f} {unit}"
            size_bytes /= 1024.0
        return f"{size_bytes:.2f} PB"

    def parse_docker_size(self, size_str: str) -> tuple[int, int]:
        """Parses docker ps -s size strings (e.g. '21MB (virtual 1.5GB)' or '0B (virtual 1.2GB)')
        and returns (writeable_layer_bytes, virtual_size_bytes)."""
        # Default
        write_bytes = 0
        virt_bytes = 0
        
        try:
            match = re.search(r'([0-9.]+)\s*([a-zA-Z]+)\s*\(virtual\s*([0-9.]+)\s*([a-zA-Z]+)\)', size_str)
            if match:
                w_val, w_unit, v_val, v_unit = match.groups()
                
                # Convert units
                units = {'B': 1, 'KB': 1024, 'MB': 1024**2, 'GB': 1024**3, 'TB': 1024**4, 'b': 1, 'kb': 1024, 'mb': 1024**2, 'gb': 1024**3}
                write_bytes = int(float(w_val) * units.get(w_unit, 1))
                virt_bytes = int(float(v_val) * units.get(v_unit, 1))
            else:
                # Fallback: simple parse if no virtual size
                match2 = re.search(r'([0-9.]+)\s*([a-zA-Z]+)', size_str)
                if match2:
                    val, unit = match2.groups()
                    units = {'B': 1, 'KB': 1024, 'MB': 1024**2, 'GB': 1024**3, 'TB': 1024**4}
                    write_bytes = int(float(val) * units.get(unit, 1))
        except Exception:
            pass
            
        return write_bytes, virt_bytes

    def analyze_docker(self, ssh_client=None) -> dict:
        """Queries Docker container write layers and volume sizes remotely via SSH client.
        If ssh_client is None, queries local machine if docker command is available."""
        result = {
            "docker_active": False,
            "containers": [],
            "volumes": []
        }

        # Command to run
        cmd = 'docker ps -a --size --format "{{.ID}}|{{.Names}}|{{.Image}}|{{.Size}}" 2>/dev/null'

        try:
            if ssh_client:
                stdin, stdout, stderr = ssh_client.exec_command(cmd)
                lines = stdout.readlines()
            else:
                # Query local command line
                import subprocess
                res = subprocess.run("docker ps -a --size --format \"{{.ID}}|{{.Names}}|{{.Image}}|{{.Size}}\"", shell=True, capture_output=True, text=True, timeout=5)
                if res.returncode == 0:
                    lines = res.stdout.splitlines()
                else:
                    return result

            if not lines:
                return result

            result["docker_active"] = True
            for line in lines:
                line = line.strip()
                if not line:
                    continue
                parts = line.split('|')
                if len(parts) < 4:
                    continue
                
                cid, name, image, size_raw = parts[0], parts[1], parts[2], parts[3]
                w_bytes, v_bytes = self.parse_docker_size(size_raw)
                
                result["containers"].append({
                    "id": cid,
                    "name": name,
                    "image": image,
                    "write_size": w_bytes,
                    "write_size_formatted": self.format_size(w_bytes),
                    "virtual_size": v_bytes,
                    "virtual_size_formatted": self.format_size(v_bytes)
                })

            # Also query volumes sizes if docker active
            vol_cmd = "docker system df -v 2>/dev/null"
            if ssh_client:
                _, vol_stdout, _ = ssh_client.exec_command(vol_cmd)
                vol_lines = vol_stdout.readlines()
            else:
                import subprocess
                vol_res = subprocess.run("docker system df -v", shell=True, capture_output=True, text=True, timeout=5)
                vol_lines = vol_res.stdout.splitlines() if vol_res.returncode == 0 else []

            # Simple parser for docker system df -v volumes section
            # Format typically starts with "Local Volumes space:" or "VOLUME NAME   LINKS   SIZE"
            vol_started = False
            for line in vol_lines:
                line = line.strip()
                if "VOLUME NAME" in line:
                    vol_started = True
                    continue
                if vol_started and line:
                    parts = line.split()
                    if len(parts) >= 3:
                        v_name = parts[0]
                        v_size_raw = parts[2]
                        
                        # Simple size conversion
                        v_bytes = 0
                        m = re.search(r'([0-9.]+)([a-zA-Z]+)', v_size_raw)
                        if m:
                            val, unit = m.groups()
                            units = {'B': 1, 'KB': 1024, 'MB': 1024**2, 'GB': 1024**3, 'B': 1, 'KB': 1024, 'MB': 1024**2, 'GB': 1024**3}
                            v_bytes = int(float(val) * units.get(unit.upper(), 1))
                            
                        result["volumes"].append({
                            "name": v_name,
                            "size": v_bytes,
                            "size_formatted": self.format_size(v_bytes)
                        })

        except Exception as e:
            logger.debug(f"Docker analysis skipped: {e}")
            
        return result

    def analyze_windows_system(self) -> dict:
        """Inspects specific Windows folders (Minidumps, IIS Logs, Event Logs)
        to identify system cleanup opportunities."""
        result = {
            "windows_active": os.name == 'nt',
            "folders": {
                "minidumps": {"path": "C:\\Windows\\Minidump", "size": 0, "count": 0, "size_formatted": "0 B"},
                "iis_logs": {"path": "C:\\inetpub\\logs\\LogFiles", "size": 0, "count": 0, "size_formatted": "0 B"},
                "event_logs": {"path": "C:\\Windows\\System32\\Winevt\\Logs", "size": 0, "count": 0, "size_formatted": "0 B"},
                "win_temp": {"path": "C:\\Windows\\Temp", "size": 0, "count": 0, "size_formatted": "0 B"}
            }
        }

        if not result["windows_active"]:
            return result

        for f_name, f_info in result["folders"].items():
            path = f_info["path"]
            if os.path.exists(path):
                total_size = 0
                file_count = 0
                try:
                    for root, _, files in os.walk(path):
                        for file in files:
                            f_path = os.path.join(root, file)
                            try:
                                total_size += os.path.getsize(f_path)
                                file_count += 1
                            except Exception:
                                continue
                    f_info["size"] = total_size
                    f_info["count"] = file_count
                    f_info["size_formatted"] = self.format_size(total_size)
                except Exception as e:
                    logger.debug(f"Skipping folder {path} due to permissions: {e}")
                    f_info["size_formatted"] = "Access Denied"
            else:
                f_info["size_formatted"] = "Not Found"

        return result

    def calculate_health_score(self, scan_results: dict, sre_data: dict = None) -> tuple[int, list[str]]:
        """Calculates a Storage Health Score (0 to 100) based on scan metrics, duplicates,
        capacity exhaustion risk, and SRE server bottlenecks."""
        score = 100
        warnings = []

        if not scan_results:
            return 0, ["No scan results available."]

        # 1. Deduct by total disk fill rate
        days = scan_results.get("days_remaining", -1)
        # Check if days_remaining is extracted from forecast screen context
        if days != -1:
            if days < 30:
                score -= 30
                warnings.append(f"Critical: Storage exhaustion projected in {days} days.")
            elif days < 90:
                score -= 15
                warnings.append(f"Warning: Storage exhaustion projected in {days} days.")

        # 2. Deduct by wasted Duplicates space
        dup_waste = 0
        dup_groups = scan_results.get("duplicate_groups", {})
        for h, paths in dup_groups.items():
            # waste is (count - 1) * size
            if len(paths) > 1:
                size = paths[0]["size"]
                dup_waste += (len(paths) - 1) * size

        if dup_waste > 10 * 1024 * 1024 * 1024: # 10 GB
            score -= 15
            warnings.append(f"High Waste: Duplicates are wasting {self.format_size(dup_waste)} disk space.")
        elif dup_waste > 1 * 1024 * 1024 * 1024: # 1 GB
            score -= 8
            warnings.append(f"Waste: Duplicates are wasting {self.format_size(dup_waste)} disk space.")

        # 3. Deduct by unrotated Log files
        log_size = sum(f["size"] for f in scan_results.get("log_files", []))
        if log_size > 5 * 1024 * 1024 * 1024: # 5 GB
            score -= 15
            warnings.append(f"Warning: Unrotated log files are consuming {self.format_size(log_size)}.")
        elif log_size > 500 * 1024 * 1024: # 500 MB
            score -= 5
            warnings.append(f"Log consumption: Log files are consuming {self.format_size(log_size)}.")

        # 4. Deduct by Temporary file build-up
        temp_size = sum(f["size"] for f in scan_results.get("temp_files", []) + scan_results.get("cache_files", []))
        if temp_size > 5 * 1024 * 1024 * 1024: # 5 GB
            score -= 10
            warnings.append(f"Warning: Temporary files are occupying {self.format_size(temp_size)}.")

        # 5. Deduct by SRE data (Docker containers size leaks)
        if sre_data:
            # Check container layers size
            containers = sre_data.get("containers", [])
            large_write_layers = [c for c in containers if c["write_size"] > 1 * 1024 * 1024 * 1024] # > 1 GB layer
            if large_write_layers:
                score -= 10
                warnings.append(f"DevOps Warning: {len(large_write_layers)} Docker container(s) have write-layers > 1 GB (potential unrotated app logs).")

            # Check Windows crash dumps
            folders = sre_data.get("folders", {})
            dump_info = folders.get("minidumps", {})
            if dump_info and isinstance(dump_info.get("size"), int) and dump_info["size"] > 500 * 1024 * 1024:
                score -= 10
                warnings.append(f"Windows Warning: Crash memory dumps folder is consuming {dump_info['size_formatted']}.")

        score = max(0, min(100, score))
        return score, warnings
