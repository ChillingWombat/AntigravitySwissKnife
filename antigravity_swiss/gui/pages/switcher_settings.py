"""
Switcher Settings Page.
=======================
Configuration page for:
- Auto-switch threshold slider and anti-thrash cooldown
- Upstream quota polling intervals
- Post-reset keep-alive warmup automation toggles
"""

from __future__ import annotations

from typing import Any, Optional
from PySide6.QtCore import Qt, Signal
from PySide6.QtWidgets import (
    QCheckBox,
    QComboBox,
    QFrame,
    QHBoxLayout,
    QLabel,
    QMessageBox,
    QPushButton,
    QScrollArea,
    QSlider,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.core.constants import (
    DEFAULT_AUTO_SWITCH_THRESHOLD_FRACTION,
    DEFAULT_POLLING_INTERVAL_SECONDS,
    DEFAULT_WARMUP_LEAD_TIME_SECONDS,
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_HEALTHY,
    MD3_LIGHT_ACCENT_PRIMARY,
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


class SwitcherSettingsPage(QWidget):
    """
    Switcher settings page allowing live adjustment of thresholds and warmup rules.
    """

    settings_saved = Signal()

    def __init__(
        self,
        controller: SwissKnifeController,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.controller = controller
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

        t1 = QLabel("ACCOUNT SWITCHER & WARMUP SETTINGS")
        t1.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        t2 = QLabel("Tune auto-rotation rules, anti-thrash hysteresis, and keep-alive triggers.")
        t2.setStyleSheet(f"font-size: 13px; color: {MD3_LIGHT_TEXT_PRIMARY};")
        h_vbox.addWidget(t1)
        h_vbox.addWidget(t2)
        h_layout.addLayout(h_vbox)

        h_layout.addStretch()

        self._save_btn = QPushButton("Save Configuration")
        self._save_btn.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_LIGHT_ACCENT_PRIMARY};
                color: #ffffff;
                border: none;
                border-radius: 6px;
                padding: 8px 20px;
                font-size: 12px;
                font-weight: 600;
            }}
            QPushButton:hover {{
                background-color: #1a73e8;
            }}
        """)
        self._save_btn.clicked.connect(self._on_save_settings)
        h_layout.addWidget(self._save_btn)
        c_layout.addWidget(header_card)

        # Section 1: Auto-Switch Rule Engine Card
        rules_card = QFrame()
        rules_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 20px;
            }}
        """)
        rc_layout = QVBoxLayout(rules_card)
        rc_layout.setSpacing(16)

        rc_title = QLabel("AUTO-SWITCH TRIGGER RULES")
        rc_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        rc_layout.addWidget(rc_title)

        # Auto Switch Enable Toggle
        self._chk_auto_switch = QCheckBox("Enable Automatic Keyring Account Switching")
        self._chk_auto_switch.setChecked(True)
        self._chk_auto_switch.setStyleSheet(f"font-size: 14px; font-weight: 600; color: {MD3_LIGHT_TEXT_PRIMARY};")
        rc_layout.addWidget(self._chk_auto_switch)

        # Threshold Slider
        thresh_vbox = QVBoxLayout()
        thresh_vbox.setSpacing(6)

        thresh_header = QHBoxLayout()
        th_lbl = QLabel("Switch Threshold (% Quota Remaining):")
        th_lbl.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_PRIMARY};")
        self._thresh_val_lbl = QLabel("5%")
        self._thresh_val_lbl.setStyleSheet(f"font-size: 13px; font-weight: 700; color: {MD3_LIGHT_ACCENT_PRIMARY};")
        thresh_header.addWidget(th_lbl)
        thresh_header.addStretch()
        thresh_header.addWidget(self._thresh_val_lbl)
        thresh_vbox.addLayout(thresh_header)

        self._slider_thresh = QSlider(Qt.Orientation.Horizontal)
        self._slider_thresh.setRange(1, 30)
        self._slider_thresh.setValue(5)
        self._slider_thresh.valueChanged.connect(lambda v: self._thresh_val_lbl.setText(f"{v}%"))
        thresh_vbox.addWidget(self._slider_thresh)
        rc_layout.addLayout(thresh_vbox)

        # Polling Interval
        poll_row = QHBoxLayout()
        poll_lbl = QLabel("Quota Polling Frequency:")
        poll_lbl.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_PRIMARY};")
        poll_row.addWidget(poll_lbl)

        self._poll_combo = QComboBox()
        self._poll_combo.addItems(["Every 30 seconds", "Every 60 seconds (Default)", "Every 120 seconds", "Every 300 seconds"])
        self._poll_combo.setCurrentIndex(1)
        self._poll_combo.setStyleSheet(f"""
            QComboBox {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                color: {MD3_LIGHT_TEXT_PRIMARY};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 6px;
                padding: 6px 12px;
                font-size: 12px;
            }}
        """)
        poll_row.addWidget(self._poll_combo)
        poll_row.addStretch()
        rc_layout.addLayout(poll_row)

        c_layout.addWidget(rules_card)

        # Section 2: Post-Reset Horizon Keep-Alive Warmup Card
        warmup_card = QFrame()
        warmup_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 20px;
            }}
        """)
        wc_layout = QVBoxLayout(warmup_card)
        wc_layout.setSpacing(16)

        wc_title = QLabel("POST-RESET HORIZON KEEP-ALIVE WARMUP")
        wc_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px;")
        wc_layout.addWidget(wc_title)

        self._chk_warmup = QCheckBox("Enable Automated 1-Token Keep-Alive Ping")
        self._chk_warmup.setChecked(True)
        self._chk_warmup.setStyleSheet(f"font-size: 14px; font-weight: 600; color: {MD3_LIGHT_TEXT_PRIMARY};")
        wc_layout.addWidget(self._chk_warmup)

        warmup_desc = QLabel(
            "Immediately enters the next quota reset horizon upon resetTime arrival by issuing a lightweight\n"
            "ping (maxOutputTokens: 1) without requiring user interaction or wasting quota."
        )
        warmup_desc.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_SECONDARY}; line-height: 1.4;")
        wc_layout.addWidget(warmup_desc)

        lead_row = QHBoxLayout()
        lead_lbl = QLabel("Warmup Lead Time Ahead of Horizon:")
        lead_lbl.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_PRIMARY};")
        lead_row.addWidget(lead_lbl)

        self._lead_combo = QComboBox()
        self._lead_combo.addItems(["1.0 second", "2.0 seconds (Recommended)", "3.0 seconds", "5.0 seconds"])
        self._lead_combo.setCurrentIndex(1)
        self._lead_combo.setStyleSheet(f"""
            QComboBox {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                color: {MD3_LIGHT_TEXT_PRIMARY};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 6px;
                padding: 6px 12px;
                font-size: 12px;
            }}
        """)
        lead_row.addWidget(self._lead_combo)
        lead_row.addStretch()
        wc_layout.addLayout(lead_row)

        c_layout.addWidget(warmup_card)

        scroll.setWidget(container)
        main_layout.addWidget(scroll)

        self.load_config()

    def load_config(self) -> None:
        """Load current rule config from controller."""
        try:
            cfg = self.controller.get_rule_config()
            auto_sw = cfg.get("auto_switch_enabled", True)
            thresh = int(round(cfg.get("auto_switch_threshold", 0.05) * 100))

            self._chk_auto_switch.setChecked(auto_sw)
            self._slider_thresh.setValue(thresh)
            self._thresh_val_lbl.setText(f"{thresh}%")
        except Exception:
            pass

    def _on_save_settings(self) -> None:
        auto_sw = self._chk_auto_switch.isChecked()
        thresh_val = self._slider_thresh.value() / 100.0

        try:
            self.controller.set_rule_config(
                auto_switch_enabled=auto_sw,
                auto_switch_threshold=thresh_val,
            )
            QMessageBox.information(self, "Settings Saved", "Configuration saved and applied successfully.")
            self.settings_saved.emit()
        except Exception as exc:
            QMessageBox.critical(self, "Save Error", f"Failed to save settings: {exc}")
