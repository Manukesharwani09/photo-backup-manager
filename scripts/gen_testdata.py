#!/usr/bin/env python3
"""Generate small test photos laid out like a phone's storage, for cmd/phonesim.

Usage:
  python3 -I scripts/gen_testdata.py <phone_dir> <set>

Sets:
  before  - photos already on the phone at the start point (must never be uploaded)
  after   - new photos taken after the start point, covering the rule cases:
            camera (one with GPS), Documents (rule: primary, protected),
            Private (rule: excluded), and a byte-identical copy in another folder.

Each image has its label drawn on it so it is recognisable in Google Photos.
"""
import os
import shutil
import sys

from PIL import Image, ImageDraw

SETS = {
    "before": [
        ("DCIM/Camera", "before-1.jpg", (70, 70, 160), None),
        ("WhatsApp/Images", "IMG-20240101-WA0001.jpg", (40, 140, 60), None),
    ],
    "after": [
        ("DCIM/Camera", "after-camera-gps.jpg", (200, 90, 40), (12.9716, 77.5946)),  # Bengaluru
        ("DCIM/Camera", "after-camera-2.jpg", (160, 40, 120), None),
        ("Documents/Bills", "after-bill.jpg", (230, 200, 60), None),
        ("Private", "after-private.jpg", (90, 90, 90), None),
    ],
}


def to_dms(value):
    value = abs(value)
    d = int(value)
    m = int((value - d) * 60)
    s = round((value - d - m / 60) * 3600, 2)
    return (float(d), float(m), s)


def make(path, label, color, gps):
    img = Image.new("RGB", (640, 480), color)
    draw = ImageDraw.Draw(img)
    draw.text((20, 20), "photo-backup-manager test", fill=(255, 255, 255))
    draw.text((20, 60), label, fill=(255, 255, 255))
    exif = Image.Exif()
    exif[0x0132] = "2026:10:24 12:00:00"  # DateTime
    if gps:
        lat, lng = gps
        exif[0x8825] = {  # GPSInfo IFD
            1: "N" if lat >= 0 else "S",
            2: to_dms(lat),
            3: "E" if lng >= 0 else "W",
            4: to_dms(lng),
        }
    img.save(path, "JPEG", quality=85, exif=exif)


def main():
    if len(sys.argv) != 3 or sys.argv[2] not in SETS:
        sys.exit(__doc__)
    root, which = sys.argv[1], sys.argv[2]
    for folder, name, color, gps in SETS[which]:
        d = os.path.join(root, folder)
        os.makedirs(d, exist_ok=True)
        make(os.path.join(d, name), f"{folder}/{name}", color, gps)
        print("wrote", os.path.join(folder, name))
    if which == "after":
        # Same bytes as an existing camera photo, saved in another folder: must upload only once.
        src = os.path.join(root, "DCIM/Camera/after-camera-2.jpg")
        dst_dir = os.path.join(root, "Pictures/Saved")
        os.makedirs(dst_dir, exist_ok=True)
        shutil.copyfile(src, os.path.join(dst_dir, "copy-of-camera-2.jpg"))
        print("wrote Pictures/Saved/copy-of-camera-2.jpg (duplicate)")


if __name__ == "__main__":
    main()
