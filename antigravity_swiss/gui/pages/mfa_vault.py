"""
Accounts & MFA Vault Page.
=========================
Multi-account inventory, credential management, and integrated RFC 6238 TOTP authenticator.
Features:
- Live 6-digit verification code with letter spacing
- Smooth circular 30-second countdown ring
- 1-click clipboard copy
- Base32 secret configuration and live validation
- Backup codes manager
"""

from __future__ import annotations

from typing import Any, Optional
from PySide6.QtCore import Qt, QTimer, Signal
from PySide6.QtGui import QClipboard, QGuiApplication
from PySide6.QtWidgets import (
    QFrame,
    QHBoxLayout,
    QHeaderView,
    QLabel,
    QLineEdit,
    QMessageBox,
    QPlainTextEdit,
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
from antigravity_swiss.gui.widgets.countdown_ring import CountdownRing
from antigravity_swiss.ipc.controller import SwissKnifeController
from antigravity_swiss.totp.engine import TOTPEngine


class MfaVaultPage(QWidget):
    """
    Accounts inventory and MFA/TOTP vault page.
    """

    account_updated = Signal(str)

    def __init__(
        self,
        controller: SwissKnifeController,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.controller = controller
        self._selected_account: Optional[str] = None
        self._selected_secret: str = ""
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

        # Top Section: Active TOTP Authenticator Card
        auth_card = QFrame()
        auth_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_SURFACE_CONTAINER};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 16px;
                padding: 20px;
            }}
        """)
        ac_layout = QVBoxLayout(auth_card)
        ac_layout.setSpacing(14)

        card_title = QLabel("LIVE MFA / TOTP AUTHENTICATOR")
        card_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_TEXT_SECONDARY}; letter-spacing: 1px;")
        ac_layout.addWidget(card_title)

        # Live Code Display Box
        code_box = QFrame()
        code_box.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_SURFACE_CONTAINER_HIGH};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
        """)
        cb_layout = QHBoxLayout(code_box)
        cb_layout.setContentsMargins(16, 12, 16, 12)
        cb_layout.setSpacing(20)

        # Countdown Ring
        self._countdown_ring = CountdownRing(secret="")
        self._countdown_ring.tick.connect(self._on_ring_tick)
        self._countdown_ring.time_expired.connect(self._on_code_expired)
        cb_layout.addWidget(self._countdown_ring)

        # Code text layout
        code_vbox = QVBoxLayout()
        code_vbox.setSpacing(2)

        self._target_account_lbl = QLabel("Select an account below")
        self._target_account_lbl.setStyleSheet(f"font-size: 13px; font-weight: 500; color: {MD3_TEXT_SECONDARY};")
        code_vbox.addWidget(self._target_account_lbl)

        self._code_label = QLabel("------")
        self._code_label.setProperty("class", "code-display")
        self._code_label.setStyleSheet(f"""
            QLabel {{
                font-family: 'JetBrains Mono', 'Fira Code', 'DejaVu Sans Mono', monospace;
                font-size: 32px;
                font-weight: 700;
                letter-spacing: 6px;
                color: {MD3_ACCENT_PRIMARY};
            }}
        """)
        code_vbox.addWidget(self._code_label)
        cb_layout.addLayout(code_vbox)

        cb_layout.addStretch()

        # Copy Button
        self._copy_btn = QPushButton("📋 Copy Code")
        self._copy_btn.setStyleSheet(f"""
            QPushButton {{
                background-color: #2b394f;
                color: {MD3_ACCENT_PRIMARY};
                border: 1px solid {MD3_ACCENT_PRIMARY};
                border-radius: 18px;
                padding: 10px 20px;
                font-size: 13px;
                font-weight: 600;
            }}
            QPushButton:hover {{
                background-color: {MD3_ACCENT_PRIMARY};
                color: #041e42;
            }}
        """)
        self._copy_btn.clicked.connect(self._on_copy_code)
        cb_layout.addWidget(self._copy_btn)

        ac_layout.addWidget(code_box)
        c_layout.addWidget(auth_card)

        # Middle Section: Accounts Inventory Table
        inv_card = QFrame()
        inv_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_SURFACE_CONTAINER};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 16px;
                padding: 16px;
            }}
        """)
        ic_layout = QVBoxLayout(inv_card)
        ic_layout.setSpacing(10)

        ic_title = QLabel("REGISTERED ACCOUNTS INVENTORY")
        ic_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_TEXT_SECONDARY}; letter-spacing: 1px;")
        ic_layout.addWidget(ic_title)

        self._table = QTableWidget(0, 4)
        self._table.setHorizontalHeaderLabels(["Account Email", "Role / Label", "MFA Status", "Action"])
        self._table.horizontalHeader().setSectionResizeMode(0, QHeaderView.ResizeMode.Stretch)
        self._table.horizontalHeader().setSectionResizeMode(1, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setSectionResizeMode(2, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setSectionResizeMode(3, QHeaderView.ResizeMode.ResizeToContents)
        self._table.setAlternatingRowColors(True)
        self._table.verticalHeader().setVisible(False)
        self._table.setSelectionBehavior(QTableWidget.SelectionBehavior.SelectRows)
        self._table.itemSelectionChanged.connect(self._on_table_row_selected)
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
        ic_layout.addWidget(self._table)
        c_layout.addWidget(inv_card)

        # Bottom Section: MFA Secret Key Configuration Form
        config_card = QFrame()
        config_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_SURFACE_CONTAINER};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 16px;
                padding: 20px;
            }}
        """)
        cfg_layout = QVBoxLayout(config_card)
        cfg_layout.setSpacing(14)

        cfg_title = QLabel("CONFIGURE TOTP SECRET KEY")
        cfg_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_TEXT_SECONDARY}; letter-spacing: 1px;")
        cfg_layout.addWidget(cfg_title)

        secret_hbox = QHBoxLayout()
        secret_hbox.setSpacing(10)

        self._secret_input = QLineEdit()
        self._secret_input.setPlaceholderText("Enter Base32 TOTP secret (e.g. JBSWY3DPEHPK3PXP)")
        self._secret_input.setStyleSheet(f"""
            QLineEdit {{
                background-color: {MD3_SURFACE_CONTAINER_HIGH};
                color: {MD3_TEXT_PRIMARY};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 8px;
                padding: 8px 12px;
                font-family: monospace;
            }}
            QLineEdit:focus {{
                border: 1px solid {MD3_ACCENT_PRIMARY};
            }}
        """)
        self._secret_input.textChanged.connect(self._on_secret_changed)
        secret_hbox.addWidget(self._secret_input)

        self._save_secret_btn = QPushButton("Save Secret")
        self._save_secret_btn.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_ACCENT_PRIMARY};
                color: #041e42;
                border: none;
                border-radius: 16px;
                padding: 8px 18px;
                font-weight: 600;
                font-size: 12px;
            }}
            QPushButton:hover {{
                background-color: #a8c7fa;
            }}
        """)
        self._save_secret_btn.clicked.connect(self._on_save_secret)
        secret_hbox.addWidget(self._save_secret_btn)

        self._clear_secret_btn = QPushButton("Remove MFA")
        self._clear_secret_btn.setStyleSheet(f"""
            QPushButton {{
                background-color: #3b2323;
                color: {MD3_COLOR_EXHAUSTED};
                border: 1px solid #5c2b29;
                border-radius: 16px;
                padding: 8px 14px;
                font-weight: 500;
                font-size: 12px;
            }}
            QPushButton:hover {{
                background-color: #4f2929;
            }}
        """)
        self._clear_secret_btn.clicked.connect(self._on_clear_secret)
        secret_hbox.addWidget(self._clear_secret_btn)

        cfg_layout.addLayout(secret_hbox)

        self._val_feedback = QLabel("Enter valid Base32 secret (A-Z, 2-7)")
        self._val_feedback.setStyleSheet(f"font-size: 11px; color: {MD3_TEXT_SECONDARY};")
        cfg_layout.addWidget(self._val_feedback)

        # Backup Codes subsection
        backup_title = QLabel("EMERGENCY BACKUP CODES (OPTIONAL)")
        backup_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_TEXT_SECONDARY}; letter-spacing: 1px; margin-top: 8px;")
        cfg_layout.addWidget(backup_title)

        self._backup_text = QPlainTextEdit()
        self._backup_text.setPlaceholderText("Store one-time 8-digit backup codes here (one per line)...")
        self._backup_text.setMaximumHeight(80)
        self._backup_text.setStyleSheet(f"""
            QPlainTextEdit {{
                background-color: {MD3_SURFACE_CONTAINER_HIGH};
                color: {MD3_TEXT_PRIMARY};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 8px;
                padding: 8px;
                font-family: monospace;
            }}
        """)
        cfg_layout.addWidget(self._backup_text)

        c_layout.addWidget(config_card)

        scroll.setWidget(container)
        main_layout.addWidget(scroll)

        self.load_accounts()

    def load_accounts(self) -> None:
        """Fetch accounts list and update UI."""
        try:
            self._accounts = self.controller.list_accounts()
            self._table.setRowCount(0)

            for acc in self._accounts:
                email = acc.get("email", "")
                label = acc.get("label", "")
                has_mfa = acc.get("has_totp", False) or bool(acc.get("totp_secret"))
                is_active = acc.get("is_active", False)

                r = self._table.rowCount()
                self._table.insertRow(r)

                email_str = f"★ {email}" if is_active else email
                self._table.setItem(r, 0, QTableWidgetItem(email_str))
                self._table.setItem(r, 1, QTableWidgetItem(label))

                mfa_status_item = QTableWidgetItem("PROTECTED" if has_mfa else "NO MFA")
                self._table.setItem(r, 2, mfa_status_item)

                action_item = QTableWidgetItem("Select")
                self._table.setItem(r, 3, action_item)

            # Auto-select active account or first row
            if self._accounts:
                target_idx = 0
                for idx, acc in enumerate(self._accounts):
                    if acc.get("is_active"):
                        target_idx = idx
                        break
                self._table.selectRow(target_idx)
        except Exception as exc:
            self._val_feedback.setText(f"Error loading accounts: {exc}")

    def _on_table_row_selected(self) -> None:
        sel_ranges = self._table.selectedRanges()
        if not sel_ranges or not self._accounts:
            return

        row = sel_ranges[0].topRow()
        if 0 <= row < len(self._accounts):
            acc = self._accounts[row]
            email = acc.get("email", "")
            totp_secret = acc.get("totp_secret", "")

            self._selected_account = email
            self._selected_secret = totp_secret
            self._target_account_lbl.setText(f"Account: {email}")
            self._secret_input.setText(totp_secret)
            self._countdown_ring.secret = totp_secret
            self._update_code_display()

    def _on_secret_changed(self, text: str) -> None:
        cleaned = text.strip().replace(" ", "").upper()
        if not cleaned:
            self._val_feedback.setText("Enter valid Base32 secret (A-Z, 2-7)")
            self._val_feedback.setStyleSheet(f"font-size: 11px; color: {MD3_TEXT_SECONDARY};")
            return

        is_valid = TOTPEngine.validate_secret(cleaned)
        if is_valid:
            try:
                preview_code, _ = TOTPEngine.generate_code(cleaned)
                self._val_feedback.setText(f"✓ Valid Base32 Secret (Sample preview code: {preview_code})")
                self._val_feedback.setStyleSheet(f"font-size: 11px; color: {MD3_COLOR_HEALTHY}; font-weight: 500;")
            except Exception as exc:
                self._val_feedback.setText(f"Validation error: {exc}")
                self._val_feedback.setStyleSheet(f"font-size: 11px; color: {MD3_COLOR_EXHAUSTED};")
        else:
            self._val_feedback.setText("✗ Invalid Base32 characters detected (must be A-Z, 2-7)")
            self._val_feedback.setStyleSheet(f"font-size: 11px; color: {MD3_COLOR_EXHAUSTED};")

    def _on_save_secret(self) -> None:
        if not self._selected_account:
            QMessageBox.warning(self, "No Account Selected", "Please select an account from the inventory list first.")
            return

        secret = self._secret_input.text().strip().replace(" ", "").upper()
        if secret and not TOTPEngine.validate_secret(secret):
            QMessageBox.critical(self, "Invalid Secret", "Secret is not valid Base32 format.")
            return

        try:
            self.controller.set_totp_secret(self._selected_account, secret)
            self._selected_secret = secret
            self._countdown_ring.secret = secret
            self._update_code_display()
            self._val_feedback.setText("✓ MFA Secret saved successfully.")
            self._val_feedback.setStyleSheet(f"font-size: 11px; color: {MD3_COLOR_HEALTHY};")
            self.account_updated.emit(self._selected_account)
            self.load_accounts()
        except Exception as exc:
            QMessageBox.critical(self, "Save Failed", f"Could not save TOTP secret: {exc}")

    def _on_clear_secret(self) -> None:
        if not self._selected_account:
            return

        try:
            self.controller.set_totp_secret(self._selected_account, "")
            self._selected_secret = ""
            self._secret_input.clear()
            self._countdown_ring.secret = ""
            self._code_label.setText("------")
            self._val_feedback.setText("MFA removed.")
            self.account_updated.emit(self._selected_account)
            self.load_accounts()
        except Exception as exc:
            QMessageBox.critical(self, "Error", f"Failed to remove MFA: {exc}")

    def _on_ring_tick(self, rem_sec: int, progress: float) -> None:
        self._update_code_display()

    def _on_code_expired(self) -> None:
        self._update_code_display()

    def _update_code_display(self) -> None:
        code = self._countdown_ring.current_code
        if code and code != "------":
            formatted = f"{code[:3]} {code[3:]}"
            self._code_label.setText(formatted)
        else:
            self._code_label.setText("------")

    def _on_copy_code(self) -> None:
        raw_code = self._countdown_ring.current_code
        if raw_code and raw_code != "------":
            clipboard: QClipboard = QGuiApplication.clipboard()
            clipboard.setText(raw_code)
            self._copy_btn.setText("✓ Copied!")
            QTimer.singleShot(1500, lambda: self._copy_btn.setText("📋 Copy Code"))
