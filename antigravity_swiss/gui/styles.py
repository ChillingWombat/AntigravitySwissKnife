"""
Google Gemini / Material Design 3 Dark Theme Stylesheet and Design Tokens.
==========================================================================
Matches Google Gemini dark mode aesthetics:
- Surface: #131314
- Container / Cards: #1e1f20
- High Container / Hover: #282a2c
- Outline / Border: #3c4043
- Accent Primary: #8ab4f8 (Gemini blue)
- Text Primary: #e3e3e3
- Text Secondary: #9aa0a6
- Healthy Green: #81c995
- Warning Yellow: #fdd663
- Exhausted Red: #f28b82
"""

from antigravity_swiss.core.constants import (
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_EXHAUSTED,
    MD3_COLOR_HEALTHY,
    MD3_COLOR_WARNING,
    MD3_OUTLINE,
    MD3_RADIUS_CARD,
    MD3_RADIUS_PILL,
    MD3_SURFACE,
    MD3_SURFACE_CONTAINER,
    MD3_SURFACE_CONTAINER_HIGH,
    MD3_TEXT_PRIMARY,
    MD3_TEXT_SECONDARY,
)

GEMINI_QSS = f"""
QMainWindow, QDialog {{
    background-color: {MD3_SURFACE};
    color: {MD3_TEXT_PRIMARY};
    font-family: 'Google Sans', 'Roboto', 'Segoe UI', sans-serif;
}}

QWidget {{
    background-color: transparent;
    color: {MD3_TEXT_PRIMARY};
    font-family: 'Google Sans', 'Roboto', 'Segoe UI', sans-serif;
}}

/* Surface Containers / Cards */
QFrame.gemini-card {{
    background-color: {MD3_SURFACE_CONTAINER};
    border: 1px solid {MD3_OUTLINE};
    border-radius: {MD3_RADIUS_CARD}px;
    padding: 16px;
}}

QFrame.gemini-card:hover {{
    border: 1px solid {MD3_ACCENT_PRIMARY};
}}

/* Push Buttons (Pill shape) */
QPushButton {{
    background-color: {MD3_SURFACE_CONTAINER_HIGH};
    color: {MD3_TEXT_PRIMARY};
    border: 1px solid {MD3_OUTLINE};
    border-radius: {MD3_RADIUS_PILL}px;
    padding: 8px 18px;
    font-weight: 500;
    font-size: 13px;
}}

QPushButton:hover {{
    background-color: #333538;
    border-color: {MD3_ACCENT_PRIMARY};
    color: #ffffff;
}}

QPushButton:pressed {{
    background-color: #3c4043;
}}

QPushButton.primary-btn {{
    background-color: {MD3_ACCENT_PRIMARY};
    color: #041e42;
    border: none;
    font-weight: 600;
}}

QPushButton.primary-btn:hover {{
    background-color: #a8c7fa;
    color: #041e42;
}}

QPushButton.danger-btn {{
    background-color: #3b2323;
    color: {MD3_COLOR_EXHAUSTED};
    border: 1px solid #5c2b29;
}}

QPushButton.danger-btn:hover {{
    background-color: #4f2929;
    color: #ffb4ab;
}}

/* Navigation Rail Buttons */
QPushButton.nav-rail-btn {{
    background-color: transparent;
    border: none;
    border-radius: 12px;
    padding: 12px;
    color: {MD3_TEXT_SECONDARY};
    text-align: center;
}}

QPushButton.nav-rail-btn:hover {{
    background-color: {MD3_SURFACE_CONTAINER_HIGH};
    color: {MD3_TEXT_PRIMARY};
}}

QPushButton.nav-rail-btn[active="true"] {{
    background-color: #2b394f;
    color: {MD3_ACCENT_PRIMARY};
}}

/* Top Ribbon Tabs (Pill Tabs) */
QPushButton.ribbon-tab {{
    background-color: transparent;
    border: none;
    border-radius: 16px;
    padding: 8px 16px;
    color: {MD3_TEXT_SECONDARY};
    font-size: 13px;
    font-weight: 500;
}}

QPushButton.ribbon-tab:hover {{
    background-color: {MD3_SURFACE_CONTAINER_HIGH};
    color: {MD3_TEXT_PRIMARY};
}}

QPushButton.ribbon-tab[active="true"] {{
    background-color: #2b394f;
    color: {MD3_ACCENT_PRIMARY};
    font-weight: 600;
}}

/* Text Inputs / LineEdits */
QLineEdit, QTextEdit, QPlainTextEdit {{
    background-color: {MD3_SURFACE_CONTAINER};
    color: {MD3_TEXT_PRIMARY};
    border: 1px solid {MD3_OUTLINE};
    border-radius: 8px;
    padding: 8px 12px;
    selection-background-color: {MD3_ACCENT_PRIMARY};
    selection-color: #041e42;
}}

QLineEdit:focus, QTextEdit:focus, QPlainTextEdit:focus {{
    border: 2px solid {MD3_ACCENT_PRIMARY};
}}

/* ScrollBars */
QScrollBar:vertical {{
    background: transparent;
    width: 8px;
    margin: 0px;
}}

QScrollBar::handle:vertical {{
    background: {MD3_OUTLINE};
    min-height: 20px;
    border-radius: 4px;
}}

QScrollBar::handle:vertical:hover {{
    background: {MD3_TEXT_SECONDARY};
}}

QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical {{
    height: 0px;
}}

/* Labels */
QLabel.heading-1 {{
    font-size: 20px;
    font-weight: 600;
    color: {MD3_TEXT_PRIMARY};
}}

QLabel.heading-2 {{
    font-size: 16px;
    font-weight: 600;
    color: {MD3_TEXT_PRIMARY};
}}

QLabel.caption {{
    font-size: 12px;
    color: {MD3_TEXT_SECONDARY};
}}

QLabel.code-display {{
    font-family: 'JetBrains Mono', 'Fira Code', 'Courier New', monospace;
    font-size: 26px;
    font-weight: 700;
    letter-spacing: 4px;
    color: {MD3_ACCENT_PRIMARY};
}}

/* Sliders */
QSlider::groove:horizontal {{
    height: 6px;
    background: {MD3_SURFACE_CONTAINER_HIGH};
    border-radius: 3px;
}}

QSlider::sub-page:horizontal {{
    background: {MD3_ACCENT_PRIMARY};
    border-radius: 3px;
}}

QSlider::handle:horizontal {{
    background: {MD3_ACCENT_PRIMARY};
    border: 2px solid #ffffff;
    width: 16px;
    margin-top: -5px;
    margin-bottom: -5px;
    border-radius: 8px;
}}
"""
