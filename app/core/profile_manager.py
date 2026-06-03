import os
import json
import base64
import sqlite3
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from argon2.low_level import hash_secret_raw, Type

from app.database.db_manager import db
from app.security.vault import vault
from app.core.config import logger

class ProfileManager:
    @staticmethod
    def derive_archive_key(password: str, salt: bytes) -> bytes:
        """Derives a 256-bit key from a password and salt using Argon2 for archive encryption."""
        return hash_secret_raw(
            secret=password.encode('utf-8'),
            salt=salt,
            time_cost=2,
            memory_cost=32768,
            parallelism=2,
            hash_len=32,
            type=Type.ID
        )

    @staticmethod
    def export_profile(profile_id: int, file_path: str, password: str) -> bool:
        """Decrypts profile settings/credentials, packages them in JSON,
        encrypts the payload using AES-256-GCM with the backup password, and saves to file."""
        try:
            with db.connection() as conn:
                cursor = conn.cursor()

                # 1. Fetch Profile Name
                cursor.execute("SELECT profile_name FROM profiles WHERE id = ?", (profile_id,))
                profile_row = cursor.fetchone()
                if not profile_row:
                    logger.error(f"Profile ID {profile_id} not found for export.")
                    return False
                profile_name = profile_row["profile_name"]

                # 2. Fetch Settings
                cursor.execute("SELECT setting_key, setting_value FROM settings WHERE profile_id = ?", (profile_id,))
                settings_rows = cursor.fetchall()
                settings = [{"key": r["setting_key"], "value": r["setting_value"]} for r in settings_rows]

                # 3. Fetch AI Providers (Decrypt using session vault)
                cursor.execute("SELECT name, type, config_json, is_selected FROM ai_providers WHERE profile_id = ?", (profile_id,))
                providers_rows = cursor.fetchall()
                providers = []
                for r in providers_rows:
                    decrypted_config = vault.decrypt(r["config_json"])
                    providers.append({
                        "name": r["name"],
                        "type": r["type"],
                        "config_json": decrypted_config,
                        "is_selected": r["is_selected"]
                    })

                # 4. Fetch SSH Hosts (Decrypt credentials using session vault)
                cursor.execute("SELECT name, host, port, username, auth_type, credentials FROM ssh_hosts WHERE profile_id = ?", (profile_id,))
                hosts_rows = cursor.fetchall()
                hosts = []
                for r in hosts_rows:
                    cred_decrypted = vault.decrypt(r["credentials"]) if r["credentials"] else None
                    hosts.append({
                        "name": r["name"],
                        "host": r["host"],
                        "port": r["port"],
                        "username": r["username"],
                        "auth_type": r["auth_type"],
                        "credentials": cred_decrypted
                    })

                # 5. Fetch Scan History
                cursor.execute("SELECT id, scan_path, scan_time, total_size, file_count, metadata FROM scan_history WHERE profile_id = ?", (profile_id,))
                scan_rows = cursor.fetchall()
                history = []
                for s_row in scan_rows:
                    scan_id = s_row["id"]
                    # Fetch analysis results associated with this scan
                    cursor.execute("SELECT path, category, size, risk_score, recommendation, is_ignored FROM analysis_results WHERE scan_id = ?", (scan_id,))
                    res_rows = cursor.fetchall()
                    analysis = [{
                        "path": r["path"],
                        "category": r["category"],
                        "size": r["size"],
                        "risk_score": r["risk_score"],
                        "recommendation": r["recommendation"],
                        "is_ignored": r["is_ignored"]
                    } for r in res_rows]

                    history.append({
                        "scan_path": s_row["scan_path"],
                        "scan_time": s_row["scan_time"],
                        "total_size": s_row["total_size"],
                        "file_count": s_row["file_count"],
                        "metadata": s_row["metadata"],
                        "analysis_results": analysis
                    })

                # Combine to JSON
                payload_data = {
                    "profile_name": profile_name,
                    "settings": settings,
                    "ai_providers": providers,
                    "ssh_hosts": hosts,
                    "scan_history": history
                }
                
                payload_bytes = json.dumps(payload_data).encode('utf-8')

                # Encrypt Archive
                salt = os.urandom(16)
                key = ProfileManager.derive_archive_key(password, salt)
                
                nonce = os.urandom(12)
                aesgcm = AESGCM(key)
                ciphertext = aesgcm.encrypt(nonce, payload_bytes, None)
                
                # Combine salt + nonce + ciphertext
                combined = salt + nonce + ciphertext
                
                with open(file_path, "wb") as f:
                    f.write(combined)
                    
                logger.info(f"Profile {profile_name} exported successfully to {file_path}")
                return True

        except Exception as e:
            logger.error(f"Profile export failed: {e}")
            return False

    @staticmethod
    def import_profile(user_id: int, file_path: str, password: str) -> tuple[bool, str]:
        """Decrypts the archive, inserts the profile, and encrypts credentials
        under the new user's active session vault key."""
        if not vault.is_unlocked():
            return False, "Vault is locked. User must log in first to import profiles."

        try:
            with open(file_path, "rb") as f:
                combined = f.read()

            if len(combined) < 28:
                return False, "Invalid profile file (too small)."

            salt = combined[:16]
            nonce = combined[16:28]
            ciphertext = combined[28:]

            # Derive Key & Decrypt
            key = ProfileManager.derive_archive_key(password, salt)
            aesgcm = AESGCM(key)
            
            payload_bytes = aesgcm.decrypt(nonce, ciphertext, None)
            payload = json.loads(payload_bytes.decode('utf-8'))

            profile_name = payload.get("profile_name", "Imported Workspace")
            
            with db.connection() as conn:
                cursor = conn.cursor()

                # Insert Profile
                try:
                    cursor.execute(
                        "INSERT INTO profiles (user_id, profile_name, is_active) VALUES (?, ?, 0)",
                        (user_id, profile_name)
                    )
                except sqlite3.IntegrityError:
                    # Add timestamp suffix if duplicate name
                    timestamp = int(os.path.getmtime(file_path))
                    profile_name = f"{profile_name} ({timestamp})"
                    cursor.execute(
                        "INSERT INTO profiles (user_id, profile_name, is_active) VALUES (?, ?, 0)",
                        (user_id, profile_name)
                    )
                    
                profile_id = cursor.lastrowid

                # Insert Settings
                for s in payload.get("settings", []):
                    cursor.execute(
                        "INSERT INTO settings (profile_id, setting_key, setting_value) VALUES (?, ?, ?)",
                        (profile_id, s["key"], s["value"])
                    )

                # Insert AI Providers (Encrypt using current session vault)
                for p in payload.get("ai_providers", []):
                    encrypted_config = vault.encrypt(p["config_json"])
                    cursor.execute(
                        "INSERT INTO ai_providers (profile_id, name, type, config_json, is_selected) VALUES (?, ?, ?, ?, ?)",
                        (profile_id, p["name"], p["type"], encrypted_config, p["is_selected"])
                    )

                # Insert SSH Hosts (Encrypt using current session vault)
                for h in payload.get("ssh_hosts", []):
                    encrypted_cred = vault.encrypt(h["credentials"]) if h["credentials"] else None
                    cursor.execute(
                        "INSERT INTO ssh_hosts (profile_id, name, host, port, username, auth_type, credentials) VALUES (?, ?, ?, ?, ?, ?, ?)",
                        (profile_id, h["name"], h["host"], h["port"], h["username"], h["auth_type"], encrypted_cred)
                    )

                # Insert Scan History and Analysis Results
                for s in payload.get("scan_history", []):
                    cursor.execute(
                        "INSERT INTO scan_history (profile_id, scan_path, scan_time, total_size, file_count, metadata) VALUES (?, ?, ?, ?, ?, ?)",
                        (profile_id, s["scan_path"], s["scan_time"], s["total_size"], s["file_count"], s["metadata"])
                    )
                    scan_id = cursor.lastrowid
                    
                    for ar in s.get("analysis_results", []):
                        cursor.execute(
                            "INSERT INTO analysis_results (scan_id, path, category, size, risk_score, recommendation, is_ignored) VALUES (?, ?, ?, ?, ?, ?, ?)",
                            (scan_id, ar["path"], ar["category"], ar["size"], ar["risk_score"], ar["recommendation"], ar["is_ignored"])
                        )

            logger.info(f"Profile {profile_name} imported successfully.")
            return True, profile_name

        except Exception as e:
            logger.error(f"Profile import failed: {e}")
            return False, f"Decryption failed or file is corrupted: {str(e)}"
