import sqlite3
import json
from contextlib import contextmanager
from app.core.config import DB_PATH, logger

class DBManager:
    def __init__(self, db_path=str(DB_PATH)):
        self.db_path = db_path
        self.init_db()

    def get_connection(self):
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        return conn

    @contextmanager
    def connection(self):
        conn = sqlite3.connect(self.db_path, timeout=10.0)
        try:
            conn.execute("PRAGMA journal_mode=WAL;")
            conn.execute("PRAGMA busy_timeout=10000;")
            conn.row_factory = sqlite3.Row
            with conn:
                yield conn
        finally:
            conn.close()

    def init_db(self):
        logger.info("Initializing database...")
        queries = [
            """
            CREATE TABLE IF NOT EXISTS users (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                username TEXT UNIQUE NOT NULL,
                password_hash TEXT NOT NULL,
                salt TEXT NOT NULL,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
            """,
            """
            CREATE TABLE IF NOT EXISTS profiles (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                user_id INTEGER NOT NULL,
                profile_name TEXT NOT NULL,
                is_active INTEGER DEFAULT 0,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
                UNIQUE(user_id, profile_name)
            )
            """,
            """
            CREATE TABLE IF NOT EXISTS ai_providers (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                profile_id INTEGER NOT NULL,
                name TEXT NOT NULL,
                type TEXT NOT NULL,          -- 'api', 'local', 'session'
                config_json TEXT NOT NULL,    -- Encrypted JSON string (containing api keys, session cookies, etc.)
                is_selected INTEGER DEFAULT 0,
                FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
            )
            """,
            """
            CREATE TABLE IF NOT EXISTS ssh_hosts (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                profile_id INTEGER NOT NULL,
                name TEXT NOT NULL,
                host TEXT NOT NULL,
                port INTEGER DEFAULT 22,
                username TEXT NOT NULL,
                auth_type TEXT NOT NULL,      -- 'password', 'key'
                credentials TEXT,             -- Encrypted password or key path
                FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
            )
            """,
            """
            CREATE TABLE IF NOT EXISTS scan_history (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                profile_id INTEGER NOT NULL,
                scan_path TEXT NOT NULL,
                scan_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                total_size INTEGER NOT NULL,
                file_count INTEGER NOT NULL,
                metadata TEXT,                -- Extra JSON data
                FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
            )
            """,
            """
            CREATE TABLE IF NOT EXISTS analysis_results (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                scan_id INTEGER NOT NULL,
                path TEXT NOT NULL,
                category TEXT NOT NULL,       -- 'large', 'temp', 'log', 'duplicate', 'cache', etc.
                size INTEGER NOT NULL,
                risk_score INTEGER DEFAULT 0,  -- 0 to 100
                recommendation TEXT,
                is_ignored INTEGER DEFAULT 0,
                FOREIGN KEY (scan_id) REFERENCES scan_history(id) ON DELETE CASCADE
            )
            """,
            """
            CREATE TABLE IF NOT EXISTS cleanup_history (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                profile_id INTEGER NOT NULL,
                cleaned_path TEXT NOT NULL,
                size_freed INTEGER NOT NULL,
                clean_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                status TEXT NOT NULL,         -- 'success', 'failed'
                error_message TEXT,
                FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
            )
            """,
            """
            CREATE TABLE IF NOT EXISTS duplicate_results (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                scan_id INTEGER NOT NULL,
                file_hash TEXT NOT NULL,
                file_path TEXT NOT NULL,
                file_size INTEGER NOT NULL,
                FOREIGN KEY (scan_id) REFERENCES scan_history(id) ON DELETE CASCADE
            )
            """,
            """
            CREATE TABLE IF NOT EXISTS forecast_history (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                profile_id INTEGER NOT NULL,
                forecast_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                predicted_days_to_full INTEGER,
                growth_rate_bytes_day INTEGER,
                FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
            )
            """,
            """
            CREATE TABLE IF NOT EXISTS settings (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                profile_id INTEGER NOT NULL,
                setting_key TEXT NOT NULL,
                setting_value TEXT NOT NULL,   -- Encrypted string if sensitive
                FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE,
                UNIQUE(profile_id, setting_key)
            )
            """
        ]

        with self.connection() as conn:
            cursor = conn.cursor()
            for q in queries:
                cursor.execute(q)
            
            # Auto-create default user and profile (ID 1) for direct startup
            cursor.execute("SELECT id FROM users WHERE id = 1")
            if not cursor.fetchone():
                cursor.execute("INSERT OR REPLACE INTO users (id, username, password_hash, salt) VALUES (1, 'default_user', 'N/A', 'N/A')")
            cursor.execute("SELECT id FROM profiles WHERE id = 1")
            if not cursor.fetchone():
                cursor.execute("INSERT OR REPLACE INTO profiles (id, user_id, profile_name, is_active) VALUES (1, 1, 'default_profile', 1)")
                
            cursor.execute("SELECT setting_value FROM settings WHERE profile_id = 1 AND setting_key = 'theme'")
            if not cursor.fetchone():
                cursor.execute("INSERT INTO settings (profile_id, setting_key, setting_value) VALUES (1, 'theme', 'dark')")
                
        logger.info("Database schema initialized and default profile setup completed.")

# Single instance
db = DBManager()
