#!/usr/bin/env python3
"""Render a GIF from actual reqdiff output. Requires Pillow, only for this script."""
import argparse
import subprocess
import threading
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont
from demo import make_server

ROOT = Path(__file__).resolve().parent.parent


def font(size):
    for path in ("/System/Library/Fonts/Menlo.ttc",
                 "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"):
        if Path(path).exists():
            return ImageFont.truetype(path, size)
    return ImageFont.load_default()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", default=str(ROOT / "dist/reqdiff"))
    args = parser.parse_args()
    server = make_server(0)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    base = f"http://127.0.0.1:{server.server_port}"
    command = [args.binary, f"{base}/staging/users", f"{base}/production/users"]
    try:
        result = subprocess.run(command, capture_output=True, text=True, check=False)
    finally:
        server.shutdown()
        server.server_close()
        worker.join()
    if result.returncode != 1:
        raise SystemExit(f"Expected diff exit 1: {result.stderr}")
    # The two URLs remain the exact endpoints that were requested.
    typed = f"reqdiff {base}/staging/users \\\n        {base}/production/users"
    output = result.stdout.rstrip().splitlines()
    normal, small = font(23), font(19)
    bg, ink = "#0d1117", "#dce3ed"

    def frame(command_text, visible):
        im = Image.new("RGB", (1280, 790), "#080b10")
        draw = ImageDraw.Draw(im)
        draw.rounded_rectangle((18, 18, 1262, 772), radius=18, fill=bg, outline="#293243", width=2)
        for x, color in ((48, "#ff5f57"), (72, "#febc2e"), (96, "#28c840")):
            draw.ellipse((x, 42, x + 12, 54), fill=color)
        draw.text((135, 37), "reqdiff  /  live responses, visible differences", font=small, fill="#8594aa")
        draw.line((20, 78, 1260, 78), fill="#293243", width=1)
        y = 108
        draw.text((44, y), "$ " + command_text, font=normal, fill="#ae9dff", spacing=10)
        y += 88
        for line in visible:
            color = ink
            if line.startswith("+ "): color = "#66d9a3"
            elif line.startswith("- "): color = "#ff818b"
            elif line.startswith("~ "): color = "#f5ce79"
            elif line.startswith("Result:"): color = "#f5ce79"
            elif line.startswith(("Left:", "Right:")): color = "#8594aa"
            draw.text((44, y), line, font=normal, fill=color)
            y += 35
        draw.text((44, 730), "Two environments. One command.   •   Synthetic local API demo", font=small, fill="#8594aa")
        return im

    frames, durations = [], []
    for n in range(0, len(typed), 4):
        frames.append(frame(typed[:n], [])); durations.append(65)
    for n in range(len(output) + 1):
        frames.append(frame(typed, output[:n])); durations.append(100)
    durations[-1] = 4200
    destination = ROOT / "docs/assets"
    destination.mkdir(parents=True, exist_ok=True)
    frames[-1].save(destination / "demo.png")
    frames[0].save(destination / "demo.gif", save_all=True, append_images=frames[1:],
                   duration=durations, loop=0, optimize=True)
    (destination / "demo-output.txt").write_text(result.stdout)
    print(f"Saved GIF from {len(output)} actual output lines")


if __name__ == "__main__":
    main()
