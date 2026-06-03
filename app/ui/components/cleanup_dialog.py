import os
from PySide6.QtWidgets import (
    QDialog, QVBoxLayout, QHBoxLayout, QLabel, 
    QPushButton, QCheckBox, QListWidget, QFrame, 
    QProgressBar, QMessageBox
)
from PySide6.QtCore import Qt, Signal, Slot, QThread
from PySide6.QtGui import QFont

from app.modules.cleanup_advisor import CleanupAdvisor
from app.core.config import logger

class DeletionWorker(QThread):
    progress = Signal(int, str) # (current_index, current_file)
    finished = Signal(int, int, list) # (deleted_count, size_freed, failed_paths)

    def __init__(self, advisor: CleanupAdvisor, paths: list[str], use_recycle_bin: bool):
        super().__init__()
        self.advisor = advisor
        self.paths = paths
        self.use_recycle_bin = use_recycle_bin

    def run(self):
        deleted_count = 0
        size_freed = 0
        failed_paths = []
        
        # We perform safe_delete in chunks to update progress in UI
        for idx, path in enumerate(self.paths):
            self.progress.emit(idx, path)
            cnt, freed, failed = self.advisor.safe_delete([path], self.use_recycle_bin)
            deleted_count += cnt
            size_freed += freed
            failed_paths.extend(failed)
            
        self.finished.emit(deleted_count, size_freed, failed_paths)


class CleanupDialog(QDialog):
    cleanup_completed = Signal(int, int) # (files_deleted, bytes_freed)

    def __init__(self, profile_id: int, file_paths: list[str], parent=None):
        super().__init__(parent)
        self.advisor = CleanupAdvisor(profile_id)
        self.file_paths = file_paths
        
        self.setWindowTitle("Confirm Disk Cleanup")
        self.setMinimumSize(600, 450)
        self.setStyleSheet("""
            QDialog {
                background-color: #0d0e12;
            }
            QLabel {
                color: #e2e8f0;
            }
            QPushButton {
                font-weight: 600;
            }
        """)
        
        # Calculate dry run immediately
        self.report = self.advisor.dry_run(self.file_paths)
        self.init_ui()

    def init_ui(self):
        layout = QVBoxLayout(self)
        layout.setContentsMargins(20, 20, 20, 20)
        layout.setSpacing(15)

        # 1. Summary Label
        summary_lbl = QLabel("Cleanup Dry Run Summary")
        summary_lbl.setFont(QFont("Segoe UI", 14, QFont.Bold))
        summary_lbl.setStyleSheet("color: #7c4dff;")
        layout.addWidget(summary_lbl)

        # Info Box Card
        info_card = QFrame()
        info_card.setObjectName("InfoCard")
        info_card.setStyleSheet("""
            QFrame#InfoCard {
                background-color: #13151a;
                border: 1px solid #1f232b;
                border-radius: 8px;
            }
        """)
        info_layout = QVBoxLayout(info_card)
        info_layout.setContentsMargins(15, 15, 15, 15)
        info_layout.setSpacing(6)

        stats_lbl = QLabel(
            f"• Files to delete: {len(self.report['writable_files'])}\n"
            f"• Estimated Space to Free: {self.report['total_size_formatted']}"
        )
        stats_lbl.setFont(QFont("Segoe UI", 11))
        info_layout.addWidget(stats_lbl)

        if self.report["restricted_files"]:
            warn_lbl = QLabel(
                f"⚠️ Note: {len(self.report['restricted_files'])} file(s) are read-only or locked "
                f"and cannot be cleaned."
            )
            warn_lbl.setStyleSheet("color: #f59e0b; font-size: 11px;")
            info_layout.addWidget(warn_lbl)

        layout.addWidget(info_card)

        # 2. Deletions List View
        list_lbl = QLabel("Files selected for deletion:")
        list_lbl.setStyleSheet("font-weight: bold; color: #94a3b8;")
        layout.addWidget(list_lbl)

        self.list_files = QListWidget()
        self.list_files.setStyleSheet("""
            QListWidget {
                background-color: #13151a;
                border: 1px solid #1f232b;
                border-radius: 6px;
                padding: 5px;
                color: #e2e8f0;
            }
        """)
        for item, size in self.report["writable_files"]:
            self.list_files.addItem(f"{item} ({self.advisor.format_size(size)})")
        layout.addWidget(self.list_files)

        # 3. Settings / Options
        self.chk_recycle = QCheckBox("Send files to Recycle Bin / Trash (Recommended)")
        self.chk_recycle.setChecked(True)
        self.chk_recycle.setStyleSheet("color: #e2e8f0; font-weight: 500;")
        self.chk_recycle.stateChanged.connect(self.on_recycle_state_changed)
        layout.addWidget(self.chk_recycle)

        # 4. Progress Layout (hidden by default)
        self.progress_frame = QFrame()
        self.progress_frame.setVisible(False)
        prog_layout = QVBoxLayout(self.progress_frame)
        prog_layout.setContentsMargins(0, 5, 0, 5)
        
        self.progress_lbl = QLabel("Deleting file 0 of 0...")
        prog_layout.addWidget(self.progress_lbl)
        
        self.progress_bar = QProgressBar()
        self.progress_bar.setStyleSheet("""
            QProgressBar {
                background-color: #13151a;
                border: 1px solid #2d3343;
                border-radius: 4px;
                height: 10px;
                text-align: center;
            }
            QProgressBar::chunk {
                background-color: #ef4444;
                border-radius: 4px;
            }
        """)
        prog_layout.addWidget(self.progress_bar)
        layout.addWidget(self.progress_frame)

        # 5. Buttons Action Panel
        btn_layout = QHBoxLayout()
        btn_layout.addStretch()

        self.btn_cancel = QPushButton("Cancel")
        self.btn_cancel.setProperty("class", "SecondaryBtn")
        self.btn_cancel.clicked.connect(self.reject)
        self.btn_cancel.setFixedSize(100, 35)
        btn_layout.addWidget(self.btn_cancel)

        self.btn_confirm = QPushButton("Confirm Clean")
        self.btn_confirm.setProperty("class", "PrimaryBtn")
        self.btn_confirm.setStyleSheet("background-color: #ef4444;") # Red delete button
        self.btn_confirm.clicked.connect(self.start_cleanup)
        self.btn_confirm.setFixedSize(140, 35)
        self.btn_confirm.setEnabled(self.report["can_proceed"])
        btn_layout.addWidget(self.btn_confirm)

        layout.addLayout(btn_layout)

    def on_recycle_state_changed(self, state):
        if not self.chk_recycle.isChecked():
            res = QMessageBox.warning(
                self, "Warning: Permanent Deletion",
                "Unchecking this option will permanently delete these files from your storage.\n"
                "Are you sure you want to proceed with permanent deletion?",
                QMessageBox.Yes | QMessageBox.No,
                QMessageBox.No
            )
            if res == QMessageBox.No:
                self.chk_recycle.setChecked(True)

    def start_cleanup(self):
        # Disable controls
        self.btn_confirm.setEnabled(False)
        self.btn_cancel.setEnabled(False)
        self.chk_recycle.setEnabled(False)
        self.list_files.setEnabled(False)
        
        # Enable progress views
        self.progress_frame.setVisible(True)
        self.progress_bar.setRange(0, len(self.report["writable_files"]))
        self.progress_bar.setValue(0)
        
        # Prepare list of writable file paths to delete
        paths_to_delete = [item[0] for item in self.report["writable_files"]]
        
        # Start deletion thread worker
        self.worker = DeletionWorker(
            self.advisor, paths_to_delete, self.chk_recycle.isChecked()
        )
        self.worker.progress.connect(self.on_deletion_progress)
        self.worker.finished.connect(self.on_deletion_finished)
        self.worker.start()

    @Slot(int, str)
    def on_deletion_progress(self, index: int, current_file: str):
        self.progress_lbl.setText(f"Deleting file {index + 1} of {len(self.report['writable_files'])}: {os.path.basename(current_file)}")
        self.progress_bar.setValue(index + 1)

    @Slot(int, int, list)
    def on_deletion_finished(self, deleted_count: int, size_freed: int, failed_paths: list):
        self.progress_frame.setVisible(False)
        
        formatted_freed = self.advisor.format_size(size_freed)
        if failed_paths:
            msg = (
                f"Cleanup completed with warnings:\n"
                f"• Cleaned: {deleted_count} files ({formatted_freed} freed).\n"
                f"• Failed to clean: {len(failed_paths)} files due to locked access."
            )
            QMessageBox.warning(self, "Cleanup Warnings", msg)
        else:
            msg = f"Cleanup completed successfully!\nCleared {deleted_count} files ({formatted_freed} disk space freed)."
            QMessageBox.information(self, "Cleanup Completed", msg)

        # Notify parent dashboard to update its results lists
        self.cleanup_completed.emit(deleted_count, size_freed)
        self.accept()
