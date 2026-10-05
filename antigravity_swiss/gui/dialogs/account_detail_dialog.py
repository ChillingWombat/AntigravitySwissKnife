"""
Account Detail & Edit Dialog.
=============================
Modal window allowing users to inspect account quota horizons,
modify friendly labels, set account priority (High, Mid, Low),
manage vault password, TOTP MFA secrets, OAuth refresh tokens
(with Google OAuth extraction button), and record account notes.
"""

from __future__ import annotations

import threading
from typing import Optional
from PySide6.QtCore import Qt, Signal, QObject, QTimer
from PySide6.QtWidgets import (
    QApplication,
    QCheckBox,
    QComboBox,
    QDialog,
    QFrame,
    QHBoxLayout,
    QLabel,
    QLineEdit,
    QMessageBox,
    QPlainTextEdit,
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
from antigravity_swiss.keyring.google_oauth import GoogleOAuthExtractor
from antigravity_swiss.quota.calculator import AccountQuotaState
from antigravity_swiss.totp.engine import generate_totp_code, sanitize_secret


class OAuthExtractionBridge(QObject):
    """Bridge for cross-thread OAuth extraction signals."""
    success = Signal(dict)
    error = Signal(str)


class AccountDetailDialog(QDialog):
    """
    Pop-up detail window for an individual managed account.
    Allows editing label, priority, password, TOTP seed, refresh token,
    and account notes.
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
        self.resize(560, 680)
        self.setModal(True)
        self.setStyleSheet(f"""
            QDialog {{
                background-color: {MD3_LIGHT_SURFACE};
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
        """)
        self._bridge = OAuthExtractionBridge()
        self._bridge.success.connect(self._on_oauth_extracted)
        self._bridge.error.connect(self._on_oauth_error)
        self._current_totp_raw = ""
        self._init_ui()
        self._totp_timer = QTimer(self)
        self._totp_timer.setInterval(1000)
        self._totp_timer.timeout.connect(self._update_totp_preview)
        self._totp_timer.start()

    def _init_ui(self) -> None:
        layout = QVBoxLayout(self)
        layout.setContentsMargins(24, 20, 24, 20)
        layout.setSpacing(14)

        # 1. Header Card: Email & Status
        head_card = QFrame()
        head_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 14px;
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
        st = (getattr(self.account, "status", "") or ("ACTIVE" if self.account.is_active else "STANDBY")).upper()
        if st == "BANNED":
            badge_text = "BANNED"
            badge_bg = "#fce8e6"
            badge_color = "#b3261e"
        elif st == "ERROR":
            badge_text = "ERROR"
            badge_bg = "#fef7e0"
            badge_color = "#b06000"
        elif self.account.is_active:
            badge_text = "ACTIVE"
            badge_bg = "#e6f4ea"
            badge_color = MD3_LIGHT_COLOR_HEALTHY
        else:
            badge_text = "STANDBY"
            badge_bg = MD3_LIGHT_SURFACE_CONTAINER_HIGH
            badge_color = MD3_LIGHT_TEXT_SECONDARY

        badge = QLabel(badge_text)
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

        if st == "BANNED":
            alert_box = QLabel("⚠️ BANNED: Account suspended upstream. Appeal with Google or delete/discard this account.")
            alert_box.setWordWrap(True)
            alert_box.setStyleSheet("background-color: #fce8e6; color: #b3261e; border: 1px solid #fad2cf; border-radius: 8px; padding: 6px 10px; font-size: 11px; font-weight: 600;")
            head_layout.addWidget(alert_box)
        elif st == "ERROR":
            alert_box = QLabel("⚠️ ERROR: Authentication or verification required. Re-authenticate or update refresh token.")
            alert_box.setWordWrap(True)
            alert_box.setStyleSheet("background-color: #fef7e0; color: #b06000; border: 1px solid #feefc3; border-radius: 8px; padding: 6px 10px; font-size: 11px; font-weight: 600;")
            head_layout.addWidget(alert_box)

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
                padding: 12px;
            }}
        """)
        q_layout = QVBoxLayout(quota_card)
        q_layout.setSpacing(8)

        q_head = QLabel("LIVE QUOTA METRICS")
        q_head.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        q_layout.addWidget(q_head)

        # 5h Quota Bar
        row_5h = QVBoxLayout()
        row_5h.setSpacing(2)
        lbl_5h = QLabel(f"Next 5-Hour Available Quota (effective capacity: {self.account.reset_horizon_text}):")
        lbl_5h.setStyleSheet(f"font-size: 11px; color: {MD3_LIGHT_TEXT_PRIMARY};")
        bar_5h = AccountQuotaBarWidget(self.account.quota_5h_available)
        row_5h.addWidget(lbl_5h)
        row_5h.addWidget(bar_5h)
        q_layout.addLayout(row_5h)

        # Weekly Quota Bar
        row_wk = QVBoxLayout()
        row_wk.setSpacing(2)
        lbl_wk = QLabel("Weekly Horizon Quota Left:")
        lbl_wk.setStyleSheet(f"font-size: 11px; color: {MD3_LIGHT_TEXT_PRIMARY};")
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
                padding: 14px;
            }}
        """)
        f_layout = QVBoxLayout(form_card)
        f_layout.setSpacing(10)

        f_head = QLabel("ACCOUNT CREDENTIALS & METADATA")
        f_head.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        f_layout.addWidget(f_head)

        # Account Alias field
        alias_prio_row = QHBoxLayout()
        alias_col = QVBoxLayout()
        alias_col.setSpacing(4)
        alias_col.addWidget(QLabel("Account Alias:"))
        self.label_edit = QLineEdit(self.account.label)
        self.label_edit.setPlaceholderText("e.g. Account 1, Primary, Backup")
        self.label_edit.setStyleSheet(self._input_style())
        alias_col.addWidget(self.label_edit)
        alias_prio_row.addLayout(alias_col, 2)

        # Account Priority field directly below / alongside alias
        prio_col = QVBoxLayout()
        prio_col.setSpacing(4)
        prio_col.addWidget(QLabel("Account Priority:"))
        self.priority_combo = QComboBox()
        self.priority_combo.addItems(["High", "Mid", "Low"])
        curr_prio = (getattr(self.account, "priority", "High") or "High").strip().capitalize()
        p_idx = self.priority_combo.findText(curr_prio)
        self.priority_combo.setCurrentIndex(p_idx if p_idx >= 0 else 0)
        self.priority_combo.setStyleSheet(f"""
            QComboBox {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 7px 12px;
                font-size: 13px;
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
        """)
        prio_col.addWidget(self.priority_combo)
        alias_prio_row.addLayout(prio_col, 1)
        f_layout.addLayout(alias_prio_row)

        # Optional Password (Vault) field (Hidden by default)
        f_layout.addWidget(QLabel("Password (Optional / Vault):"))
        pwd_row = QHBoxLayout()
        self.pwd_edit = QLineEdit(getattr(self.account, "password", "") or "")
        self.pwd_edit.setPlaceholderText("Optional login password or vault credential")
        self.pwd_edit.setEchoMode(QLineEdit.EchoMode.Password)
        self.pwd_edit.setStyleSheet(self._input_style())
        pwd_row.addWidget(self.pwd_edit)

        self.btn_toggle_pwd = QPushButton("Show")
        self.btn_toggle_pwd.setToolTip("Show / Hide Password")
        self.btn_toggle_pwd.setFixedWidth(54)
        self.btn_toggle_pwd.clicked.connect(self._toggle_pwd_echo)
        pwd_row.addWidget(self.btn_toggle_pwd)
        f_layout.addLayout(pwd_row)

        # OAuth Refresh Token field (Hidden by default + Google Login extraction button)
        f_layout.addWidget(QLabel("OAuth Refresh Token (Auto-rotated):"))
        ref_row = QHBoxLayout()
        self.ref_edit = QLineEdit(self.account.refresh_token)
        self.ref_edit.setPlaceholderText("1//0e... (or click Sign in with Google)")
        self.ref_edit.setEchoMode(QLineEdit.EchoMode.Password)
        self.ref_edit.setStyleSheet(self._input_style(mono=True))
        ref_row.addWidget(self.ref_edit)

        self.btn_toggle_ref = QPushButton("Show")
        self.btn_toggle_ref.setToolTip("Show / Hide OAuth Token")
        self.btn_toggle_ref.setFixedWidth(54)
        self.btn_toggle_ref.clicked.connect(self._toggle_ref_echo)
        ref_row.addWidget(self.btn_toggle_ref)

        self.btn_extract_oauth = QPushButton("Sign in with Google")
        self.btn_extract_oauth.setToolTip("Open browser to log into Google and extract OAuth token")
        self.btn_extract_oauth.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_LIGHT_ACCENT_CONTAINER};
                color: #0b57d0;
                border: 1px solid #a8c7fa;
                border-radius: 8px;
                padding: 7px 12px;
                font-size: 12px;
                font-weight: 600;
            }}
            QPushButton:hover {{
                background-color: #d3e3fd;
            }}
        """)
        self.btn_extract_oauth.clicked.connect(self._start_google_login)
        ref_row.addWidget(self.btn_extract_oauth)
        f_layout.addLayout(ref_row)

        # MFA / TOTP Secret Key field (Hidden by default) with header verification code & copy button
        totp_header_row = QHBoxLayout()
        totp_header_row.addWidget(QLabel("MFA / TOTP Secret Key (Base32):"))
        totp_header_row.addStretch()

        self.totp_code_display = QLabel("--- ---")
        self.totp_code_display.setStyleSheet(f"""
            QLabel {{
                font-family: monospace;
                font-size: 12px;
                font-weight: 500;
                letter-spacing: 1px;
                color: {MD3_LIGHT_TEXT_SECONDARY};
                background-color: #f1f3f4;
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 6px;
                padding: 2px 8px;
            }}
        """)
        totp_header_row.addWidget(self.totp_code_display)

        self.btn_copy_totp = QPushButton("Copy")
        self.btn_copy_totp.setToolTip("Copy 6-digit verification code directly")
        self.btn_copy_totp.setFixedHeight(22)
        self.btn_copy_totp.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 6px;
                padding: 2px 8px;
                font-size: 11px;
                font-weight: 600;
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
            QPushButton:hover {{
                background-color: #e8f0fe;
                color: {MD3_LIGHT_ACCENT_PRIMARY};
            }}
            QPushButton:disabled {{
                opacity: 0.5;
                color: {MD3_LIGHT_TEXT_SECONDARY};
            }}
        """)
        self.btn_copy_totp.clicked.connect(self._copy_totp_code)
        totp_header_row.addWidget(self.btn_copy_totp)
        f_layout.addLayout(totp_header_row)
        totp_row = QHBoxLayout()
        self.totp_edit = QLineEdit(self.account.totp_secret)
        self.totp_edit.setPlaceholderText("JBSWY3DPEHPK3PXP or otpauth://totp/...")
        self.totp_edit.setEchoMode(QLineEdit.EchoMode.Password)
        self.totp_edit.setStyleSheet(self._input_style(mono=True))
        self.totp_edit.textChanged.connect(self._on_totp_changed)
        totp_row.addWidget(self.totp_edit)

        self.btn_toggle_totp = QPushButton("Show")
        self.btn_toggle_totp.setToolTip("Show / Hide MFA Secret")
        self.btn_toggle_totp.setFixedWidth(54)
        self.btn_toggle_totp.clicked.connect(self._toggle_totp_echo)
        totp_row.addWidget(self.btn_toggle_totp)
        f_layout.addLayout(totp_row)

        # TOTP Live Code Preview
        self.totp_preview_lbl = QLabel()
        self.totp_preview_lbl.setStyleSheet(f"font-size: 11px; font-weight: 600; color: {MD3_LIGHT_COLOR_HEALTHY};")
        self._update_totp_preview()
        f_layout.addWidget(self.totp_preview_lbl)

        # Status & Active controls row
        st_row = QHBoxLayout()
        st_col = QVBoxLayout()
        st_col.setSpacing(4)
        st_col.addWidget(QLabel("Account Status:"))
        self.status_combo = QComboBox()
        self.status_combo.addItems(["STANDBY", "ACTIVE", "ERROR", "BANNED"])
        curr_st = (getattr(self.account, "status", "") or ("ACTIVE" if self.account.is_active else "STANDBY")).upper()
        idx = self.status_combo.findText(curr_st)
        if idx >= 0:
            self.status_combo.setCurrentIndex(idx)
        self.status_combo.setStyleSheet(f"""
            QComboBox {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 6px 12px;
                font-size: 12px;
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
        """)
        st_col.addWidget(self.status_combo)
        st_row.addLayout(st_col, 1)

        act_col = QVBoxLayout()
        act_col.addStretch()
        self.chk_active = QCheckBox("Set as active account")
        self.chk_active.setChecked(self.account.is_active)
        self.chk_active.setStyleSheet(f"color: {MD3_LIGHT_TEXT_PRIMARY}; font-size: 12px;")
        act_col.addWidget(self.chk_active)
        st_row.addLayout(act_col, 1)
        f_layout.addLayout(st_row)

        # Account Notes section at bottom
        f_layout.addWidget(QLabel("Account Notes:"))
        self.notes_edit = QPlainTextEdit(getattr(self.account, "notes", "") or "")
        self.notes_edit.setPlaceholderText("Write notes, reminders, or usage tags for this account...")
        self.notes_edit.setFixedHeight(72)
        self.notes_edit.setStyleSheet(f"""
            QPlainTextEdit {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 8px 10px;
                font-size: 12px;
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
        """)
        f_layout.addWidget(self.notes_edit)

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

    def _input_style(self, mono: bool = False) -> str:
        font_family = "monospace" if mono else "sans-serif"
        return f"""
            QLineEdit {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 7px 12px;
                font-size: 12px;
                font-family: {font_family};
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
        """

    def _toggle_pwd_echo(self) -> None:
        if self.pwd_edit.echoMode() == QLineEdit.EchoMode.Password:
            self.pwd_edit.setEchoMode(QLineEdit.EchoMode.Normal)
            self.btn_toggle_pwd.setText("Hide")
        else:
            self.pwd_edit.setEchoMode(QLineEdit.EchoMode.Password)
            self.btn_toggle_pwd.setText("Show")

    def _toggle_ref_echo(self) -> None:
        if self.ref_edit.echoMode() == QLineEdit.EchoMode.Password:
            self.ref_edit.setEchoMode(QLineEdit.EchoMode.Normal)
            self.btn_toggle_ref.setText("Hide")
        else:
            self.ref_edit.setEchoMode(QLineEdit.EchoMode.Password)
            self.btn_toggle_ref.setText("Show")

    def _toggle_totp_echo(self) -> None:
        if self.totp_edit.echoMode() == QLineEdit.EchoMode.Password:
            self.totp_edit.setEchoMode(QLineEdit.EchoMode.Normal)
            self.btn_toggle_totp.setText("Hide")
        else:
            self.totp_edit.setEchoMode(QLineEdit.EchoMode.Password)
            self.btn_toggle_totp.setText("Show")

    def _on_totp_changed(self, text: str) -> None:
        self._update_totp_preview()

    def _copy_totp_code(self) -> None:
        raw_code = getattr(self, "_current_totp_raw", "")
        if not raw_code:
            return
        QApplication.clipboard().setText(raw_code)
        self.btn_copy_totp.setText("Copied!")
        QTimer.singleShot(1500, lambda: self.btn_copy_totp.setText("Copy"))

    def _update_totp_preview(self) -> None:
        raw = self.totp_edit.text().strip()
        if not raw:
            self._current_totp_raw = ""
            if hasattr(self, "totp_code_display"):
                self.totp_code_display.setText("--- ---")
                self.totp_code_display.setStyleSheet(f"""
                    QLabel {{
                        font-family: monospace;
                        font-size: 12px;
                        font-weight: 500;
                        letter-spacing: 1px;
                        color: {MD3_LIGHT_TEXT_SECONDARY};
                        background-color: #f1f3f4;
                        border: 1px solid {MD3_LIGHT_OUTLINE};
                        border-radius: 6px;
                        padding: 2px 8px;
                    }}
                """)
            if hasattr(self, "btn_copy_totp"):
                self.btn_copy_totp.setEnabled(False)
            self.totp_preview_lbl.setText("No TOTP secret configured")
            self.totp_preview_lbl.setStyleSheet(f"font-size: 11px; color: {MD3_LIGHT_TEXT_SECONDARY};")
            return
        try:
            clean = sanitize_secret(raw)
            code, sec_rem = generate_totp_code(clean)
            self._current_totp_raw = code
            formatted = f"{code[:3]} {code[3:]}" if len(code) == 6 else code
            if hasattr(self, "totp_code_display"):
                self.totp_code_display.setText(formatted)
                self.totp_code_display.setStyleSheet(f"""
                    QLabel {{
                        font-family: monospace;
                        font-size: 12px;
                        font-weight: 700;
                        letter-spacing: 1px;
                        color: {MD3_LIGHT_COLOR_HEALTHY};
                        background-color: #e6f4ea;
                        border: 1px solid #ceead6;
                        border-radius: 6px;
                        padding: 2px 8px;
                    }}
                """)
            if hasattr(self, "btn_copy_totp"):
                self.btn_copy_totp.setEnabled(True)
            self.totp_preview_lbl.setText(f"Live MFA Preview: {code} ({sec_rem}s remaining)")
            self.totp_preview_lbl.setStyleSheet(f"font-size: 11px; font-weight: 600; color: {MD3_LIGHT_COLOR_HEALTHY};")
        except Exception:
            self._current_totp_raw = ""
            if hasattr(self, "totp_code_display"):
                self.totp_code_display.setText("--- ---")
                self.totp_code_display.setStyleSheet(f"""
                    QLabel {{
                        font-family: monospace;
                        font-size: 12px;
                        font-weight: 500;
                        letter-spacing: 1px;
                        color: {MD3_LIGHT_COLOR_EXHAUSTED};
                        background-color: #fce8e6;
                        border: 1px solid #fad2cf;
                        border-radius: 6px;
                        padding: 2px 8px;
                    }}
                """)
            if hasattr(self, "btn_copy_totp"):
                self.btn_copy_totp.setEnabled(False)
            self.totp_preview_lbl.setText("Invalid TOTP secret format")
            self.totp_preview_lbl.setStyleSheet(f"font-size: 11px; font-weight: 600; color: {MD3_LIGHT_COLOR_WARNING};")

    def _start_google_login(self) -> None:
        self.btn_extract_oauth.setEnabled(False)
        self.btn_extract_oauth.setText("Waiting for login...")

        def _worker() -> None:
            try:
                extractor = GoogleOAuthExtractor()
                res = extractor.start_flow(timeout_seconds=120.0, open_browser=True)
                self._bridge.success.emit(res)
            except Exception as exc:
                self._bridge.error.emit(str(exc))

        thread = threading.Thread(target=_worker, daemon=True)
        thread.start()

    def _on_oauth_extracted(self, res: dict) -> None:
        self.btn_extract_oauth.setEnabled(True)
        self.btn_extract_oauth.setText("Sign in with Google")
        token = res.get("refresh_token", "")
        if token:
            self.ref_edit.setText(token)
            QMessageBox.information(
                self,
                "Token Extracted",
                f"Successfully extracted OAuth refresh token for {res.get('email', self.account.email)}!",
            )

    def _on_oauth_error(self, err_msg: str) -> None:
        self.btn_extract_oauth.setEnabled(True)
        self.btn_extract_oauth.setText("Sign in with Google")
        QMessageBox.warning(self, "Google Login Failed", f"Could not extract OAuth token:\n{err_msg}")

    def _on_save_changes(self) -> None:
        email = self.account.email
        new_label = self.label_edit.text().strip()
        new_priority = self.priority_combo.currentText()
        raw_pwd = self.pwd_edit.text()
        raw_totp = self.totp_edit.text().strip()
        raw_ref = self.ref_edit.text().strip()
        notes = self.notes_edit.toPlainText().strip()
        set_active = self.chk_active.isChecked()

        cleaned_totp = ""
        if raw_totp:
            try:
                cleaned_totp = sanitize_secret(raw_totp)
            except Exception as exc:
                QMessageBox.warning(self, "Invalid TOTP Secret", f"Could not parse TOTP secret: {exc}")
                return

        new_status = self.status_combo.currentText()
        if new_status == "ACTIVE":
            set_active = True
        elif new_status in ("BANNED", "ERROR"):
            set_active = False

        try:
            self.controller.update_account(
                email=email,
                label=new_label,
                status=new_status,
                priority=new_priority,
                password=raw_pwd if raw_pwd else None,
                notes=notes,
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
