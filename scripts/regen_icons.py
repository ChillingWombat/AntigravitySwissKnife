#!/usr/bin/env python3
"""Regenerate Windows/Linux icon set: enlarge knife graphic to ~86% of canvas,
add squircle contrast border, transparent corners, multi-size .ico."""
import os
from collections import deque
from PIL import Image, ImageFilter

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(ROOT, "assets", "icons", "icon_1024x1024.png")
OUT_DIR = os.path.join(ROOT, "assets", "icons")

BG = (248, 249, 251)            # original squircle background
NEW_BG = (233, 237, 242)        # slightly darker/cooler for light-taskbar contrast
BORDER = (203, 211, 220)        # subtle squircle edge ring
CANVAS = 1024
KNIFE_FILL = 0.88               # knife longest side as fraction of canvas

def main():
    src = Image.open(SRC).convert("RGBA")
    assert src.size == (CANVAS, CANVAS)
    px = src.load()

    # --- 1. Squircle alpha shape (reuse original, keeps design language) ---
    alpha = src.getchannel("A")
    shape = alpha.point(lambda a: 255 if a >= 128 else 0)

    # --- 2. Knife extraction with interior hole fill (white "G") ---
    # coverage t estimated from color distance to flat background
    dist = [[0] * CANVAS for _ in range(0)]  # unused; use per-pixel loop below
    knife = Image.new("RGBA", (CANVAS, CANVAS), (0, 0, 0, 0))
    kp = knife.load()
    mask = bytearray(CANVAS * CANVAS)  # 1 = definitely knife
    for y in range(CANVAS):
        for x in range(CANVAS):
            r, g, b, a = px[x, y]
            d = max(abs(r - BG[0]), abs(g - BG[1]), abs(b - BG[2]))
            if d > 40 and a >= 128:
                mask[y * CANVAS + x] = 1

    # flood fill from borders through non-mask pixels; unreached = enclosed holes
    seen = bytearray(CANVAS * CANVAS)
    q = deque()
    for i in range(CANVAS):
        for x, y in ((i, 0), (i, CANVAS - 1), (0, i), (CANVAS - 1, i)):
            idx = y * CANVAS + x
            if not mask[idx] and not seen[idx]:
                seen[idx] = 1
                q.append((x, y))
    while q:
        x, y = q.popleft()
        for nx, ny in ((x+1,y),(x-1,y),(x,y+1),(x,y-1)):
            if 0 <= nx < CANVAS and 0 <= ny < CANVAS:
                idx = ny * CANVAS + nx
                if not mask[idx] and not seen[idx]:
                    seen[idx] = 1
                    q.append((nx, ny))

    # label enclosed holes; keep small ones (white "G", blade notches),
    # leave large inter-tool gaps transparent so the new bg shows through
    HOLE_FILL_MAX = 20000  # px^2 at 1024 canvas
    hole = bytearray(CANVAS * CANVAS)
    for sy in range(CANVAS):
        for sx in range(CANVAS):
            sidx = sy * CANVAS + sx
            if mask[sidx] or seen[sidx] or hole[sidx]:
                continue
            comp = []
            q.append((sx, sy))
            hole[sidx] = 2  # visiting
            while q:
                x, y = q.popleft()
                comp.append((x, y))
                for nx, ny in ((x+1,y),(x-1,y),(x,y+1),(x,y-1)):
                    if 0 <= nx < CANVAS and 0 <= ny < CANVAS:
                        idx = ny * CANVAS + nx
                        if not mask[idx] and not seen[idx] and not hole[idx]:
                            hole[idx] = 2
                            q.append((nx, ny))
            fill = 1 if len(comp) <= HOLE_FILL_MAX else 0
            for x, y in comp:
                hole[y * CANVAS + x] = fill

    # dilated mask: edge antialias only applies near real knife pixels,
    # so the squircle's faint inner shadow is not captured as knife content
    mask_img = Image.frombytes("L", (CANVAS, CANVAS), bytes(m * 255 for m in mask))
    dilated = mask_img.filter(ImageFilter.MaxFilter(15)).load()

    for y in range(CANVAS):
        for x in range(CANVAS):
            idx = y * CANVAS + x
            r, g, b, a = px[x, y]
            if mask[idx] or hole[idx] == 1:
                kp[x, y] = (r, g, b, 255)
            else:
                if a < 250 or not dilated[x, y]:
                    continue  # outside squircle / shadow / far from knife — skip
                # antialiased edge: partial coverage from distance to bg
                d = max(abs(r - BG[0]), abs(g - BG[1]), abs(b - BG[2]))
                t = max(0.0, min(1.0, (d - 12) / 28.0))
                if t <= 0.0:
                    continue
                cr = int(max(0, min(255, (r - BG[0] * (1 - t)) / t))) if t < 1 else r
                cg = int(max(0, min(255, (g - BG[1] * (1 - t)) / t))) if t < 1 else g
                cb = int(max(0, min(255, (b - BG[2] * (1 - t)) / t))) if t < 1 else b
                kp[x, y] = (cr, cg, cb, int(t * 255))

    kbbox = knife.getchannel("A").getbbox()
    print("knife bbox on source:", kbbox)
    knife_crop = knife.crop(kbbox)
    kw, kh = knife_crop.size

    # --- 3. Compose new master ---
    target = int(CANVAS * KNIFE_FILL)
    scale = target / max(kw, kh)
    nw, nh = round(kw * scale), round(kh * scale)
    knife_big = knife_crop.resize((nw, nh), Image.LANCZOS)

    master = Image.new("RGBA", (CANVAS, CANVAS), (0, 0, 0, 0))
    # squircle base
    base = Image.new("RGBA", (CANVAS, CANVAS), NEW_BG + (255,))
    base.putalpha(shape)
    master = Image.alpha_composite(master, base)

    # subtle border ring: shape minus eroded shape
    eroded = shape.filter(ImageFilter.MinFilter(13))  # ~6px erosion each side at 1024
    sp, ep = shape.load(), eroded.load()
    ring = Image.new("RGBA", (CANVAS, CANVAS), (0, 0, 0, 0))
    rp = ring.load()
    for y in range(CANVAS):
        for x in range(CANVAS):
            if sp[x, y] and not ep[x, y]:
                rp[x, y] = BORDER + (255,)
    master = Image.alpha_composite(master, ring)

    ox, oy = (CANVAS - nw) // 2, (CANVAS - nh) // 2
    master.alpha_composite(knife_big, (ox, oy))

    # --- 4. Export sizes ---
    sizes = [16, 24, 32, 48, 64, 128, 256, 512, 1024]
    resized = {}
    for s in sizes:
        img = master if s == CANVAS else master.resize((s, s), Image.LANCZOS)
        resized[s] = img
        img.save(os.path.join(OUT_DIR, f"icon_{s}x{s}.png"))
    # multi-size ICO for Windows (PNG-compressed 256)
    resized[256].save(
        os.path.join(OUT_DIR, "icon.ico"),
        sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (256, 256)],
    )
    # favicon + logo refresh for consistency
    resized[48].save(os.path.join(OUT_DIR, "favicon.ico"),
                     sizes=[(16, 16), (32, 32), (48, 48)])
    resized[512].save(os.path.join(ROOT, "assets", "logo.png"))

    # --- 5. Report coverage ---
    for s in sizes:
        img = resized[s]
        a = img.getchannel("A")
        bb = a.getbbox()
        print(f"icon_{s}x{s}.png alpha-bbox={bb}")
    print("knife scaled to", (nw, nh), "->", round(100 * nw / CANVAS, 1), "% of canvas")
    print("done")

if __name__ == "__main__":
    main()
