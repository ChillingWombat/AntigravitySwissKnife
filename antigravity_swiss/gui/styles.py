"""
Google Gemini / Material Design 3 Light & Dark Theme Stylesheets and Design Tokens.
==================================================================================
Matches Google Gemini Material Design 3 aesthetics with full Light Theme support:
- Light Surface: #f0f4f9
- Light Container / Cards: #ffffff
- Light High Container / Hover: #e9eef6
- Light Outline / Border: #d3dbe5
- Light Accent Primary: #0b57d0 (Google blue)
- Light Accent Container: #c2e7ff
- Light Accent On Container: #001d35
- Light Text Primary: #1f1f1f
- Light Text Secondary: #444746
- Light Healthy Green: #137333
- Light Warning Yellow: #b06000
- Light Exhausted Red: #b3261e
"""

from antigravity_swiss.core.constants import (
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_EXHAUSTED,
    MD3_COLOR_HEALTHY,
    MD3_COLOR_WARNING,
    MD3_LIGHT_ACCENT_CONTAINER,
    MD3_LIGHT_ACCENT_ON_CONTAINER,
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
    MD3_OUTLINE,
    MD3_RADIUS_CARD,
    MD3_RADIUS_PILL,
    MD3_SURFACE,
    MD3_SURFACE_CONTAINER,
    MD3_SURFACE_CONTAINER_HIGH,
    MD3_TEXT_PRIMARY,
    MD3_TEXT_SECONDARY,
)

# Material Design 3 Light Theme (Google AI Studio aesthetic)
GEMINI_LIGHT_QSS = f"""
/* Google AI Studio & Gemini Theme Tokens */

QMainWindow, QDialog, QWidget#centralWidget {{
    background-color: {MD3_LIGHT_SURFACE};
    color: {MD3_LIGHT_TEXT_PRIMARY};
    font-family: 'Google Sans', 'Roboto', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}}

QWidget {{
    color: {MD3_LIGHT_TEXT_PRIMARY};
    font-family: 'Google Sans', 'Roboto', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}}

/* Surface Containers / Cards */
QFrame.gemini-card {{
    background-color: #ffffff;
    border: 1px solid {MD3_LIGHT_OUTLINE};
    border-radius: {MD3_RADIUS_CARD}px;
    padding: 20px;
}}

QFrame.gemini-card:hover {{
    border-color: #bcc6d4;
}}

/* Push Buttons (Google Material 3 Pill shape) */
QPushButton {{
    background-color: #ffffff;
    color: {MD3_LIGHT_TEXT_PRIMARY};
    border: 1px solid #747775;
    border-radius: {MD3_RADIUS_PILL}px;
    padding: 7px 18px;
    font-weight: 500;
    font-size: 13px;
}}

QPushButton:hover {{
    background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
    border-color: {MD3_LIGHT_ACCENT_PRIMARY};
    color: {MD3_LIGHT_ACCENT_PRIMARY};
}}

QPushButton:pressed {{
    background-color: #dfe3eb;
}}

QPushButton.primary-btn {{
    background-color: {MD3_LIGHT_ACCENT_PRIMARY};
    color: #ffffff;
    border: none;
    border-radius: {MD3_RADIUS_PILL}px;
    font-weight: 600;
}}

QPushButton.primary-btn:hover {{
    background-color: #0842a0;
    color: #ffffff;
}}

QPushButton.tonal-btn {{
    background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
    color: {MD3_LIGHT_ACCENT_ON_CONTAINER};
    border: none;
    border-radius: {MD3_RADIUS_PILL}px;
    font-weight: 600;
}}

QPushButton.tonal-btn:hover {{
    background-color: {MD3_LIGHT_ACCENT_CONTAINER};
}}

QPushButton.danger-btn {{
    background-color: #fce8e6;
    color: {MD3_LIGHT_COLOR_EXHAUSTED};
    border: 1px solid #fad2cf;
    border-radius: {MD3_RADIUS_PILL}px;
    font-weight: 600;
}}

QPushButton.danger-btn:hover {{
    background-color: #fad2cf;
    color: #8c1d18;
}}

/* Navigation Rail Buttons */
QPushButton.nav-rail-btn {{
    background-color: transparent;
    border: none;
    border-radius: 20px;
    padding: 10px 16px;
    color: {MD3_LIGHT_TEXT_SECONDARY};
    text-align: left;
    font-size: 13px;
    font-weight: 500;
}}

QPushButton.nav-rail-btn:hover {{
    background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
    color: {MD3_LIGHT_TEXT_PRIMARY};
}}

QPushButton.nav-rail-btn[active="true"] {{
    background-color: {MD3_LIGHT_ACCENT_CONTAINER};
    color: {MD3_LIGHT_ACCENT_ON_CONTAINER};
    font-weight: 600;
}}

/* Vertical Separator Line between Left Panel and Feature Pages */
QFrame#railVerticalSeparator {{
    background-color: {MD3_LIGHT_OUTLINE};
    min-width: 1px;
    max-width: 1px;
    border: none;
}}

/* Left Navigation Rail Frame */
QFrame#navigationRail {{
    background-color: #ffffff;
    border: none;
}}

/* Top Ribbon Bar Frame */
QFrame#topRibbon {{
    background-color: #ffffff;
    border-bottom: 1px solid {MD3_LIGHT_OUTLINE};
}}

/* Top Ribbon Tabs (Google Segmented Pill Tabs) */
QPushButton.ribbon-tab {{
    background-color: transparent;
    border: none;
    border-radius: 16px;
    padding: 6px 16px;
    color: {MD3_LIGHT_TEXT_SECONDARY};
    font-size: 12px;
    font-weight: 500;
}}

QPushButton.ribbon-tab:hover {{
    color: {MD3_LIGHT_TEXT_PRIMARY};
    background-color: rgba(0, 0, 0, 0.04);
}}

QPushButton.ribbon-tab[active="true"] {{
    background-color: #ffffff;
    color: {MD3_LIGHT_ACCENT_PRIMARY};
    font-weight: 600;
    border: 1px solid {MD3_LIGHT_OUTLINE};
}}

/* Text Inputs / LineEdits (Google Outlined TextFields) */
QLineEdit, QTextEdit, QPlainTextEdit {{
    background-color: #ffffff;
    color: {MD3_LIGHT_TEXT_PRIMARY};
    border: 1px solid #747775;
    border-radius: 8px;
    padding: 8px 12px;
    font-size: 13px;
    selection-background-color: {MD3_LIGHT_ACCENT_CONTAINER};
    selection-color: {MD3_LIGHT_ACCENT_ON_CONTAINER};
}}

QLineEdit:focus, QTextEdit:focus, QPlainTextEdit:focus {{
    border: 2px solid {MD3_LIGHT_ACCENT_PRIMARY};
}}

/* ComboBoxes */
QComboBox {{
    background-color: #ffffff;
    color: {MD3_LIGHT_TEXT_PRIMARY};
    border: 1px solid #747775;
    border-radius: 8px;
    padding: 6px 12px;
    font-size: 13px;
}}

QComboBox:focus {{
    border: 2px solid {MD3_LIGHT_ACCENT_PRIMARY};
}}

QComboBox QAbstractItemView {{
    background-color: #ffffff;
    border: 1px solid {MD3_LIGHT_OUTLINE};
    border-radius: 8px;
    selection-background-color: {MD3_LIGHT_ACCENT_CONTAINER};
    selection-color: {MD3_LIGHT_ACCENT_ON_CONTAINER};
}}

/* Data Tables (Material 3 Flat Data Table) */
QTableWidget {{
    background-color: #ffffff;
    gridline-color: transparent;
    border: none;
    color: {MD3_LIGHT_TEXT_PRIMARY};
    outline: none;
}}

QHeaderView::section {{
    background-color: #f8fafd;
    color: {MD3_LIGHT_TEXT_SECONDARY};
    padding: 10px 14px;
    font-weight: 600;
    font-size: 11px;
    letter-spacing: 0.5px;
    text-transform: uppercase;
    border: none;
    border-bottom: 1px solid {MD3_LIGHT_OUTLINE};
}}

QTableWidget::item {{
    padding: 10px 14px;
    border-bottom: 1px solid #f2f2f2;
}}

QTableWidget::item:selected {{
    background-color: #e8f0fe;
    color: {MD3_LIGHT_TEXT_PRIMARY};
}}

/* CheckBoxes */
QCheckBox {{
    spacing: 8px;
    font-size: 13px;
    color: {MD3_LIGHT_TEXT_PRIMARY};
}}

QCheckBox::indicator {{
    width: 18px;
    height: 18px;
    border-radius: 4px;
    border: 1px solid #747775;
    background-color: #ffffff;
}}

QCheckBox::indicator:hover {{
    border-color: {MD3_LIGHT_ACCENT_PRIMARY};
}}

QCheckBox::indicator:checked {{
    background-color: {MD3_LIGHT_ACCENT_PRIMARY};
    border-color: {MD3_LIGHT_ACCENT_PRIMARY};
}}

/* ScrollBars */
QScrollBar:vertical {{
    background: transparent;
    width: 8px;
    margin: 0px;
}}

QScrollBar::handle:vertical {{
    background: #c4c7c5;
    min-height: 24px;
    border-radius: 4px;
}}

QScrollBar::handle:vertical:hover {{
    background: #8e918f;
}}

QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical {{
    height: 0px;
}}

/* Labels */
QLabel.heading-1 {{
    font-size: 20px;
    font-weight: 600;
    color: {MD3_LIGHT_TEXT_PRIMARY};
}}

QLabel.heading-2 {{
    font-size: 16px;
    font-weight: 600;
    color: {MD3_LIGHT_TEXT_PRIMARY};
}}

QLabel.caption {{
    font-size: 12px;
    color: {MD3_LIGHT_TEXT_SECONDARY};
}}

QLabel.code-display {{
    font-family: 'JetBrains Mono', 'Roboto Mono', 'Fira Code', 'Courier New', monospace;
    font-size: 28px;
    font-weight: 700;
    letter-spacing: 5px;
    color: {MD3_LIGHT_ACCENT_PRIMARY};
}}

/* Sliders */
QSlider::groove:horizontal {{
    height: 6px;
    background: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
    border-radius: 3px;
}}

QSlider::sub-page:horizontal {{
    background: {MD3_LIGHT_ACCENT_PRIMARY};
    border-radius: 3px;
}}

QSlider::handle:horizontal {{
    background: {MD3_LIGHT_ACCENT_PRIMARY};
    border: 2px solid #ffffff;
    width: 18px;
    height: 18px;
    margin-top: -6px;
    margin-bottom: -6px;
    border-radius: 9px;
}}
"""

# Default QSS supports tokens
GEMINI_QSS = f"/* Theme Tokens: {MD3_SURFACE} {MD3_SURFACE_CONTAINER} {MD3_ACCENT_PRIMARY} */\n" + GEMINI_LIGHT_QSS
