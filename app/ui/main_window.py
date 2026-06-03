import sys
from PySide6.QtWidgets import (
    QMainWindow, QWidget, QHBoxLayout, QVBoxLayout, 
    QPushButton, QStackedWidget, QLabel, QFrame, QGraphicsDropShadowEffect
)
from PySide6.QtCore import Qt, QSize
from PySide6.QtGui import QColor, QFont, QIcon

from app.core.config import APP_NAME, VERSION, logger, APP_ROOT
from app.ui.login_screen import LoginScreen
from app.ui.dashboard_screen import DashboardScreen
from app.ui.forecast_screen import ForecastScreen
from app.ui.settings_screen import SettingsScreen
from app.ui.sre_screen import SREScreen

# Modern Dark Theme Stylesheet (QSS)
# Modern Dark Theme Stylesheet (QSS)
DARK_STYLE = """
QMainWindow {
    background-color: #0d0e12;
}

QWidget {
    color: #e2e8f0;
    font-family: 'Segoe UI', -apple-system, BlinkMacSystemFont, Roboto, sans-serif;
    font-size: 13px;
}

/* Sidebar Styling */
QFrame#Sidebar {
    background-color: #13151a;
    border-right: 1px solid #1f232b;
    min-width: 220px;
    max-width: 220px;
}

QLabel#SidebarTitle {
    color: #7c4dff;
    font-size: 18px;
    font-weight: bold;
    padding: 20px 10px;
}

QPushButton.SidebarBtn {
    background-color: transparent;
    color: #94a3b8;
    border: none;
    border-radius: 6px;
    padding: 12px 16px;
    text-align: left;
    font-weight: 500;
}

QPushButton.SidebarBtn:hover {
    background-color: #1e222b;
    color: #f8fafc;
}

QPushButton.SidebarBtn:checked {
    background-color: #271e47;
    color: #a78bfa;
    border-left: 3px solid #7c4dff;
}

/* Screen container */
QFrame#ContentArea {
    background-color: #0d0e12;
    border: none;
}

/* Buttons */
QPushButton.PrimaryBtn {
    background-color: #7c4dff;
    color: white;
    border: none;
    border-radius: 6px;
    padding: 10px 20px;
    font-weight: bold;
}

QPushButton.PrimaryBtn:hover {
    background-color: #6c3ce9;
}

QPushButton.PrimaryBtn:pressed {
    background-color: #5b2cd0;
}

QPushButton.SecondaryBtn {
    background-color: #1e222b;
    color: #e2e8f0;
    border: 1px solid #2d3343;
    border-radius: 6px;
    padding: 10px 20px;
}

QPushButton.SecondaryBtn:hover {
    background-color: #272c38;
}

/* Text Inputs */
QLineEdit {
    background-color: #13151a;
    border: 1px solid #2d3343;
    border-radius: 6px;
    padding: 8px 12px;
    color: #f8fafc;
}

QLineEdit:focus {
    border: 1px solid #7c4dff;
}

/* Combo Box */
QComboBox {
    background-color: #13151a;
    border: 1px solid #2d3343;
    border-radius: 6px;
    padding: 8px 12px;
    color: #f8fafc;
}

QComboBox QAbstractItemView {
    background-color: #13151a;
    border: 1px solid #2d3343;
    selection-background-color: #271e47;
    selection-color: #a78bfa;
    color: #f8fafc;
}

/* Card Styling */
QFrame.Card {
    background-color: #13151a;
    border: 1px solid #1f232b;
    border-radius: 12px;
}

QFrame.GlassCard {
    background-color: rgba(19, 21, 26, 0.8);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 12px;
}

/* TextBrowser Styling */
QTextBrowser {
    background-color: #0d0e12;
    border: 1px solid #1f232b;
    border-radius: 6px;
    padding: 10px;
    color: #e2e8f0;
}

/* Status Bar & Progress Card styling */
QFrame#StatusBar {
    background-color: #0d0e12;
    border-top: 1px solid #1f232b;
}

QLabel#StatusLabel {
    color: #64748b;
    font-size: 11px;
}

QProgressBar {
    background-color: #13151a;
    border: 1px solid #2d3343;
    border-radius: 4px;
    height: 12px;
    text-align: center;
}

QProgressBar::chunk {
    background-color: #7c4dff;
    border-radius: 4px;
}

/* Lists and Tables */
QListWidget {
    background-color: transparent;
    border: none;
}

QListWidget::item {
    background-color: transparent;
    padding: 10px;
    border-radius: 6px;
    color: #94a3b8;
}

QListWidget::item:hover {
    background-color: #1e222b;
    color: #f8fafc;
}

QListWidget::item:selected {
    background-color: #271e47;
    color: #a78bfa;
}

QTableWidget {
    background-color: #13151a;
    gridline-color: #1f232b;
    border: 1px solid #1f232b;
    border-radius: 6px;
}

QHeaderView::section {
    background-color: #1a1e26;
    color: #e2e8f0;
    padding: 5px;
    border: 1px solid #1f232b;
}
"""

# Modern Light Theme Stylesheet (QSS)
LIGHT_STYLE = """
QMainWindow {
    background-color: #f8fafc;
}

QWidget {
    color: #0f172a;
    font-family: 'Segoe UI', -apple-system, BlinkMacSystemFont, Roboto, sans-serif;
    font-size: 13px;
}

/* Sidebar Styling */
QFrame#Sidebar {
    background-color: #ffffff;
    border-right: 1px solid #e2e8f0;
    min-width: 220px;
    max-width: 220px;
}

QLabel#SidebarTitle {
    color: #7c4dff;
    font-size: 18px;
    font-weight: bold;
    padding: 20px 10px;
}

QPushButton.SidebarBtn {
    background-color: transparent;
    color: #475569;
    border: none;
    border-radius: 6px;
    padding: 12px 16px;
    text-align: left;
    font-weight: 500;
}

QPushButton.SidebarBtn:hover {
    background-color: #f1f5f9;
    color: #0f172a;
}

QPushButton.SidebarBtn:checked {
    background-color: #eedffc;
    color: #6d28d9;
    border-left: 3px solid #7c4dff;
}

/* Screen container */
QFrame#ContentArea {
    background-color: #f8fafc;
    border: none;
}

/* Buttons */
QPushButton.PrimaryBtn {
    background-color: #7c4dff;
    color: white;
    border: none;
    border-radius: 6px;
    padding: 10px 20px;
    font-weight: bold;
}

QPushButton.PrimaryBtn:hover {
    background-color: #6c3ce9;
}

QPushButton.PrimaryBtn:pressed {
    background-color: #5b2cd0;
}

QPushButton.SecondaryBtn {
    background-color: #f1f5f9;
    color: #0f172a;
    border: 1px solid #cbd5e1;
    border-radius: 6px;
    padding: 10px 20px;
}

QPushButton.SecondaryBtn:hover {
    background-color: #e2e8f0;
}

/* Text Inputs */
QLineEdit {
    background-color: #ffffff;
    border: 1px solid #cbd5e1;
    border-radius: 6px;
    padding: 8px 12px;
    color: #0f172a;
}

QLineEdit:focus {
    border: 1px solid #7c4dff;
}

/* Combo Box */
QComboBox {
    background-color: #ffffff;
    border: 1px solid #cbd5e1;
    border-radius: 6px;
    padding: 8px 12px;
    color: #0f172a;
}

QComboBox QAbstractItemView {
    background-color: #ffffff;
    border: 1px solid #cbd5e1;
    selection-background-color: #eedffc;
    selection-color: #6d28d9;
    color: #0f172a;
}

/* Card Styling */
QFrame.Card {
    background-color: #ffffff;
    border: 1px solid #e2e8f0;
    border-radius: 12px;
}

QFrame.GlassCard {
    background-color: rgba(255, 255, 255, 0.8);
    border: 1px solid rgba(0, 0, 0, 0.05);
    border-radius: 12px;
}

/* TextBrowser Styling */
QTextBrowser {
    background-color: #f8fafc;
    border: 1px solid #cbd5e1;
    border-radius: 6px;
    padding: 10px;
    color: #0f172a;
}

/* Status Bar & Progress Card styling */
QFrame#StatusBar {
    background-color: #f1f5f9;
    border-top: 1px solid #cbd5e1;
}

QLabel#StatusLabel {
    color: #64748b;
    font-size: 11px;
}

QProgressBar {
    background-color: #f1f5f9;
    border: 1px solid #cbd5e1;
    border-radius: 4px;
    height: 12px;
    text-align: center;
}

QProgressBar::chunk {
    background-color: #7c4dff;
    border-radius: 4px;
}

/* Lists and Tables */
QListWidget {
    background-color: transparent;
    border: none;
}

QListWidget::item {
    background-color: transparent;
    padding: 10px;
    border-radius: 6px;
    color: #475569;
}

QListWidget::item:hover {
    background-color: #f1f5f9;
    color: #0f172a;
}

QListWidget::item:selected {
    background-color: #eedffc;
    color: #6d28d9;
}

QTableWidget {
    background-color: #ffffff;
    gridline-color: #e2e8f0;
    border: 1px solid #e2e8f0;
    border-radius: 6px;
}

QHeaderView::section {
    background-color: #f1f5f9;
    color: #0f172a;
    padding: 5px;
    border: 1px solid #e2e8f0;
}
"""

from app.database.db_manager import db

class MainWindow(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle(f"{APP_NAME} v{VERSION}")
        self.setMinimumSize(1100, 750)
        
        # Set Window Icon
        icon_path = APP_ROOT / "app_icon.png"
        if icon_path.exists():
            self.setWindowIcon(QIcon(str(icon_path)))
        
        self.current_user_id = 1
        self.current_username = "default_user"
        self.current_profile_id = 1

        # Load persisted theme preference from database
        self.theme = "dark"
        try:
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute("SELECT setting_value FROM settings WHERE profile_id = 1 AND setting_key = 'theme'")
                row = cursor.fetchone()
                if row:
                    self.theme = row["setting_value"]
        except Exception as e:
            logger.error(f"Failed to load theme setting: {e}")

        self.apply_theme(self.theme)

        self.main_stacked = QStackedWidget()
        self.setCentralWidget(self.main_stacked)

        # Create Core Dashboard Workspace Page
        self.workspace_widget = QWidget()
        self.setup_workspace_ui()
        self.main_stacked.addWidget(self.workspace_widget)
        
        # Initialize sub-screens with default profile ID 1
        self.dashboard_screen.set_profile(1)
        self.settings_screen.set_profile(1)

        self.main_stacked.setCurrentWidget(self.workspace_widget)
        self.switch_tab(0)

    def setup_workspace_ui(self):
        layout = QHBoxLayout(self.workspace_widget)
        layout.setContentsMargins(0, 0, 0, 0)
        layout.setSpacing(0)

        # 1. Sidebar Panel
        sidebar = QFrame()
        sidebar.setObjectName("Sidebar")
        sidebar_layout = QVBoxLayout(sidebar)
        sidebar_layout.setContentsMargins(12, 20, 12, 20)
        sidebar_layout.setSpacing(8)

        title = QLabel(APP_NAME)
        title.setObjectName("SidebarTitle")
        title.setAlignment(Qt.AlignCenter)
        sidebar_layout.addWidget(title)
        
        div = QFrame()
        div.setFrameShape(QFrame.HLine)
        div.setFrameShadow(QFrame.Sunken)
        div.setStyleSheet("background-color: #1f232b; max-height: 1px; margin: 10px 0px;")
        sidebar_layout.addWidget(div)

        # Navigation Buttons (Dashboard & Settings only)
        self.btn_dashboard = QPushButton("📊  Dashboard")
        self.btn_dashboard.setCheckable(True)
        self.btn_dashboard.setChecked(True)
        self.btn_dashboard.setFlat(True)
        self.btn_dashboard.setProperty("class", "SidebarBtn")
        self.btn_dashboard.clicked.connect(lambda: self.switch_tab(0))
        sidebar_layout.addWidget(self.btn_dashboard)

        self.btn_settings = QPushButton("⚙️  Settings")
        self.btn_settings.setCheckable(True)
        self.btn_settings.setFlat(True)
        self.btn_settings.setProperty("class", "SidebarBtn")
        self.btn_settings.clicked.connect(lambda: self.switch_tab(1))
        sidebar_layout.addWidget(self.btn_settings)

        # Theme Selector Toggle Button
        self.btn_theme = QPushButton()
        self.btn_theme.setFlat(True)
        self.btn_theme.setProperty("class", "SidebarBtn")
        self.btn_theme.clicked.connect(self.toggle_theme)
        sidebar_layout.addWidget(self.btn_theme)
        
        # Apply initial text to the theme button
        if self.theme == "light":
            self.btn_theme.setText("🌙 Темна тема")
        else:
            self.btn_theme.setText("☀️ Світла тема")

        sidebar_layout.addStretch()

        self.nav_buttons = [self.btn_dashboard, self.btn_settings]

        # 2. Content Stack (Right)
        content_frame = QFrame()
        content_frame.setObjectName("ContentArea")
        content_layout = QVBoxLayout(content_frame)
        content_layout.setContentsMargins(0, 0, 0, 0)

        self.content_stack = QStackedWidget()
        content_layout.addWidget(self.content_stack)

        # Initialize screens
        self.dashboard_screen = DashboardScreen(self)
        self.settings_screen = SettingsScreen(self)

        self.content_stack.addWidget(self.dashboard_screen)
        self.content_stack.addWidget(self.settings_screen)

        layout.addWidget(sidebar)
        layout.addWidget(content_frame)

    def switch_tab(self, index: int):
        self.content_stack.setCurrentIndex(index)
        
        for idx, btn in enumerate(self.nav_buttons):
            btn.setChecked(idx == index)

        if index == 0:
            self.dashboard_screen.load_recent_scans()
        elif index == 1:
            self.settings_screen.load_providers()

    def toggle_theme(self):
        new_theme = "light" if self.theme == "dark" else "dark"
        self.theme = new_theme
        
        # Save to DB
        try:
            with db.connection() as conn:
                cursor = conn.cursor()
                cursor.execute(
                    "INSERT INTO settings (profile_id, setting_key, setting_value) VALUES (1, 'theme', ?) "
                    "ON CONFLICT(profile_id, setting_key) DO UPDATE SET setting_value = excluded.setting_value",
                    (new_theme,)
                )
        except Exception as e:
            logger.error(f"Failed to save theme setting to DB: {e}")
            
        self.apply_theme(new_theme)

    def apply_theme(self, theme_name: str):
        if theme_name == "light":
            self.setStyleSheet(LIGHT_STYLE)
            if hasattr(self, 'btn_theme'):
                self.btn_theme.setText("🌙 Темна тема")
        else:
            self.setStyleSheet(DARK_STYLE)
            if hasattr(self, 'btn_theme'):
                self.btn_theme.setText("☀️ Світла тема")
