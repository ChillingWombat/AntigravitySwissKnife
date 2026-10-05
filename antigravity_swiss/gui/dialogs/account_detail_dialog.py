"""
Account Detail & Edit Dialog.
=============================
Modal window allowing users to inspect account quota horizons,
modify friendly labels, update TOTP MFA secrets, rotate OAuth refresh tokens,
and toggle active account status.
"""

from __future__ import annotations

from typing import Optional
from PySide6.QtCore import Qt, Signal
from PySide6.QtWidgets import (
    QCheckBox,
    QDialog,
    QFrame,
    QHBoxLayout,
    QLabel,
    QLineEdit,
    QMessageBox,
    QPushButton,
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
from antigravity_swiss.gui.widgets.account_quota_bar import AccountQuotaBarWidget
from antigravity_swiss.ipc.controller import SwissKnifeController
from antigravity_swiss.quota.calculator import AccountQuotaState
from antigravity_swiss.totp.engine import generate_totp_code, sanitize_secret


class AccountDetailDialog(QDialog):
    """
    Pop-up detail window for an individual managed account.
    Allows editing label, TOTP seed, refresh token, and switching active state.
    """

    account_saved = Signal(str)
    account_removed = Signal(str)

    def __init__(
        self,
        account: AccountQuotaState,
        controller: SwissKnifeController,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.account = account
        self.controller = controller
        self.setWindowTitle(f"Account Details — {account.email}")
        self.resize(540, 600)
        self.setModal(True)
        self.setStyleSheet(f"""
            QDialog {{
                background-color: {MD3_LIGHT_SURFACE};
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
        """)
        self._init_ui()

    def _init_ui(self) -> None:
        layout = QVBoxLayout(self)
        layout.setContentsMargins(24, 24, 24, 20)
        layout.setSpacing(16)

        # 1. Header Card: Email & Status
        head_card = QFrame()
        head_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
        """)
        head_layout = QVBoxLayout(head_card)
        head_layout.setSpacing(6)

        title_row = QHBoxLayout()
        acc_title = QLabel(self.account.email)
        acc_title.setStyleSheet(f"font-size: 16px; font-weight: 700; color: {MD3_LIGHT_TEXT_PRIMARY};")
        title_row.addWidget(acc_title)
        title_row.addStretch()

        # Status badge
        badge = QLabel("ACTIVE" if self.account.is_active else "STANDBY")
        badge_bg = "#e6f4ea" if self.account.is_active else MD3_LIGHT_SURFACE_CONTAINER_HIGH
        badge_color = MD3_LIGHT_COLOR_HEALTHY if self.account.is_active else MD3_LIGHT_TEXT_SECONDARY
        badge.setStyleSheet(f"""
            background-color: {badge_bg};
            color: {badge_color};
            font-size: 11px;
            font-weight: 700;
            padding: 4px 10px;
            border-radius: 12px;
        """)
        title_row.addWidget(badge)
        head_layout.addLayout(title_row)

        desc = QLabel(f"Managed Session Account • {self.account.reset_horizon_text}")
        desc.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_SECONDARY};")
        head_layout.addWidget(desc)
        layout.addWidget(head_card)

        # 2. Live Quota Horizon Card
        quota_card = QFrame()
        quota_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
        """)
        q_layout = QVBoxLayout(quota_card)
        q_layout.setSpacing(12)

        q_head = QLabel("LIVE QUOTA METRICS")
        q_head.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        q_layout.addWidget(q_head)

        # 5h Quota Bar
        row_5h = QVBoxLayout()
        row_5h.setSpacing(2)
        lbl_5h = QLabel(f"Next 5-Hour Available Quota (effective capacity: {self.account.reset_horizon_text}):")
        lbl_5h.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_PRIMARY};")
        bar_5h = AccountQuotaBarWidget(self.account.quota_5h_available)
        row_5h.addWidget(lbl_5h)
        row_5h.addWidget(bar_5h)
        q_layout.addLayout(row_5h)

        # Weekly Quota Bar
        row_wk = QVBoxLayout()
        row_wk.setSpacing(2)
        lbl_wk = QLabel("Weekly Horizon Quota Left:")
        lbl_wk.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_PRIMARY};")
        bar_wk = AccountQuotaBarWidget(self.account.quota_weekly)
        row_wk.addWidget(lbl_wk)
        row_wk.addWidget(bar_wk)
        q_layout.addLayout(row_wk)

        layout.addWidget(quota_card)

        # 3. Editable Account Settings Form
        form_card = QFrame()
        form_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
        """)
        f_layout = QVBoxLayout(form_card)
        f_layout.setSpacing(12)

        f_head = QLabel("ACCOUNT CREDENTIALS & METADATA")
        f_head.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        f_layout.addWidget(f_head)

        # Label field
        f_layout.addWidget(QLabel("Account Alias:"))
        self.label_edit = QLineEdit(self.account.label)
        self.label_edit.setPlaceholderText("e.g. Account 1, Primary, Backup")
        self.label_edit.setStyleSheet(f"""
            QLineEdit {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 8px 12px;
                font-size: 13px;
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
        """)
        f_layout.addWidget(self.label_edit)

        # TOTP Secret field
        f_layout.addWidget(QLabel("MFA / TOTP Secret Key (Base32 or otpauth:// URI):"))
        totp_row = QHBoxLayout()
        self.totp_edit = QLineEdit(self.account.totp_secret)
        self.totp_edit.setPlaceholderText("JBSWY3DPEHPK3PXP or otpauth://totp/...")
        self.totp_edit.setEchoMode(QLineEdit.EchoMode.Password)
        self.totp_edit.setStyleSheet(f"""
            QLineEdit {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 8px 12px;
                font-size: 12px;
                font-family: monospace;
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
        """)
        self.totp_edit.textChanged.connect(self._on_totp_changed)
        totp_row.addWidget(self.totp_edit)

        self.btn_toggle_totp = QPushButton("Show")
        self.btn_toggle_totp.setFixedWidth(54)
        self.btn_toggle_totp.clicked.connect(self._toggle_totp_echo)
        totp_row.addWidget(self.btn_toggle_totp)
        f_layout.addLayout(totp_row)

        # TOTP Live Code Preview
        self.totp_preview_lbl = QLabel()
        self.totp_preview_lbl.setStyleSheet(f"font-size: 11px; font-weight: 600; color: {MD3_LIGHT_COLOR_HEALTHY};")
        self._update_totp_preview()
        f_layout.addWidget(self.totp_preview_lbl)

        # OAuth Refresh Token field
        f_layout.addWidget(QLabel("OAuth Refresh Token (Optional / Auto-rotated):"))
        ref_row = QHBoxLayout()
        self.ref_edit = QLineEdit(self.account.refresh_token)
        self.ref_edit.setPlaceholderText("1//0e... (leave blank to keep current)")
        self.ref_edit.setEchoMode(QLineEdit.EchoMode.Password)
        self.ref_edit.setStyleSheet(f"""
            QLineEdit {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 8px 12px;
                font-size: 12px;
                font-family: monospace;
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
        """)
        ref_row.addWidget(self.ref_edit)

        self.btn_toggle_ref = QPushButton("Show")
        self.btn_toggle_ref.setFixedWidth(54)
        self.btn_toggle_ref.clicked.connect(self._toggle_ref_echo)
        ref_row.addWidget(self.btn_toggle_ref)
        f_layout.addLayout(ref_row)

        # Active account checkbox
        self.chk_active = QCheckBox("Set as active account in Antigravity")
        self.chk_active.setChecked(self.account.is_active)
        self.chk_active.setStyleSheet(f"color: {MD3_LIGHT_TEXT_PRIMARY}; font-size: 13px;")
        f_layout.addWidget(self.chk_active)

        layout.addWidget(form_card)

        # 4. Action Buttons (Save, Delete, Cancel)
        btn_row = QHBoxLayout()
        btn_row.setSpacing(10)

        self.btn_delete = QPushButton("Delete Account")
        self.btn_delete.setStyleSheet(f"""
            QPushButton {{
                background-color: #fce8e6;
                color: {MD3_LIGHT_COLOR_EXHAUSTED};
                border: 1px solid #f5c2c7;
                border-radius: 8px;
                padding: 8px 16px;
                font-weight: 600;
                font-size: 12px;
            }}
            QPushButton:hover {{
                background-color: #fad2cf;
            }}
        """)
        self.btn_delete.clicked.connect(self._on_delete_account)
        btn_row.addWidget(self.btn_delete)

        btn_row.addStretch()

        self.btn_cancel = QPushButton("Cancel")
        self.btn_cancel.clicked.connect(self.reject)
        btn_row.addWidget(self.btn_cancel)

        self.btn_save = QPushButton("Save Changes")
        self.btn_save.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_LIGHT_ACCENT_PRIMARY};
                color: #ffffff;
                border: none;
                border-radius: 8px;
                padding: 8px 20px;
                font-weight: 600;
                font-size: 12px;
            }}
            QPushButton:hover {{
                background-color: #1a73e8;
            }}
        """)
        self.btn_save.clicked.connect(self._on_save_changes)
        btn_row.addWidget(self.btn_save)

        layout.addLayout(btn_row)

    def _toggle_totp_echo(self) -> None:
        if self.totp_edit.echoMode() == QLineEdit.EchoMode.Password:
            self.totp_edit.setEchoMode(QLineEdit.EchoMode.Normal)
            self.btn_toggle_totp.setText("Hide")
        else:
            self.totp_edit.setEchoMode(QLineEdit.EchoMode.Password)
            self.btn_toggle_totp.setText("Show")

    def _toggle_ref_echo(self) -> None:
        if self.ref_edit.echoMode() == QLineEdit.EchoMode.Password:
            self.ref_edit.setEchoMode(QLineEdit.EchoMode.Normal)
            self.btn_toggle_ref.setText("Hide")
        else:
            self.ref_edit.setEchoMode(QLineEdit.EchoMode.Password)
            self.btn_toggle_ref.setText("Show")

    def _on_totp_changed(self, text: str) -> None:
        self._update_totp_preview()

    def _update_totp_preview(self) -> None:
        raw = self.totp_edit.text().strip()
        if not raw:
            self.totp_preview_lbl.setText("No TOTP secret configured")
            self.totp_preview_lbl.setStyleSheet(f"font-size: 11px; color: {MD3_LIGHT_TEXT_SECONDARY};")
            return
        try:
            clean = sanitize_secret(raw)
            code, sec_rem = generate_totp_code(clean)
            self.totp_preview_lbl.setText(f"Live MFA Preview: {code} ({sec_rem}s remaining)")
            self.totp_preview_lbl.setStyleSheet(f"font-size: 11px; font-weight: 600; color: {MD3_LIGHT_COLOR_HEALTHY};")
        except Exception:
            self.totp_preview_lbl.setText("Invalid TOTP secret format")
            self.totp_preview_lbl.setStyleSheet(f"font-size: 11px; font-weight: 600; color: {MD3_LIGHT_COLOR_WARNING};")

    def _on_save_changes(self) -> None:
        email = self.account.email
        new_label = self.label_edit.text().strip()
        raw_totp = self.totp_edit.text().strip()
        raw_ref = self.ref_edit.text().strip()
        set_active = self.chk_active.isChecked()

        cleaned_totp = ""
        if raw_totp:
            try:
                cleaned_totp = sanitize_secret(raw_totp)
            except Exception as exc:
                QMessageBox.warning(self, "Invalid TOTP Secret", f"Could not parse TOTP secret: {exc}")
                return

        try:
            self.controller.update_account(
                email=email,
                label=new_label,
                totp_secret=cleaned_totp,
                refresh_token=raw_ref if raw_ref else None,
                set_active=set_active,
            )
            self.account_saved.emit(email)
            self.accept()
        except Exception as exc:
            QMessageBox.critical(self, "Save Failed", f"Failed to update account: {exc}")

    def _on_delete_account(self) -> None:
        confirm = QMessageBox.question(
            self,
            "Confirm Account Deletion",
            f"Are you sure you want to remove account '{self.account.email}' from the Swiss Knife vault?\nThis will not delete the Google account itself.",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
            QMessageBox.StandardButton.No,
        )
        if confirm == QMessageBox.StandardButton.Yes:
            try:
                self.controller.remove_account(self.account.email)
                self.account_removed.emit(self.account.email)
                self.accept()
            except Exception as exc:
                QMessageBox.critical(self, "Removal Failed", f"Failed to delete account: {exc}")
