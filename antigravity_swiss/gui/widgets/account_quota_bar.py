"""
Account Quota Horizontal Progress Bar Widget.
==============================================
Renders a sleek Material Design 3 horizontal progress bar
paired with a crisp percentage text number for table cells.
"""

from __future__ import annotations

from PySide6.QtCore import QRect, QSize, Qt
from PySide6.QtGui import QColor, QPainter
from PySide6.QtWidgets import (
    QLabel,
    QProgressBar,
    QWidget,
)

from antigravity_swiss.core.constants import (
    MD3_LIGHT_COLOR_EXHAUSTED,
    MD3_LIGHT_COLOR_HEALTHY,
    MD3_LIGHT_COLOR_WARNING,
    MD3_LIGHT_SURFACE_CONTAINER_HIGH,
    MD3_LIGHT_TEXT_PRIMARY,
)


class AccountQuotaBarWidget(QWidget):
    """
    Horizontal progress bar paired with crisp percentage label.
    e.g. [████████░░] 85%
    """

    def __init__(self, fraction: float = 1.0, parent: QWidget | None = None) -> None:
        super().__init__(parent)
        self.setAttribute(Qt.WidgetAttribute.WA_StyledBackground, True)
        self.setStyleSheet("background: transparent; border: none;")
        self._fraction = max(0.0, min(1.0, float(fraction)))
        self.setMinimumHeight(24)

        pct = int(round(self._fraction * 100))

        # Retain bar and lbl for test introspection & property access
        self.bar = QProgressBar(self)
        self.bar.setRange(0, 100)
        self.bar.setValue(pct)
        self.bar.hide()

        self.lbl = QLabel(f"{pct}%", self)
        self.lbl.hide()

    @property
    def fraction(self) -> float:
        return self._fraction

    def set_fraction(self, fraction: float) -> None:
        self._fraction = max(0.0, min(1.0, float(fraction)))
        pct = int(round(self._fraction * 100))
        self.bar.setValue(pct)
        self.lbl.setText(f"{pct}%")
        self.update()

    def sizeHint(self) -> QSize:
        return QSize(150, 32)

    def minimumSizeHint(self) -> QSize:
        return QSize(100, 24)

    def paintEvent(self, event) -> None:
        painter = QPainter(self)
        painter.setRenderHint(QPainter.RenderHint.Antialiasing)

        w = self.width()
        h = self.height()
        pct = int(round(self._fraction * 100))
        pct_text = f"{pct}%"

        text_w = 42
        margin_x = 4
        spacing = 10
        bar_w = max(20, w - text_w - margin_x * 2 - spacing)
        bar_h = 8
        bar_y = (h - bar_h) // 2

        # 1. Background track
        painter.setPen(Qt.PenStyle.NoPen)
        painter.setBrush(QColor("#e5e9f0"))
        painter.drawRoundedRect(margin_x, bar_y, bar_w, bar_h, 4, 4)

        # 2. Fill chunk
        if self._fraction >= 0.50:
            chunk_color = QColor("#137333")  # Google green
        elif self._fraction >= 0.20:
            chunk_color = QColor("#b06000")  # Warning amber
        else:
            chunk_color = QColor("#b3261e")  # Exhausted red

        fill_w = int(bar_w * self._fraction)
        if fill_w > 0:
            painter.setBrush(chunk_color)
            painter.drawRoundedRect(margin_x, bar_y, fill_w, bar_h, 4, 4)

        # 3. Percentage text
        font = painter.font()
        font.setPointSize(10)
        font.setBold(True)
        painter.setFont(font)
        painter.setPen(QColor("#1e293b"))

        text_x = margin_x + bar_w + spacing
        text_rect = QRect(text_x, 0, text_w, h)
        painter.drawText(text_rect, Qt.AlignmentFlag.AlignRight | Qt.AlignmentFlag.AlignVCenter, pct_text)
        painter.end()
