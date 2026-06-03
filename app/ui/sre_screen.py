import os
import json
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, 
    QPushButton, QFrame, QTabWidget, QTableWidget, 
    QTableWidgetItem, QHeaderView, QMessageBox, QScrollArea
)
from PySide6.QtCore import Qt, QRectF, QPointF
from PySide6.QtGui import QPainter, QPen, QColor, QFont, QBrush, QGuiApplication

from app.modules.sre_analyzer import SREAnalyzer
from app.core.config import logger

class HealthGauge(QWidget):
    def __init__(self, parent=None):
        super().__init__(parent)
        self.score = 100
        self.setMinimumSize(220, 220)

    def set_score(self, score: int):
        self.score = score
        self.update()

    def paintEvent(self, event):
        painter = QPainter(self)
        painter.setRenderHint(QPainter.Antialiasing)
        
        width = self.width()
        height = self.height()
        side = min(width, height)
        
        # Draw bounding rect
        rect = QRectF((width - side) / 2.0 + 20.0, (height - side) / 2.0 + 20.0, side - 40.0, side - 40.0)
        
        # 1. Draw background track (270-degree arc open at bottom)
        pen_bg = QPen(QColor("#1f232b"), 14, Qt.SolidLine, Qt.RoundCap)
        painter.setPen(pen_bg)
        painter.drawArc(rect, 225 * 16, -270 * 16)
        
        # 2. Determine color based on score
        if self.score >= 80:
            color = QColor("#10b981") # Emerald green
        elif self.score >= 50:
            color = QColor("#f59e0b") # Warning amber
        else:
            color = QColor("#ef4444") # Danger red
            
        # 3. Draw foreground arc matching the score
        span_angle = -270.0 * (self.score / 100.0)
        pen_fg = QPen(color, 14, Qt.SolidLine, Qt.RoundCap)
        painter.setPen(pen_fg)
        painter.drawArc(rect, 225 * 16, int(span_angle * 16))
        
        # 4. Draw Score text inside the center of the arc
        painter.setPen(QColor("#f8fafc"))
        painter.setFont(QFont("Segoe UI", 36, QFont.Bold))
        text_rect = QRectF(rect.x(), rect.y() - 10, rect.width(), rect.height())
        painter.drawText(text_rect, Qt.AlignCenter, str(self.score))
        
        # 5. Draw Health Score Label below the score number
        painter.setPen(QColor("#94a3b8"))
        painter.setFont(QFont("Segoe UI", 9, QFont.Bold))
        label_rect = QRectF(rect.x(), rect.y() + side * 0.28, rect.width(), 30)
        painter.drawText(label_rect, Qt.AlignCenter, "HEALTH SCORE")


class SREScreen(QWidget):
    def __init__(self, parent=None):
        super().__init__(parent)
        self.profile_id = None
        self.scan_results = None
        self.sre_analyzer = SREAnalyzer()
        self.init_ui()

    def set_profile(self, profile_id: int):
        self.profile_id = profile_id

    def set_scan_results(self, results: dict):
        self.scan_results = results

    def init_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(20, 20, 20, 20)
        layout.setSpacing(15)

        # 1. Header Row
        header_layout = QHBoxLayout()
        title_lbl = QLabel("📟 System Reliability Engineering (SRE) Storage Dashboard")
        title_lbl.setFont(QFont("Segoe UI", 16, QFont.Bold))
        title_lbl.setStyleSheet("color: #7c4dff;")
        header_layout.addWidget(title_lbl)
        
        header_layout.addStretch()
        
        # Report generation button
        self.btn_report = QPushButton("📤 Generate SRE Report")
        self.btn_report.setProperty("class", "PrimaryBtn")
        self.btn_report.clicked.connect(self.generate_sre_report)
        self.btn_report.setEnabled(False)
        header_layout.addWidget(self.btn_report)

        layout.addLayout(header_layout)

        # 2. Main Content Split Panel
        content_layout = QHBoxLayout()
        content_layout.setSpacing(15)

        # Left Column: Health Gauge and quick statistics
        left_column = QVBoxLayout()
        left_column.setSpacing(15)

        gauge_card = QFrame()
        gauge_card.setProperty("class", "Card")
        gauge_layout = QVBoxLayout(gauge_card)
        gauge_layout.setContentsMargins(15, 20, 15, 20)
        gauge_layout.setAlignment(Qt.AlignCenter)

        self.gauge = HealthGauge()
        gauge_layout.addWidget(self.gauge)
        
        self.health_status_lbl = QLabel("Run a storage scan to calculate health score")
        self.health_status_lbl.setFont(QFont("Segoe UI", 11, QFont.Bold))
        self.health_status_lbl.setStyleSheet("color: #94a3b8;")
        self.health_status_lbl.setAlignment(Qt.AlignCenter)
        self.health_status_lbl.setWordWrap(True)
        gauge_layout.addWidget(self.health_status_lbl)

        left_column.addWidget(gauge_card)

        # Quick stats card
        stats_card = QFrame()
        stats_card.setProperty("class", "Card")
        stats_layout = QVBoxLayout(stats_card)
        stats_layout.setContentsMargins(15, 15, 15, 15)
        stats_layout.setSpacing(10)

        stats_title = QLabel("SYSTEM METRICS")
        stats_title.setFont(QFont("Segoe UI", 10, QFont.Bold))
        stats_title.setStyleSheet("color: #7c4dff;")
        stats_layout.addWidget(stats_title)

        self.lbl_total_size = QLabel("Scanned Storage: N/A")
        self.lbl_total_size.setStyleSheet("color: #e2e8f0;")
        stats_layout.addWidget(self.lbl_total_size)

        self.lbl_days_remaining = QLabel("Exhaustion Risk: N/A")
        self.lbl_days_remaining.setStyleSheet("color: #e2e8f0;")
        stats_layout.addWidget(self.lbl_days_remaining)

        self.lbl_duplicate_waste = QLabel("Duplicate waste: N/A")
        self.lbl_duplicate_waste.setStyleSheet("color: #e2e8f0;")
        stats_layout.addWidget(self.lbl_duplicate_waste)

        left_column.addWidget(stats_card)
        left_column.addStretch()

        content_layout.addLayout(left_column, stretch=2)

        # Right Column: Tab View
        self.tabs = QTabWidget()
        self.tabs.setStyleSheet("""
            QTabWidget::pane {
                border: 1px solid #1f232b;
                border-radius: 6px;
                background-color: #13151a;
            }
            QTabBar::tab {
                background-color: #1a1e26;
                color: #94a3b8;
                padding: 10px 15px;
                font-weight: 500;
                border-top-left-radius: 4px;
                border-top-right-radius: 4px;
                margin-right: 2px;
            }
            QTabBar::tab:hover {
                background-color: #222733;
                color: #f8fafc;
            }
            QTabBar::tab:selected {
                background-color: #13151a;
                color: #7c4dff;
                border-bottom: 2px solid #7c4dff;
            }
        """)

        # Tab 1: Warnings & Alerts
        self.warnings_widget = QWidget()
        warnings_tab_layout = QVBoxLayout(self.warnings_widget)
        warnings_tab_layout.setContentsMargins(15, 15, 15, 15)
        
        self.warnings_scroll = QScrollArea()
        self.warnings_scroll.setWidgetResizable(True)
        self.warnings_scroll.setStyleSheet("background-color: transparent; border: none;")
        self.warnings_scroll_widget = QWidget()
        self.warnings_list_layout = QVBoxLayout(self.warnings_scroll_widget)
        self.warnings_list_layout.setAlignment(Qt.AlignTop)
        self.warnings_scroll.setWidget(self.warnings_scroll_widget)
        
        warnings_tab_layout.addWidget(self.warnings_scroll)
        self.tabs.addTab(self.warnings_widget, "⚠️ Health Warnings")

        # Tab 2: Docker Containers & Volumes
        self.docker_widget = QWidget()
        docker_tab_layout = QVBoxLayout(self.docker_widget)
        docker_tab_layout.setContentsMargins(15, 15, 15, 15)

        self.docker_placeholder = QLabel("Docker daemon was not active or SSH Docker search was skipped.")
        self.docker_placeholder.setStyleSheet("color: #64748b; font-style: italic;")
        self.docker_placeholder.setAlignment(Qt.AlignCenter)
        docker_tab_layout.addWidget(self.docker_placeholder)

        # Containers Table
        self.docker_containers_lbl = QLabel("Running Container Layers:")
        self.docker_containers_lbl.setFont(QFont("Segoe UI", 10, QFont.Bold))
        self.docker_containers_lbl.setStyleSheet("color: #a78bfa;")
        self.docker_containers_lbl.setVisible(False)
        docker_tab_layout.addWidget(self.docker_containers_lbl)

        self.table_docker_containers = QTableWidget()
        self.setup_table(self.table_docker_containers, ["ID", "Name", "Image", "Write Layer Size", "Virtual Size"])
        self.table_docker_containers.setVisible(False)
        docker_tab_layout.addWidget(self.table_docker_containers, stretch=2)

        # Volumes Table
        self.docker_volumes_lbl = QLabel("Local Docker Volumes:")
        self.docker_volumes_lbl.setFont(QFont("Segoe UI", 10, QFont.Bold))
        self.docker_volumes_lbl.setStyleSheet("color: #a78bfa;")
        self.docker_volumes_lbl.setVisible(False)
        docker_tab_layout.addWidget(self.docker_volumes_lbl)

        self.table_docker_volumes = QTableWidget()
        self.setup_table(self.table_docker_volumes, ["Volume Name", "Size"])
        self.table_docker_volumes.setVisible(False)
        docker_tab_layout.addWidget(self.table_docker_volumes, stretch=1)

        self.tabs.addTab(self.docker_widget, "🐳 Docker Containers")

        # Tab 3: Windows System Folders
        self.windows_widget = QWidget()
        win_tab_layout = QVBoxLayout(self.windows_widget)
        win_tab_layout.setContentsMargins(15, 15, 15, 15)

        self.win_placeholder = QLabel("Windows System analytics is only available when scanning the local Windows machine.")
        self.win_placeholder.setStyleSheet("color: #64748b; font-style: italic;")
        self.win_placeholder.setAlignment(Qt.AlignCenter)
        win_tab_layout.addWidget(self.win_placeholder)

        self.table_win_folders = QTableWidget()
        self.setup_table(self.table_win_folders, ["Target Folder Description", "Default Path", "File Count", "Total Size"])
        self.table_win_folders.setVisible(False)
        win_tab_layout.addWidget(self.table_win_folders)

        self.tabs.addTab(self.windows_widget, "🖥️ Windows System")

        content_layout.addWidget(self.tabs, stretch=3)

        layout.addLayout(content_layout)

    def setup_table(self, table: QTableWidget, headers: list[str]):
        table.setColumnCount(len(headers))
        table.setHorizontalHeaderLabels(headers)
        table.horizontalHeader().setSectionResizeMode(0, QHeaderView.ResizeToContents)
        for i in range(1, len(headers) - 1):
            table.horizontalHeader().setSectionResizeMode(i, QHeaderView.ResizeToContents)
        table.horizontalHeader().setSectionResizeMode(len(headers) - 1, QHeaderView.Stretch)
        table.setStyleSheet("""
            QTableWidget {
                background-color: #13151a;
                gridline-color: #1f232b;
                border: none;
            }
            QHeaderView::section {
                background-color: #1a1e26;
                color: #e2e8f0;
                padding: 6px;
                border: 1px solid #1f232b;
            }
        """)

    def load_sre_analytics(self):
        if not self.scan_results:
            self.gauge.set_score(100)
            self.health_status_lbl.setText("No scan results. Run a disk scan first.")
            self.btn_report.setEnabled(False)
            return

        sre_data = self.scan_results.get("sre_data")
        score, warnings = self.sre_analyzer.calculate_health_score(self.scan_results, sre_data)
        
        self.gauge.set_score(score)
        self.btn_report.setEnabled(True)

        # Update status text
        if score >= 80:
            self.health_status_lbl.setText("Storage health is Healthy. Nominal conditions.")
            self.health_status_lbl.setStyleSheet("color: #10b981;")
        elif score >= 50:
            self.health_status_lbl.setText("Storage health has Warnings. Action recommended.")
            self.health_status_lbl.setStyleSheet("color: #f59e0b;")
        else:
            self.health_status_lbl.setText("Storage health is Critical! Urgent cleanups required.")
            self.health_status_lbl.setStyleSheet("color: #ef4444;")

        # Update Quick Stats
        self.lbl_total_size.setText(f"Scanned Storage: {self.scan_results.get('total_size_formatted', '0 B')}")
        days = self.scan_results.get("days_remaining", -1)
        if days == -1:
            self.lbl_days_remaining.setText("Exhaustion Risk: Stable (No growth trend)")
        else:
            self.lbl_days_remaining.setText(f"Exhaustion Risk: {days} Days remaining")

        dup_waste = 0
        for paths in self.scan_results.get("duplicate_groups", {}).values():
            if len(paths) > 1:
                dup_waste += (len(paths) - 1) * paths[0]["size"]
        self.lbl_duplicate_waste.setText(f"Duplicate Waste: {self.sre_analyzer.format_size(dup_waste)}")

        # 1. Populate Health Warnings
        # Clear old items
        for i in reversed(range(self.warnings_list_layout.count())): 
            self.warnings_list_layout.itemAt(i).widget().setParent(None)

        if not warnings:
            lbl = QLabel("🎉 No warnings detected! Your storage parameters are operating within nominal limits.")
            lbl.setStyleSheet("color: #10b981; font-weight: bold; font-size: 14px; padding: 20px;")
            lbl.setAlignment(Qt.AlignCenter)
            self.warnings_list_layout.addWidget(lbl)
        else:
            for w in warnings:
                card = QFrame()
                card.setStyleSheet("background-color: #1e1b29; border: 1px solid #3b204c; border-radius: 6px; padding: 12px; margin-bottom: 8px;")
                card_layout = QHBoxLayout(card)
                
                icon = QLabel("⚠️")
                icon.setStyleSheet("font-size: 16px;")
                card_layout.addWidget(icon)
                
                txt = QLabel(w)
                txt.setStyleSheet("color: #e2e8f0; font-weight: 500;")
                txt.setWordWrap(True)
                card_layout.addWidget(txt, stretch=1)
                
                self.warnings_list_layout.addWidget(card)

        # 2. Populate Docker Tab
        if sre_data and sre_data.get("docker_active"):
            self.docker_placeholder.setVisible(False)
            self.docker_containers_lbl.setVisible(True)
            self.table_docker_containers.setVisible(True)
            
            containers = sre_data.get("containers", [])
            self.table_docker_containers.setRowCount(0)
            for idx, c in enumerate(containers):
                self.table_docker_containers.insertRow(idx)
                self.table_docker_containers.setItem(idx, 0, QTableWidgetItem(c["id"]))
                self.table_docker_containers.setItem(idx, 1, QTableWidgetItem(c["name"]))
                self.table_docker_containers.setItem(idx, 2, QTableWidgetItem(c["image"]))
                self.table_docker_containers.setItem(idx, 3, QTableWidgetItem(c["write_size_formatted"]))
                self.table_docker_containers.setItem(idx, 4, QTableWidgetItem(c["virtual_size_formatted"]))
                
                # Check for container leak
                if c["write_size"] > 1024 * 1024 * 1024: # >1 GB
                    for col in range(5):
                        item = self.table_docker_containers.item(idx, col)
                        if item:
                            item.setBackground(QColor("#331e24")) # highlight reddish

            volumes = sre_data.get("volumes", [])
            if volumes:
                self.docker_volumes_lbl.setVisible(True)
                self.table_docker_volumes.setVisible(True)
                self.table_docker_volumes.setRowCount(0)
                for idx, v in enumerate(volumes):
                    self.table_docker_volumes.insertRow(idx)
                    self.table_docker_volumes.setItem(idx, 0, QTableWidgetItem(v["name"]))
                    self.table_docker_volumes.setItem(idx, 1, QTableWidgetItem(v["size_formatted"]))
            else:
                self.docker_volumes_lbl.setVisible(False)
                self.table_docker_volumes.setVisible(False)
        else:
            self.docker_placeholder.setVisible(True)
            self.docker_containers_lbl.setVisible(False)
            self.table_docker_containers.setVisible(False)
            self.docker_volumes_lbl.setVisible(False)
            self.table_docker_volumes.setVisible(False)

        # 3. Populate Windows System Tab
        if sre_data and sre_data.get("windows_active"):
            self.win_placeholder.setVisible(False)
            self.table_win_folders.setVisible(True)
            
            folders = sre_data.get("folders", {})
            self.table_win_folders.setRowCount(0)
            
            row_idx = 0
            for key, val in folders.items():
                self.table_win_folders.insertRow(row_idx)
                
                desc = key.upper().replace("_", " ")
                self.table_win_folders.setItem(row_idx, 0, QTableWidgetItem(desc))
                self.table_win_folders.setItem(row_idx, 1, QTableWidgetItem(val["path"]))
                self.table_win_folders.setItem(row_idx, 2, QTableWidgetItem(str(val["count"])))
                self.table_win_folders.setItem(row_idx, 3, QTableWidgetItem(val["size_formatted"]))
                
                # Check for warning threshold (e.g. event log, win temp or dumps size > 500MB)
                size = val.get("size", 0)
                if isinstance(size, int) and size > 500 * 1024 * 1024:
                    for col in range(4):
                        item = self.table_win_folders.item(row_idx, col)
                        if item:
                            item.setBackground(QColor("#331e24"))
                
                row_idx += 1
        else:
            self.win_placeholder.setVisible(True)
            self.table_win_folders.setVisible(False)

    def generate_sre_report(self):
        if not self.scan_results:
            return

        sre_data = self.scan_results.get("sre_data")
        score, warnings = self.sre_analyzer.calculate_health_score(self.scan_results, sre_data)

        # Create markdown report
        import datetime
        timestamp = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        host_label = self.scan_results.get("remote_host", "Local Machine")
        
        md = []
        md.append(f"# SRE Storage Infrastructure & Health Report")
        md.append(f"**Generated on**: {timestamp}")
        md.append(f"**Host / Target**: `{host_label}`")
        md.append(f"**Storage Health Score**: `{score} / 100`\n")
        
        md.append("## 📊 Capacity Metrics")
        md.append(f"- **Total Scanned Size**: {self.scan_results.get('total_size_formatted', '0 B')}")
        md.append(f"- **Total Files Scanned**: {self.scan_results.get('files_scanned', 0)}")
        
        days = self.scan_results.get("days_remaining", -1)
        if days == -1:
            md.append("- **Exhaustion Horizon**: Stable (No daily growth rate detected or stable trend)")
        else:
            md.append(f"- **Exhaustion Horizon**: `{days} Days` remaining at current consumption speed")
            
        md.append("")
        md.append("## ⚠️ Health Violations & SRE Warnings")
        if not warnings:
            md.append("- ✅ No storage health violations. All metrics are normal.")
        else:
            for w in warnings:
                md.append(f"- ⚠️ {w}")
                
        md.append("")

        # Docker Containers
        if sre_data and sre_data.get("docker_active"):
            md.append("## 🐳 DevOps Container Layers Analysis")
            containers = sre_data.get("containers", [])
            if containers:
                md.append("| Container Name | ID | Image | Write Layer Size | Virtual Size |")
                md.append("| --- | --- | --- | --- | --- |")
                for c in containers:
                    md.append(f"| {c['name']} | `{c['id']}` | {c['image']} | {c['write_size_formatted']} | {c['virtual_size_formatted']} |")
            else:
                md.append("*No containers found.*")
                
            volumes = sre_data.get("volumes", [])
            if volumes:
                md.append("\n### Local Docker Volumes")
                md.append("| Volume Name | Size |")
                md.append("| --- | --- |")
                for v in volumes:
                    md.append(f"| `{v['name']}` | {v['size_formatted']} |")
            md.append("")

        # Windows
        if sre_data and sre_data.get("windows_active"):
            md.append("## 🖥️ Windows System Diagnostics")
            md.append("| Description | Target Path | File Count | Total Size |")
            md.append("| --- | --- | --- | --- |")
            folders = sre_data.get("folders", {})
            for key, val in folders.items():
                desc = key.upper().replace("_", " ")
                md.append(f"| {desc} | `{val['path']}` | {val['count']} | {val['size_formatted']} |")
            md.append("")

        # Duplicates
        dup_groups = self.scan_results.get("duplicate_groups", {})
        dup_waste = 0
        dup_count = 0
        for paths in dup_groups.values():
            if len(paths) > 1:
                dup_waste += (len(paths) - 1) * paths[0]["size"]
                dup_count += len(paths) - 1
                
        if dup_waste > 0:
            md.append("## 👥 Duplicate Storage Waste")
            md.append(f"- **Wasted space**: `{self.sre_analyzer.format_size(dup_waste)}` across {dup_count} redundant files.")
            md.append("- **Recommendation**: Trigger deduplication under the main dashboard tab.")
            md.append("")

        md.append("## 🛠️ Infrastructure Recommendations")
        if score < 50:
            md.append("- 🟥 **CRITICAL**: Immediate cleanups needed. Target large event logs, old temporary directories, and clear unrotated Docker containers (usually application stdout logs) with `docker container prune` or standard log rotates.")
        elif score < 80:
            md.append("- 🟨 **WARNING**: Plan disk expansion or setup daily log rotate daemon rules. Deduplicate identical archives to reclaim disk headroom.")
        else:
            md.append("- 🟩 **INFO**: Storage is healthy. No actions are required at this time.")

        report_txt = "\n".join(md)
        
        # Copy to clipboard
        QGuiApplication.clipboard().setText(report_txt)
        
        QMessageBox.information(
            self, "Report Copied",
            "Enterprise SRE Storage Report compiled successfully and copied to your clipboard!"
        )
