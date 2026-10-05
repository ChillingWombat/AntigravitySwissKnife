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
        fleet_card.setObjectName("fleetCard")
        fleet_card.setStyleSheet("""
            QFrame#fleetCard {
                background-color: #ffffff;
                border: 1px solid #e2e8f0;
                border-radius: 12px;
            }
            QFrame#fleetCard QLabel {
                background: transparent;
                border: none;
                padding: 0px;
            }
        """)
        f_layout = QVBoxLayout(fleet_card)
        f_layout.setContentsMargins(20, 20, 20, 20)
        f_layout.setSpacing(8)

        f_header_row = QHBoxLayout()
        f_title = QLabel("MANAGED ACCOUNTS FLEET")
        f_title.setStyleSheet("font-size: 11px; font-weight: 700; color: #64748b; letter-spacing: 0.8px;")
        f_header_row.addWidget(f_title)
        f_header_row.addStretch()

        # Auto-Switch Toggle Button
        self.btn_auto_switch = QPushButton("Auto-Switch: OFF")
        self.btn_auto_switch.clicked.connect(self._on_toggle_auto_switch)
        f_header_row.addWidget(self.btn_auto_switch)
        f_layout.addLayout(f_header_row)

        self._total_accounts_lbl = QLabel("0 Accounts Managed")
        self._total_accounts_lbl.setStyleSheet("font-size: 26px; font-weight: 700; color: #1e293b; margin-top: 4px;")
        f_layout.addWidget(self._total_accounts_lbl)

        self._acc_label = QLabel("● Active: Not Logged In")
        self._acc_label.setStyleSheet("font-size: 13px; font-weight: 600; color: #0b57d0;")
        f_layout.addWidget(self._acc_label)

        self._last_poll_label = QLabel("Last Quota Sync: Never")
        self._last_poll_label.setStyleSheet("font-size: 11px; color: #64748b;")
        f_layout.addWidget(self._last_poll_label)

        f_layout.addStretch()

        # Quick Switcher & Refresh Row
        switch_row = QHBoxLayout()
        switch_row.setSpacing(8)

        self._account_combo = QComboBox()
        self._account_combo.setMinimumWidth(180)
        self._account_combo.setStyleSheet("""
            QComboBox {
                background-color: #f8fafd;
                color: #1e293b;
                border: 1px solid #d3dbe5;
                border-radius: 8px;
                padding: 6px 12px;
                font-size: 12px;
            }
            QComboBox::drop-down {
                border: none;
            }
            QComboBox QAbstractItemView {
                background-color: #ffffff;
                color: #1e293b;
                border: 1px solid #d3dbe5;
                border-radius: 8px;
                selection-background-color: #e8f0fe;
                selection-color: #0b57d0;
            }
        """)
        switch_row.addWidget(self._account_combo, stretch=1)

        self._switch_btn = QPushButton("⇄ Switch")
        self._switch_btn.setStyleSheet("""
            QPushButton {
                background-color: #1a73e8;
                color: #ffffff;
                border: none;
                border-radius: 8px;
                padding: 7px 16px;
                font-size: 12px;
                font-weight: 600;
            }
            QPushButton:hover {
                background-color: #1557b0;
            }
        """)
        self._switch_btn.clicked.connect(self._on_manual_switch)
        switch_row.addWidget(self._switch_btn)

        self._refresh_btn = QPushButton("↻")
        self._refresh_btn.setStyleSheet("""
            QPushButton {
                background-color: #ffffff;
                color: #475569;
                border: 1px solid #d3dbe5;
                border-radius: 8px;
                padding: 7px 12px;
                font-size: 14px;
                font-weight: 600;
            }
            QPushButton:hover {
                border-color: #1a73e8;
                background-color: #f8fafd;
            }
        """)
        self._refresh_btn.clicked.connect(self.refresh_quota)
        switch_row.addWidget(self._refresh_btn)

        f_layout.addLayout(switch_row)
        top_row.addWidget(fleet_card, stretch=1)

        # Card B: Merged Total Quota Progress Rings Card
        total_quota_card = QFrame()
        total_quota_card.setObjectName("totalQuotaCard")
        total_quota_card.setStyleSheet("""
            QFrame#totalQuotaCard {
                background-color: #ffffff;
                border: 1px solid #e2e8f0;
                border-radius: 12px;
            }
            QFrame#totalQuotaCard QLabel {
                background: transparent;
                border: none;
                padding: 0px;
            }
        """)
        tq_layout = QVBoxLayout(total_quota_card)
        tq_layout.setContentsMargins(20, 20, 20, 20)
        tq_layout.setSpacing(8)

        tq_title = QLabel("TOTAL QUOTA")
        tq_title.setAlignment(Qt.AlignmentFlag.AlignLeft | Qt.AlignmentFlag.AlignVCenter)
        tq_title.setStyleSheet("font-size: 11px; font-weight: 700; color: #64748b; letter-spacing: 0.8px;")
        tq_layout.addWidget(tq_title)

        rings_row = QHBoxLayout()
        rings_row.setSpacing(24)

        self._ring_5h = CircularGauge(
            model_name="5H",
            fraction=1.0,
            reset_text="",
            parent=self,
        )
        self._gauges["next_5h"] = self._ring_5h
        rings_row.addWidget(self._ring_5h, alignment=Qt.AlignmentFlag.AlignCenter)

        self._ring_weekly = CircularGauge(
            model_name="Weekly",
            fraction=1.0,
            reset_text="",
            parent=self,
        )
        self._gauges["weekly"] = self._ring_weekly
        rings_row.addWidget(self._ring_weekly, alignment=Qt.AlignmentFlag.AlignCenter)

        tq_layout.addLayout(rings_row)
        top_row.addWidget(total_quota_card, stretch=1)

        c_layout.addLayout(top_row)

        # ==============================================================
        # 2. BOTTOM SECTION: Accounts Inventory Table & Detail Click
        # ==============================================================
        table_card = QFrame()
        table_card.setObjectName("tableCard")
        table_card.setStyleSheet("""
            QFrame#tableCard {
                background-color: #ffffff;
                border: 1px solid #e2e8f0;
                border-radius: 12px;
            }
            QFrame#tableCard QLabel {
                background: transparent;
                border: none;
                padding: 0px;
            }
        """)
        t_vbox = QVBoxLayout(table_card)
        t_vbox.setContentsMargins(0, 0, 0, 0)
        t_vbox.setSpacing(0)

        header_container = QWidget()
        header_container.setStyleSheet("background: transparent; border-bottom: 1px solid #e2e8f0;")
        t_header_hbox = QHBoxLayout(header_container)
        t_header_hbox.setContentsMargins(20, 16, 20, 14)

        t_title = QLabel("ALL MANAGED ACCOUNTS (STATUS & QUOTAS)")
        t_title.setStyleSheet("font-size: 11px; font-weight: 700; color: #64748b; letter-spacing: 0.8px;")
        t_header_hbox.addWidget(t_title)
        t_header_hbox.addStretch()

        hint_lbl = QLabel("Click any row to inspect details, modify credentials, or configure MFA")
        hint_lbl.setStyleSheet("font-size: 11px; color: #94a3b8;")
        t_header_hbox.addWidget(hint_lbl)
        t_vbox.addWidget(header_container)

        self._table = QTableWidget(0, 7)
        self._table.setFrameShape(QFrame.Shape.NoFrame)
        self._table.setVerticalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAlwaysOff)
        self._table.setHorizontalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAlwaysOff)
        self._table.setSizePolicy(QSizePolicy.Policy.Expanding, QSizePolicy.Policy.Fixed)
        self._table.setHorizontalHeaderLabels([
            "ACCOUNT IDENTITY",
            "PLAN TIER",
            "STATUS",
            "NEXT 5H QUOTA",
            "WEEKLY AVAILABLE",
            "RESET HORIZON",
            "ACTION",
        ])
        self._table.horizontalHeader().setSectionResizeMode(0, QHeaderView.ResizeMode.Stretch)
        self._table.horizontalHeader().setSectionResizeMode(1, QHeaderView.ResizeMode.Fixed)
        self._table.horizontalHeader().setSectionResizeMode(2, QHeaderView.ResizeMode.Fixed)
        self._table.horizontalHeader().setSectionResizeMode(3, QHeaderView.ResizeMode.Fixed)
        self._table.horizontalHeader().setSectionResizeMode(4, QHeaderView.ResizeMode.Fixed)
        self._table.horizontalHeader().setSectionResizeMode(5, QHeaderView.ResizeMode.Fixed)
        self._table.horizontalHeader().setSectionResizeMode(6, QHeaderView.ResizeMode.Fixed)

        self._table.setColumnWidth(1, 95)
        self._table.setColumnWidth(2, 80)
        self._table.setColumnWidth(3, 130)
        self._table.setColumnWidth(4, 140)
        self._table.setColumnWidth(5, 155)
        self._table.setColumnWidth(6, 115)
        self._table.horizontalHeader().setStretchLastSection(False)
        self._table.horizontalHeader().setDefaultAlignment(Qt.AlignmentFlag.AlignLeft | Qt.AlignmentFlag.AlignVCenter)
        self._table.horizontalHeader().setFixedHeight(40)

        self._table.setAlternatingRowColors(False)
        self._table.setShowGrid(False)
        self._table.verticalHeader().setVisible(False)
        self._table.setSelectionBehavior(QTableWidget.SelectionBehavior.SelectRows)
        self._table.setEditTriggers(QTableWidget.EditTrigger.NoEditTriggers)
        self._table.setContextMenuPolicy(Qt.ContextMenuPolicy.CustomContextMenu)
        self._table.customContextMenuRequested.connect(self._on_table_context_menu)
        self._table.setStyleSheet("""
            QTableWidget {
                background-color: #ffffff;
                border: none;
                gridline-color: transparent;
                selection-background-color: #f8fafc;
            }
            QHeaderView {
                background-color: #f8fafd;
                border: none;
                border-bottom: 1px solid #e2e8f0;
            }
            QHeaderView::section {
                background-color: #f8fafd;
                color: #64748b;
                padding: 10px 14px;
                font-weight: 600;
                font-size: 11px;
                letter-spacing: 0.5px;
                text-transform: uppercase;
                border: none;
                border-bottom: 1px solid #e2e8f0;
            }
            QTableWidget::item {
                border-bottom: 1px solid #f1f5f9;
                background-color: #ffffff;
            }
            QTableWidget::item:hover {
                background-color: #f8fafc;
            }
            QTableWidget::item:selected {
                background-color: #f1f5f9;
            }
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
            self.btn_auto_switch.setText("⚡ Auto-Switch: ON")
            self.btn_auto_switch.setStyleSheet("""
                QPushButton {
                    background-color: #e6f4ea;
                    color: #137333;
                    border: 1px solid #ceead6;
                    border-radius: 14px;
                    padding: 5px 14px;
                    font-size: 11px;
                    font-weight: 700;
                }
                QPushButton:hover {
                    background-color: #ceead6;
                }
            """)
        else:
            self.btn_auto_switch.setText("Auto-Switch: OFF")
            self.btn_auto_switch.setStyleSheet("""
                QPushButton {
                    background-color: #f1f5f9;
                    color: #64748b;
                    border: 1px solid #e2e8f0;
                    border-radius: 14px;
                    padding: 5px 14px;
                    font-size: 11px;
                    font-weight: 700;
                }
                QPushButton:hover {
                    background-color: #e2e8f0;
                    color: #1e293b;
                }
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
                self._acc_label.setText(f"● Active: {self._active_account}")
            else:
                self._acc_label.setText("● Active: Not Logged In")

            self._sync_auto_switch_state()
            self.refresh_quota()
        except Exception as exc:
            self._last_poll_label.setText(f"Sync error: {exc}")

    def _create_identity_cell(self, email: str, label: Optional[str]) -> QWidget:
        container = QWidget()
        container.setStyleSheet("background: transparent; border: none;")
        vbox = QVBoxLayout(container)
        vbox.setContentsMargins(16, 6, 8, 6)
        vbox.setSpacing(2)
        vbox.setAlignment(Qt.AlignmentFlag.AlignVCenter | Qt.AlignmentFlag.AlignLeft)

        email_lbl = QLabel(email)
        email_lbl.setStyleSheet("font-size: 13px; font-weight: 600; color: #1e293b; background: transparent; border: none;")
        vbox.addWidget(email_lbl)

        if label and label.strip() and label.strip() != email:
            label_lbl = QLabel(label.strip())
            label_lbl.setStyleSheet("font-size: 11px; color: #64748b; background: transparent; border: none;")
            vbox.addWidget(label_lbl)

        return container

    def _create_plan_tier_badge(self, tier: str) -> QWidget:
        container = QWidget()
        container.setStyleSheet("background: transparent; border: none;")
        layout = QHBoxLayout(container)
        layout.setContentsMargins(0, 0, 0, 0)
        layout.setAlignment(Qt.AlignmentFlag.AlignCenter)

        t = (tier or "Free").strip()
        lbl = QLabel(t)
        lbl.setAlignment(Qt.AlignmentFlag.AlignCenter)

        if t == "Plus":
            bg, fg, border = "#e6f4ea", "#137333", "none"
        elif t == "Pro":
            bg, fg, border = "#e8f0fe", "#1a73e8", "none"
        elif t == "Pro - Trial":
            bg, fg, border = "#e0f7fa", "#007b83", "none"
        elif t == "Edu":
            bg, fg, border = "#ede7f6", "#512da8", "none"
        elif "Ultra 5X" in t:
            bg, fg, border = "#fce7f3", "#db2777", "none"
        elif "Ultra 10X" in t:
            bg, fg, border = "#fae8ff", "#c026d3", "none"
        elif "Ultra 20X" in t:
            bg, fg, border = "#ede9fe", "#7c3aed", "1px solid #d1c4e9"
        elif "Ultra" in t:
            bg, fg, border = "#f3e8fd", "#7b1fa2", "none"
        else:
            bg, fg, border = "#f1f5f9", "#475569", "none"

        lbl.setStyleSheet(f"""
            QLabel {{
                background-color: {bg};
                color: {fg};
                border: {border};
                border-radius: 12px;
                padding: 3px 8px;
                font-size: 11px;
                font-weight: 600;
            }}
        """)
        layout.addWidget(lbl)
        return container

    def _create_status_cell(self, is_active: bool, status: str) -> QWidget:
        container = QWidget()
        container.setStyleSheet("background: transparent; border: none;")
        layout = QHBoxLayout(container)
        layout.setContentsMargins(0, 0, 0, 0)
        layout.setAlignment(Qt.AlignmentFlag.AlignCenter)

        if is_active:
            lbl = QLabel("ACTIVE")
            lbl.setStyleSheet("""
                QLabel {
                    background-color: #e6f4ea;
                    color: #137333;
                    border-radius: 10px;
                    padding: 3px 8px;
                    font-size: 11px;
                    font-weight: 600;
                    border: none;
                }
            """)
        else:
            lbl = QLabel("STANDBY")
            lbl.setStyleSheet("""
                QLabel {
                    background-color: transparent;
                    color: #64748b;
                    font-size: 11px;
                    font-weight: 600;
                    border: none;
                }
            """)
        layout.addWidget(lbl)
        return container

    def _create_reset_horizon_cell(self, reset_text: str) -> QWidget:
        container = QWidget()
        container.setStyleSheet("background: transparent; border: none;")
        layout = QHBoxLayout(container)
        layout.setContentsMargins(8, 0, 8, 0)
        layout.setAlignment(Qt.AlignmentFlag.AlignVCenter | Qt.AlignmentFlag.AlignLeft)
        layout.setSpacing(6)

        clock_lbl = QLabel("🕒")
        clock_lbl.setStyleSheet("font-size: 12px; background: transparent; border: none;")
        layout.addWidget(clock_lbl)

        text_str = reset_text.strip() if reset_text and reset_text.strip() else "Active cycle"
        text_lbl = QLabel(text_str)
        text_lbl.setStyleSheet("font-size: 12px; color: #64748b; background: transparent; border: none;")
        layout.addWidget(text_lbl)
        layout.addStretch()

        return container

    def _on_table_context_menu(self, pos) -> None:
        row = self._table.rowAt(pos.y())
        if row >= 0:
            from PySide6.QtWidgets import QMenu
            menu = QMenu(self)
            action_edit = menu.addAction("Edit Account Details...")
            action_edit.triggered.connect(lambda: self._open_account_detail(row))
            menu.exec(self._table.viewport().mapToGlobal(pos))

    def _create_action_cell(self, row_idx: int, is_active: bool = False, email: str = "") -> QWidget:
        container = QWidget()
        container.setStyleSheet("background: transparent; border: none;")
        layout = QHBoxLayout(container)
        layout.setContentsMargins(0, 0, 0, 0)
        layout.setAlignment(Qt.AlignmentFlag.AlignCenter)

        if is_active:
            lbl_active = QLabel("Active")
            lbl_active.setStyleSheet("""
                QLabel {
                    background-color: #e6f4ea;
                    color: #137333;
                    border-radius: 12px;
                    padding: 3px 12px;
                    font-size: 11px;
                    font-weight: 600;
                }
            """)
            layout.addWidget(lbl_active)
        else:
            btn_switch = QPushButton("Switch")
            btn_switch.setStyleSheet("""
                QPushButton {
                    background-color: #e9eef6;
                    color: #041e49;
                    border: 1px solid #d3e3fd;
                    border-radius: 12px;
                    padding: 4px 12px;
                    font-size: 11px;
                    font-weight: 600;
                }
                QPushButton:hover {
                    background-color: #d3e3fd;
                    color: #0b57d0;
                }
            """)
            btn_switch.clicked.connect(lambda checked=False, em=email: self._on_quick_switch(em) if hasattr(self, '_on_quick_switch') else self._controller.switch_account(em))
            layout.addWidget(btn_switch)

        return container

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
            now_str = datetime.datetime.now().strftime("%I:%M:%S %p")
            self._last_poll_label.setText(f"Last Quota Sync: {now_str}")

            # 1. Update Top Section Metrics
            fleet_5h, fleet_weekly, total_count = compute_fleet_quota_summary(self._accounts_cache)
            self._total_accounts_lbl.setText(f"{total_count} Account{'s' if total_count != 1 else ''} Managed")

            self._ring_5h.fraction = fleet_5h
            self._ring_5h.reset_text = ""

            self._ring_weekly.fraction = fleet_weekly
            self._ring_weekly.reset_text = ""

            # Update Dropdown
            self._account_combo.clear()
            for acc in self._accounts_cache:
                display_label = f"{acc.email} ({acc.label})" if acc.label and acc.label != acc.email else acc.email
                self._account_combo.addItem(display_label, acc.email)
            if self._active_account:
                for i in range(self._account_combo.count()):
                    if self._account_combo.itemData(i) == self._active_account:
                        self._account_combo.setCurrentIndex(i)
                        break

            # 2. Populate Bottom Section Accounts Table
            self._table.setRowCount(0)
            for r, acc in enumerate(self._accounts_cache):
                self._table.insertRow(r)

                # Col 0: Identity (Email & Label, 2-line layout)
                self._table.setCellWidget(r, 0, self._create_identity_cell(acc.email, acc.label))

                # Col 1: Plan Tier Badge
                tier_badge = self._create_plan_tier_badge(getattr(acc, "plan_tier", "Free"))
                self._table.setCellWidget(r, 1, tier_badge)

                # Col 2: Status Pill
                self._table.setCellWidget(r, 2, self._create_status_cell(acc.is_active, acc.status))

                # Col 3: 5h Available Quota (Horizontal Bar + Text %)
                bar_5h_widget = AccountQuotaBarWidget(acc.quota_5h_available)
                self._table.setCellWidget(r, 3, bar_5h_widget)

                # Col 4: Weekly Available (Horizontal Bar + Text %)
                bar_wk_widget = AccountQuotaBarWidget(acc.quota_weekly)
                self._table.setCellWidget(r, 4, bar_wk_widget)

                # Col 5: Reset Horizon Text
                self._table.setCellWidget(r, 5, self._create_reset_horizon_cell(acc.reset_horizon_text))

                # Col 6: Action Button (Switch / Active)
                self._table.setCellWidget(r, 6, self._create_action_cell(r, acc.is_active, acc.email))
                self._table.setRowHeight(r, 56)

            self._adjust_table_height()

        except Exception as exc:
            self._last_poll_label.setText(f"Quota fetch failed: {exc}")

    def _adjust_table_height(self) -> None:
        """Dynamically resizes table to fit all rows, preventing internal table scrollbars."""
        self._table.doItemsLayout()
        header_h = self._table.horizontalHeader().height()
        if header_h <= 0:
            header_h = self._table.horizontalHeader().sizeHint().height() or 40
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
        target_account = self._account_combo.currentData()
        if not target_account:
            txt = self._account_combo.currentText().strip()
            target_account = txt.split(" ")[0].strip()
        if not target_account or target_account == self._active_account:
            return

        try:
            self.controller.switch_account(target_account, force=True, relaunch=True)
            self._active_account = target_account
            self._acc_label.setText(f"● Active: {target_account}")
            self.account_switched.emit(target_account)
            self.refresh_quota()
        except Exception as exc:
            self._last_poll_label.setText(f"Switch failed: {exc}")
