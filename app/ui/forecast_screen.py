import os
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, 
    QPushButton, QFrame, QMessageBox, QInputDialog, QFileDialog
)
from PySide6.QtCore import Slot, Qt
from PySide6.QtGui import QFont

from app.modules.forecast_engine import ForecastEngine
from app.ui.components.growth_chart import GrowthChart
from app.core.profile_manager import ProfileManager
from app.core.config import logger

class ForecastScreen(QWidget):
    def __init__(self, parent=None):
        super().__init__(parent)
        self.profile_id = None
        self.engine = None
        self.scan_path = os.path.abspath(".")
        self.init_ui()

    def set_profile(self, profile_id: int):
        self.profile_id = profile_id
        self.engine = ForecastEngine(profile_id)
        self.load_forecast()

    def init_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(20, 20, 20, 20)
        layout.setSpacing(15)

        # 1. Header Bar
        header = QHBoxLayout()
        title_lbl = QLabel("📈 Storage Forecast & SRE Analytics")
        title_lbl.setFont(QFont("Segoe UI", 16, QFont.Bold))
        title_lbl.setStyleSheet("color: #7c4dff;")
        header.addWidget(title_lbl)
        
        header.addStretch()
        
        # Profile Export Trigger
        self.btn_export = QPushButton("📤 Export Profile Backup")
        self.btn_export.setProperty("class", "PrimaryBtn")
        self.btn_export.clicked.connect(self.export_profile)
        header.addWidget(self.btn_export)

        layout.addLayout(header)

        # 2. Metric Cards Panel
        cards_layout = QHBoxLayout()
        cards_layout.setSpacing(15)

        # Card 1: Days remaining
        self.card_days = QFrame()
        self.card_days.setProperty("class", "Card")
        days_layout = QVBoxLayout(self.card_days)
        days_layout.setContentsMargins(15, 15, 15, 15)
        
        lbl1 = QLabel("DAYS REMAINING")
        lbl1.setFont(QFont("Segoe UI", 9, QFont.Bold))
        lbl1.setStyleSheet("color: #94a3b8;")
        days_layout.addWidget(lbl1)
        
        self.val_days = QLabel("Calculating...")
        self.val_days.setFont(QFont("Segoe UI", 24, QFont.Bold))
        self.val_days.setStyleSheet("color: #a78bfa;")
        days_layout.addWidget(self.val_days)
        
        cards_layout.addWidget(self.card_days)

        # Card 2: Growth Rate
        self.card_growth = QFrame()
        self.card_growth.setProperty("class", "Card")
        growth_layout = QVBoxLayout(self.card_growth)
        growth_layout.setContentsMargins(15, 15, 15, 15)
        
        lbl2 = QLabel("DAILY CONSUMPTION SPEED")
        lbl2.setFont(QFont("Segoe UI", 9, QFont.Bold))
        lbl2.setStyleSheet("color: #94a3b8;")
        growth_layout.addWidget(lbl2)
        
        self.val_growth = QLabel("Calculating...")
        self.val_growth.setFont(QFont("Segoe UI", 24, QFont.Bold))
        self.val_growth.setStyleSheet("color: #e2e8f0;")
        growth_layout.addWidget(self.val_growth)
        
        cards_layout.addWidget(self.card_growth)

        # Card 3: Disk capacity
        self.card_space = QFrame()
        self.card_space.setProperty("class", "Card")
        space_layout = QVBoxLayout(self.card_space)
        space_layout.setContentsMargins(15, 15, 15, 15)
        
        lbl3 = QLabel("AVAILABLE STORAGE SPACE")
        lbl3.setFont(QFont("Segoe UI", 9, QFont.Bold))
        lbl3.setStyleSheet("color: #94a3b8;")
        space_layout.addWidget(lbl3)
        
        self.val_space = QLabel("Calculating...")
        self.val_space.setFont(QFont("Segoe UI", 20, QFont.Bold))
        self.val_space.setStyleSheet("color: #10b981;")
        space_layout.addWidget(self.val_space)
        
        cards_layout.addWidget(self.card_space)

        layout.addLayout(cards_layout)

        # 3. Custom Line Chart Area
        chart_card = QFrame()
        chart_card.setProperty("class", "Card")
        chart_layout = QVBoxLayout(chart_card)
        chart_layout.setContentsMargins(15, 15, 15, 15)
        
        chart_title = QLabel("Storage Consumption Trend (Days vs Space)")
        chart_title.setFont(QFont("Segoe UI", 11, QFont.Bold))
        chart_layout.addWidget(chart_title)

        self.chart = GrowthChart()
        self.chart.setMinimumHeight(300)
        chart_layout.addWidget(self.chart)
        
        layout.addWidget(chart_card)

        # 4. SRE Recommendations Panel
        advice_card = QFrame()
        advice_card.setProperty("class", "Card")
        advice_layout = QVBoxLayout(advice_card)
        advice_layout.setContentsMargins(15, 15, 15, 15)
        
        advice_title = QLabel("SRE Storage Recommendations")
        advice_title.setFont(QFont("Segoe UI", 11, QFont.Bold))
        advice_title.setStyleSheet("color: #a78bfa;")
        advice_layout.addWidget(advice_title)

        self.advice_lbl = QLabel(
            "Scan your storage path multiple times to capture daily growth trends.\n"
            "AI Storage Advisor will linear-project your disk exhaustion and help you plan storage node cleanups."
        )
        self.advice_lbl.setWordWrap(True)
        self.advice_lbl.setStyleSheet("color: #94a3b8; font-size: 13px; line-height: 1.4;")
        advice_layout.addWidget(self.advice_lbl)
        
        layout.addWidget(advice_card)

    def format_size(self, size_bytes: int) -> str:
        for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
            if size_bytes < 1024.0:
                return f"{size_bytes:.2f} {unit}"
            size_bytes /= 1024.0
        return f"{size_bytes:.2f} PB"

    def load_forecast(self):
        if not self.engine:
            return

        report = self.engine.calculate_forecast(self.scan_path)
        
        if report["status"] == "insufficient_data":
            self.val_days.setText("Need Scans")
            self.val_days.setStyleSheet("color: #f59e0b;")
            self.val_growth.setText("No Trend")
            self.val_space.setText("Run scan first")
            self.advice_lbl.setText(
                "SRE Storage Info: Insufficient history data points to compute daily growth velocity.\n"
                "Please run a disk scan on the Dashboard, make changes (e.g. create dummy files), "
                "and scan again to build data points."
            )
            self.chart.set_data([], 0)
            return

        # 1. Update metric values
        days = report["days_remaining"]
        if days == -1:
            self.val_days.setText("Stable")
            self.val_days.setStyleSheet("color: #10b981;") # Green
        else:
            self.val_days.setText(f"{days} Days")
            if days < 30:
                self.val_days.setStyleSheet("color: #ef4444;") # Red
            elif days < 90:
                self.val_days.setStyleSheet("color: #f59e0b;") # Orange
            else:
                self.val_days.setStyleSheet("color: #a78bfa;") # Purple

        growth_formatted = self.format_size(abs(report["daily_growth_bytes"]))
        if report["daily_growth_bytes"] >= 0:
            self.val_growth.setText(f"+{growth_formatted}/day")
            self.val_growth.setStyleSheet("color: #e2e8f0;")
        else:
            self.val_growth.setText(f"-{growth_formatted}/day")
            self.val_growth.setStyleSheet("color: #10b981;")

        free = self.format_size(report["free_bytes"])
        total = self.format_size(report["total_bytes"])
        self.val_space.setText(f"{free} free / {total}")

        # 2. Bind custom line chart
        self.chart.set_data(report["trend_points"], report["total_bytes"])

        # 3. SRE text advice
        if report["daily_growth_bytes"] > 0:
            text = (
                f"SRE Storage Health Advisory:\n"
                f"• Average storage growth is running at +{growth_formatted} per day.\n"
                f"• At this rate, your local partition is projected to reach full capacity in approximately {days} days.\n"
                f"• Actions advised: Review temporary folder filters and clean obsolete files immediately to restore headroom."
            )
        else:
            text = (
                f"SRE Storage Health Advisory:\n"
                f"• Disk usage trend is stable or declining (-{growth_formatted} per day).\n"
                f"• No risk of disk exhaustion detected within the forecast horizon.\n"
                f"• Actions advised: Maintain current storage configuration."
            )
        self.advice_lbl.setText(text)

    def set_scan_path(self, path: str):
        self.scan_path = path

    def export_profile(self):
        if not self.profile_id:
            return

        file_path, _ = QFileDialog.getSaveFileName(
            self, "Save Profile Backup", "profile.aisprofile", "AI Storage Profile (*.aisprofile)"
        )
        if not file_path:
            return

        password, ok = QInputDialog.getText(
            self, "Backup Password",
            "Enter a password to encrypt this backup archive:",
            QLineEdit.Password
        )
        if not ok or not password:
            return

        success = ProfileManager.export_profile(self.profile_id, file_path, password)
        if success:
            QMessageBox.information(
                self, "Export Successful",
                f"Profile backup successfully encrypted and written to:\n{os.path.basename(file_path)}"
            )
        else:
            QMessageBox.critical(self, "Export Failed", "Could not export profile. Check logs for details.")
