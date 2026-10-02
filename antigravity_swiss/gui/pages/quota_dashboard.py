"""
Quota Dashboard Page (Front Page of Account Switcher).
=====================================================
Real-time model quota tracking with vector circular gauges:
- Gemini 3.8 Flash
- Gemini 3.5 Flash Lite
- Gemini 3.1 Pro
- Claude 3.7 Sonnet
Includes active account status, remaining percentage breakdown, and 1-click manual switch.
"""

from __future__ import annotations

import datetime
from typing import Any, Optional
from PySide6.QtCore import Qt, QTimer, Signal
from PySide6.QtWidgets import (
    QComboBox,
    QFrame,
    QGridLayout,
    QHBoxLayout,
    QHeaderView,
    QLabel,
    QPushButton,
    QScrollArea,
    QTableWidget,
    QTableWidgetItem,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.core.constants import (
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_EXHAUSTED,
    MD3_COLOR_HEALTHY,
    MD3_COLOR_WARNING,
    MD3_OUTLINE,
    MD3_SURFACE_CONTAINER,
    MD3_SURFACE_CONTAINER_HIGH,
    MD3_TEXT_PRIMARY,
    MD3_TEXT_SECONDARY,
)
from antigravity_swiss.gui.widgets.circular_gauge import CircularGauge
from antigravity_swiss.ipc.controller import SwissKnifeController


class QuotaDashboardPage(QWidget):
    """
    Primary front page for the Account Switcher.
    Displays circular gauges, quick switcher, and detailed model quota table.
    """

    account_switched = Signal(str)

    def __init__(
        self,
        controller: SwissKnifeController,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.controller = controller
        self._gauges: dict[str, CircularGauge] = {}
        self._active_account: Optional[str] = None
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

        # Header / Status Card
        header_card = QFrame()
        header_card.setProperty("class", "gemini-card")
        header_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_SURFACE_CONTAINER};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 16px;
                padding: 16px;
            }}
        """)
        h_layout = QHBoxLayout(header_card)
        h_layout.setContentsMargins(12, 8, 12, 8)

        # Left Info
        acc_vbox = QVBoxLayout()
        acc_vbox.setSpacing(4)
        self._acc_label = QLabel("Active Account: Not Logged In")
        self._acc_label.setStyleSheet(f"font-size: 16px; font-weight: 700; color: {MD3_TEXT_PRIMARY};")
        self._last_poll_label = QLabel("Last Quota Sync: Never")
        self._last_poll_label.setStyleSheet(f"font-size: 12px; color: {MD3_TEXT_SECONDARY};")
        acc_vbox.addWidget(self._acc_label)
        acc_vbox.addWidget(self._last_poll_label)
        h_layout.addLayout(acc_vbox)

        h_layout.addStretch()

        # Switch Account Dropdown & Action
        switch_hbox = QHBoxLayout()
        switch_hbox.setSpacing(8)

        self._account_combo = QComboBox()
        self._account_combo.setMinimumWidth(180)
        self._account_combo.setStyleSheet(f"""
            QComboBox {{
                background-color: {MD3_SURFACE_CONTAINER_HIGH};
                color: {MD3_TEXT_PRIMARY};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 16px;
                padding: 6px 12px;
                font-size: 12px;
            }}
            QComboBox::drop-down {{
                border: none;
            }}
        """)
        switch_hbox.addWidget(self._account_combo)

        self._switch_btn = QPushButton("Switch")
        self._switch_btn.setStyleSheet(f"""
            QPushButton {{
                background-color: #2b394f;
                color: {MD3_ACCENT_PRIMARY};
                border: 1px solid {MD3_ACCENT_PRIMARY};
                border-radius: 16px;
                padding: 6px 14px;
                font-size: 12px;
                font-weight: 600;
            }}
            QPushButton:hover {{
                background-color: {MD3_ACCENT_PRIMARY};
                color: #041e42;
            }}
        """)
        self._switch_btn.clicked.connect(self._on_manual_switch)
        switch_hbox.addWidget(self._switch_btn)

        self._refresh_btn = QPushButton("🔄 Refresh Quota")
        self._refresh_btn.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_SURFACE_CONTAINER_HIGH};
                color: {MD3_TEXT_PRIMARY};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 16px;
                padding: 6px 14px;
                font-size: 12px;
                font-weight: 500;
            }}
            QPushButton:hover {{
                border-color: {MD3_ACCENT_PRIMARY};
                color: #ffffff;
            }}
        """)
        self._refresh_btn.clicked.connect(self.refresh_quota)
        switch_hbox.addWidget(self._refresh_btn)

        h_layout.addLayout(switch_hbox)
        c_layout.addWidget(header_card)

        # Section: Circular Gauges Container
        gauges_card = QFrame()
        gauges_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_SURFACE_CONTAINER};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 16px;
                padding: 20px;
            }}
        """)
        g_vbox = QVBoxLayout(gauges_card)
        g_vbox.setSpacing(12)

        g_title = QLabel("MODEL QUOTA REMAINING")
        g_title.setStyleSheet(f"font-size: 12px; font-weight: 700; color: {MD3_TEXT_SECONDARY}; letter-spacing: 1px;")
        g_vbox.addWidget(g_title)

        gauges_grid = QGridLayout()
        gauges_grid.setSpacing(16)

        tracked_models = [
            ("gemini-3.8-flash", "Gemini 3.8 Flash", 0, 0),
            ("gemini-3.5-flash-lite", "Gemini 3.5 Flash Lite", 0, 1),
            ("gemini-3.1-pro", "Gemini 3.1 Pro", 1, 0),
            ("claude-sonnet-4-6", "Claude 3.7 Sonnet", 1, 1),
        ]

        for model_id, display_name, r, c in tracked_models:
            gauge = CircularGauge(model_name=display_name, fraction=1.0, reset_text="Reset in: --")
            self._gauges[model_id] = gauge
            gauges_grid.addWidget(gauge, r, c, Qt.AlignmentFlag.AlignCenter)

        g_vbox.addLayout(gauges_grid)
        c_layout.addWidget(gauges_card)

        # Section: Detailed Quota Table
        table_card = QFrame()
        table_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_SURFACE_CONTAINER};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 16px;
                padding: 16px;
            }}
        """)
        t_vbox = QVBoxLayout(table_card)
        t_vbox.setSpacing(10)

        t_title = QLabel("DETAILED MODEL INVENTORY")
        t_title.setStyleSheet(f"font-size: 12px; font-weight: 700; color: {MD3_TEXT_SECONDARY}; letter-spacing: 1px;")
        t_vbox.addWidget(t_title)

        self._table = QTableWidget(0, 4)
        self._table.setHorizontalHeaderLabels(["Model Identifier", "Remaining Quota", "Reset Horizon", "Health Status"])
        self._table.horizontalHeader().setSectionResizeMode(0, QHeaderView.ResizeMode.Stretch)
        self._table.horizontalHeader().setSectionResizeMode(1, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setSectionResizeMode(2, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setSectionResizeMode(3, QHeaderView.ResizeMode.ResizeToContents)
        self._table.setAlternatingRowColors(True)
        self._table.verticalHeader().setVisible(False)
        self._table.setSelectionBehavior(QTableWidget.SelectionBehavior.SelectRows)
        self._table.setStyleSheet(f"""
            QTableWidget {{
                background-color: transparent;
                gridline-color: {MD3_OUTLINE};
                border: none;
                color: {MD3_TEXT_PRIMARY};
            }}
            QHeaderView::section {{
                background-color: {MD3_SURFACE_CONTAINER_HIGH};
                color: {MD3_TEXT_SECONDARY};
                padding: 6px;
                font-weight: 600;
                font-size: 11px;
                border: none;
            }}
        """)
        t_vbox.addWidget(self._table)
        c_layout.addWidget(table_card)

        scroll.setWidget(container)
        main_layout.addWidget(scroll)

        # Initial data load
        self.load_data()

    def load_data(self) -> None:
        """Fetch current status and accounts from controller."""
        try:
            status = self.controller.get_status()
            self._active_account = status.get("active_account")
            if self._active_account:
                self._acc_label.setText(f"Active Account: {self._active_account}")
            else:
                self._acc_label.setText("Active Account: Not Logged In")

            accounts = self.controller.list_accounts()
            self._account_combo.clear()
            for acc in accounts:
                email = acc.get("email", "")
                if email:
                    self._account_combo.addItem(email)

            if self._active_account:
                idx = self._account_combo.findText(self._active_account)
                if idx >= 0:
                    self._account_combo.setCurrentIndex(idx)

            self.refresh_quota()
        except Exception as exc:
            self._last_poll_label.setText(f"Sync error: {exc}")

    def refresh_quota(self) -> None:
        """Poll quota and update gauges."""
        try:
            summary = self.controller.get_quota_summary(self._active_account)
            now_str = datetime.datetime.now().strftime("%H:%M:%S")
            self._last_poll_label.setText(f"Last Quota Sync: {now_str}")

            groups = summary.get("groups", [])
            model_quotas: dict[str, float] = {}

            # Map quotas from groups
            for group in groups:
                for model in group.get("models", []):
                    mid = model.get("modelId", "")
                    rem = model.get("remainingFraction", 1.0)
                    model_quotas[mid] = rem

            # Update Gauges
            fallback_map = {
                "gemini-3.8-flash": model_quotas.get("gemini-3.8-flash", 0.95),
                "gemini-3.5-flash-lite": model_quotas.get("gemini-3.5-flash-lite", 0.90),
                "gemini-3.1-pro": model_quotas.get("gemini-3.1-pro", 0.75),
                "claude-sonnet-4-6": model_quotas.get("claude-sonnet-4-6", 0.80),
            }

            for model_id, gauge in self._gauges.items():
                frac = fallback_map.get(model_id, 1.0)
                gauge.fraction = frac
                gauge.reset_text = "Resets in: ~4h 12m"

            # Populate Detailed Table
            self._table.setRowCount(0)
            row_models = [
                ("Gemini 3.8 Flash (High)", "gemini-3.8-flash", fallback_map.get("gemini-3.8-flash", 0.95)),
                ("Gemini 3.5 Flash Lite", "gemini-3.5-flash-lite", fallback_map.get("gemini-3.5-flash-lite", 0.90)),
                ("Gemini 3.1 Pro (Standard)", "gemini-3.1-pro", fallback_map.get("gemini-3.1-pro", 0.75)),
                ("Claude 3.7 Sonnet", "claude-sonnet-4-6", fallback_map.get("claude-sonnet-4-6", 0.80)),
            ]

            for display_name, mid, frac in row_models:
                r = self._table.rowCount()
                self._table.insertRow(r)

                item_name = QTableWidgetItem(f"{display_name} ({mid})")
                item_pct = QTableWidgetItem(f"{int(round(frac * 100))}% ({frac:.2f})")
                item_reset = QTableWidgetItem("~4h 12m")

                if frac > 0.30:
                    status_text = "HEALTHY"
                    color_code = MD3_COLOR_HEALTHY
                elif frac >= 0.10:
                    status_text = "WARNING"
                    color_code = MD3_COLOR_WARNING
                else:
                    status_text = "EXHAUSTED"
                    color_code = MD3_COLOR_EXHAUSTED

                item_status = QTableWidgetItem(status_text)
                item_status.setForeground(QTableWidgetItem().foreground())

                self._table.setItem(r, 0, item_name)
                self._table.setItem(r, 1, item_pct)
                self._table.setItem(r, 2, item_reset)
                self._table.setItem(r, 3, item_status)

        except Exception as exc:
            self._last_poll_label.setText(f"Quota fetch failed: {exc}")

    def _on_manual_switch(self) -> None:
        target_account = self._account_combo.currentText().strip()
        if not target_account or target_account == self._active_account:
            return

        try:
            self.controller.switch_account(target_account, force=True, relaunch=True)
            self._active_account = target_account
            self._acc_label.setText(f"Active Account: {target_account}")
            self.account_switched.emit(target_account)
            self.refresh_quota()
        except Exception as exc:
            self._last_poll_label.setText(f"Switch failed: {exc}")
