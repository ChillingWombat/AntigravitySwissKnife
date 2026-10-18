#!/usr/bin/env python3
"""Regenerate the app icon set from the ORIGINAL artwork.

Source of truth: assets/logo_with_background.png (1024x1024, white squircle
with a soft shadow on a very light gray canvas). The icon is the squircle
itself, cropped to its bounds, with the outer canvas keyed to transparency so
the icon keeps the original white background, original knife proportions
(~80% of the squircle width) and the original rounded corners.

Every size is downsampled directly from the ~636px crop with Lanczos, which
keeps small taskbar sizes (16/24/32) crisp instead of resampling a derived
256px image.

Run:  python scripts/regen_icons.py
"""
import os

from PIL import Image

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(ROOT, "assets", "logo_with_background.png")
OUT_DIR = os.path.join(ROOT, "assets", "icons")

# The squircle occupies (194,174)-(829,809) inside the 1024 canvas. Measured
# from the source: white bbox x 194..829 (636 px wide), y 174..809 (636 tall).
SQUIRCLE_BOX = (194, 174, 830, 810)

# Outer canvas colour of the source artwork; everything this close to it is
# keyed out so the icon has transparent corners on any taskbar colour.
OUTER_BG = (249, 249, 251)

PNG_SIZES = [16, 24, 32, 48, 64, 128, 256, 512, 1024]
ICO_SIZES = [(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
FAVICON_SIZES = [(16, 16), (32, 32), (48, 48)]
TRAY_SIZES = [16, 24, 32, 48]


def squircle_alpha(crop):
    """Alpha derived from the distance to the outer canvas colour.

    The squircle fill (white) and every artwork pixel (knife, band, arc)
    differ from the light gray canvas, so they stay fully opaque; the flat
    outer canvas becomes transparent and antialiased edges get partial alpha.
    """
    px = crop.load()
    w, h = crop.size
    alpha = Image.new("L", (w, h), 0)
    ap = alpha.load()
    for y in range(h):
        for x in range(w):
            r, g, b = px[x, y][:3]
            d = max(abs(r - OUTER_BG[0]), abs(g - OUTER_BG[1]), abs(b - OUTER_BG[2]))
            if d <= 0:
                continue
            # The white squircle is only 6 levels away from the canvas, so a
            # 6-level ramp keeps the fill opaque while feathering the edge.
            ap[x, y] = 255 if d >= 6 else int(d / 6.0 * 255 + 0.5)
    return alpha


def tray_knife(crop):
    """Extract the knife glyph on transparency for notification-area icons.

    Tray icons sit on a white flyout (and on dark taskbars), so the white
    squircle background of the app icon would either disappear or look like a
    blank tile. Flood-fill the white background from the crop border: only the
    enclosed white areas (the "G" inside the red band) stay opaque.
    """
    from collections import deque

    import numpy as np

    arr = np.array(crop.convert("RGB")).astype(int)
    h, w = arr.shape[:2]
    white = arr.min(axis=2) >= 244
    outside = np.zeros((h, w), dtype=bool)
    q = deque()
    for x in range(w):
        for y in (0, h - 1):
            if white[y, x] and not outside[y, x]:
                outside[y, x] = True
                q.append((x, y))
    for y in range(h):
        for x in (0, w - 1):
            if white[y, x] and not outside[y, x]:
                outside[y, x] = True
                q.append((x, y))
    while q:
        x, y = q.popleft()
        for nx, ny in ((x + 1, y), (x - 1, y), (x, y + 1), (x, y - 1)):
            if 0 <= nx < w and 0 <= ny < h and white[ny, nx] and not outside[ny, nx]:
                outside[ny, nx] = True
                q.append((nx, ny))

    alpha = np.where(outside, 0, 255).astype("uint8")
    rgba = np.dstack([arr.astype("uint8"), alpha])
    glyph = Image.fromarray(rgba, "RGBA")
    return glyph.crop(glyph.getchannel("A").getbbox())


def tray_icon(glyph, size, fill=0.92):
    """Center the glyph on a transparent square, filling `fill` of the canvas."""
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    target = max(1, int(round(size * fill)))
    scale = target / max(glyph.size)
    nw, nh = max(1, round(glyph.width * scale)), max(1, round(glyph.height * scale))
    canvas.alpha_composite(glyph.resize((nw, nh), Image.LANCZOS), ((size - nw) // 2, (size - nh) // 2))
    return canvas


def main():
    src = Image.open(SRC).convert("RGB")
    if src.size != (1024, 1024):
        raise SystemExit(f"unexpected source size {src.size}, expected 1024x1024")

    crop = src.crop(SQUIRCLE_BOX)
    master = crop.convert("RGBA")
    master.putalpha(squircle_alpha(crop))

    resized = {}
    for size in PNG_SIZES:
        img = master if size == master.size[0] else master.resize((size, size), Image.LANCZOS)
        resized[size] = img
        img.save(os.path.join(OUT_DIR, f"icon_{size}x{size}.png"))

    # Multi-size Windows icon; 256 stays PNG-compressed inside the .ico.
    resized[256].save(os.path.join(OUT_DIR, "icon.ico"), sizes=ICO_SIZES)
    resized[48].save(os.path.join(OUT_DIR, "favicon.ico"), sizes=FAVICON_SIZES)
    # Shared logo used as the last-resort window icon fallback.
    resized[512].save(os.path.join(ROOT, "assets", "logo.png"))

    # Notification-area icons: transparent background, glyph fills the tile.
    glyph = tray_knife(crop)
    for size in TRAY_SIZES:
        tray_icon(glyph, size).save(os.path.join(OUT_DIR, f"tray_{size}x{size}.png"))

    for size in PNG_SIZES:
        img = resized[size]
        print(f"icon_{size}x{size}.png alpha-bbox={img.getchannel('A').getbbox()}")
    print(f"tray glyph {glyph.size} -> tray_{{{','.join(map(str, TRAY_SIZES))}}}.png")
    print("source squircle:", master.size, "from", SQUIRCLE_BOX)

    print("done")


if __name__ == "__main__":
    main()
