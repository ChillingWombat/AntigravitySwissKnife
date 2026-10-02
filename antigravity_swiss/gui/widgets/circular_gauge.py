"""
Circular Quota Gauge Widget.
===========================
Vector-rendered circular gauge using QPainter for model quota visualization.
Features smooth antialiasing, color-coded health tiers, and reset horizon countdown.
"""

from __future__ import annotations

from PySide6.QtCore import Qt, QRectF, QSize
from PySide6.QtGui import QColor, QFont, QPaintEvent, QPainter, QPen
from PySide6.QtWidgets import QWidget, QVBoxLayout, QLabel

from antigravity_swiss.core.constants import (
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_EXHAUSTED,
    MD3_COLOR_HEALTHY,
    MD3_COLOR_WARNING,
    MD3_OUTLINE,
    MD3_SURFACE_CONTAINER_HIGH,
    MD3_TEXT_PRIMARY,
    MD3_TEXT_SECONDARY,
)


class CircularGauge(QWidget):
    """
    Circular gauge displaying remaining quota percentage for a model.
    Colors dynamically adapt:
    - > 30%: Healthy (Green #81c995)
    - 10% - 30%: Warning (Yellow #fdd663)
    - < 10%: Exhausted (Red #f28b82)
    """

    def __init__(
        self,
        model_name: str = "Gemini Model",
        fraction: float = 1.0,
        reset_text: str = "",
        parent: QWidget | None = None,
    ) -> None:
        super().__init__(parent)
        self._model_name = model_name
        self._fraction = max(0.0, min(1.0, fraction))
        self._reset_text = reset_text

        self.setMinimumSize(140, 160)
        self.setSizePolicy(
            QWidget.sizePolicy(self).horizontalPolicy(),
            QWidget.sizePolicy(self).verticalPolicy(),
        )

    @property
    def fraction(self) -> float:
        return self._fraction

    @fraction.setter
    def fraction(self, val: float) -> None:
        self._fraction = max(0.0, min(1.0, float(val)))
        self.update()

    @property
    def model_name(self) -> str:
        return self._model_name

    @model_name.setter
    def model_name(self, val: str) -> None:
        self._model_name = val
        self.update()

    @property
    def reset_text(self) -> str:
        return self._reset_text

    @reset_text.setter
    def reset_text(self, val: str) -> None:
        self._reset_text = val
        self.update()

    def sizeHint(self) -> QSize:
        return QSize(160, 180)

    def get_status_color(self, fraction: float | None = None) -> str:
        """Returns the hex color corresponding to healthy, warning, or exhausted quota."""
        f = self._fraction if fraction is None else fraction
        if f > 0.30:
            return MD3_COLOR_HEALTHY
        elif f >= 0.10:
            return MD3_COLOR_WARNING
        else:
            return MD3_COLOR_EXHAUSTED

    def paintEvent(self, event: QPaintEvent) -> None:
        painter = QPainter(self)
        painter.setRenderHint(QPainter.RenderHint.Antialiasing)

        width = self.width()
        height = self.height()
        side = min(width, height - 40)
        radius = side / 2.0
        center_x = width / 2.0
        center_y = radius + 8.0

        track_width = max(8.0, side * 0.08)
        rect = QRectF(
            center_x - radius + track_width,
            center_y - radius + track_width,
            (radius - track_width) * 2,
            (radius - track_width) * 2,
        )

        # Draw Background Track
        track_pen = QPen(QColor(MD3_SURFACE_CONTAINER_HIGH), track_width)
        track_pen.setCapStyle(Qt.PenCapStyle.RoundCap)
        painter.setPen(track_pen)
        painter.drawArc(rect, 0, 360 * 16)

        # Determine Progress Arc Color
        arc_color = QColor(self.get_status_color())


        # Draw Foreground Progress Arc (starts at 90 deg = 12 o'clock, clockwise negative span)
        if self._fraction > 0:
            arc_pen = QPen(arc_color, track_width)
            arc_pen.setCapStyle(Qt.PenCapStyle.RoundCap)
            painter.setPen(arc_pen)
            start_angle = 90 * 16
            span_angle = int(-self._fraction * 360 * 16)
            painter.drawArc(rect, start_angle, span_angle)

        # Draw Center Percentage Text
        percent_str = f"{int(round(self._fraction * 100))}%"
        painter.setPen(QColor(MD3_TEXT_PRIMARY))
        font_pct = QFont("Google Sans", int(side * 0.18), QFont.Weight.Bold)
        painter.setFont(font_pct)
        painter.drawText(
            QRectF(center_x - radius, center_y - 18, radius * 2, 28),
            Qt.AlignmentFlag.AlignCenter,
            percent_str,
        )

        # Draw Fraction Subtext (e.g. 0.85)
        painter.setPen(QColor(MD3_TEXT_SECONDARY))
        font_sub = QFont("Roboto", int(side * 0.08))
        painter.setFont(font_sub)
        fraction_str = f"{self._fraction:.2f} remaining"
        painter.drawText(
            QRectF(center_x - radius, center_y + 12, radius * 2, 20),
            Qt.AlignmentFlag.AlignCenter,
            fraction_str,
        )

        # Draw Model Name at bottom
        painter.setPen(QColor(MD3_TEXT_PRIMARY))
        font_title = QFont("Google Sans", 11, QFont.Weight.DemiBold)
        painter.setFont(font_title)
        painter.drawText(
            QRectF(0, height - 36, width, 18),
            Qt.AlignmentFlag.AlignCenter,
            self._model_name,
        )

        # Draw Reset Horizon text at very bottom
        if self._reset_text:
            painter.setPen(QColor(MD3_TEXT_SECONDARY))
            font_reset = QFont("Roboto", 9)
            painter.setFont(font_reset)
            painter.drawText(
                QRectF(0, height - 18, width, 16),
                Qt.AlignmentFlag.AlignCenter,
                self._reset_text,
            )

        painter.end()
