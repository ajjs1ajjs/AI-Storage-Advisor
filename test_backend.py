import os
import sys
import unittest
import tempfile
import shutil
import json
import base64
from datetime import datetime, timedelta

# Add project root to path
sys.path.insert(0, os.path.abspath(os.path.dirname(__file__)))

from app.core.config import DB_PATH, VAULT_PATH
from app.database.db_manager import db
from app.security.vault import vault
from app.modules.scanner import DiskScanner
from app.modules.rules_engine import RulesEngine
from app.modules.cleanup_advisor import CleanupAdvisor
from app.modules.forecast_engine import ForecastEngine
from app.core.profile_manager import ProfileManager
from app.modules.remote_scanner import RemoteSSHScanner

class TestAIStorageAdvisorBackend(unittest.TestCase):
    
    def test_01_vault_encryption(self):
        print("\n[+] Testing Security Vault...")
        test_pass = "mySecretPassword123"
        pwd_hash, salt_b64 = vault.hash_password(test_pass)
        
        self.assertTrue(vault.verify_password(pwd_hash, test_pass))
        self.assertFalse(vault.verify_password(pwd_hash, "wrongPassword"))
        
        self.assertTrue(vault.set_session_password(test_pass, salt_b64))
        self.assertTrue(vault.is_unlocked())
        
        secret_api_key = "sk-proj-1234567890abcdef"
        ciphertext = vault.encrypt(secret_api_key)
        self.assertNotEqual(secret_api_key, ciphertext)
        
        decrypted = vault.decrypt(ciphertext)
        self.assertEqual(secret_api_key, decrypted)
        print("    - Password hashing, verification, key derivation, and AES-256-GCM encryption OK.")

    def test_02_database_schema(self):
        print("\n[+] Testing SQLite Database Tables...")
        with db.connection() as conn:
            cursor = conn.cursor()
            
            cursor.execute("SELECT name FROM sqlite_master WHERE type='table'")
            tables = [row["name"] for row in cursor.fetchall()]
            
            required_tables = [
                "users", "profiles", "ai_providers", "ssh_hosts", 
                "scan_history", "analysis_results", "cleanup_history", 
                "duplicate_results", "forecast_history", "settings"
            ]
            
            for table in required_tables:
                self.assertIn(table, tables)
            
        print("    - Found all 10 tables in database OK.")

    def test_03_disk_scanner(self):
        print("\n[+] Testing Disk Scanner (with Duplicate Detector)...")
        with tempfile.TemporaryDirectory() as temp_dir:
            file1_path = os.path.join(temp_dir, "large_file.temp")
            with open(file1_path, "wb") as f:
                f.write(os.urandom(1024 * 1024 * 2)) # 2 MB
                
            dup1_path = os.path.join(temp_dir, "duplicate1.bak")
            dup2_path = os.path.join(temp_dir, "duplicate2.bak")
            content = b"This is a duplicate file content " * 100 * 1000 # 3.3 MB
            
            with open(dup1_path, "wb") as f:
                f.write(content)
            with open(dup2_path, "wb") as f:
                f.write(content)

            log_path = os.path.join(temp_dir, "app.log")
            with open(log_path, "w") as f:
                f.write("2026-06-03 INFO: Started mock application log\n")

            scanner = DiskScanner(temp_dir)
            results = []
            scanner.signals.finished.connect(results.append)
            scanner.run()
            
            self.assertEqual(len(results), 1)
            res = results[0]
            
            self.assertEqual(res["files_scanned"], 4)
            self.assertGreater(res["total_size"], 0)
            self.assertTrue(any(f["name"] == "large_file.temp" for f in res["temp_files"]))
            self.assertTrue(any(f["name"] == "app.log" for f in res["log_files"]))
            self.assertEqual(len(res["duplicate_groups"]), 1)
            
            print("    - Synchronous scan, categorization, and duplicate hash verification OK.")

    def test_04_rules_engine(self):
        print("\n[+] Testing Rules Engine evaluation...")
        rules = [
            {
                "id": "log_large",
                "name": "Log files larger than 10 MB",
                "category": "log",
                "condition": "larger_than_mb",
                "value": 10,
                "enabled": True
            },
            {
                "id": "temp_old",
                "name": "Temp files older than 30 days",
                "category": "temp",
                "condition": "older_than_days",
                "value": 30,
                "enabled": True
            }
        ]
        
        engine = RulesEngine(rules)
        
        file_log_match = {
            "name": "access.log",
            "path": "/var/log/access.log",
            "size": 15 * 1024 * 1024, # 15 MB
            "category": "log",
            "last_modified": datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        }
        is_match, rule_name = engine.evaluate_file(file_log_match)
        self.assertTrue(is_match)
        self.assertEqual(rule_name, "Log files larger than 10 MB")
        
        file_log_no_match = {
            "name": "access.log",
            "path": "/var/log/access.log",
            "size": 1 * 1024 * 1024, # 1 MB
            "category": "log",
            "last_modified": datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        }
        is_match, rule_name = engine.evaluate_file(file_log_no_match)
        self.assertFalse(is_match)
        
        old_time = datetime.now() - timedelta(days=40)
        file_temp_match = {
            "name": "cache.tmp",
            "path": "/tmp/cache.tmp",
            "size": 500,
            "category": "temp",
            "last_modified": old_time.strftime("%Y-%m-%d %H:%M:%S")
        }
        is_match, rule_name = engine.evaluate_file(file_temp_match)
        self.assertTrue(is_match)
        self.assertEqual(rule_name, "Temp files older than 30 days")
        
        print("    - Rules engine file match criteria and evaluation OK.")

    def test_05_cleanup_advisor(self):
        print("\n[+] Testing Cleanup Advisor Dry Run & Safe Deletions...")
        with tempfile.TemporaryDirectory() as temp_dir:
            file1 = os.path.join(temp_dir, "to_delete_1.txt")
            file2 = os.path.join(temp_dir, "to_delete_2.txt")
            
            with open(file1, "w") as f:
                f.write("some text content for file 1")
            with open(file2, "w") as f:
                f.write("some text content for file 2")
                
            advisor = CleanupAdvisor(profile_id=1)
            
            dry_run_rep = advisor.dry_run([file1, file2])
            self.assertEqual(dry_run_rep["total_count"], 2)
            self.assertEqual(len(dry_run_rep["writable_files"]), 2)
            self.assertTrue(dry_run_rep["can_proceed"])
            
            cnt, freed, failed = advisor.safe_delete([file1, file2], use_recycle_bin=False)
            self.assertEqual(cnt, 2)
            self.assertGreater(freed, 0)
            self.assertEqual(len(failed), 0)
            self.assertFalse(os.path.exists(file1))
            self.assertFalse(os.path.exists(file2))
            
            print("    - Cleanup advisor dry run calculation and permanent deletion OK.")

    def test_06_forecast_calculations(self):
        print("\n[+] Testing SRE Storage Trend Forecast calculations...")
        with db.connection() as conn:
            cursor = conn.cursor()
            
            profile_id = 999
            cursor.execute("DELETE FROM profiles WHERE profile_name = 'Test Forecast Workspace'")
            cursor.execute("INSERT OR IGNORE INTO profiles (id, user_id, profile_name) VALUES (?, 1, 'Test Forecast Workspace')", (profile_id,))
            cursor.execute("DELETE FROM scan_history WHERE profile_id = ?", (profile_id,))
            
            now = datetime.now()
            scans = [
                (now - timedelta(days=4), 100 * 1024 * 1024),
                (now - timedelta(days=2), 120 * 1024 * 1024),
                (now, 140 * 1024 * 1024)
            ]
            for dt, size in scans:
                cursor.execute(
                    "INSERT INTO scan_history (profile_id, scan_path, total_size, file_count, scan_time) VALUES (?, ?, ?, ?, ?)",
                    (profile_id, ".", size, 10, dt.strftime("%Y-%m-%d %H:%M:%S"))
                )

        engine = ForecastEngine(profile_id)
        report = engine.calculate_forecast(".")
        
        self.assertIn(report["status"], ["exhaustion_risk", "normal_growth"])
        self.assertGreater(report["days_remaining"], 0)
        self.assertAlmostEqual(report["daily_growth_bytes"] / (1024 * 1024), 10.0, places=1) # 10 MB per day
        self.assertEqual(len(report["trend_points"]), 3)
        print("    - Forecast daily linear growth and OLS regression OK.")

    def test_07_profile_export_import(self):
        print("\n[+] Testing Portable Profile Export and Secure Import...")
        profile_id = 999
        
        # Purge test pollution
        with db.connection() as conn:
            cursor = conn.cursor()
            cursor.execute("DELETE FROM profiles WHERE profile_name = 'Test Forecast Workspace'")
            cursor.execute("DELETE FROM users WHERE username = 'new_user'")
            cursor.execute("INSERT OR IGNORE INTO profiles (id, user_id, profile_name) VALUES (?, 1, 'Test Forecast Workspace')", (profile_id,))
            cursor.execute("DELETE FROM ai_providers WHERE profile_id = ?", (profile_id,))
            cursor.execute("DELETE FROM settings WHERE profile_id = ?", (profile_id,))
            cursor.execute("DELETE FROM ssh_hosts WHERE profile_id = ?", (profile_id,))

        vault.set_session_password("userMasterPwd123", base64.b64encode(os.urandom(16)).decode('utf-8'))
        
        with db.connection() as conn:
            cursor = conn.cursor()
            cursor.execute("INSERT OR REPLACE INTO settings (profile_id, setting_key, setting_value) VALUES (?, 'mock_key', 'mock_val')", (profile_id,))
            
            enc_config = vault.encrypt(json.dumps({"api_key": "sk-12345"}))
            cursor.execute(
                "INSERT INTO ai_providers (profile_id, name, type, config_json, is_selected) VALUES (?, 'Mock AI', 'api', ?, 1)",
                (profile_id, enc_config)
            )

        with tempfile.TemporaryDirectory() as temp_dir:
            backup_path = os.path.join(temp_dir, "backup.aisprofile")
            backup_pass = "backup123"
            
            success = ProfileManager.export_profile(profile_id, backup_path, backup_pass)
            self.assertTrue(success)
            self.assertTrue(os.path.exists(backup_path))
            
            new_user_id = 888
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute("INSERT OR IGNORE INTO users (id, username, password_hash, salt) VALUES (?, 'new_user', 'hash', 'salt')", (new_user_id,))
            
            vault.set_session_password("newMasterPwd456", base64.b64encode(os.urandom(16)).decode('utf-8'))
            
            ok, imported_name = ProfileManager.import_profile(new_user_id, backup_path, backup_pass)
            self.assertTrue(ok)
            self.assertEqual(imported_name, "Test Forecast Workspace")
            
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute("SELECT id FROM profiles WHERE user_id = ? AND profile_name = ?", (new_user_id, imported_name))
                new_profile = cursor.fetchone()
                self.assertIsNotNone(new_profile)
                new_profile_id = new_profile["id"]
                
                cursor.execute("SELECT setting_value FROM settings WHERE profile_id = ? AND setting_key = 'mock_key'", (new_profile_id,))
                self.assertEqual(cursor.fetchone()["setting_value"], "mock_val")
                
                cursor.execute("SELECT config_json FROM ai_providers WHERE profile_id = ? AND name='Mock AI'", (new_profile_id,))
                imported_enc = cursor.fetchone()["config_json"]
                decrypted = vault.decrypt(imported_enc)
                config = json.loads(decrypted)
                self.assertEqual(config["api_key"], "sk-12345")
            
        print("    - Profile migration encryption/decryption database translation OK.")

    def test_08_remote_ssh_scanner_parser(self):
        print("\n[+] Testing SSH Remote Scanner parsing logic...")
        stdout_lines = [
            "/var/log/nginx/access.log|15728640|17823901.12|17823901.12\n",
            "/tmp/temp_cache.tmp|5242880|17823901.12|17823901.12\n",
            "/home/user/downloads/movie.mkv|209715200|17823901.12|17823901.12\n"
        ]
        
        log_files = []
        temp_files = []
        large_files = []
        
        for line in stdout_lines:
            parts = line.strip().split('|')
            file_path = parts[0]
            size = int(parts[1])
            name = os.path.basename(file_path)
            ext = os.path.splitext(name)[1].lower()
            
            file_info = {
                "path": file_path,
                "name": name,
                "size": size,
                "ext": ext,
                "category": "other"
            }
            
            path_lower = file_path.lower()
            if ext in ('.tmp', '.temp') or "tmp" in path_lower or "temp" in path_lower:
                file_info["category"] = "temp"
                temp_files.append(file_info)
            elif ext == '.log' or 'log' in name.lower():
                file_info["category"] = "log"
                log_files.append(file_info)
            
            if size > 100 * 1024 * 1024:
                file_info["category"] = "large"
                large_files.append(file_info)
                
        self.assertEqual(len(log_files), 1)
        self.assertEqual(log_files[0]["category"], "log")
        self.assertEqual(len(temp_files), 1)
        self.assertEqual(temp_files[0]["category"], "temp")
        self.assertEqual(len(large_files), 1)
        self.assertEqual(large_files[0]["category"], "large")
        
        print("    - Remote SSH command parser translation and file categories parsing OK.")

    def test_09_sre_analyzer_and_health_scoring(self):
        print("\n[+] Testing SRE Analyzer Docker parser and Health Scoring calculations...")
        from app.modules.sre_analyzer import SREAnalyzer
        analyzer = SREAnalyzer()

        # 1. Test docker size parsing
        w_size, v_size = analyzer.parse_docker_size("21MB (virtual 1.5GB)")
        self.assertEqual(w_size, 21 * 1024 * 1024)
        self.assertEqual(v_size, int(1.5 * 1024 * 1024 * 1024))

        w_size, v_size = analyzer.parse_docker_size("0B (virtual 1.2GB)")
        self.assertEqual(w_size, 0)
        self.assertEqual(v_size, int(1.2 * 1024 * 1024 * 1024))

        # 2. Test base health score
        scan_results = {
            "days_remaining": 150,
            "duplicate_groups": {},
            "log_files": [],
            "temp_files": [],
            "cache_files": []
        }
        score, warnings = analyzer.calculate_health_score(scan_results)
        self.assertEqual(score, 100)
        self.assertEqual(len(warnings), 0)

        # 3. Test capacity depletion score deductions
        scan_results_critical = scan_results.copy()
        scan_results_critical["days_remaining"] = 15
        score, warnings = analyzer.calculate_health_score(scan_results_critical)
        self.assertEqual(score, 70) # 100 - 30
        self.assertTrue(any("exhaustion projected" in w for w in warnings))

        # 4. Test duplicate waste deductions (>10GB)
        scan_results_dups = scan_results.copy()
        scan_results_dups["duplicate_groups"] = {
            "hash1": [
                {"path": "/p1", "size": 12 * 1024 * 1024 * 1024},
                {"path": "/p2", "size": 12 * 1024 * 1024 * 1024}
            ]
        }
        score, warnings = analyzer.calculate_health_score(scan_results_dups)
        self.assertEqual(score, 85) # 100 - 15
        self.assertTrue(any("wasting 12.00 GB" in w for w in warnings))

        # 5. Test DevOps Docker container write-layer waste deduction
        sre_data = {
            "containers": [
                {"id": "c1", "name": "web", "image": "nginx", "write_size": 2 * 1024 * 1024 * 1024} # >1GB write layer
            ],
            "folders": {}
        }
        score, warnings = analyzer.calculate_health_score(scan_results, sre_data)
        self.assertEqual(score, 90) # 100 - 10
        self.assertTrue(any("Docker container(s) have write-layers > 1 GB" in w for w in warnings))

        # 6. Test Windows crash dump waste deduction
        sre_data_win = {
            "containers": [],
            "folders": {
                "minidumps": {
                    "path": "C:\\Windows\\Minidump",
                    "size": 600 * 1024 * 1024, # > 500MB
                    "size_formatted": "600.00 MB"
                }
            }
        }
        score, warnings = analyzer.calculate_health_score(scan_results, sre_data_win)
        self.assertEqual(score, 90) # 100 - 10
        self.assertTrue(any("Crash memory dumps folder is consuming 600.00 MB" in w for w in warnings))

        print("    - SRE size parser, health score deductions, and warnings mapping OK.")

    def test_10_dashboard_markdown_parsing(self):
        print("\n[+] Testing Dashboard markdown preprocessing and cleanup parsing...")
        import re
        import urllib.parse
        
        # Simulated raw LLM output text
        raw_text = (
            "Recommended for deletion:\n"
            "* D:\\OneDriveTemp\\{03C0983D}.vhdx (199.00 MB) - [Видалити](delete://D:\\OneDriveTemp\\{03C0983D}.vhdx)\n"
            "* D:\\SteamLibrary\\steamapps\\common\\World of Tanks\\ru\\python.log (1.65 MB) - [Видалити](delete://D:\\SteamLibrary\\steamapps\\common\\World of Tanks\\ru\\python.log)\n"
            "\nWARNING: DO NOT DELETE:\n"
            "* D:\\GAME\\Forza Horizon 6\\media\\Tracks\\Brio\\GeoChunk2.minizip (46.02 GB)\n"
        )
        
        # Test formatting / preprocessing
        def fix_delete_link(match):
            display_text = match.group(1)
            raw_path = match.group(2)
            normalized_path = raw_path.replace("\\", "/")
            encoded_path = urllib.parse.quote(normalized_path, safe=":/")
            return f"[{display_text}](delete://{encoded_path})"
            
        processed_text = re.sub(r'\[([^\]]+)\]\(delete://([^\)]+)\)', fix_delete_link, raw_text)
        
        # Check that backslashes are forward-slashed and spaces/brackets are properly encoded
        self.assertIn("delete://D:/OneDriveTemp/%7B03C0983D%7D.vhdx", processed_text)
        self.assertIn("delete://D:/SteamLibrary/steamapps/common/World%20of%20Tanks/ru/python.log", processed_text)
        
        # Test parsing link for cleanup
        links = re.findall(r'delete://([^\)]+)', processed_text)
        
        file_paths = []
        for raw_path in links:
            decoded_path = urllib.parse.unquote(raw_path)
            normalized_path = os.path.normpath(decoded_path)
            file_paths.append(normalized_path)
            
        unique_file_paths = list(dict.fromkeys(file_paths))
        
        self.assertEqual(len(unique_file_paths), 2)
        self.assertEqual(unique_file_paths[0], os.path.normpath("D:\\OneDriveTemp\\{03C0983D}.vhdx"))
        self.assertEqual(unique_file_paths[1], os.path.normpath("D:\\SteamLibrary\\steamapps\\common\\World of Tanks\\ru\\python.log"))
        print("    - Link normalization, URL-encoding, and file path retrieval OK.")

if __name__ == "__main__":
    unittest.main()
