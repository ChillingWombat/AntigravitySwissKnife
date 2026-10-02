"""
TOTP Animated Countdown Ring Widget.
====================================
Smooth circular countdown ring displaying remaining validity seconds for RFC 6238 TOTP codes.
Features QPainter vector rendering, QTimer auto-refresh, and code expiration signaling.
"""

from __future__ import annotations

from typing import Optional
from PySide6.QtCore import Qt, QRectF, QSize, QTimer, Signal
from PySide6.QtGui import QColor, QFont, QPaintEvent, QPainter, QPen
from PySide6.QtWidgets import QWidget

from antigravity_swiss.core.constants import (
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_EXHAUSTED,
    MD3_COLOR_WARNING,
    MD3_SURFACE_CONTAINER_HIGH,
    MD3_TEXT_PRIMARY,
)
from antigravity_swiss.totp.engine import TotpEngine, TOTPEngine


class CountdownRing(QWidget):
    """
    Circular countdown ring for TOTP token validity (30-second standard step).
    Signals:
        time_expired: Emitted when the 30-second window completes and a new code is needed.
        tick: Emitted each update with (seconds_remaining, fraction_remaining).
    """

    time_expired = Signal()
    tick = Signal(int, float)

    def __init__(
        self,
        secret: str = "",
        interval_seconds: int = 30,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self._secret = secret.strip().replace(" ", "").upper()
        self._interval = interval_seconds
        self._current_code = ""
        self._remaining_sec = interval_seconds
        self._progress = 1.0  # 1.0 down to 0.0

        self.setFixedSize(54, 54)

        # High-frequency timer for smooth countdown ring animation (every 250ms)
        self._timer = QTimer(self)
        self._timer.setInterval(250)
        self._timer.timeout.connect(self._on_timer_tick)
        self._timer.start()

        self._refresh_state()

    @property
    def secret(self) -> str:
        return self._secret

    @secret.setter
    def secret(self, val: str) -> None:
        self._secret = val.strip().replace(" ", "").upper()
        self._refresh_state()

    @property
    def current_code(self) -> str:
        return self._current_code

    @property
    def remaining_seconds(self) -> int:
        return self._remaining_sec

    @property
    def fraction(self) -> float:
        return self._progress

    def set_progress(self, remaining_seconds: int, fraction: float) -> None:
        """Manually sets progress state and triggers update."""
        self._remaining_sec = max(0, int(remaining_seconds))
        self._progress = max(0.0, min(1.0, float(fraction)))
        self.update()


    def _refresh_state(self) -> None:
        if self._secret:
            try:
                res = TotpEngine.get_current_totp(self._secret, step_seconds=self._interval)
                prev_code = self._current_code
                self._current_code = res.code
                self._remaining_sec = res.remaining_seconds
                self._progress = res.progress_fraction

                if prev_code and prev_code != self._current_code:
                    self.time_expired.emit()
            except Exception:
                self._current_code = "------"
                self._remaining_sec = 0
                self._progress = 0.0
        else:
            self._current_code = ""
            self._remaining_sec = 0
            self._progress = 0.0

        self.tick.emit(self._remaining_sec, self._progress)
        self.update()

    def _on_timer_tick(self) -> None:
        self._refresh_state()

    def sizeHint(self) -> QSize:
        return QSize(54, 54)

    def paintEvent(self, event: QPaintEvent) -> None:
        painter = QPainter(self)
        painter.setRenderHint(QPainter.RenderHint.Antialiasing)

        width = self.width()
        height = self.height()
        radius = min(width, height) / 2.0
        center_x = width / 2.0
        center_y = height / 2.0

        track_width = 4.0
        rect = QRectF(
            center_x - radius + track_width,
            center_y - radius + track_width,
            (radius - track_width) * 2,
            (radius - track_width) * 2,
        )

        # Background track
        track_pen = QPen(QColor(MD3_SURFACE_CONTAINER_HIGH), track_width)
        painter.setPen(track_pen)
        painter.drawArc(rect, 0, 360 * 16)

        # Arc Color changes as time runs out
        if self._remaining_sec > 8:
            arc_color = QColor(MD3_ACCENT_PRIMARY)
        elif self._remaining_sec > 4:
            arc_color = QColor(MD3_COLOR_WARNING)
        else:
            arc_color = QColor(MD3_COLOR_EXHAUSTED)

        # Progress Arc
        if self._progress > 0:
            arc_pen = QPen(arc_color, track_width)
            arc_pen.setCapStyle(Qt.PenCapStyle.RoundCap)
            painter.setPen(arc_pen)
            start_angle = 90 * 16
            span_angle = int(self._progress * 360 * 16)
            painter.drawArc(rect, start_angle, span_angle)

        # Center Seconds Display (e.g. 23s)
        painter.setPen(QColor(MD3_TEXT_PRIMARY))
        font_sec = QFont("Roboto", 9, QFont.Weight.Bold)
        painter.setFont(font_sec)
        sec_text = f"{self._remaining_sec}s" if self._secret else "--"
        painter.drawText(
            QRectF(0, 0, width, height),
            Qt.AlignmentFlag.AlignCenter,
            sec_text,
        )

        painter.end()
