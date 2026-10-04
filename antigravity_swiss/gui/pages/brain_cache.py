"""
Brain Cache Manager Page.
========================
Storage and context cache optimizer for:
- ~/.gemini/antigravity/brain/
- ~/.gemini/antigravity/conversations/
Features disk breakdown, prompt bloat analysis, and safe pruning with active session shield.
"""

from __future__ import annotations

from typing import Any, Optional
from PySide6.QtCore import Qt, Signal
from PySide6.QtWidgets import (
    QCheckBox,
    QComboBox,
    QFrame,
    QHBoxLayout,
    QHeaderView,
    QLabel,
    QMessageBox,
    QProgressBar,
    QPushButton,
    QScrollArea,
    QTableWidget,
    QTableWidgetItem,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.cache_optimizer.inspector import CacheInspector
from antigravity_swiss.cache_optimizer.models import CacheBreakdown, PruneOptions
from antigravity_swiss.cache_optimizer.prompt_cache import PromptCacheOptimizer
from antigravity_swiss.cache_optimizer.pruner import CachePruner
from antigravity_swiss.core.constants import (
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_EXHAUSTED,
    MD3_COLOR_HEALTHY,
    MD3_COLOR_WARNING,
    MD3_LIGHT_ACCENT_CONTAINER,
    MD3_LIGHT_ACCENT_PRIMARY,
    MD3_LIGHT_COLOR_EXHAUSTED,
    MD3_LIGHT_COLOR_HEALTHY,
    MD3_LIGHT_COLOR_WARNING,
    MD3_LIGHT_OUTLINE,
    MD3_LIGHT_SURFACE,
    MD3_LIGHT_SURFACE_CONTAINER,
    MD3_LIGHT_SURFACE_CONTAINER_HIGH,
    MD3_LIGHT_TEXT_PRIMARY,
    MD3_LIGHT_TEXT_SECONDARY,
    MD3_OUTLINE,
    MD3_SURFACE_CONTAINER,
    MD3_SURFACE_CONTAINER_HIGH,
    MD3_TEXT_PRIMARY,
    MD3_TEXT_SECONDARY,
)
from antigravity_swiss.ipc.controller import SwissKnifeController


class BrainCachePage(QWidget):
    """
    Brain Cache Manager page with storage breakdown and safe pruning controls.
    """

    cache_cleaned = Signal(int)

    def __init__(
        self,
        controller: SwissKnifeController,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.controller = controller
        self._inspector = CacheInspector()
        self._pruner = CachePruner()
        self._analyzer = PromptCacheOptimizer()
        self._breakdown: Optional[CacheBreakdown] = None

        self._init_ui()

    def _init_ui(self) -> None:
        main_layout = QVBoxLayout(self)
        main_layout.setContentsMargins(24, 16, 24, 24)
        main_layout.setSpacing(16)

        scroll = QScrollArea()
        scroll.setWidgetResizable(True)
        scroll.setFrameShape(QFrame.Shape.NoFrame)
        scroll.setStyleSheet("background: transparent;")

        container = QWidget()
        c_layout = QVBoxLayout(container)
        c_layout.setContentsMargins(0, 0, 0, 0)
        c_layout.setSpacing(20)

        # Header Info Card
        header_card = QFrame()
        header_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
        """)
        h_layout = QHBoxLayout(header_card)
        h_vbox = QVBoxLayout()
        h_vbox.setSpacing(4)

        t1 = QLabel("BRAIN CACHE & CONTEXT OPTIMIZER")
        t1.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        t2 = QLabel("Inspect disk usage in ~/.gemini/antigravity/ and reclaim gigabytes of stale scratch data safely.")
        t2.setStyleSheet(f"font-size: 13px; color: {MD3_LIGHT_TEXT_PRIMARY};")
        h_vbox.addWidget(t1)
        h_vbox.addWidget(t2)
        h_layout.addLayout(h_vbox)

        h_layout.addStretch()

        self._scan_btn = QPushButton("Scan Storage")
        self._scan_btn.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                color: {MD3_LIGHT_TEXT_PRIMARY};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 6px;
                padding: 8px 16px;
                font-size: 12px;
                font-weight: 600;
            }}
            QPushButton:hover {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
            }}
        """)
        self._scan_btn.clicked.connect(self.scan_cache)
        h_layout.addWidget(self._scan_btn)
        c_layout.addWidget(header_card)

        # Storage Overview Statistics Card
        stats_card = QFrame()
        stats_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 20px;
            }}
        """)
        sc_layout = QVBoxLayout(stats_card)
        sc_layout.setSpacing(14)

        sc_title = QLabel("STORAGE BREAKDOWN")
        sc_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        sc_layout.addWidget(sc_title)

        stats_row = QHBoxLayout()
        stats_row.setSpacing(16)

        # 4 Metric Boxes
        self._lbl_total_brain = self._create_stat_box("Brain Data (~/.gemini)", "-- MB", stats_row)
        self._lbl_conversations = self._create_stat_box("Conversation DBs", "-- MB", stats_row)
        self._lbl_reclaimable = self._create_stat_box("Reclaimable Space", "-- MB", stats_row, highlight=True)
        self._lbl_protected = self._create_stat_box("Active Session Shield", "Protected", stats_row)

        sc_layout.addLayout(stats_row)
        c_layout.addWidget(stats_card)

        # Category Table Card
        cat_card = QFrame()
        cat_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
        """)
        cc_layout = QVBoxLayout(cat_card)
        cc_layout.setSpacing(10)

        cc_title = QLabel("CATEGORY USAGE DETAILS")
        cc_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        cc_layout.addWidget(cc_title)

        self._table = QTableWidget(0, 4)
        self._table.setHorizontalHeaderLabels(["Category", "Disk Size", "Files", "Description"])
        self._table.horizontalHeader().setSectionResizeMode(0, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setSectionResizeMode(1, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setSectionResizeMode(2, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setSectionResizeMode(3, QHeaderView.ResizeMode.Stretch)
        self._table.setAlternatingRowColors(True)
        self._table.verticalHeader().setVisible(False)
        self._table.setStyleSheet(f"""
            QTableWidget {{
                background-color: {MD3_LIGHT_SURFACE};
                gridline-color: {MD3_LIGHT_OUTLINE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
            QHeaderView::section {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                color: {MD3_LIGHT_TEXT_SECONDARY};
                padding: 6px;
                font-weight: 600;
                font-size: 11px;
                border: none;
                border-bottom: 1px solid {MD3_LIGHT_OUTLINE};
            }}
        """)
        cc_layout.addWidget(self._table)
        c_layout.addWidget(cat_card)

        # Safe Pruning Actions Card
        prune_card = QFrame()
        prune_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 20px;
            }}
        """)
        pc_layout = QVBoxLayout(prune_card)
        pc_layout.setSpacing(14)

        pc_title = QLabel("SAFE CACHE PRUNING & RECLAIM")
        pc_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        pc_layout.addWidget(pc_title)

        shield_lbl = QLabel("Active Conversation Shield: The currently active cascadeId is permanently protected from pruning.")
        shield_lbl.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_COLOR_HEALTHY}; font-weight: 500;")
        pc_layout.addWidget(shield_lbl)

        # Checkbox Options
        opts_box = QHBoxLayout()
        opts_box.setSpacing(20)

        self._chk_scratch = QCheckBox("Prune Scratchpads (scratch/)")
        self._chk_scratch.setChecked(True)
        self._chk_scratch.setStyleSheet(f"color: {MD3_LIGHT_TEXT_PRIMARY}; font-size: 12px;")
        opts_box.addWidget(self._chk_scratch)

        self._chk_steps = QCheckBox("Prune Execution Steps (steps/)")
        self._chk_steps.setChecked(True)
        self._chk_steps.setStyleSheet(f"color: {MD3_LIGHT_TEXT_PRIMARY}; font-size: 12px;")
        opts_box.addWidget(self._chk_steps)

        self._chk_tasks = QCheckBox("Prune Completed Task Logs (tasks/)")
        self._chk_tasks.setChecked(True)
        self._chk_tasks.setStyleSheet(f"color: {MD3_LIGHT_TEXT_PRIMARY}; font-size: 12px;")
        opts_box.addWidget(self._chk_tasks)

        self._chk_wal = QCheckBox("Vacuum Inactive DBs")
        self._chk_wal.setChecked(False)
        self._chk_wal.setStyleSheet(f"color: {MD3_LIGHT_TEXT_PRIMARY}; font-size: 12px;")
        opts_box.addWidget(self._chk_wal)

        opts_box.addStretch()
        pc_layout.addLayout(opts_box)

        # Age selector and prune button
        action_row = QHBoxLayout()
        action_row.setSpacing(12)

        age_lbl = QLabel("Minimum Inactivity Age:")
        age_lbl.setStyleSheet(f"color: {MD3_LIGHT_TEXT_SECONDARY}; font-size: 12px;")
        action_row.addWidget(age_lbl)

        self._age_combo = QComboBox()
        self._age_combo.addItems(["Older than 1 day", "Older than 3 days", "Older than 7 days", "Older than 14 days"])
        self._age_combo.setCurrentIndex(1)
        self._age_combo.setStyleSheet(f"""
            QComboBox {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                color: {MD3_LIGHT_TEXT_PRIMARY};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 6px;
                padding: 6px 12px;
                font-size: 12px;
            }}
        """)
        action_row.addWidget(self._age_combo)

        action_row.addStretch()

        self._prune_btn = QPushButton("Prune Cache")
        self._prune_btn.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_LIGHT_ACCENT_PRIMARY};
                color: #ffffff;
                border: none;
                border-radius: 6px;
                padding: 10px 22px;
                font-size: 12px;
                font-weight: 600;
            }}
            QPushButton:hover {{
                background-color: #1a73e8;
            }}
        """)
        self._prune_btn.clicked.connect(self._on_prune_clicked)
        action_row.addWidget(self._prune_btn)

        pc_layout.addLayout(action_row)
        c_layout.addWidget(prune_card)

        scroll.setWidget(container)
        main_layout.addWidget(scroll)

        # Initial scan
        self.scan_cache()

    def _create_stat_box(self, label: str, value: str, layout: QHBoxLayout, highlight: bool = False) -> QLabel:
        box = QFrame()
        box.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 12px;
            }}
        """)
        b_layout = QVBoxLayout(box)
        b_layout.setSpacing(4)

        t_lbl = QLabel(label)
        t_lbl.setStyleSheet(f"font-size: 11px; color: {MD3_LIGHT_TEXT_SECONDARY}; font-weight: 500;")
        b_layout.addWidget(t_lbl)

        v_lbl = QLabel(value)
        val_color = MD3_LIGHT_COLOR_HEALTHY if highlight else MD3_LIGHT_TEXT_PRIMARY
        v_lbl.setStyleSheet(f"font-size: 18px; font-weight: 700; color: {val_color};")
        b_layout.addWidget(v_lbl)

        layout.addWidget(box)
        return v_lbl

    def scan_cache(self) -> None:
        """Scan cache filesystem and update breakdown stats."""
        try:
            self._breakdown = self._inspector.scan_breakdown()
            bd = self._breakdown

            brain_mb = bd.brain_total_bytes / (1024 * 1024)
            conv_mb = bd.conversations_total_bytes / (1024 * 1024)
            reclaim_mb = bd.reclaimable_bytes / (1024 * 1024)

            self._lbl_total_brain.setText(f"{brain_mb:.1f} MB")
            self._lbl_conversations.setText(f"{conv_mb:.1f} MB")
            self._lbl_reclaimable.setText(f"{reclaim_mb:.1f} MB")
            self._lbl_protected.setText("Protected (Active)")

            # Populate categories table
            self._table.setRowCount(0)
            for cat_name, cat in bd.categories.items():
                r = self._table.rowCount()
                self._table.insertRow(r)

                size_mb = cat.total_bytes / (1024 * 1024)
                self._table.setItem(r, 0, QTableWidgetItem(cat.category))
                self._table.setItem(r, 1, QTableWidgetItem(f"{size_mb:.2f} MB"))
                self._table.setItem(r, 2, QTableWidgetItem(str(cat.file_count)))
                self._table.setItem(r, 3, QTableWidgetItem(cat.description))
        except Exception as exc:
            self._lbl_reclaimable.setText("Error scanning")

    def _on_prune_clicked(self) -> None:
        if not self._breakdown or self._breakdown.reclaimable_bytes == 0:
            QMessageBox.information(self, "Cache Clean", "No reclaimable stale cache files found.")
            return

        age_map = {0: 1.0, 1: 3.0, 2: 7.0, 3: 14.0}
        min_age = age_map.get(self._age_combo.currentIndex(), 3.0)

        options = PruneOptions(
            prune_scratch=self._chk_scratch.isChecked(),
            prune_steps=self._chk_steps.isChecked(),
            prune_tasks=self._chk_tasks.isChecked(),
            prune_wal=self._chk_wal.isChecked(),
            min_age_days=min_age,
            dry_run=False,
        )

        reply = QMessageBox.question(
            self,
            "Confirm Safe Pruning",
            f"Are you sure you want to prune stale cache files older than {int(min_age)} days?\n"
            "Active session conversations will remain 100% safe and intact.",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
        )

        if reply == QMessageBox.StandardButton.Yes:
            try:
                res = self._pruner.prune(options)
                freed_mb = res.bytes_reclaimed / (1024 * 1024)
                QMessageBox.information(
                    self,
                    "Pruning Complete",
                    f"Successfully reclaimed {freed_mb:.2f} MB across {res.files_deleted} stale files.",
                )
                self.cache_cleaned.emit(res.bytes_reclaimed)
                self.scan_cache()
            except Exception as exc:
                QMessageBox.critical(self, "Prune Error", f"Failed to prune cache: {exc}")
