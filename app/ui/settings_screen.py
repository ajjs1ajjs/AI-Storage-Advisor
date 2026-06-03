import json
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QLineEdit, 
    QPushButton, QComboBox, QFrame, QMessageBox, QDialog, 
    QStackedWidget, QListWidget, QListWidgetItem, QInputDialog, QCheckBox,
    QTableWidget, QTableWidgetItem, QHeaderView
)
from PySide6.QtCore import Qt, Signal, Slot, QThread
from PySide6.QtGui import QFont

from app.database.db_manager import db
from app.security.vault import vault
from app.providers.local_providers import OllamaProvider, LMStudioProvider
from app.providers.api_providers import OpenAIAPIProvider, AnthropicAPIProvider, GeminiAPIProvider, DeepSeekAPIProvider, CustomAPIProvider
from app.ui.components.ssh_host_dialog import SSHHostDialog
from app.modules.rules_engine import RulesEngine
from app.core.config import logger





class FetchModelsWorker(QThread):
    finished = Signal(list, str) # (list of models, error)
    
    def __init__(self, provider):
        super().__init__()
        self.provider = provider
        
    def run(self):
        try:
            models = self.provider.get_available_models()
            if models:
                self.finished.emit(models, "")
            else:
                self.finished.emit([], "No models found from this provider.")
        except Exception as e:
            self.finished.emit([], str(e))


class SettingsScreen(QWidget):
    def __init__(self, parent=None):
        super().__init__(parent)
        self.profile_id = None
        self.current_setting_name = None
        self.hosts_table = None
        self.model_list_cache = {}
        self.init_ui()

    def set_profile(self, profile_id: int):
        self.profile_id = profile_id

    def init_ui(self):
        main_layout = QHBoxLayout(self)
        main_layout.setContentsMargins(20, 20, 20, 20)
        main_layout.setSpacing(15)

        # Left Column: Configuration List
        left_panel = QFrame()
        left_panel.setProperty("class", "Card")
        left_panel.setFixedWidth(250)
        left_layout = QVBoxLayout(left_panel)
        left_layout.setContentsMargins(10, 15, 10, 15)

        list_title = QLabel("Settings")
        list_title.setFont(QFont("Segoe UI", 12, QFont.Bold))
        list_title.setStyleSheet("margin-left: 5px;")
        left_layout.addWidget(list_title)

        self.setting_list = QListWidget()
        self.setting_list.itemClicked.connect(self.on_setting_selected)
        left_layout.addWidget(self.setting_list)

        self.setting_names = [
            "Ollama", "LM Studio",
            "OpenAI API", "Anthropic API", "Gemini API", "DeepSeek API", "Custom API",
            "📋 Scan Rules", "🖥️ Remote Hosts"
        ]
        for name in self.setting_names:
            item = QListWidgetItem(name)
            self.setting_list.addItem(item)

        main_layout.addWidget(left_panel)

        # Right Column: Config Form
        right_panel = QFrame()
        right_panel.setProperty("class", "Card")
        self.right_layout = QVBoxLayout(right_panel)
        self.right_layout.setContentsMargins(20, 20, 20, 20)
        self.right_layout.setSpacing(15)

        self.form_widget = QWidget()
        self.form_layout = QVBoxLayout(self.form_widget)
        self.form_layout.setContentsMargins(0, 0, 0, 0)
        self.form_layout.setSpacing(12)
        
        self.right_layout.addWidget(self.form_widget)
        self.right_layout.addStretch()

        # Save & Test Action Bar
        self.action_bar = QWidget()
        self.action_layout = QHBoxLayout(self.action_bar)
        self.action_layout.setContentsMargins(0, 0, 0, 0)
        
        self.btn_test = QPushButton("⚡ Test Connection")
        self.btn_test.setProperty("class", "SecondaryBtn")
        self.btn_test.clicked.connect(self.test_connection)
        self.action_layout.addWidget(self.btn_test)

        self.btn_save = QPushButton("💾 Save Configuration")
        self.btn_save.setProperty("class", "PrimaryBtn")
        self.btn_save.clicked.connect(self.save_configuration)
        self.action_layout.addWidget(self.btn_save)

        self.right_layout.addWidget(self.action_bar)

        main_layout.addWidget(right_panel)

    def on_setting_selected(self, item):
        self.current_setting_name = item.text()
        self.render_form(self.current_setting_name)
        self.load_setting_data(self.current_setting_name)

    def clear_form(self):
        for i in reversed(range(self.form_layout.count())):
            widget = self.form_layout.itemAt(i).widget()
            if widget is not None:
                widget.setParent(None)
                widget.deleteLater()

    def render_form(self, name: str):
        self.clear_form()

        # Title
        title = QLabel(f"Configure {name}")
        title.setFont(QFont("Segoe UI", 14, QFont.Bold))
        self.form_layout.addWidget(title)

        if name == "📋 Scan Rules":
            self.action_bar.setVisible(True)
            self.btn_test.setVisible(False)
            self.btn_save.setText("💾 Save Rules")
            
            desc = QLabel("Define filters to flag automatic cleanup candidates during scans:")
            desc.setStyleSheet("color: #94a3b8; margin-bottom: 5px;")
            self.form_layout.addWidget(desc)
            
            self.add_rule_fields("temp_old", "Temp files older than (Days):", "30")
            self.add_rule_fields("log_large", "Log files larger than (MB):", "100")
            self.add_rule_fields("log_old", "Log files older than (Days):", "14")
            self.add_rule_fields("backup_old", "Backups older than (Days):", "90")
            
        elif name == "🖥️ Remote Hosts":
            # Save button is not needed as it uses popups dialogs
            self.action_bar.setVisible(False)
            
            desc = QLabel("Manage remote server credentials (SSH) to perform remote scans:")
            desc.setStyleSheet("color: #94a3b8; margin-bottom: 5px;")
            self.form_layout.addWidget(desc)
            
            # Action Panel
            host_actions = QWidget()
            ha_layout = QHBoxLayout(host_actions)
            ha_layout.setContentsMargins(0, 0, 0, 0)
            
            btn_add = QPushButton("➕ Add SSH Server")
            btn_add.setProperty("class", "PrimaryBtn")
            btn_add.clicked.connect(self.add_ssh_host)
            ha_layout.addWidget(btn_add)
            
            self.btn_edit_host = QPushButton("✏️ Edit Selected")
            self.btn_edit_host.setProperty("class", "SecondaryBtn")
            self.btn_edit_host.clicked.connect(self.edit_ssh_host)
            self.btn_edit_host.setEnabled(False)
            ha_layout.addWidget(self.btn_edit_host)

            self.btn_del_host = QPushButton("❌ Delete")
            self.btn_del_host.setProperty("class", "SecondaryBtn")
            self.btn_del_host.clicked.connect(self.delete_ssh_host)
            self.btn_del_host.setEnabled(False)
            ha_layout.addWidget(self.btn_del_host)
            ha_layout.addStretch()

            self.form_layout.addWidget(host_actions)

            # Table for Hosts
            self.hosts_table = QTableWidget()
            self.hosts_table.setColumnCount(4)
            self.hosts_table.setHorizontalHeaderLabels(["Name", "Host", "Port", "Username"])
            self.hosts_table.horizontalHeader().setSectionResizeMode(QHeaderView.Stretch)
            self.hosts_table.itemSelectionChanged.connect(self.on_host_selection_changed)
            self.form_layout.addWidget(self.hosts_table)
            
        else:
            self.action_bar.setVisible(True)
            self.btn_test.setVisible(True)
            self.btn_save.setText("💾 Save Configuration")
            
            # Dynamic Forms depending on provider type
            if name in ("Ollama", "LM Studio"):
                self.add_field("base_url", "Base URL:", "http://localhost:11434" if name == "Ollama" else "http://localhost:1234/v1")
                
                models = [] if name == "Ollama" else \
                         ["meta-llama-3-8b-instruct", "phi-3-mini-4k-instruct", "local-model"]
                default_model = "" if name == "Ollama" else "meta-llama-3-8b-instruct"
                self.add_dropdown_field("model", "Model Name:", models, default_model, has_fetch_btn=True)

            elif name == "Custom API":
                self.add_field("base_url", "Base URL:", "https://api.openai.com/v1")
                self.add_field("api_key", "API Key:", "", is_password=True)
                self.add_field("model", "Model Name:", "gpt-4")

            elif name in ("OpenAI API", "Anthropic API", "Gemini API", "DeepSeek API"):
                self.add_field("api_key", "API Key:", "", is_password=True)
                
                default_model = "gpt-5.4-mini"
                models = ["gpt-5.5", "gpt-5.5-pro", "gpt-5.4-pro", "gpt-5.4-mini", "gpt-5.4-nano", "gpt-5.4", "gpt-4.5"]
                if name == "Anthropic API":
                    default_model = "claude-sonnet-4.6"
                    models = ["claude-opus-4.8", "claude-sonnet-4.6", "claude-opus-4.7"]
                elif name == "Gemini API":
                    default_model = "gemini-3.5-flash"
                    models = ["gemini-3.5-flash", "gemini-3.1-pro", "gemini-3.1-flash-lite", "gemini-2.5-pro", "gemini-2.5-flash"]
                elif name == "DeepSeek API":
                    default_model = "deepseek-v4-flash"
                    models = ["deepseek-v4-flash", "deepseek-v4-pro"]

                self.add_dropdown_field("model", "Model Name:", models, default_model, has_fetch_btn=True)



    def add_field(self, key_name: str, label_text: str, default_value: str, is_password=False):
        lbl = QLabel(label_text)
        lbl.setStyleSheet("font-weight: 500;")
        self.form_layout.addWidget(lbl)

        inp = QLineEdit(default_value)
        inp.setObjectName(key_name)
        if is_password:
            inp.setEchoMode(QLineEdit.Password)
        self.form_layout.addWidget(inp)

    def add_dropdown_field(self, key_name: str, label_text: str, items: list[str], default_value: str, has_fetch_btn=False):
        lbl = QLabel(label_text)
        lbl.setStyleSheet("font-weight: 500;")
        self.form_layout.addWidget(lbl)

        row = QWidget()
        row_layout = QHBoxLayout(row)
        row_layout.setContentsMargins(0, 0, 0, 0)
        row_layout.setSpacing(10)

        combo = QComboBox()
        combo.setObjectName(key_name)
        combo.setEditable(True)
        combo.addItems(items)
        
        # Select default
        idx = combo.findText(default_value)
        if idx >= 0:
            combo.setCurrentIndex(idx)
        else:
            combo.setCurrentText(default_value)
            
        row_layout.addWidget(combo, stretch=1)

        if has_fetch_btn:
            btn_fetch = QPushButton("🔄 Fetch Models")
            btn_fetch.setProperty("class", "SecondaryBtn")
            btn_fetch.clicked.connect(self.fetch_available_models)
            row_layout.addWidget(btn_fetch)

        self.form_layout.addWidget(row)

    def fetch_available_models(self):
        name = self.current_setting_name
        config = {}
        
        # Gather inputs
        base_url_inp = self.form_widget.findChild(QLineEdit, "base_url")
        if base_url_inp:
            config["base_url"] = base_url_inp.text().strip()
            
        api_key_inp = self.form_widget.findChild(QLineEdit, "api_key")
        if api_key_inp:
            config["api_key"] = api_key_inp.text().strip()
        
        provider = None
        if name == "Ollama":
            provider = OllamaProvider(config)
        elif name == "LM Studio":
            provider = LMStudioProvider(config)
        elif name == "OpenAI API":
            provider = OpenAIAPIProvider(config)
        elif name == "Anthropic API":
            provider = AnthropicAPIProvider(config)
        elif name == "Gemini API":
            provider = GeminiAPIProvider(config)
        elif name == "DeepSeek API":
            provider = DeepSeekAPIProvider(config)
        elif name == "Custom API":
            provider = CustomAPIProvider(config)
            
        if not provider:
            return
            
        # Find fetch button to disable it
        sender_btn = self.sender()
        if sender_btn:
            sender_btn.setEnabled(False)
            sender_btn.setText("Fetching...")
            
        self.fetch_worker = FetchModelsWorker(provider)
        
        def on_fetch_finished(models, err):
            if sender_btn:
                sender_btn.setEnabled(True)
                sender_btn.setText("🔄 Fetch Models")
                
            if err:
                QMessageBox.critical(self, "Fetch Failed", f"Failed to fetch models: {err}")
                return
                
            # Find the model combobox
            combo = self.form_widget.findChild(QComboBox, "model")
            if combo:
                current_text = combo.currentText().strip()
                selected_text = current_text if current_text in models else ""
                self._replace_combo_items(combo, models, selected_text)
                self.model_list_cache.setdefault(name, {})["model"] = [
                    combo.itemText(i) for i in range(combo.count())
                ]

                saved = self.save_configuration(silent=True)
                if saved:
                    QMessageBox.information(self, "Success", f"Successfully loaded and saved {len(models)} models from provider!")
                else:
                    QMessageBox.warning(self, "Loaded but not saved", f"Successfully loaded {len(models)} models, but failed to save them. Please click Save Configuration manually.")

        self.fetch_worker.finished.connect(on_fetch_finished)
        self.fetch_worker.start()

    def _replace_combo_items(self, combo: QComboBox, items: list[str], selected_text: str = ""):
        selected_text = selected_text.strip()
        combo.clear()
        combo.addItems(items)

        if selected_text:
            if selected_text not in items:
                combo.addItem(selected_text)
            idx = combo.findText(selected_text)
            if idx >= 0:
                combo.setCurrentIndex(idx)
            else:
                combo.setCurrentText(selected_text)
        elif items:
            combo.setCurrentIndex(0)

    def add_rule_fields(self, rule_id: str, label_text: str, default_value: str):
        row = QWidget()
        row.setObjectName(f"row_{rule_id}")
        row_layout = QHBoxLayout(row)
        row_layout.setContentsMargins(0, 0, 0, 0)
        row_layout.setSpacing(10)

        chk = QCheckBox()
        chk.setObjectName(f"chk_{rule_id}")
        chk.setChecked(True)
        row_layout.addWidget(chk)

        lbl = QLabel(label_text)
        lbl.setStyleSheet("font-weight: 500;")
        row_layout.addWidget(lbl)

        inp = QLineEdit(default_value)
        inp.setObjectName(f"val_{rule_id}")
        inp.setFixedWidth(80)
        row_layout.addWidget(inp)
        row_layout.addStretch()

        self.form_layout.addWidget(row)



    def load_setting_data(self, name: str):
        try:
            with db.connection() as conn:
                cursor = conn.cursor()
                
                if name == "📋 Scan Rules":
                    cursor.execute(
                        "SELECT setting_value FROM settings WHERE profile_id = ? AND setting_key = 'scan_rules'",
                        (self.profile_id,)
                    )
                    row = cursor.fetchone()
                    if row:
                        rules = json.loads(row["setting_value"])
                        for r in rules:
                            rid = r["id"]
                            chk = self.form_widget.findChild(QCheckBox, f"chk_{rid}")
                            val_input = self.form_widget.findChild(QLineEdit, f"val_{rid}")
                            if chk:
                                chk.setChecked(r.get("enabled", True))
                            if val_input:
                                val_input.setText(str(r.get("value", "")))
                    return
                    
                elif name == "🖥️ Remote Hosts":
                    self.load_ssh_hosts_table()
                    return

                cursor.execute(
                    "SELECT config_json FROM ai_providers WHERE profile_id = ? AND name = ? "
                    "ORDER BY is_selected DESC, id DESC LIMIT 1",
                    (self.profile_id, name)
                )
                row = cursor.fetchone()
                if row:
                    config_encrypted = row["config_json"]
                    config_json = vault.decrypt(config_encrypted)
                    config = json.loads(config_json)

                    # First pass: restore combo item lists (from fetched models)
                    for k, v in config.items():
                        if k.endswith('_list') and isinstance(v, list):
                            combo_name = k[:-5]  # strip '_list' suffix
                            combo = self.form_widget.findChild(QComboBox, combo_name)
                            if combo:
                                self._replace_combo_items(combo, v, combo.currentText())

                    # Second pass: restore values (check QComboBox BEFORE QLineEdit
                    # because editable QComboBox contains an internal QLineEdit)
                    for k, v in config.items():
                        if k.endswith('_list'):
                            continue
                        v_str = str(v)
                        if not v_str:
                            continue
                        # Try QComboBox first
                        combo = self.form_widget.findChild(QComboBox, k)
                        if combo:
                            idx = combo.findText(v_str)
                            if idx >= 0:
                                combo.setCurrentIndex(idx)
                            else:
                                combo.addItem(v_str)
                                combo.setCurrentText(v_str)
                            continue
                        # Then try QLineEdit
                        inp = self.form_widget.findChild(QLineEdit, k)
                        if inp:
                            inp.setText(v_str)

                for combo_name, items in self.model_list_cache.get(name, {}).items():
                    combo = self.form_widget.findChild(QComboBox, combo_name)
                    if combo and combo.count() == 0:
                        self._replace_combo_items(combo, items, combo.currentText())
        except Exception as e:
            logger.error(f"Error loading setting data: {e}")

    def load_ssh_hosts_table(self):
        if not self.hosts_table:
            return
            
        try:
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute("SELECT id, name, host, port, username FROM ssh_hosts WHERE profile_id = ?", (self.profile_id,))
                rows = cursor.fetchall()
            
            self.hosts_table.setRowCount(0)
            for idx, r in enumerate(rows):
                self.hosts_table.insertRow(idx)
                
                name_item = QTableWidgetItem(r["name"])
                # Store SSH Host ID in the row first item
                name_item.setData(Qt.UserRole, r["id"])
                
                self.hosts_table.setItem(idx, 0, name_item)
                self.hosts_table.setItem(idx, 1, QTableWidgetItem(r["host"]))
                self.hosts_table.setItem(idx, 2, QTableWidgetItem(str(r["port"])))
                self.hosts_table.setItem(idx, 3, QTableWidgetItem(r["username"]))
                
            self.btn_edit_host.setEnabled(False)
            self.btn_del_host.setEnabled(False)
        except Exception as e:
            logger.error(f"Failed to query SSH hosts: {e}")

    def on_host_selection_changed(self):
        selected = self.hosts_table.selectedItems()
        has_sel = len(selected) > 0
        self.btn_edit_host.setEnabled(has_sel)
        self.btn_del_host.setEnabled(has_sel)

    def add_ssh_host(self):
        dialog = SSHHostDialog(self.profile_id, parent=self)
        dialog.host_saved.connect(self.load_ssh_hosts_table)
        dialog.exec()

    def edit_ssh_host(self):
        selected_row = self.hosts_table.currentRow()
        if selected_row < 0:
            return
        host_id = self.hosts_table.item(selected_row, 0).data(Qt.UserRole)
        
        dialog = SSHHostDialog(self.profile_id, host_id=host_id, parent=self)
        dialog.host_saved.connect(self.load_ssh_hosts_table)
        dialog.exec()

    def delete_ssh_host(self):
        selected_row = self.hosts_table.currentRow()
        if selected_row < 0:
            return
            
        host_name = self.hosts_table.item(selected_row, 0).text()
        host_id = self.hosts_table.item(selected_row, 0).data(Qt.UserRole)
        
        res = QMessageBox.question(
            self, "Confirm Delete",
            f"Are you sure you want to remove remote host '{host_name}'?",
            QMessageBox.Yes | QMessageBox.No
        )
        if res == QMessageBox.Yes:
            try:
                with db.connection() as conn:
                    cursor = conn.cursor()
                    cursor.execute("DELETE FROM ssh_hosts WHERE id = ?", (host_id,))
                self.load_ssh_hosts_table()
            except Exception as e:
                logger.error(f"Failed to delete SSH Host: {e}")

    def save_configuration(self, silent: bool = False):
        if not self.current_setting_name:
            return False

        try:
            if self.current_setting_name == "📋 Scan Rules":
                rules = RulesEngine.get_default_rules()
                for r in rules:
                    rid = r["id"]
                    chk = self.form_widget.findChild(QCheckBox, f"chk_{rid}")
                    val_input = self.form_widget.findChild(QLineEdit, f"val_{rid}")
                    if chk:
                        r["enabled"] = chk.isChecked()
                    if val_input:
                        try:
                            r["value"] = int(val_input.text().strip())
                        except ValueError:
                            pass
                
                rules_json = json.dumps(rules)
                with db.connection() as conn:
                    cursor = conn.cursor()
                    cursor.execute(
                        "INSERT INTO settings (profile_id, setting_key, setting_value) VALUES (?, 'scan_rules', ?) "
                        "ON CONFLICT(profile_id, setting_key) DO UPDATE SET setting_value = excluded.setting_value",
                        (self.profile_id, rules_json)
                    )
                if not silent:
                    QMessageBox.information(self, "Success", "Scan rules configuration saved successfully!")
                return True

            # Save AI config
            config = {}
            for inp in self.form_widget.findChildren(QLineEdit):
                # Skip internal QLineEdit widgets inside editable QComboBoxes
                if isinstance(inp.parent(), QComboBox):
                    continue
                if inp.objectName():
                    config[inp.objectName()] = inp.text().strip()
            for combo in self.form_widget.findChildren(QComboBox):
                if combo.objectName():
                    current_text = combo.currentText().strip()
                    config[combo.objectName()] = current_text
                    # Save the full list of items so we can restore fetched models later.
                    # Editable combos do not always add typed/current text as an item.
                    items = [combo.itemText(i) for i in range(combo.count())]
                    if current_text and current_text not in items:
                        items.append(current_text)
                    if items:
                        config[f"{combo.objectName()}_list"] = items
                        self.model_list_cache.setdefault(self.current_setting_name, {})[combo.objectName()] = items

            config_json = json.dumps(config)
            config_encrypted = vault.encrypt(config_json)
            
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute(
                    "SELECT id FROM ai_providers WHERE profile_id = ? AND name = ? "
                    "ORDER BY is_selected DESC, id DESC LIMIT 1",
                    (self.profile_id, self.current_setting_name)
                )
                row = cursor.fetchone()

                cursor.execute("UPDATE ai_providers SET is_selected = 0 WHERE profile_id = ?", (self.profile_id,))
                
                if row:
                    cursor.execute(
                        "UPDATE ai_providers SET config_json = ?, is_selected = 1 WHERE id = ?",
                        (config_encrypted, row["id"])
                    )
                    cursor.execute(
                        "DELETE FROM ai_providers WHERE profile_id = ? AND name = ? AND id <> ?",
                        (self.profile_id, self.current_setting_name, row["id"])
                    )
                else:
                    prov_type = "local" if self.current_setting_name in ("Ollama", "LM Studio") else "api"
                    cursor.execute(
                        "INSERT INTO ai_providers (profile_id, name, type, config_json, is_selected) VALUES (?, ?, ?, ?, 1)",
                        (self.profile_id, self.current_setting_name, prov_type, config_encrypted)
                    )
            if not silent:
                QMessageBox.information(self, "Success", f"Configuration for {self.current_setting_name} saved and set active!")
            return True
        except Exception as e:
            logger.error(f"Save failed: {e}")
            if not silent:
                QMessageBox.critical(self, "Error", f"Failed to save: {e}")
            return False

    def test_connection(self):
        if not self.current_setting_name or self.current_setting_name in ("📋 Scan Rules", "🖥️ Remote Hosts"):
            return

        config = {}
        for inp in self.form_widget.findChildren(QLineEdit):
            # Skip internal QLineEdit widgets inside editable QComboBoxes
            if isinstance(inp.parent(), QComboBox):
                continue
            if inp.objectName():
                config[inp.objectName()] = inp.text().strip()
        for combo in self.form_widget.findChildren(QComboBox):
            if combo.objectName():
                config[combo.objectName()] = combo.currentText().strip()

        provider = None
        name = self.current_setting_name
        if name == "Ollama":
            provider = OllamaProvider(config)
        elif name == "LM Studio":
            provider = LMStudioProvider(config)
        elif name == "OpenAI API":
            provider = OpenAIAPIProvider(config)
        elif name == "Anthropic API":
            provider = AnthropicAPIProvider(config)
        elif name == "Gemini API":
            provider = GeminiAPIProvider(config)
        elif name == "DeepSeek API":
            provider = DeepSeekAPIProvider(config)


        if not provider:
            return

        self.btn_test.setText("Testing...")
        self.btn_test.setEnabled(False)

        class TestWorker(QThread):
            finished = Signal(bool, str)
            
            def __init__(self, prov):
                super().__init__()
                self.prov = prov
                
            def run(self):
                success, msg = self.prov.test_connection()
                self.finished.emit(success, msg)

        self.test_worker = TestWorker(provider)
        self.test_worker.finished.connect(self.on_test_finished)
        self.test_worker.start()

    def on_test_finished(self, success: bool, msg: str):
        self.btn_test.setText("⚡ Test Connection")
        self.btn_test.setEnabled(True)
        if success:
            QMessageBox.information(self, "Test Success", msg)
        else:
            QMessageBox.critical(self, "Test Failed", msg)

    def load_providers(self):
        self.setting_list.setCurrentRow(0)
        self.on_setting_selected(self.setting_list.currentItem())

    def clear_data(self):
        self.clear_form()
        self.current_setting_name = None
