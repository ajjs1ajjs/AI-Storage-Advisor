import os
import json
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QLineEdit, 
    QPushButton, QFileDialog, QTextBrowser, QProgressBar, 
    QFrame, QMessageBox, QComboBox
)
from PySide6.QtCore import Qt, Slot, QThread, Signal
from PySide6.QtGui import QFont


from app.modules.scanner import DiskScanner
from app.modules.remote_scanner import RemoteSSHScanner
from app.modules.rules_engine import RulesEngine
from app.ui.components.cleanup_dialog import CleanupDialog
from app.database.db_manager import db
from app.security.vault import vault
from app.providers.base import AIProvider
from app.providers.local_providers import OllamaProvider, LMStudioProvider
from app.providers.api_providers import OpenAIAPIProvider, AnthropicAPIProvider, GeminiAPIProvider, DeepSeekAPIProvider, CustomAPIProvider

from app.core.config import logger

class DashboardScreen(QWidget):
    def __init__(self, parent=None):
        super().__init__(parent)
        self.profile_id = None
        self.scanner = None
        self.scan_results = None
        self.init_ui()

    def set_profile(self, profile_id: int):
        self.profile_id = profile_id

    def init_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(20, 20, 20, 20)
        layout.setSpacing(15)

        # 1. Scan Control Top Bar (Extended with Remote Connection Type selector)
        top_bar = QFrame()
        top_bar.setProperty("class", "Card")
        top_bar_layout = QVBoxLayout(top_bar)
        top_bar_layout.setContentsMargins(15, 15, 15, 15)
        top_bar_layout.setSpacing(12)

        # First row: Connection Type + Hosts dropdown
        row_conn = QHBoxLayout()
        row_conn.setSpacing(10)
        
        conn_lbl = QLabel("Connection Type:")
        conn_lbl.setFont(QFont("Segoe UI", 11, QFont.Bold))
        row_conn.addWidget(conn_lbl)

        self.combo_conn = QComboBox()
        self.combo_conn.addItems(["Local Scan", "SSH Remote Linux", "Network Share (UNC)"])
        self.combo_conn.currentTextChanged.connect(self.on_connection_type_changed)
        row_conn.addWidget(self.combo_conn)

        self.host_lbl = QLabel("SSH Server:")
        self.host_lbl.setFont(QFont("Segoe UI", 11, QFont.Bold))
        self.host_lbl.setVisible(False)
        row_conn.addWidget(self.host_lbl)

        self.combo_hosts = QComboBox()
        self.combo_hosts.setVisible(False)
        self.combo_hosts.currentIndexChanged.connect(self.on_ssh_host_changed)
        row_conn.addWidget(self.combo_hosts)
        
        row_conn.addStretch()
        top_bar_layout.addLayout(row_conn)

        # Direct SSH Connection Credentials (Visible only when SSH Remote Linux is active)
        self.row_ssh_creds = QWidget()
        ssh_creds_layout = QHBoxLayout(self.row_ssh_creds)
        ssh_creds_layout.setContentsMargins(0, 0, 0, 0)
        ssh_creds_layout.setSpacing(10)
        
        lbl_ssh_host = QLabel("IP/Хост:")
        lbl_ssh_host.setFont(QFont("Segoe UI", 11, QFont.Bold))
        ssh_creds_layout.addWidget(lbl_ssh_host)
        
        self.ssh_host_input = QLineEdit()
        self.ssh_host_input.setPlaceholderText("напр. 192.168.1.100")
        ssh_creds_layout.addWidget(self.ssh_host_input)
        
        lbl_ssh_user = QLabel("Користувач:")
        lbl_ssh_user.setFont(QFont("Segoe UI", 11, QFont.Bold))
        ssh_creds_layout.addWidget(lbl_ssh_user)
        
        self.ssh_user_input = QLineEdit()
        self.ssh_user_input.setPlaceholderText("напр. root")
        ssh_creds_layout.addWidget(self.ssh_user_input)
        
        lbl_ssh_pass = QLabel("Пароль:")
        lbl_ssh_pass.setFont(QFont("Segoe UI", 11, QFont.Bold))
        ssh_creds_layout.addWidget(lbl_ssh_pass)
        
        self.ssh_pass_input = QLineEdit()
        self.ssh_pass_input.setEchoMode(QLineEdit.Password)
        self.ssh_pass_input.setPlaceholderText("Пароль")
        ssh_creds_layout.addWidget(self.ssh_pass_input)
        
        top_bar_layout.addWidget(self.row_ssh_creds)
        self.row_ssh_creds.setVisible(False)

        # Second row: Path Inputs + Buttons
        row_inputs = QHBoxLayout()
        row_inputs.setSpacing(10)

        self.path_lbl = QLabel("Scan Path:")
        self.path_lbl.setFont(QFont("Segoe UI", 11, QFont.Bold))
        row_inputs.addWidget(self.path_lbl)

        self.path_input = QLineEdit(os.path.abspath("."))
        row_inputs.addWidget(self.path_input)

        self.btn_browse = QPushButton("📁 Browse")
        self.btn_browse.setProperty("class", "SecondaryBtn")
        self.btn_browse.clicked.connect(self.browse_path)
        row_inputs.addWidget(self.btn_browse)

        self.btn_scan = QPushButton("🔍 Start Scan")
        self.btn_scan.setProperty("class", "PrimaryBtn")
        self.btn_scan.clicked.connect(self.toggle_scan)
        row_inputs.addWidget(self.btn_scan)

        top_bar_layout.addLayout(row_inputs)
        layout.addWidget(top_bar)

        # 2. Scanning Progress Bar
        self.progress_card = QFrame()
        self.progress_card.setProperty("class", "Card")
        self.progress_card.setVisible(False)
        prog_layout = QVBoxLayout(self.progress_card)
        prog_layout.setContentsMargins(15, 15, 15, 15)
        prog_layout.setSpacing(8)

        self.progress_lbl = QLabel("Scanning folder...")
        prog_layout.addWidget(self.progress_lbl)

        self.progress_bar = QProgressBar()
        self.progress_bar.setRange(0, 0)
        prog_layout.addWidget(self.progress_bar)
        layout.addWidget(self.progress_card)

        # 3. Main View: AI Assistant Recommendations
        ai_panel = QFrame()
        ai_panel.setProperty("class", "Card")
        ai_layout = QVBoxLayout(ai_panel)
        ai_layout.setContentsMargins(15, 15, 15, 15)
        ai_layout.setSpacing(10)

        ai_header = QHBoxLayout()
        ai_title = QLabel("🤖 Рекомендації ШІ")
        ai_title.setFont(QFont("Segoe UI", 12, QFont.Bold))
        ai_title.setStyleSheet("color: #7c4dff;")
        ai_header.addWidget(ai_title)

        self.btn_ai_plan = QPushButton("Згенерувати рекомендації ШІ")
        self.btn_ai_plan.setProperty("class", "PrimaryBtn")
        self.btn_ai_plan.clicked.connect(self.generate_ai_recommendation)
        self.btn_ai_plan.setEnabled(False)
        ai_header.addWidget(self.btn_ai_plan)

        self.btn_ai_cleanup = QPushButton("🧹 Видалити рекомендоване ШІ")
        self.btn_ai_cleanup.setProperty("class", "SecondaryBtn")
        self.btn_ai_cleanup.setStyleSheet("background-color: #ef4444; color: white;")
        self.btn_ai_cleanup.clicked.connect(self.cleanup_ai_recommended)
        self.btn_ai_cleanup.setVisible(False)
        ai_header.addWidget(self.btn_ai_cleanup)

        ai_header.addStretch()
        ai_layout.addLayout(ai_header)

        self.ai_browser = QTextBrowser()
        self.ai_browser.setOpenLinks(False)
        self.ai_browser.anchorClicked.connect(self.on_ai_link_clicked)
        self.ai_browser.setHtml("<p style='color: #64748b; font-style: italic;'>Запустіть сканування та натисніть 'Згенерувати рекомендації ШІ'...</p>")
        ai_layout.addWidget(self.ai_browser)
        
        layout.addWidget(ai_panel, stretch=1)

        # 4. Status Bar
        status_bar = QFrame()
        status_bar.setObjectName("StatusBar")
        status_bar.setFixedHeight(30)
        status_layout = QHBoxLayout(status_bar)
        status_layout.setContentsMargins(10, 0, 10, 0)
        
        self.status_lbl = QLabel("Ready")
        self.status_lbl.setObjectName("StatusLabel")
        status_layout.addWidget(self.status_lbl)
        
        layout.addWidget(status_bar)



    def browse_path(self):
        dir_path = QFileDialog.getExistingDirectory(self, "Select Directory to Scan", self.path_input.text())
        if dir_path:
            self.path_input.setText(os.path.abspath(dir_path))

    def on_connection_type_changed(self, text):
        if text == "Local Scan":
            self.btn_browse.setVisible(True)
            self.host_lbl.setVisible(False)
            self.combo_hosts.setVisible(False)
            self.row_ssh_creds.setVisible(False)
            self.path_input.setText(os.path.abspath("."))
        elif text == "SSH Remote Linux":
            self.btn_browse.setVisible(False)
            self.host_lbl.setVisible(True)
            self.combo_hosts.setVisible(True)
            self.row_ssh_creds.setVisible(True)
            self.path_input.setText("Автоматичний пошук")
            self.load_ssh_hosts_dropdown()
        elif text == "Network Share (UNC)":
            self.btn_browse.setVisible(True)
            self.host_lbl.setVisible(False)
            self.combo_hosts.setVisible(False)
            self.row_ssh_creds.setVisible(False)
            self.path_input.setText(r"\\server\share")

    def load_ssh_hosts_dropdown(self):
        self.combo_hosts.blockSignals(True)
        self.combo_hosts.clear()
        self.combo_hosts.addItem("-- Пряме введення --", None)
        try:
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute("SELECT id, name FROM ssh_hosts WHERE profile_id = ?", (self.profile_id,))
                rows = cursor.fetchall()
                for r in rows:
                    self.combo_hosts.addItem(r["name"], r["id"])
        except Exception as e:
            logger.error(f"Failed to load SSH hosts for dropdown: {e}")
        self.combo_hosts.blockSignals(False)
        self.on_ssh_host_changed()

    def on_ssh_host_changed(self):
        host_id = self.combo_hosts.currentData()
        if not host_id:
            self.ssh_host_input.clear()
            self.ssh_user_input.clear()
            self.ssh_pass_input.clear()
            return
            
        try:
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute("SELECT * FROM ssh_hosts WHERE id = ?", (host_id,))
                row = cursor.fetchone()
                if row:
                    self.ssh_host_input.setText(row["host"])
                    self.ssh_user_input.setText(row["username"])
                    if row["auth_type"] == "password" and row["credentials"]:
                        decrypted_pwd = vault.decrypt(row["credentials"])
                        self.ssh_pass_input.setText(decrypted_pwd)
                    else:
                        self.ssh_pass_input.clear()
        except Exception as e:
            logger.error(f"Failed to load SSH host details for direct login: {e}")

    def toggle_scan(self):
        if self.scanner and self.scanner.isRunning():
            self.scanner.stop()
            self.btn_scan.setText("🔍 Start Scan")
            self.progress_card.setVisible(False)
            self.status_lbl.setText("Scan cancelled.")
        else:
            scan_path = self.path_input.text().strip()
            conn_type = self.combo_conn.currentText()
            
            if conn_type == "Local Scan" and not os.path.exists(scan_path):
                QMessageBox.warning(self, "Invalid Path", "The specified folder path does not exist.")
                return

            self.btn_scan.setText("🛑 Stop Scan")
            self.progress_card.setVisible(True)
            self.btn_ai_plan.setEnabled(False)

            # Load rules from DB to execute in background thread
            rules = None
            try:
                with db.connection() as conn:
                    cursor = conn.cursor()
                    cursor.execute(
                        "SELECT setting_value FROM settings WHERE profile_id = ? AND setting_key = 'scan_rules'",
                        (self.profile_id,)
                    )
                    row = cursor.fetchone()
                    if row:
                        rules = json.loads(row["setting_value"])
            except Exception as e:
                logger.error(f"Failed to fetch rules for scan thread: {e}")

            # Start Scanner depending on connection type
            if conn_type == "SSH Remote Linux":
                ssh_host = self.ssh_host_input.text().strip()
                ssh_user = self.ssh_user_input.text().strip()
                ssh_pass = self.ssh_pass_input.text().strip()
                
                if not ssh_host or not ssh_user:
                    QMessageBox.warning(self, "Помилка введення", "Будь ласка, вкажіть IP/Хост та Користувача для SSH підключення.")
                    self.btn_scan.setText("🔍 Start Scan")
                    self.progress_card.setVisible(False)
                    return
                
                host_id = self.combo_hosts.currentData()
                host_config = None
                
                if host_id:
                    try:
                        with db.connection() as conn:
                            cursor = conn.cursor()
                            cursor.execute("SELECT * FROM ssh_hosts WHERE id = ?", (host_id,))
                            h_row = cursor.fetchone()
                            if h_row:
                                decrypted_cred = vault.decrypt(h_row["credentials"]) if h_row["credentials"] else ""
                                host_config = {
                                    "host": ssh_host,
                                    "port": h_row["port"],
                                    "username": ssh_user,
                                    "auth_type": h_row["auth_type"],
                                    "credentials": decrypted_cred if h_row["auth_type"] == "key" or not ssh_pass else ssh_pass
                                }
                    except Exception as e:
                        logger.error(f"Failed to load SSH saved host credentials: {e}")
                
                if not host_config:
                    host_config = {
                        "host": ssh_host,
                        "port": 22,
                        "username": ssh_user,
                        "auth_type": "password",
                        "credentials": ssh_pass
                    }
                
                try:
                    self.scanner = RemoteSSHScanner(host_config, scan_path, rules=rules)
                except Exception as e:
                    logger.error(f"Failed to initialize remote scanner: {e}")
                    QMessageBox.critical(self, "Error", f"Failed to start remote scan: {e}")
                    self.btn_scan.setText("🔍 Start Scan")
                    self.progress_card.setVisible(False)
                    return
            else: # Local or Network Share
                self.scanner = DiskScanner(scan_path, rules=rules)

            self.scanner.signals.progress.connect(self.on_scan_progress)
            self.scanner.signals.finished.connect(self.on_scan_finished)
            self.scanner.start()

    @Slot(str, int, float)
    def on_scan_progress(self, current_dir: str, files_scanned: int, total_size_bytes: float):
        formatted_size = self.scanner.format_size(total_size_bytes)
        self.progress_lbl.setText(f"Scanning: {current_dir[:60]}... ({files_scanned} files, {formatted_size} scanned)")
        self.status_lbl.setText(f"Scanning files... Total size: {formatted_size}")

    @Slot(dict)
    def on_scan_finished(self, results: dict):
        self.btn_scan.setText("🔍 Start Scan")
        self.progress_card.setVisible(False)

        if results.get("cancelled"):
            self.status_lbl.setText("Scan cancelled.")
            return

        self.btn_ai_plan.setEnabled(True)

        if "error" in results:
            QMessageBox.critical(self, "Scan Error", results["error"])
            self.status_lbl.setText(results["error"])
            return

        self.scan_results = results
        self.status_lbl.setText(
            f"Scan completed: Scanned {results['files_scanned']} files. Total space: {results['total_size_formatted']}."
        )

        # Save scan history to DB
        try:
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute(
                    "INSERT INTO scan_history (profile_id, scan_path, total_size, file_count) VALUES (?, ?, ?, ?)",
                    (self.profile_id, self.path_input.text(), results["total_size"], results["files_scanned"])
                )
                scan_id = cursor.lastrowid
                
                for f in results["large_files"][:5] + results["temp_files"][:5] + results["log_files"][:5]:
                    cursor.execute(
                        "INSERT INTO analysis_results (scan_id, path, category, size) VALUES (?, ?, ?, ?)",
                        (scan_id, f["path"], f["category"], f["size"])
                    )
        except Exception as e:
            logger.error(f"Failed to write scan history to DB: {e}")



    @Slot(str)
    def on_ai_link_clicked(self, url):
        import urllib.parse
        url_str = url.toString()
        if url_str.startswith("delete://"):
            file_path = url_str[len("delete://"):]
            file_path = urllib.parse.unquote(file_path)
            file_path = os.path.normpath(file_path)
            self.delete_single_file(file_path)

    def delete_single_file(self, file_path: str):
        conn_type = self.combo_conn.currentText()
        if conn_type == "Local Scan" or conn_type == "Network Share (UNC)":
            if not os.path.exists(file_path):
                QMessageBox.warning(self, "Файл не знайдено", f"Файл за шляхом {file_path} більше не існує.")
                return
                
        dialog = CleanupDialog(self.profile_id, [file_path], self)
        
        if conn_type == "SSH Remote Linux":
            dialog.chk_recycle.setChecked(False)
            dialog.chk_recycle.setEnabled(False)
            
        dialog.exec()

    @Slot(int, int)
    def on_cleanup_completed(self, files_deleted: int, bytes_freed: int):
        self.toggle_scan()

    def get_selected_ai_provider(self) -> AIProvider:
        try:
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute(
                    "SELECT * FROM ai_providers WHERE profile_id = ? AND is_selected = 1",
                    (self.profile_id,)
                )
                provider = cursor.fetchone()
            if not provider:
                return None

            prov_type = provider["type"]
            prov_name = provider["name"]
            
            encrypted_config = provider["config_json"]
            decrypted_config = vault.decrypt(encrypted_config)
            config = json.loads(decrypted_config)

            if prov_name == "Ollama":
                return OllamaProvider(config)
            elif prov_name == "LM Studio":
                return LMStudioProvider(config)
            elif prov_name == "OpenAI API":
                return OpenAIAPIProvider(config)
            elif prov_name == "Anthropic API":
                return AnthropicAPIProvider(config)
            elif prov_name == "Gemini API":
                return GeminiAPIProvider(config)
            elif prov_name == "DeepSeek API":
                return DeepSeekAPIProvider(config)
            elif prov_name == "Custom API":
                return CustomAPIProvider(config)

            
        except Exception as e:
            logger.error(f"Failed to construct AI Provider: {e}")
        return None

    def generate_ai_recommendation(self):
        if not self.scan_results:
            return

        provider = self.get_selected_ai_provider()
        if not provider:
            QMessageBox.warning(
                self, "ШІ провайдер відсутній",
                "Будь ласка, налаштуйте та виберіть ШІ провайдера в налаштуваннях перед запитом рекомендацій."
            )
            return

        self.btn_ai_plan.setText("🤖 Аналіз...")
        self.btn_ai_plan.setEnabled(False)
        self.btn_ai_cleanup.setVisible(False)
        self.status_lbl.setText("Звернення до моделі ШІ для отримання рекомендацій...")
        
        disk_summary = (
            f"Path: {self.path_input.text()}\n"
            f"Total scanned: {self.scan_results['total_size_formatted']}\n"
            f"File count: {self.scan_results['files_scanned']}"
        )
        
        files_to_send = (
            self.scan_results["large_files"][:5] + 
            self.scan_results["temp_files"][:5] + 
            self.scan_results["log_files"][:5]
        )
        self.last_ai_files = files_to_send

        class AIWorker(QThread):
            success = Signal(str)
            error = Signal(str)
            
            def __init__(self, prov, summary, flist):
                super().__init__()
                self.prov = prov
                self.summary = summary
                self.flist = flist
                
            def run(self):
                try:
                    result = self.prov.generate_recommendations(self.summary, self.flist)
                    self.success.emit(result)
                except Exception as e:
                    self.error.emit(str(e))

        self.ai_worker = AIWorker(provider, disk_summary, files_to_send)
        self.ai_worker.success.connect(self.on_ai_success)
        self.ai_worker.error.connect(self.on_ai_error)
        self.ai_worker.start()

    def on_ai_success(self, text: str):
        self.btn_ai_plan.setText("Згенерувати рекомендації ШІ")
        self.btn_ai_plan.setEnabled(True)
        self.status_lbl.setText("Рекомендації ШІ згенеровано.")
        
        # Preprocess the markdown to ensure all delete:// links are properly formatted for QTextBrowser
        import re
        import urllib.parse
        
        def fix_delete_link(match):
            display_text = match.group(1)
            raw_path = match.group(2)
            # Replace backslashes with forward slashes
            normalized_path = raw_path.replace("\\", "/")
            # URL-encode the path, keeping ':' and '/' safe
            encoded_path = urllib.parse.quote(normalized_path, safe=":/")
            return f"[{display_text}](delete://{encoded_path})"
            
        processed_text = re.sub(r'\[([^\]]+)\]\(delete://([^\)]+)\)', fix_delete_link, text)
        self.last_ai_recommendation = processed_text
        
        self.ai_browser.setMarkdown(processed_text)
        self.btn_ai_cleanup.setVisible(True)

    def on_ai_error(self, err_msg: str):
        self.btn_ai_plan.setText("Згенерувати рекомендації ШІ")
        self.btn_ai_plan.setEnabled(True)
        self.status_lbl.setText(f"Помилка ШІ: {err_msg}")
        self.ai_browser.setHtml(f"<p style='color: #ef4444;'>Не вдалося отримати рекомендації: {err_msg}</p>")

    def cleanup_ai_recommended(self):
        if not hasattr(self, "last_ai_recommendation") or not self.last_ai_recommendation:
            return
            
        import re
        import urllib.parse
        
        # Find all delete:// links inside the recommendation markdown
        links = re.findall(r'delete://([^\)]+)', self.last_ai_recommendation)
        
        file_paths = []
        for raw_path in links:
            decoded_path = urllib.parse.unquote(raw_path)
            normalized_path = os.path.normpath(decoded_path)
            file_paths.append(normalized_path)
            
        # Deduplicate
        unique_file_paths = []
        for p in file_paths:
            if p not in unique_file_paths:
                unique_file_paths.append(p)
                
        if not unique_file_paths:
            QMessageBox.information(
                self, "Немає рекомендованих файлів",
                "ШІ не запропонував жодного файлу для безпечного видалення."
            )
            return
            
        # Open the CleanupDialog with these files
        dialog = CleanupDialog(self.profile_id, unique_file_paths, self)
        
        # Override recycle bin checks for remote hosts
        conn_type = self.combo_conn.currentText()
        if conn_type == "SSH Remote Linux":
            dialog.chk_recycle.setChecked(False)
            dialog.chk_recycle.setEnabled(False)
            
        dialog.cleanup_completed.connect(self.on_cleanup_completed)
        dialog.exec()

    def load_recent_scans(self):
        if self.combo_conn.currentText() == "SSH Remote Linux":
            self.load_ssh_hosts_dropdown()

    def clear_data(self):
        self.path_input.setText(os.path.abspath("."))
        self.ai_browser.setHtml("<p style='color: #64748b; font-style: italic;'>Запустіть сканування та натисніть 'Згенерувати рекомендації ШІ'...</p>")
        self.btn_ai_plan.setEnabled(False)
        self.btn_ai_cleanup.setVisible(False)
        self.scan_results = None
        self.last_ai_recommendation = ""
        self.status_lbl.setText("Ready")
