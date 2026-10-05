"""
System Settings Page.
====================
System configuration, daemon status, socket paths, app access password,
and host process diagnostics.
"""

from __future__ import annotations

import os
from pathlib import Path
from typing import Optional
from PySide6.QtCore import Qt
from PySide6.QtWidgets import (
    QFrame,
    QHBoxLayout,
    QLabel,
    QLineEdit,
    QMessageBox,
    QPushButton,
    QScrollArea,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_BIN,
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_SWISS_CONFIG_DIR,
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_HEALTHY,
    MD3_LIGHT_ACCENT_PRIMARY,
    MD3_LIGHT_COLOR_EXHAUSTED,
    MD3_LIGHT_COLOR_HEALTHY,
    MD3_LIGHT_OUTLINE,
    MD3_LIGHT_SURFACE,
    MD3_LIGHT_SURFACE_CONTAINER,
    MD3_LIGHT_TEXT_PRIMARY,
    MD3_LIGHT_TEXT_SECONDARY,
    MD3_OUTLINE,
    MD3_SURFACE_CONTAINER,
    MD3_SURFACE_CONTAINER_HIGH,
    MD3_TEXT_PRIMARY,
    MD3_TEXT_SECONDARY,
)
from antigravity_swiss.core.crypto import hash_app_password, validate_app_password, verify_app_password
from antigravity_swiss.ipc.controller import SwissKnifeController


class SystemSettingsPage(QWidget):
    """
    System Settings page displaying environment paths, app access password,
    IPC socket info, and daemon health.
    """

    def __init__(
        self,
        controller: SwissKnifeController,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.controller = controller
        self.config = SwissKnifeConfig.load()
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
        header_card.setObjectName("headerCard")
        header_card.setStyleSheet(f"""
            QFrame#headerCard {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
            QFrame#headerCard QLabel {{
                border: none;
                background: transparent;
            }}
        """)
        h_layout = QHBoxLayout(header_card)
        h_vbox = QVBoxLayout()
        h_vbox.setSpacing(4)

        t1 = QLabel("SYSTEM & PROCESS SETTINGS")
        t1.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px; border: none; background: transparent;")
        t2 = QLabel("Runtime diagnostics, IPC Unix domain sockets, and Antigravity process safety shield.")
        t2.setStyleSheet(f"font-size: 13px; color: {MD3_LIGHT_TEXT_PRIMARY}; border: none; background: transparent;")
        h_vbox.addWidget(t1)
        h_vbox.addWidget(t2)
        h_layout.addLayout(h_vbox)
        c_layout.addWidget(header_card)

        # Diagnostics & Safety Shield Card
        diag_card = QFrame()
        diag_card.setObjectName("diagCard")
        diag_card.setStyleSheet(f"""
            QFrame#diagCard {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 20px;
            }}
            QFrame#diagCard QLabel {{
                border: none;
                background: transparent;
            }}
        """)
        dc_layout = QVBoxLayout(diag_card)
        dc_layout.setSpacing(14)

        dc_title = QLabel("PROCESS SAFETY SHIELD & ENVIRONMENT")
        dc_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px; border: none; background: transparent;")
        dc_layout.addWidget(dc_title)

        shield_status = QLabel("Host Process Shield: ACTIVE (Host IDE PID protected against accidental signals)")
        shield_status.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_COLOR_HEALTHY}; font-weight: 600; border: none; background: transparent;")
        dc_layout.addWidget(shield_status)

        # Fields
        env_items = [
            ("Antigravity Binary:", str(DEFAULT_ANTIGRAVITY_BIN)),
            ("Antigravity Config:", str(DEFAULT_ANTIGRAVITY_CONFIG_DIR)),
            ("Swiss Knife Config:", str(DEFAULT_SWISS_CONFIG_DIR)),
            ("Daemon IPC Socket:", f"{os.environ.get('XDG_RUNTIME_DIR', '/run/user/1000')}/antigravity-swiss/daemon.sock"),
        ]

        for label, val in env_items:
            row = QHBoxLayout()
            row.setSpacing(10)
            lbl = QLabel(label)
            lbl.setFixedWidth(160)
            lbl.setStyleSheet(f"color: {MD3_LIGHT_TEXT_SECONDARY}; font-size: 12px; font-weight: 500;")
            row.addWidget(lbl)

            inp = QLineEdit(val)
            inp.setReadOnly(True)
            inp.setStyleSheet(f"""
                QLineEdit {{
                    background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                    color: {MD3_LIGHT_TEXT_PRIMARY};
                    border: 1px solid {MD3_LIGHT_OUTLINE};
                    border-radius: 6px;
                    padding: 6px 10px;
                    font-family: monospace;
                    font-size: 11px;
                }}
            """)
            row.addWidget(inp)
            dc_layout.addLayout(row)

        c_layout.addWidget(diag_card)

        # App Access Password Protection Card
        pwd_card = QFrame()
        pwd_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 20px;
            }}
            QFrame QLabel {{
                border: none;
                background: transparent;
            }}
        """)
        pc_layout = QVBoxLayout(pwd_card)
        pc_layout.setSpacing(14)

        head_row = QHBoxLayout()
        pc_title = QLabel("APP ACCESS PASSWORD PROTECTION")
        pc_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        head_row.addWidget(pc_title)
        head_row.addStretch()

        self.status_badge = QLabel("PROTECTED" if self.config.app_password_enabled else "OPTIONAL / DISABLED")
        badge_bg = "#e6f4ea" if self.config.app_password_enabled else "#f1f3f4"
        badge_fg = MD3_LIGHT_COLOR_HEALTHY if self.config.app_password_enabled else MD3_LIGHT_TEXT_SECONDARY
        self.status_badge.setStyleSheet(f"background-color: {badge_bg}; color: {badge_fg}; font-size: 11px; font-weight: 700; padding: 4px 10px; border-radius: 12px;")
        head_row.addWidget(self.status_badge)
        pc_layout.addLayout(head_row)

        pc_desc = QLabel("Require an entry password to unlock and use Antigravity Swiss Knife. Minimum 6 characters (combinations of numbers, letters, symbols).")
        pc_desc.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_PRIMARY};")
        pc_desc.setWordWrap(True)
        pc_layout.addWidget(pc_desc)

        # Current password row (only if already enabled)
        self.curr_pwd_row = QHBoxLayout()
        lbl_curr = QLabel("Current Password:")
        lbl_curr.setFixedWidth(160)
        lbl_curr.setStyleSheet(f"color: {MD3_LIGHT_TEXT_SECONDARY}; font-size: 12px; font-weight: 500;")
        self.curr_pwd_row.addWidget(lbl_curr)
        self.curr_pwd_edit = QLineEdit()
        self.curr_pwd_edit.setEchoMode(QLineEdit.EchoMode.Password)
        self.curr_pwd_edit.setPlaceholderText("Enter current password to verify")
        self.curr_pwd_edit.setStyleSheet(self._input_style())
        self.curr_pwd_row.addWidget(self.curr_pwd_edit)
        self.btn_toggle_curr = QPushButton("👁️")
        self.btn_toggle_curr.setFixedWidth(40)
        self.btn_toggle_curr.clicked.connect(lambda: self._toggle_echo(self.curr_pwd_edit, self.btn_toggle_curr))
        self.curr_pwd_row.addWidget(self.btn_toggle_curr)
        pc_layout.addLayout(self.curr_pwd_row)

        # New password row
        new_row = QHBoxLayout()
        self.lbl_new = QLabel("New Password:" if self.config.app_password_enabled else "Set App Password:")
        self.lbl_new.setFixedWidth(160)
        self.lbl_new.setStyleSheet(f"color: {MD3_LIGHT_TEXT_SECONDARY}; font-size: 12px; font-weight: 500;")
        new_row.addWidget(self.lbl_new)
        self.new_pwd_edit = QLineEdit()
        self.new_pwd_edit.setEchoMode(QLineEdit.EchoMode.Password)
        self.new_pwd_edit.setPlaceholderText("Minimum 6 characters (numbers, letters, symbols)")
        self.new_pwd_edit.setStyleSheet(self._input_style())
        new_row.addWidget(self.new_pwd_edit)
        self.btn_toggle_new = QPushButton("👁️")
        self.btn_toggle_new.setFixedWidth(40)
        self.btn_toggle_new.clicked.connect(lambda: self._toggle_echo(self.new_pwd_edit, self.btn_toggle_new))
        new_row.addWidget(self.btn_toggle_new)
        pc_layout.addLayout(new_row)

        # Confirm password row
        conf_row = QHBoxLayout()
        lbl_conf = QLabel("Confirm Password:")
        lbl_conf.setFixedWidth(160)
        lbl_conf.setStyleSheet(f"color: {MD3_LIGHT_TEXT_SECONDARY}; font-size: 12px; font-weight: 500;")
        conf_row.addWidget(lbl_conf)
        self.conf_pwd_edit = QLineEdit()
        self.conf_pwd_edit.setEchoMode(QLineEdit.EchoMode.Password)
        self.conf_pwd_edit.setPlaceholderText("Re-enter password to confirm")
        self.conf_pwd_edit.setStyleSheet(self._input_style())
        conf_row.addWidget(self.conf_pwd_edit)
        self.btn_toggle_conf = QPushButton("👁️")
        self.btn_toggle_conf.setFixedWidth(40)
        self.btn_toggle_conf.clicked.connect(lambda: self._toggle_echo(self.conf_pwd_edit, self.btn_toggle_conf))
        conf_row.addWidget(self.btn_toggle_conf)
        pc_layout.addLayout(conf_row)

        # Action buttons
        btn_box = QHBoxLayout()
        btn_box.setSpacing(10)

        self.btn_save_pwd = QPushButton("Update Password" if self.config.app_password_enabled else "Enable App Password")
        self.btn_save_pwd.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_LIGHT_ACCENT_PRIMARY};
                color: #ffffff;
                border: none;
                border-radius: 8px;
                padding: 8px 18px;
                font-weight: 600;
                font-size: 12px;
            }}
            QPushButton:hover {{
                background-color: #1a73e8;
            }}
        """)
        self.btn_save_pwd.clicked.connect(self._on_save_password)
        btn_box.addWidget(self.btn_save_pwd)

        self.btn_remove_pwd = QPushButton("Remove Password")
        self.btn_remove_pwd.setStyleSheet(f"""
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
        self.btn_remove_pwd.setVisible(self.config.app_password_enabled)
        self.btn_remove_pwd.clicked.connect(self._on_remove_password)
        btn_box.addWidget(self.btn_remove_pwd)
        btn_box.addStretch()

        pc_layout.addLayout(btn_box)
        c_layout.addWidget(pwd_card)

        # Update visibility of current password row
        self._update_password_ui_state()

        scroll.setWidget(container)
        main_layout.addWidget(scroll)

    def _input_style(self) -> str:
        return f"""
            QLineEdit {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                color: {MD3_LIGHT_TEXT_PRIMARY};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 7px 12px;
                font-size: 12px;
            }}
        """

    def _toggle_echo(self, edit: QLineEdit, btn: QPushButton) -> None:
        if edit.echoMode() == QLineEdit.EchoMode.Password:
            edit.setEchoMode(QLineEdit.EchoMode.Normal)
            btn.setText("🙈")
        else:
            edit.setEchoMode(QLineEdit.EchoMode.Password)
            btn.setText("👁️")

    def _update_password_ui_state(self) -> None:
        is_en = self.config.app_password_enabled
        self.status_badge.setText("PROTECTED" if is_en else "OPTIONAL / DISABLED")
        badge_bg = "#e6f4ea" if is_en else "#f1f3f4"
        badge_fg = MD3_LIGHT_COLOR_HEALTHY if is_en else MD3_LIGHT_TEXT_SECONDARY
        self.status_badge.setStyleSheet(f"background-color: {badge_bg}; color: {badge_fg}; font-size: 11px; font-weight: 700; padding: 4px 10px; border-radius: 12px;")

        self.btn_remove_pwd.setVisible(is_en)
        self.btn_save_pwd.setText("Update Password" if is_en else "Enable App Password")
        self.lbl_new.setText("New Password (Optional):" if is_en else "Set App Password:")

        for i in range(self.curr_pwd_row.count()):
            widget = self.curr_pwd_row.itemAt(i).widget()
            if widget:
                widget.setVisible(is_en)

    def _on_save_password(self) -> None:
        new_pwd = self.new_pwd_edit.text()
        conf_pwd = self.conf_pwd_edit.text()
        curr_pwd = self.curr_pwd_edit.text()

        if self.config.app_password_enabled and self.config.app_password_hash:
            if not verify_app_password(curr_pwd, self.config.app_password_hash):
                QMessageBox.warning(self, "Password Error", "Current password is incorrect.")
                return

        valid, msg = validate_app_password(new_pwd)
        if not valid:
            QMessageBox.warning(self, "Invalid Password", msg)
            return

        if new_pwd != conf_pwd:
            QMessageBox.warning(self, "Password Mismatch", "New passwords do not match.")
            return

        self.config.app_password_hash = hash_app_password(new_pwd)
        self.config.app_password_enabled = True
        self.config.save_settings()

        self.curr_pwd_edit.clear()
        self.new_pwd_edit.clear()
        self.conf_pwd_edit.clear()
        self._update_password_ui_state()

        QMessageBox.information(self, "Success", "Application access password updated successfully.")

    def _on_remove_password(self) -> None:
        curr_pwd = self.curr_pwd_edit.text()
        if not verify_app_password(curr_pwd, self.config.app_password_hash):
            QMessageBox.warning(self, "Password Error", "Please enter your current password to remove protection.")
            return

        confirm = QMessageBox.question(
            self,
            "Remove Password",
            "Are you sure you want to disable application access password protection?",
            QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
        )
        if confirm == QMessageBox.StandardButton.Yes:
            self.config.app_password_enabled = False
            self.config.app_password_hash = ""
            self.config.save_settings()

            self.curr_pwd_edit.clear()
            self.new_pwd_edit.clear()
            self.conf_pwd_edit.clear()
            self._update_password_ui_state()
            QMessageBox.information(self, "Removed", "Application access password protection removed.")
