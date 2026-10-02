"""
Antigravity Swiss Knife GUI Package.
===================================
Google Gemini / Material Design 3 Dark Mode Desktop Interface.
"""

from antigravity_swiss.gui.app import create_app, run_app, run_gui
from antigravity_swiss.gui.main_window import MainWindow
from antigravity_swiss.gui.styles import GEMINI_QSS
from antigravity_swiss.gui.tray import SwissKnifeTray

__all__ = ["MainWindow", "SwissKnifeTray", "create_app", "run_app", "run_gui", "GEMINI_QSS"]
