"""
Quota Dashboard Page (Front Page of Account Switcher).
=====================================================
Multi-account executive quota horizon & fleet management:
1. Top Section:
   - Total managed accounts count and active account status.
   - Aggregate 5-Hour Available Quota progress ring (synthesized from individual reset horizons).
   - Aggregate Weekly Horizon Quota progress ring.
   - Auto-Switch ON/OFF toggle switch button.
2. Bottom Section:
   - Table of all registered accounts.
   - Each account has a row with status, 5-hour quota bar + percentage text,
     weekly quota bar + percentage text, and next reset horizon.
   - Clicking on any row opens the AccountDetailDialog to inspect and edit info.
"""

from __future__ import annotations

import datetime
from typing import Any, List, Optional
from PySide6.QtCore import Qt, Signal
from PySide6.QtWidgets import (
    QComboBox,
    QFrame,
    QHBoxLayout,
    QHeaderView,
    QLabel,
    QPushButton,
    QScrollArea,
    QSizePolicy,
    QTableWidget,
    QTableWidgetItem,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.core.constants import (
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
)
from antigravity_swiss.gui.dialogs.account_detail_dialog import AccountDetailDialog
from antigravity_swiss.gui.widgets.account_quota_bar import AccountQuotaBarWidget
from antigravity_swiss.gui.widgets.circular_gauge import CircularGauge
from antigravity_swiss.ipc.controller import SwissKnifeController
from antigravity_swiss.quota.calculator import (
    AccountQuotaState,
    build_account_quota_states,
    compute_fleet_quota_summary,
)


class QuotaDashboardPage(QWidget):
    """
    Fleet-wide Quota Horizon & Account Management Dashboard.
    """

    account_switched = Signal(str)

    def __init__(
        self,
        controller: SwissKnifeController,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.controller = controller
        self._active_account: Optional[str] = None
        self._accounts_cache: List[AccountQuotaState] = []
        self._auto_switch_enabled: bool = False
        self._gauges: dict[str, CircularGauge] = {}

        self._init_ui()

    def _init_ui(self) -> None:
        main_layout = QVBoxLayout(self)
        main_layout.setContentsMargins(0, 0, 0, 0)
        main_layout.setSpacing(0)

        scroll = QScrollArea(self)
        scroll.setWidgetResizable(True)
        scroll.setFrameShape(QFrame.Shape.NoFrame)
        scroll.setHorizontalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAlwaysOff)
        scroll.setVerticalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAsNeeded)
        scroll.setStyleSheet("QScrollArea { background: transparent; border: none; }")

        container = QWidget()
        c_layout = QVBoxLayout(container)
        c_layout.setContentsMargins(24, 20, 24, 24)
        c_layout.setSpacing(16)

        # ==============================================================
        # 1. TOP SECTION: Fleet Overview, 5h Ring, Weekly Ring, Controls
        # ==============================================================
        top_row = QHBoxLayout()
        top_row.setSpacing(16)

        # Card A: Managed Accounts Fleet Card
        fleet_card = QFrame()
        fleet_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
        """)
        f_layout = QVBoxLayout(fleet_card)
        f_layout.setSpacing(10)

        f_header_row = QHBoxLayout()
        f_title = QLabel("MANAGED ACCOUNTS FLEET")
        f_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 1px;")
        f_header_row.addWidget(f_title)
        f_header_row.addStretch()

        # Auto-Switch Toggle Button
        self.btn_auto_switch = QPushButton("Auto-Switch: OFF")
        self.btn_auto_switch.clicked.connect(self._on_toggle_auto_switch)
        f_header_row.addWidget(self.btn_auto_switch)
        f_layout.addLayout(f_header_row)

        self._total_accounts_lbl = QLabel("0 Accounts Managed")
        self._total_accounts_lbl.setStyleSheet(f"font-size: 24px; font-weight: 700; color: {MD3_LIGHT_TEXT_PRIMARY};")
        f_layout.addWidget(self._total_accounts_lbl)

        self._acc_label = QLabel("Active Account: Not Logged In")
        self._acc_label.setStyleSheet(f"font-size: 13px; font-weight: 600; color: {MD3_LIGHT_ACCENT_PRIMARY};")
        f_layout.addWidget(self._acc_label)

        self._last_poll_label = QLabel("Last Quota Sync: Never")
        self._last_poll_label.setStyleSheet(f"font-size: 11px; color: {MD3_LIGHT_TEXT_SECONDARY};")
        f_layout.addWidget(self._last_poll_label)

        # Quick Switcher & Refresh Row
        switch_row = QHBoxLayout()
        switch_row.setSpacing(8)

        self._account_combo = QComboBox()
        self._account_combo.setMinimumWidth(160)
        self._account_combo.setStyleSheet(f"""
            QComboBox {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                color: {MD3_LIGHT_TEXT_PRIMARY};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 6px 10px;
                font-size: 12px;
            }}
            QComboBox::drop-down {{
                border: none;
            }}
        """)
        switch_row.addWidget(self._account_combo, stretch=1)

        self._switch_btn = QPushButton("Switch")
        self._switch_btn.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_LIGHT_ACCENT_PRIMARY};
                color: #ffffff;
                border: none;
                border-radius: 8px;
                padding: 6px 14px;
                font-size: 12px;
                font-weight: 600;
            }}
            QPushButton:hover {{
                background-color: #1a73e8;
            }}
        """)
        self._switch_btn.clicked.connect(self._on_manual_switch)
        switch_row.addWidget(self._switch_btn)

        self._refresh_btn = QPushButton("Refresh")
        self._refresh_btn.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                color: {MD3_LIGHT_TEXT_PRIMARY};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 6px 12px;
                font-size: 12px;
                font-weight: 500;
            }}
            QPushButton:hover {{
                border-color: {MD3_LIGHT_ACCENT_PRIMARY};
                background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
            }}
        """)
        self._refresh_btn.clicked.connect(self.refresh_quota)
        switch_row.addWidget(self._refresh_btn)

        f_layout.addLayout(switch_row)
        top_row.addWidget(fleet_card, stretch=5)

        # Card B: Next 5 Hours Quota Progress Ring Card
        ring_5h_card = QFrame()
        ring_5h_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
        """)
        r5_layout = QVBoxLayout(ring_5h_card)
        r5_layout.setSpacing(6)
        r5_title = QLabel("NEXT 5 HOURS QUOTA")
        r5_title.setAlignment(Qt.AlignmentFlag.AlignCenter)
        r5_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        r5_layout.addWidget(r5_title)

        self._ring_5h = CircularGauge(
            model_name="5h Available",
            fraction=1.0,
            reset_text="Weighted reset horizon",
            parent=self,
        )
        self._gauges["next_5h"] = self._ring_5h
        r5_layout.addWidget(self._ring_5h, alignment=Qt.AlignmentFlag.AlignCenter)
        top_row.addWidget(ring_5h_card, stretch=3)

        # Card C: Weekly Horizon Quota Progress Ring Card
        ring_wk_card = QFrame()
        ring_wk_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
        """)
        rw_layout = QVBoxLayout(ring_wk_card)
        rw_layout.setSpacing(6)
        rw_title = QLabel("WEEKLY HORIZON QUOTA")
        rw_title.setAlignment(Qt.AlignmentFlag.AlignCenter)
        rw_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        rw_layout.addWidget(rw_title)

        self._ring_weekly = CircularGauge(
            model_name="Weekly Left",
            fraction=1.0,
            reset_text="7-day rolling allowance",
            parent=self,
        )
        self._gauges["weekly"] = self._ring_weekly
        rw_layout.addWidget(self._ring_weekly, alignment=Qt.AlignmentFlag.AlignCenter)
        top_row.addWidget(ring_wk_card, stretch=3)

        c_layout.addLayout(top_row)

        # ==============================================================
        # 2. BOTTOM SECTION: Accounts Inventory Table & Detail Click
        # ==============================================================
        table_card = QFrame()
        table_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
        """)
        t_vbox = QVBoxLayout(table_card)
        t_vbox.setSpacing(10)

        t_header_hbox = QHBoxLayout()
        t_title = QLabel("ALL MANAGED ACCOUNTS (STATUS & QUOTAS)")
        t_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 1px;")
        t_header_hbox.addWidget(t_title)
        t_header_hbox.addStretch()

        hint_lbl = QLabel("Click any row to inspect details, modify credentials, or configure MFA")
        hint_lbl.setStyleSheet(f"font-size: 11px; color: {MD3_LIGHT_TEXT_SECONDARY};")
        t_header_hbox.addWidget(hint_lbl)
        t_vbox.addLayout(t_header_hbox)

        self._table = QTableWidget(0, 6)
        self._table.setFrameShape(QFrame.Shape.NoFrame)
        self._table.setVerticalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAlwaysOff)
        self._table.setHorizontalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAlwaysOff)
        self._table.setSizePolicy(QSizePolicy.Policy.Expanding, QSizePolicy.Policy.Fixed)
        self._table.setHorizontalHeaderLabels([
            "Account Identity",
            "Status",
            "Next 5h Quota",
            "Weekly Quota Left",
            "Reset Horizon",
            "Action",
        ])
        self._table.horizontalHeader().setSectionResizeMode(0, QHeaderView.ResizeMode.Stretch)
        self._table.horizontalHeader().setSectionResizeMode(1, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setSectionResizeMode(2, QHeaderView.ResizeMode.Stretch)
        self._table.horizontalHeader().setSectionResizeMode(3, QHeaderView.ResizeMode.Stretch)
        self._table.horizontalHeader().setSectionResizeMode(4, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setSectionResizeMode(5, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setStretchLastSection(False)
        self._table.horizontalHeader().setMinimumSectionSize(110)
        self._table.horizontalHeader().setDefaultAlignment(Qt.AlignmentFlag.AlignLeft | Qt.AlignmentFlag.AlignVCenter)

        self._table.setAlternatingRowColors(True)
        self._table.verticalHeader().setVisible(False)
        self._table.setSelectionBehavior(QTableWidget.SelectionBehavior.SelectRows)
        self._table.setEditTriggers(QTableWidget.EditTrigger.NoEditTriggers)
        self._table.setStyleSheet(f"""
            QTableWidget {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                gridline-color: {MD3_LIGHT_OUTLINE};
                border: none;
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
            QHeaderView::section {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                color: {MD3_LIGHT_TEXT_SECONDARY};
                padding: 8px;
                font-weight: 600;
                font-size: 11px;
                border: none;
            }}
        """)
        self._table.cellClicked.connect(self._on_table_cell_clicked)
        t_vbox.addWidget(self._table)
        c_layout.addWidget(table_card)

        scroll.setWidget(container)
        main_layout.addWidget(scroll)

        # Initial data load
        self.load_data()

    def _sync_auto_switch_state(self) -> None:
        """Fetches auto-switch state from rule engine and updates UI."""
        try:
            cfg = self.controller.get_rule_config()
            self._auto_switch_enabled = bool(cfg.get("auto_switch_enabled", False))
        except Exception:
            self._auto_switch_enabled = False
        self._update_auto_switch_button_ui()

    def _update_auto_switch_button_ui(self) -> None:
        if self._auto_switch_enabled:
            self.btn_auto_switch.setText("Auto-Switch: ON")
            self.btn_auto_switch.setStyleSheet(f"""
                QPushButton {{
                    background-color: #e6f4ea;
                    color: {MD3_LIGHT_COLOR_HEALTHY};
                    border: 1px solid #b7e1cd;
                    border-radius: 14px;
                    padding: 5px 14px;
                    font-size: 11px;
                    font-weight: 700;
                }}
                QPushButton:hover {{
                    background-color: #ceead6;
                }}
            """)
        else:
            self.btn_auto_switch.setText("Auto-Switch: OFF")
            self.btn_auto_switch.setStyleSheet(f"""
                QPushButton {{
                    background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                    color: {MD3_LIGHT_TEXT_SECONDARY};
                    border: 1px solid {MD3_LIGHT_OUTLINE};
                    border-radius: 14px;
                    padding: 5px 14px;
                    font-size: 11px;
                    font-weight: 600;
                }}
                QPushButton:hover {{
                    border-color: {MD3_LIGHT_ACCENT_PRIMARY};
                    color: {MD3_LIGHT_TEXT_PRIMARY};
                }}
            """)

    def _on_toggle_auto_switch(self) -> None:
        new_val = not self._auto_switch_enabled
        try:
            self.controller.set_rule_config(auto_switch_enabled=new_val)
            self._auto_switch_enabled = new_val
            self._update_auto_switch_button_ui()
        except Exception as exc:
            self._last_poll_label.setText(f"Auto-switch error: {exc}")

    def load_data(self) -> None:
        """Fetch current status and accounts from controller."""
        try:
            status = self.controller.get_status()
            self._active_account = status.get("active_account")
            if self._active_account:
                self._acc_label.setText(f"Active Account: {self._active_account}")
            else:
                self._acc_label.setText("Active Account: Not Logged In")

            self._sync_auto_switch_state()
            self.refresh_quota()
        except Exception as exc:
            self._last_poll_label.setText(f"Sync error: {exc}")

    def refresh_quota(self) -> None:
        """Calculates fleet quota metrics and repopulates the accounts table."""
        try:
            raw_accounts = self.controller.list_accounts()
            active_summary = None
            try:
                active_summary = self.controller.get_quota_summary(self._active_account)
            except Exception:
                pass

            self._accounts_cache = build_account_quota_states(raw_accounts, active_summary)
            now_str = datetime.datetime.now().strftime("%H:%M:%S")
            self._last_poll_label.setText(f"Last Quota Sync: {now_str}")

            # 1. Update Top Section Metrics
            fleet_5h, fleet_weekly, total_count = compute_fleet_quota_summary(self._accounts_cache)
            self._total_accounts_lbl.setText(f"{total_count} Account{'s' if total_count != 1 else ''} Managed")

            self._ring_5h.fraction = fleet_5h
            self._ring_5h.reset_text = f"Synthesized from {total_count} accounts"

            self._ring_weekly.fraction = fleet_weekly
            self._ring_weekly.reset_text = "7-day rolling allowance"

            # Update Dropdown
            self._account_combo.clear()
            for acc in self._accounts_cache:
                self._account_combo.addItem(acc.email)
            if self._active_account:
                idx = self._account_combo.findText(self._active_account)
                if idx >= 0:
                    self._account_combo.setCurrentIndex(idx)

            # 2. Populate Bottom Section Accounts Table
            self._table.setRowCount(0)
            for r, acc in enumerate(self._accounts_cache):
                self._table.insertRow(r)

                # Col 0: Identity (Email & Label)
                display_label = f"{acc.email}\n({acc.label})" if acc.label and acc.label != acc.email else acc.email
                item_name = QTableWidgetItem(display_label)
                item_name.setFont(self.font())
                self._table.setItem(r, 0, item_name)

                # Col 1: Status Pill
                status_text = "ACTIVE" if acc.is_active else acc.status
                item_status = QTableWidgetItem(status_text)
                if acc.is_active:
                    item_status.setForeground(Qt.GlobalColor.darkGreen)
                self._table.setItem(r, 1, item_status)

                # Col 2: 5h Available Quota (Horizontal Bar + Text %)
                bar_5h_widget = AccountQuotaBarWidget(acc.quota_5h_available)
                self._table.setCellWidget(r, 2, bar_5h_widget)

                # Col 3: Weekly Quota Left (Horizontal Bar + Text %)
                bar_wk_widget = AccountQuotaBarWidget(acc.quota_weekly)
                self._table.setCellWidget(r, 3, bar_wk_widget)

                # Col 4: Reset Horizon Text
                item_reset = QTableWidgetItem(acc.reset_horizon_text)
                self._table.setItem(r, 4, item_reset)

                # Col 5: Manage Button
                btn_manage = QPushButton("Edit Details")
                btn_manage.setStyleSheet(f"""
                    QPushButton {{
                        background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                        color: {MD3_LIGHT_ACCENT_PRIMARY};
                        border: 1px solid {MD3_LIGHT_OUTLINE};
                        border-radius: 6px;
                        padding: 4px 10px;
                        font-size: 11px;
                        font-weight: 600;
                    }}
                    QPushButton:hover {{
                        background-color: {MD3_LIGHT_ACCENT_CONTAINER};
                    }}
                """)
                # Capture row index
                row_idx = r
                btn_manage.clicked.connect(lambda checked=False, idx=row_idx: self._open_account_detail(idx))
                self._table.setCellWidget(r, 5, btn_manage)
                self._table.setRowHeight(r, 48)

            self._adjust_table_height()

        except Exception as exc:
            self._last_poll_label.setText(f"Quota fetch failed: {exc}")

    def _adjust_table_height(self) -> None:
        """Dynamically resizes table to fit all rows, preventing internal table scrollbars."""
        self._table.doItemsLayout()
        header_h = self._table.horizontalHeader().height()
        if header_h <= 0:
            header_h = self._table.horizontalHeader().sizeHint().height() or 38
        total_rows_h = sum(self._table.rowHeight(r) for r in range(self._table.rowCount()))
        total_h = header_h + total_rows_h + 6
        self._table.setFixedHeight(max(80, total_h))
        self._table.updateGeometry()

    def _on_table_cell_clicked(self, row: int, col: int) -> None:
        """Clicking any cell opens the pop-up detail window."""
        self._open_account_detail(row)

    def _open_account_detail(self, row: int) -> None:
        """Opens AccountDetailDialog for the selected account row."""
        if 0 <= row < len(self._accounts_cache):
            acc_state = self._accounts_cache[row]
            dialog = AccountDetailDialog(account=acc_state, controller=self.controller, parent=self)
            dialog.account_saved.connect(self._on_account_detail_changed)
            dialog.account_removed.connect(self._on_account_detail_changed)
            dialog.exec()

    def _on_account_detail_changed(self, email: str) -> None:
        self.load_data()
        self.account_switched.emit(email)

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
