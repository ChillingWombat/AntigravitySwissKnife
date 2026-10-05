"""
Application Unlock Dialog.
==========================
Modal authentication dialog presented at startup when app_password_enabled is active.
"""

from __future__ import annotations

from typing import Optional
from PySide6.QtCore import Qt
from PySide6.QtWidgets import (
    QDialog,
    QFrame,
    QHBoxLayout,
    QLabel,
    QLineEdit,
    QPushButton,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.core.constants import (
    MD3_LIGHT_ACCENT_PRIMARY,
    MD3_LIGHT_COLOR_EXHAUSTED,
    MD3_LIGHT_OUTLINE,
    MD3_LIGHT_SURFACE,
    MD3_LIGHT_SURFACE_CONTAINER,
    MD3_LIGHT_TEXT_PRIMARY,
    MD3_LIGHT_TEXT_SECONDARY,
)
from antigravity_swiss.core.crypto import verify_app_password


class AppUnlockDialog(QDialog):
    """Prompt for application entry password."""

    def __init__(
        self,
        password_hash: str,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.password_hash = password_hash
        self.setWindowTitle("Unlock Antigravity Swiss Knife")
        self.setFixedSize(380, 240)
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
        layout.setContentsMargins(28, 24, 28, 24)
        layout.setSpacing(14)

        title = QLabel("Antigravity Swiss Knife")
        title.setStyleSheet(f"font-size: 16px; font-weight: 700; color: {MD3_LIGHT_TEXT_PRIMARY};")
        title.setAlignment(Qt.AlignmentFlag.AlignCenter)
        layout.addWidget(title)

        desc = QLabel("Enter your access password to unlock the application:")
        desc.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_SECONDARY};")
        desc.setAlignment(Qt.AlignmentFlag.AlignCenter)
        desc.setWordWrap(True)
        layout.addWidget(desc)

        self.error_lbl = QLabel("")
        self.error_lbl.setStyleSheet(f"color: {MD3_LIGHT_COLOR_EXHAUSTED}; font-size: 11px; font-weight: 600;")
        self.error_lbl.setAlignment(Qt.AlignmentFlag.AlignCenter)
        self.error_lbl.setVisible(False)
        layout.addWidget(self.error_lbl)

        pwd_row = QHBoxLayout()
        self.pwd_edit = QLineEdit()
        self.pwd_edit.setPlaceholderText("Access password")
        self.pwd_edit.setEchoMode(QLineEdit.EchoMode.Password)
        self.pwd_edit.setStyleSheet(f"""
            QLineEdit {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                color: {MD3_LIGHT_TEXT_PRIMARY};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 8px;
                padding: 8px 12px;
                font-size: 13px;
            }}
        """)
        self.pwd_edit.returnPressed.connect(self._on_unlock)
        pwd_row.addWidget(self.pwd_edit)

        self.btn_toggle = QPushButton("👁️")
        self.btn_toggle.setFixedWidth(40)
        self.btn_toggle.setToolTip("Show / Hide password")
        self.btn_toggle.clicked.connect(self._toggle_echo)
        pwd_row.addWidget(self.btn_toggle)
        layout.addLayout(pwd_row)

        btn_row = QHBoxLayout()
        btn_row.setSpacing(10)

        self.btn_cancel = QPushButton("Exit")
        self.btn_cancel.clicked.connect(self.reject)
        btn_row.addWidget(self.btn_cancel)

        self.btn_unlock = QPushButton("Unlock")
        self.btn_unlock.setStyleSheet(f"""
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
        self.btn_unlock.clicked.connect(self._on_unlock)
        btn_row.addWidget(self.btn_unlock)

        layout.addLayout(btn_row)

    def _toggle_echo(self) -> None:
        if self.pwd_edit.echoMode() == QLineEdit.EchoMode.Password:
            self.pwd_edit.setEchoMode(QLineEdit.EchoMode.Normal)
            self.btn_toggle.setText("🙈")
        else:
            self.pwd_edit.setEchoMode(QLineEdit.EchoMode.Password)
            self.btn_toggle.setText("👁️")

    def _on_unlock(self) -> None:
        candidate = self.pwd_edit.text()
        if not candidate:
            self.error_lbl.setText("Please enter your password.")
            self.error_lbl.setVisible(True)
            return

        if verify_app_password(candidate, self.password_hash):
            self.accept()
        else:
            self.error_lbl.setText("Incorrect password. Please try again.")
            self.error_lbl.setVisible(True)
            self.pwd_edit.selectAll()
