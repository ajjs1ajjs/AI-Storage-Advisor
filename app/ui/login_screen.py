import os
import sqlite3
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QLineEdit, 
    QPushButton, QTabWidget, QMessageBox, QFrame, QGraphicsDropShadowEffect, QFileDialog
)
from PySide6.QtCore import Qt, Signal
from PySide6.QtGui import QColor, QFont
from app.database.db_manager import db
from app.security.vault import vault
from app.core.profile_manager import ProfileManager
from app.core.config import logger

class LoginScreen(QWidget):
    # Signals
    login_success = Signal(int, str, int) # (user_id, username, profile_id)

    def __init__(self, parent=None):
        super().__init__(parent)
        self.init_ui()

    def init_ui(self):
        master_layout = QVBoxLayout(self)
        master_layout.setAlignment(Qt.AlignCenter)
        
        # Outer Card Container
        card = QFrame()
        card.setFixedSize(400, 520)
        card.setObjectName("LoginCard")
        card.setStyleSheet("""
            QFrame#LoginCard {
                background-color: #13151a;
                border: 1px solid #1f232b;
                border-radius: 16px;
            }
        """)
        
        shadow = QGraphicsDropShadowEffect()
        shadow.setBlurRadius(20)
        shadow.setColor(QColor(0, 0, 0, 160))
        shadow.setOffset(0, 4)
        card.setGraphicsEffect(shadow)

        card_layout = QVBoxLayout(card)
        card_layout.setContentsMargins(30, 30, 30, 30)
        card_layout.setSpacing(15)

        # Title/Logo
        header_layout = QVBoxLayout()
        header_layout.setSpacing(4)
        logo_lbl = QLabel("🛸")
        logo_lbl.setFont(QFont("Segoe UI", 36))
        logo_lbl.setAlignment(Qt.AlignCenter)
        header_layout.addWidget(logo_lbl)
        
        title_lbl = QLabel("AI Storage Advisor")
        title_lbl.setFont(QFont("Segoe UI", 20, QFont.Bold))
        title_lbl.setStyleSheet("color: #7c4dff;")
        title_lbl.setAlignment(Qt.AlignCenter)
        header_layout.addWidget(title_lbl)
        
        subtitle_lbl = QLabel("Intelligent disk cleanup")
        subtitle_lbl.setFont(QFont("Segoe UI", 11))
        subtitle_lbl.setStyleSheet("color: #64748b;")
        subtitle_lbl.setAlignment(Qt.AlignCenter)
        header_layout.addWidget(subtitle_lbl)
        
        card_layout.addLayout(header_layout)

        # Tabs for Login vs Register vs Import
        self.tabs = QTabWidget()
        self.tabs.setStyleSheet("""
            QTabWidget::pane {
                border: none;
                background: transparent;
            }
            QTabBar::tab {
                background: transparent;
                color: #64748b;
                padding: 8px 12px;
                font-weight: 600;
                font-size: 12px;
            }
            QTabBar::tab:hover {
                color: #e2e8f0;
            }
            QTabBar::tab:selected {
                color: #7c4dff;
                border-bottom: 2px solid #7c4dff;
            }
        """)

        # 1. Login Tab
        login_tab = QWidget()
        login_layout = QVBoxLayout(login_tab)
        login_layout.setContentsMargins(0, 15, 0, 0)
        login_layout.setSpacing(12)

        self.login_user = QLineEdit()
        self.login_user.setPlaceholderText("Username")
        login_layout.addWidget(self.login_user)

        self.login_pass = QLineEdit()
        self.login_pass.setPlaceholderText("Password")
        self.login_pass.setEchoMode(QLineEdit.Password)
        login_layout.addWidget(self.login_pass)

        self.btn_login = QPushButton("Unlock Vault")
        self.btn_login.setProperty("class", "PrimaryBtn")
        self.btn_login.clicked.connect(self.handle_login)
        login_layout.addWidget(self.btn_login)
        
        self.tabs.addTab(login_tab, "LOGIN")

        # 2. Register Tab
        register_tab = QWidget()
        reg_layout = QVBoxLayout(register_tab)
        reg_layout.setContentsMargins(0, 15, 0, 0)
        reg_layout.setSpacing(12)

        self.reg_user = QLineEdit()
        self.reg_user.setPlaceholderText("New Username")
        reg_layout.addWidget(self.reg_user)

        self.reg_pass = QLineEdit()
        self.reg_pass.setPlaceholderText("Master Password")
        self.reg_pass.setEchoMode(QLineEdit.Password)
        reg_layout.addWidget(self.reg_pass)

        self.btn_register = QPushButton("Create Secure Vault")
        self.btn_register.setProperty("class", "PrimaryBtn")
        self.btn_register.clicked.connect(self.handle_register)
        reg_layout.addWidget(self.btn_register)

        self.tabs.addTab(register_tab, "SIGN UP")

        # 3. Import Tab
        import_tab = QWidget()
        import_layout = QVBoxLayout(import_tab)
        import_layout.setContentsMargins(0, 15, 0, 0)
        import_layout.setSpacing(10)

        desc = QLabel("Restore vault configuration from .aisprofile backup:")
        desc.setStyleSheet("color: #94a3b8; font-size: 11px;")
        desc.setWordWrap(True)
        import_layout.addWidget(desc)

        self.imp_user = QLineEdit()
        self.imp_user.setPlaceholderText("Local Username to create")
        import_layout.addWidget(self.imp_user)

        self.imp_pass = QLineEdit()
        self.imp_pass.setPlaceholderText("Local Password to assign")
        self.imp_pass.setEchoMode(QLineEdit.Password)
        import_layout.addWidget(self.imp_pass)

        self.imp_backup_pass = QLineEdit()
        self.imp_backup_pass.setPlaceholderText("Backup Encryption Password")
        self.imp_backup_pass.setEchoMode(QLineEdit.Password)
        import_layout.addWidget(self.imp_backup_pass)

        self.btn_import = QPushButton("📤 Import Profile File")
        self.btn_import.setProperty("class", "PrimaryBtn")
        self.btn_import.clicked.connect(self.handle_import_profile)
        import_layout.addWidget(self.btn_import)

        self.tabs.addTab(import_tab, "IMPORT")
        
        card_layout.addWidget(self.tabs)
        master_layout.addWidget(card)

    def handle_login(self):
        username = self.login_user.text().strip()
        password = self.login_pass.text()

        if not username or not password:
            QMessageBox.warning(self, "Warning", "Please fill in all fields.")
            return

        try:
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute("SELECT * FROM users WHERE username = ?", (username,))
                user = cursor.fetchone()
                
                if not user:
                    QMessageBox.critical(self, "Error", "Incorrect username or password.")
                    return

                pwd_hash = user["password_hash"]
                salt = user["salt"]
                user_id = user["id"]

                if vault.verify_password(pwd_hash, password):
                    vault.set_session_password(password, salt)
                    
                    cursor.execute("SELECT id FROM profiles WHERE user_id = ? AND is_active = 1", (user_id,))
                    profile = cursor.fetchone()
                    
                    if not profile:
                        cursor.execute("SELECT id FROM profiles WHERE user_id = ?", (user_id,))
                        profile = cursor.fetchone()
                        
                    if not profile:
                        cursor.execute(
                            "INSERT INTO profiles (user_id, profile_name, is_active) VALUES (?, 'Default Workspace', 1)",
                            (user_id,)
                        )
                        profile_id = cursor.lastrowid
                    else:
                        profile_id = profile["id"]
                        cursor.execute("UPDATE profiles SET is_active = 1 WHERE id = ?", (profile_id,))

                    self.login_success.emit(user_id, username, profile_id)
                else:
                    QMessageBox.critical(self, "Error", "Incorrect username or password.")
        except Exception as e:
            logger.error(f"Login failed: {e}")
            QMessageBox.critical(self, "System Error", f"Login failed: {e}")

    def handle_register(self):
        username = self.reg_user.text().strip()
        password = self.reg_pass.text()

        if not username or len(password) < 6:
            QMessageBox.warning(self, "Security Warning", "Username required & Master Password must be at least 6 characters.")
            return

        try:
            pwd_hash, salt_b64 = vault.hash_password(password)
            
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute(
                    "INSERT INTO users (username, password_hash, salt) VALUES (?, ?, ?)",
                    (username, pwd_hash, salt_b64)
                )
                user_id = cursor.lastrowid
                
                cursor.execute(
                    "INSERT INTO profiles (user_id, profile_name, is_active) VALUES (?, 'Default Workspace', 1)",
                    (user_id,)
                )
                profile_id = cursor.lastrowid
            
            QMessageBox.information(self, "Success", "Vault created successfully! You can now log in.")
            self.tabs.setCurrentIndex(0)
            self.login_user.setText(username)
            self.login_pass.setText(password)
        except sqlite3.IntegrityError:
            QMessageBox.critical(self, "Error", "Username already exists. Please pick another one.")
        except Exception as e:
            logger.error(f"Registration failed: {e}")
            QMessageBox.critical(self, "Error", f"Failed to create vault: {e}")

    def handle_import_profile(self):
        username = self.imp_user.text().strip()
        password = self.imp_pass.text()
        backup_pass = self.imp_backup_pass.text()

        if not username or len(password) < 6 or not backup_pass:
            QMessageBox.warning(
                self, "Warning",
                "Please specify local username, a local password (min 6 chars), and the backup password."
            )
            return

        # Choose profile file
        file_path, _ = QFileDialog.getOpenFileName(
            self, "Select Profile Backup File", "", "AI Storage Profile (*.aisprofile)"
        )
        if not file_path:
            return

        try:
            # 1. Create the user account locally
            pwd_hash, salt_b64 = vault.hash_password(password)
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute(
                    "INSERT INTO users (username, password_hash, salt) VALUES (?, ?, ?)",
                    (username, pwd_hash, salt_b64)
                )
                user_id = cursor.lastrowid

            # 2. Derive vault key (unlocked session state needed for importing)
            vault.set_session_password(password, salt_b64)

            # 3. Call Profile Manager to decrypt & import profile records
            success, result_msg = ProfileManager.import_profile(user_id, file_path, backup_pass)
            
            if success:
                with db.connection() as conn:
                    cursor = conn.cursor()
                    # Set active
                    cursor.execute(
                        "UPDATE profiles SET is_active = 1 WHERE user_id = ? AND profile_name = ?",
                        (user_id, result_msg)
                    )

                    # Get profile ID
                    cursor.execute(
                        "SELECT id FROM profiles WHERE user_id = ? AND is_active = 1", (user_id,)
                    )
                    profile_row = cursor.fetchone()
                    profile_id = profile_row["id"]

                QMessageBox.information(
                    self, "Import Successful",
                    f"Profile '{result_msg}' successfully imported and loaded!"
                )
                # Auto log in the user
                self.login_success.emit(user_id, username, profile_id)
            else:
                # Rollback local user creation
                with db.connection() as conn:
                    cursor = conn.cursor()
                    cursor.execute("DELETE FROM users WHERE id = ?", (user_id,))
                vault.clear_session()
                QMessageBox.critical(self, "Import Failed", result_msg)

        except sqlite3.IntegrityError:
            QMessageBox.critical(self, "Error", "Username already exists. Please choose a different local username.")
        except Exception as e:
            logger.error(f"Onboarding import failed: {e}")
            QMessageBox.critical(self, "Error", f"Failed to import profile: {e}")

    def reset_fields(self):
        self.login_user.clear()
        self.login_pass.clear()
        self.reg_user.clear()
        self.reg_pass.clear()
        self.imp_user.clear()
        self.imp_pass.clear()
        self.imp_backup_pass.clear()
        self.tabs.setCurrentIndex(0)
