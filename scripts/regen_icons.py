#!/usr/bin/env python3
"""Regenerate the app icon set from the square artwork.

Source of truth: assets/logo_square.png (600x600, the mark on a plain canvas).
The previous generator keyed out a hard-coded "outer canvas" colour around a
hard-coded squircle crop; that produced opaque square corners (the crop's
corners are white fill, not canvas) and clipped the light-blue arc. This
version measures instead of guessing:

  * the background colour is sampled from the image border (it is an off-white
    246..253, not pure 255, which is why a fixed 255-based threshold treated the
    border as artwork),
  * the artwork box is derived from the real content mask,
  * the app icon is composed as a white squircle plate with the artwork scaled
    into the safe area and 4x supersampled, so the corners are properly rounded
    and transparent (never a square plate),
  * the tray icon keeps transparency only and uses a flood fill from the border
    so enclosed white areas (the "G") stay opaque on a white flyout.

Run:  python scripts/regen_icons.py
"""
import os
from collections import deque

from PIL import Image, ImageDraw

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(ROOT, "assets", "logo_square.png")
OUT_DIR = os.path.join(ROOT, "assets", "icons")

PNG_SIZES = [16, 24, 32, 48, 64, 128, 256, 512, 1024]
ICO_SIZES = [(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
FAVICON_SIZES = [(16, 16), (32, 32), (48, 48)]
TRAY_SIZES = [16, 24, 32, 48]

CORNER_RADIUS = 0.2237  # iOS-style continuous corner radius
CONTENT_FILL = 0.78     # artwork share of the icon edge (safe-area padding)
TRAY_FILL = 0.92
BG_TOLERANCE = 12       # colour distance that still counts as background
SUPERSAMPLE = 4


def border_colour(img):
    """Median-ish background colour sampled along the 1px border."""
    w, h = img.size
    px = img.load()
    samples = []
    for x in range(0, w, 4):
        samples.append(px[x, 0])
        samples.append(px[x, h - 1])
    for y in range(0, h, 4):
        samples.append(px[0, y])
        samples.append(px[w - 1, y])
    rs = sorted(s[0] for s in samples)
    gs = sorted(s[1] for s in samples)
    bs = sorted(s[2] for s in samples)
    mid = len(samples) // 2
    return rs[mid], gs[mid], bs[mid]


def distance(px, bg):
    return max(abs(px[0] - bg[0]), abs(px[1] - bg[1]), abs(px[2] - bg[2]))


def content_box(img, bg):
    w, h = img.size
    px = img.load()
    minx, miny, maxx, maxy = w, h, -1, -1
    for y in range(h):
        for x in range(w):
            if distance(px[x, y], bg) > BG_TOLERANCE:
                if x < minx:
                    minx = x
                if y < miny:
                    miny = y
                if x > maxx:
                    maxx = x
                if y > maxy:
                    maxy = y
    if maxx < minx or maxy < miny:
        raise SystemExit("no artwork found in the source image")
    return minx, miny, maxx + 1, maxy + 1


def keyed_glyph(crop, bg):
    """Per-pixel alpha ramp: background -> transparent, artwork -> opaque."""
    glyph = crop.convert("RGBA")
    px = glyph.load()
    for y in range(glyph.height):
        for x in range(glyph.width):
            r, g, b, _ = px[x, y]
            d = distance((r, g, b), bg)
            if d <= 0:
                a = 0
            elif d >= BG_TOLERANCE:
                a = 255
            else:
                a = int(d / BG_TOLERANCE * 255 + 0.5)
            px[x, y] = (r, g, b, a)
    return glyph


def flood_glyph(crop, bg):
    """Alpha 0 for background connected to the border; enclosed whites kept."""
    glyph = crop.convert("RGBA")
    w, h = glyph.size
    px = glyph.load()
    is_bg = [[distance(px[x, y], bg) <= BG_TOLERANCE for y in range(h)] for x in range(w)]
    outside = [[False] * h for _ in range(w)]
    q = deque()
    for x in range(w):
        for y in (0, h - 1):
            if is_bg[x][y] and not outside[x][y]:
                outside[x][y] = True
                q.append((x, y))
    for y in range(h):
        for x in (0, w - 1):
            if is_bg[x][y] and not outside[x][y]:
                outside[x][y] = True
                q.append((x, y))
    while q:
        x, y = q.popleft()
        for nx, ny in ((x + 1, y), (x - 1, y), (x, y + 1), (x, y - 1)):
            if 0 <= nx < w and 0 <= ny < h and is_bg[nx][ny] and not outside[nx][ny]:
                outside[nx][ny] = True
                q.append((nx, ny))
    for y in range(h):
        for x in range(w):
            if outside[x][y]:
                r, g, b, _ = px[x, y]
                px[x, y] = (r, g, b, 0)
    return glyph


def fit(glyph, size, fill):
    target = max(1, int(round(size * fill)))
    scale = min(target / glyph.width, target / glyph.height)
    nw = max(1, round(glyph.width * scale))
    nh = max(1, round(glyph.height * scale))
    return glyph.resize((nw, nh), Image.LANCZOS)


def app_icon(glyph, size):
    ss = size * SUPERSAMPLE
    canvas = Image.new("RGBA", (ss, ss), (0, 0, 0, 0))
    ImageDraw.Draw(canvas).rounded_rectangle(
        (0, 0, ss - 1, ss - 1), radius=CORNER_RADIUS * ss, fill=(255, 255, 255, 255)
    )
    art = fit(glyph, ss, CONTENT_FILL)
    canvas.alpha_composite(art, ((ss - art.width) // 2, (ss - art.height) // 2))
    return canvas.resize((size, size), Image.LANCZOS)


def tray_icon(glyph, size):
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    art = fit(glyph, size, TRAY_FILL)
    canvas.alpha_composite(art, ((size - art.width) // 2, (size - art.height) // 2))
    return canvas


def main():
    src = Image.open(SRC).convert("RGB")
    bg = border_colour(src)
    box = content_box(src, bg)
    crop = src.crop(box)

    app_glyph = keyed_glyph(crop, bg)
    resized = {}
    for size in PNG_SIZES:
        img = app_icon(app_glyph, size)
        resized[size] = img
        img.save(os.path.join(OUT_DIR, f"icon_{size}x{size}.png"))

    resized[256].save(os.path.join(OUT_DIR, "icon.ico"), sizes=ICO_SIZES)
    resized[48].save(os.path.join(OUT_DIR, "favicon.ico"), sizes=FAVICON_SIZES)
    resized[512].save(os.path.join(ROOT, "assets", "logo.png"))

    tray_glyph = flood_glyph(crop, bg).crop(
        flood_glyph(crop, bg).getchannel("A").getbbox()
    )
    for size in TRAY_SIZES:
        tray_icon(tray_glyph, size).save(os.path.join(OUT_DIR, f"tray_{size}x{size}.png"))

    for size in PNG_SIZES:
        img = resized[size]
        print(
            f"icon_{size}x{size}.png bbox={img.getchannel('A').getbbox()} "
            f"corner-alpha={img.getchannel('A').getpixel((0, 0))}"
        )
    print(f"sampled background: {bg}")
    print(f"content box: {box} -> {crop.size}; tray glyph: {tray_glyph.size}")
    print("done")


if __name__ == "__main__":
    main()
