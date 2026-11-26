"""Shrinks the user manual's pictures (ADR-0253).

Screenshots of the UI hold few colours, so a palette of 256 without
dithering keeps them sharp at about a third of the size. Run after
manual-shots.cjs, with Pillow installed:

    python3 -I e2e/shrink-pictures.py internal/web/static/assets/manual/img
"""
import os
import sys

from PIL import Image


def shrink(path):
    before = os.path.getsize(path)
    with Image.open(path) as im:
        rgba = im.convert("RGBA")
    alpha = rgba.getchannel("A")
    if alpha.getextrema()[0] < 255:
        # A picture with transparent corners keeps its alpha in the palette.
        out = rgba.quantize(colors=256, method=Image.Quantize.FASTOCTREE, dither=Image.Dither.NONE)
    else:
        out = rgba.convert("RGB").quantize(colors=256, method=Image.Quantize.MEDIANCUT, dither=Image.Dither.NONE)
    out.save(path, optimize=True)
    return before, os.path.getsize(path)


def main(root):
    total = [0, 0]
    for folder, _, files in os.walk(root):
        for name in sorted(files):
            if name.endswith(".png"):
                b, a = shrink(os.path.join(folder, name))
                total[0] += b
                total[1] += a
    print(f"{total[0]} -> {total[1]} bytes")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else "internal/web/static/assets/manual/img")
