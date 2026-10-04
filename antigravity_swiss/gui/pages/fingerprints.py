"""
Device Fingerprints Page.
=========================
Hardware identity virtualization and anti-ban profile generator:
- machineid (~/.config/Antigravity/machineid)
- .updaterId (~/.config/Antigravity/.updaterId)
- installation_id (~/.gemini/antigravity/installation_id)
- installation_uuid (~/.gemini/antigravity/antigravity_state.pbtxt)
"""

from __future__ import annotations

from typing import Any, Optional
from PySide6.QtCore import Qt, Signal
from PySide6.QtWidgets import (
    QComboBox,
    QFrame,
    QHBoxLayout,
    QHeaderView,
    QLabel,
    QLineEdit,
    QMessageBox,
    QPushButton,
    QScrollArea,
    QTableWidget,
    QTableWidgetItem,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.core.constants import (
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_HEALTHY,
    MD3_COLOR_WARNING,
    MD3_LIGHT_ACCENT_CONTAINER,
    MD3_LIGHT_ACCENT_PRIMARY,
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
from antigravity_swiss.fingerprint.profile_store import DeviceProfile, DeviceProfileStore
from antigravity_swiss.ipc.controller import SwissKnifeController


class DeviceFingerprintsPage(QWidget):
    """
    Device fingerprint inspector, virtualizer, and anti-ban generator page.
    """

    profile_updated = Signal(str)

    def __init__(
        self,
        controller: SwissKnifeController,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.controller = controller
        self._store = DeviceProfileStore()
        self._accounts: list[dict[str, Any]] = []

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

        t1 = QLabel("DEVICE FINGERPRINT VIRTUALIZER")
        t1.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        t2 = QLabel("Isolates hardware and installation IDs per account to prevent sybil cross-correlation.")
        t2.setStyleSheet(f"font-size: 13px; color: {MD3_LIGHT_TEXT_PRIMARY};")
        h_vbox.addWidget(t1)
        h_vbox.addWidget(t2)
        h_layout.addLayout(h_vbox)
        h_layout.addStretch()

        self._status_badge = QLabel("Virtualization Ready")
        self._status_badge.setStyleSheet(f"""
            QLabel {{
                background-color: #e6f4ea;
                color: {MD3_LIGHT_COLOR_HEALTHY};
                border: 1px solid #ceead6;
                border-radius: 12px;
                padding: 5px 12px;
                font-size: 12px;
                font-weight: 600;
            }}
        """)
        h_layout.addWidget(self._status_badge)
        c_layout.addWidget(header_card)

        # Active Profile Inspector Card
        insp_card = QFrame()
        insp_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 20px;
            }}
        """)
        ic_layout = QVBoxLayout(insp_card)
        ic_layout.setSpacing(14)

        # Account Selection Row
        acc_row = QHBoxLayout()
        acc_lbl = QLabel("Target Account:")
        acc_lbl.setStyleSheet(f"font-size: 13px; font-weight: 500; color: {MD3_LIGHT_TEXT_PRIMARY};")
        acc_row.addWidget(acc_lbl)

        self._acc_combo = QComboBox()
        self._acc_combo.setMinimumWidth(260)
        self._acc_combo.setStyleSheet(f"""
            QComboBox {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                color: {MD3_LIGHT_TEXT_PRIMARY};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 6px;
                padding: 6px 12px;
                font-size: 12px;
            }}
        """)
        self._acc_combo.currentIndexChanged.connect(self._on_account_selected)
        acc_row.addWidget(self._acc_combo)

        acc_row.addStretch()

        self._gen_btn = QPushButton("Generate Fresh Profile")
        self._gen_btn.setStyleSheet(f"""
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
        self._gen_btn.clicked.connect(self._on_generate_random)
        acc_row.addWidget(self._gen_btn)

        self._save_btn = QPushButton("Save Profile")
        self._save_btn.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_LIGHT_ACCENT_PRIMARY};
                color: #ffffff;
                border: none;
                border-radius: 6px;
                padding: 8px 16px;
                font-size: 12px;
                font-weight: 600;
            }}
            QPushButton:hover {{
                background-color: #1a73e8;
            }}
        """)
        self._save_btn.clicked.connect(self._on_save_profile)
        acc_row.addWidget(self._save_btn)

        ic_layout.addLayout(acc_row)

        # ID Fields Display
        self._inputs: dict[str, QLineEdit] = {}
        fields = [
            ("machine_id", "Machine ID (machineid)", "Host telemetry machine identifier"),
            ("updater_id", "Updater ID (.updaterId)", "Auto-update telemetry UUID"),
            ("installation_id", "Installation ID (installation_id)", "Gemini CLI installation identifier"),
            ("installation_uuid", "State UUID (antigravity_state.pbtxt)", "Internal state protobuf UUID"),
        ]

        for key, label, tooltip in fields:
            field_box = QVBoxLayout()
            field_box.setSpacing(4)

            lbl = QLabel(label)
            lbl.setStyleSheet(f"font-size: 11px; font-weight: 600; color: {MD3_LIGHT_TEXT_SECONDARY};")
            lbl.setToolTip(tooltip)
            field_box.addWidget(lbl)

            inp = QLineEdit()
            inp.setToolTip(tooltip)
            inp.setStyleSheet(f"""
                QLineEdit {{
                    background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                    color: {MD3_LIGHT_TEXT_PRIMARY};
                    border: 1px solid {MD3_LIGHT_OUTLINE};
                    border-radius: 6px;
                    padding: 8px 12px;
                    font-family: monospace;
                    font-size: 12px;
                }}
                QLineEdit:focus {{
                    border: 1px solid {MD3_LIGHT_ACCENT_PRIMARY};
                }}
            """)
            field_box.addWidget(inp)
            self._inputs[key] = inp
            ic_layout.addLayout(field_box)

        c_layout.addWidget(insp_card)

        # Table of All Profiles
        tbl_card = QFrame()
        tbl_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
        """)
        tc_layout = QVBoxLayout(tbl_card)
        tc_layout.setSpacing(10)

        tc_title = QLabel("PROFILE MAPPINGS INVENTORY")
        tc_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        tc_layout.addWidget(tc_title)

        self._table = QTableWidget(0, 3)
        self._table.setHorizontalHeaderLabels(["Account Email", "Machine ID (Truncated)", "Isolation Status"])
        self._table.horizontalHeader().setSectionResizeMode(0, QHeaderView.ResizeMode.Stretch)
        self._table.horizontalHeader().setSectionResizeMode(1, QHeaderView.ResizeMode.Stretch)
        self._table.horizontalHeader().setSectionResizeMode(2, QHeaderView.ResizeMode.ResizeToContents)
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
        tc_layout.addWidget(self._table)
        c_layout.addWidget(tbl_card)

        scroll.setWidget(container)
        main_layout.addWidget(scroll)

        self.load_data()

    def load_data(self) -> None:
        """Load registered accounts and profiles."""
        try:
            self._accounts = self.controller.list_accounts()
            self._acc_combo.clear()

            for acc in self._accounts:
                email = acc.get("email", "")
                if email:
                    self._acc_combo.addItem(email)

            self._refresh_table()

            if self._acc_combo.count() > 0:
                self._on_account_selected(0)
        except Exception as exc:
            self._status_badge.setText(f"Load Error: {exc}")

    def _refresh_table(self) -> None:
        self._table.setRowCount(0)
        for acc in self._accounts:
            email = acc.get("email", "")
            if not email:
                continue

            prof = self._store.get_profile(email)
            r = self._table.rowCount()
            self._table.insertRow(r)

            self._table.setItem(r, 0, QTableWidgetItem(email))

            if prof:
                mach_id_trunc = f"{prof.machine_id[:16]}..." if len(prof.machine_id) > 16 else prof.machine_id
                self._table.setItem(r, 1, QTableWidgetItem(mach_id_trunc))
                status_item = QTableWidgetItem("ISOLATED")
                status_item.setForeground(QTableWidgetItem().foreground())
                self._table.setItem(r, 2, status_item)
            else:
                self._table.setItem(r, 1, QTableWidgetItem("Default Host Profile"))
                status_item = QTableWidgetItem("NOT ISOLATED")
                self._table.setItem(r, 2, status_item)

    def _on_account_selected(self, index: int) -> None:
        email = self._acc_combo.currentText().strip()
        if not email:
            return

        prof = self._store.get_or_create_profile(email)
        self._inputs["machine_id"].setText(prof.machine_id)
        self._inputs["updater_id"].setText(prof.updater_id)
        self._inputs["installation_id"].setText(prof.installation_id)
        self._inputs["installation_uuid"].setText(prof.installation_uuid)

    def _on_generate_random(self) -> None:
        fresh = DeviceProfile.generate_random()
        self._inputs["machine_id"].setText(fresh.machine_id)
        self._inputs["updater_id"].setText(fresh.updater_id)
        self._inputs["installation_id"].setText(fresh.installation_id)
        self._inputs["installation_uuid"].setText(fresh.installation_uuid)

    def _on_save_profile(self) -> None:
        email = self._acc_combo.currentText().strip()
        if not email:
            return

        prof = DeviceProfile(
            machine_id=self._inputs["machine_id"].text().strip(),
            updater_id=self._inputs["updater_id"].text().strip(),
            installation_id=self._inputs["installation_id"].text().strip(),
            installation_uuid=self._inputs["installation_uuid"].text().strip(),
        )

        try:
            self._store.set_profile(email, prof)
            self._refresh_table()
            self.profile_updated.emit(email)
            QMessageBox.information(self, "Profile Saved", f"Device profile for '{email}' saved successfully.")
        except Exception as exc:
            QMessageBox.critical(self, "Save Error", f"Failed to save profile: {exc}")
