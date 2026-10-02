"""
GUI Application Launcher.
=========================
Initializes PySide6 QApplication, applies Google Gemini theme, and starts the event loop.
"""

from __future__ import annotations

import sys
from typing import Optional
from PySide6.QtWidgets import QApplication

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.gui.main_window import MainWindow
from antigravity_swiss.gui.styles import GEMINI_QSS
from antigravity_swiss.ipc.controller import create_controller, SwissKnifeController


def create_app(
    controller: Optional[SwissKnifeController] = None,
    config: Optional[SwissKnifeConfig] = None,
) -> tuple[QApplication, MainWindow]:
    """Creates QApplication and MainWindow instances."""
    app = QApplication.instance()
    if app is None:
        app = QApplication(sys.argv)

    app.setStyleSheet(GEMINI_QSS)

    ctrl = controller or create_controller(config=config, prefer_daemon=True)
    window = MainWindow(controller=ctrl)
    return app, window


def run_gui(
    controller: Optional[SwissKnifeController] = None,
    config: Optional[SwissKnifeConfig] = None,
) -> int:
    """Run GUI event loop."""
    app, window = create_app(controller=controller, config=config)
    window.show()
    return app.exec()


def run_app(
    standalone: bool = False,
    config: Optional[SwissKnifeConfig] = None,
) -> int:
    """Entry point for CLI 'python -m antigravity_swiss gui'."""
    cfg = config or SwissKnifeConfig.load()
    ctrl = create_controller(config=cfg, prefer_daemon=not standalone)
    return run_gui(controller=ctrl, config=cfg)


if __name__ == "__main__":
    sys.exit(run_app())
