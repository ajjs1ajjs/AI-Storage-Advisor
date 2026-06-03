import os
from PySide6.QtWidgets import (
    QDialog, QVBoxLayout, QHBoxLayout, QLabel, QLineEdit, 
    QPushButton, QComboBox, QMessageBox, QFileDialog
)
from PySide6.QtCore import Qt, Signal, Slot, QThread
from PySide6.QtGui import QFont

from app.modules.remote_scanner import RemoteSSHScanner
from app.database.db_manager import db
from app.security.vault import vault
from app.core.config import logger

class SSHHostDialog(QDialog):
    host_saved = Signal()

    def __init__(self, profile_id: int, host_id: int = None, parent=None):
        super().__init__(parent)
        self.profile_id = profile_id
        self.host_id = host_id # If editing existing host
        self.setWindowTitle("Configure Remote SSH Host")
        self.setMinimumSize(450, 420)
        self.setStyleSheet("""
            QDialog {
                background-color: #0d0e12;
            }
            QLabel {
                color: #e2e8f0;
                font-weight: 500;
            }
        """)
        self.init_ui()
        if self.host_id:
            self.load_host_data()

    def init_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(20, 20, 20, 20)
        layout.setSpacing(12)

        # Title
        title = QLabel("🖥️ SSH Server Credentials")
        title.setFont(QFont("Segoe UI", 14, QFont.Bold))
        title.setStyleSheet("color: #7c4dff;")
        layout.addWidget(title)

        # Field: Display Name
        layout.addWidget(QLabel("Connection Name (Label):"))
        self.inp_name = QLineEdit()
        self.inp_name.setPlaceholderText("e.g. Production Web Server")
        layout.addWidget(self.inp_name)

        # Field: Host & Port
        host_port_layout = QHBoxLayout()
        
        host_col = QVBoxLayout()
        host_col.addWidget(QLabel("Host (IP or Domain):"))
        self.inp_host = QLineEdit()
        self.inp_host.setPlaceholderText("192.168.1.50")
        host_col.addWidget(self.inp_host)
        host_port_layout.addLayout(host_col, stretch=3)

        port_col = QVBoxLayout()
        port_col.addWidget(QLabel("Port:"))
        self.inp_port = QLineEdit("22")
        port_col.addWidget(self.inp_port)
        host_port_layout.addLayout(port_col, stretch=1)
        
        layout.addLayout(host_port_layout)

        # Field: Username
        layout.addWidget(QLabel("SSH Username:"))
        self.inp_username = QLineEdit("root")
        layout.addWidget(self.inp_username)

        # Field: Auth Type
        layout.addWidget(QLabel("Authentication Method:"))
        self.combo_auth = QComboBox()
        self.combo_auth.addItems(["Password", "Private Key File"])
        self.combo_auth.currentTextChanged.connect(self.on_auth_type_changed)
        layout.addWidget(self.combo_auth)

        # Field: Password or Key Path
        self.lbl_cred = QLabel("SSH Password:")
        layout.addWidget(self.lbl_cred)

        cred_row = QHBoxLayout()
        self.inp_cred = QLineEdit()
        self.inp_cred.setEchoMode(QLineEdit.Password)
        cred_row.addWidget(self.inp_cred)

        self.btn_browse_key = QPushButton("📁 Browse")
        self.btn_browse_key.setProperty("class", "SecondaryBtn")
        self.btn_browse_key.clicked.connect(self.browse_key_file)
        self.btn_browse_key.setVisible(False)
        cred_row.addWidget(self.btn_browse_key)

        layout.addLayout(cred_row)

        # Action panel
        btn_layout = QHBoxLayout()
        btn_layout.setSpacing(10)

        self.btn_test = QPushButton("⚡ Test Connection")
        self.btn_test.setProperty("class", "SecondaryBtn")
        self.btn_test.clicked.connect(self.test_connection)
        btn_layout.addWidget(self.btn_test)
        
        btn_layout.addStretch()

        self.btn_cancel = QPushButton("Cancel")
        self.btn_cancel.setProperty("class", "SecondaryBtn")
        self.btn_cancel.clicked.connect(self.reject)
        btn_layout.addWidget(self.btn_cancel)

        self.btn_save = QPushButton("💾 Save Server")
        self.btn_save.setProperty("class", "PrimaryBtn")
        self.btn_save.clicked.connect(self.save_host)
        btn_layout.addWidget(self.btn_save)

        layout.addLayout(btn_layout)

    def on_auth_type_changed(self, text):
        if text == "Password":
            self.lbl_cred.setText("SSH Password:")
            self.inp_cred.setPlaceholderText("")
            self.inp_cred.setEchoMode(QLineEdit.Password)
            self.btn_browse_key.setVisible(False)
        else:
            self.lbl_cred.setText("Private Key File Path:")
            self.inp_cred.setPlaceholderText("C:/Users/name/.ssh/id_rsa")
            self.inp_cred.setEchoMode(QLineEdit.Normal)
            self.btn_browse_key.setVisible(True)

    def browse_key_file(self):
        file_path, _ = QFileDialog.getOpenFileName(
            self, "Select Private Key File", os.path.expanduser("~"), "All Files (*)"
        )
        if file_path:
            self.inp_cred.setText(file_path)

    def load_host_data(self):
        try:
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute("SELECT * FROM ssh_hosts WHERE id = ?", (self.host_id,))
                row = cursor.fetchone()
            
            if row:
                self.inp_name.setText(row["name"])
                self.inp_host.setText(row["host"])
                self.inp_port.setText(str(row["port"]))
                self.inp_username.setText(row["username"])
                
                auth_type = row["auth_type"]
                self.combo_auth.setCurrentText("Password" if auth_type == "password" else "Private Key File")
                
                if row["credentials"]:
                    decrypted_cred = vault.decrypt(row["credentials"])
                    self.inp_cred.setText(decrypted_cred)
        except Exception as e:
            logger.error(f"Failed to load SSH host details: {e}")

    def test_connection(self):
        host = self.inp_host.text().strip()
        port_str = self.inp_port.text().strip()
        username = self.inp_username.text().strip()
        auth_type = "password" if self.combo_auth.currentText() == "Password" else "key"
        cred = self.inp_cred.text().strip()

        if not host or not port_str or not username or not cred:
            QMessageBox.warning(self, "Warning", "Please fill in all connection parameters.")
            return

        try:
            port = int(port_str)
        except ValueError:
            QMessageBox.warning(self, "Warning", "Port must be a valid integer.")
            return

        self.btn_test.setText("Testing...")
        self.btn_test.setEnabled(False)

        class ConnectionTester(QThread):
            result = Signal(bool, str)
            
            def __init__(self, h, p, u, at, c):
                super().__init__()
                self.h = h; self.p = p; self.u = u; self.at = at; self.c = c
                
            def run(self):
                success, msg = RemoteSSHScanner.test_ssh_connection(
                    self.h, self.p, self.u, self.at, self.c
                )
                self.result.emit(success, msg)

        self.tester = ConnectionTester(host, port, username, auth_type, cred)
        self.tester.result.connect(self.on_test_finished)
        self.tester.start()

    def on_test_finished(self, success: bool, msg: str):
        self.btn_test.setText("⚡ Test Connection")
        self.btn_test.setEnabled(True)
        if success:
            QMessageBox.information(self, "Success", msg)
        else:
            QMessageBox.critical(self, "Connection Failed", msg)

    def save_host(self):
        name = self.inp_name.text().strip()
        host = self.inp_host.text().strip()
        port_str = self.inp_port.text().strip()
        username = self.inp_username.text().strip()
        auth_type_str = "password" if self.combo_auth.currentText() == "Password" else "key"
        cred = self.inp_cred.text().strip()

        if not name or not host or not port_str or not username or not cred:
            QMessageBox.warning(self, "Warning", "Please fill in all fields.")
            return

        try:
            port = int(port_str)
        except ValueError:
            QMessageBox.warning(self, "Warning", "Port must be an integer.")
            return

        try:
            encrypted_cred = vault.encrypt(cred)
            with db.connection() as conn:
                cursor = conn.cursor()

                if self.host_id:
                    cursor.execute(
                        "UPDATE ssh_hosts SET name=?, host=?, port=?, username=?, auth_type=?, credentials=? WHERE id=?",
                        (name, host, port, username, auth_type_str, encrypted_cred, self.host_id)
                    )
                else:
                    cursor.execute(
                        "INSERT INTO ssh_hosts (profile_id, name, host, port, username, auth_type, credentials) VALUES (?, ?, ?, ?, ?, ?, ?)",
                        (self.profile_id, name, host, port, username, auth_type_str, encrypted_cred)
                    )
            
            self.host_saved.emit()
            self.accept()
        except Exception as e:
            logger.error(f"Failed to save SSH server: {e}")
            QMessageBox.critical(self, "Database Error", f"Failed to save host: {e}")
