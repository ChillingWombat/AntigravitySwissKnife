"""
Antigravity Swiss Knife GUI Custom Widgets.
===========================================
Google Gemini Material Design 3 interactive components.
"""

from antigravity_swiss.gui.widgets.circular_gauge import CircularGauge
from antigravity_swiss.gui.widgets.countdown_ring import CountdownRing
from antigravity_swiss.gui.widgets.nav_rail import NavigationRail
from antigravity_swiss.gui.widgets.top_ribbon import TopRibbon

CircularGaugeWidget = CircularGauge
TotpCountdownRingWidget = CountdownRing

__all__ = [
    "CircularGauge",
    "CircularGaugeWidget",
    "CountdownRing",
    "TotpCountdownRingWidget",
    "NavigationRail",
    "TopRibbon",
]

