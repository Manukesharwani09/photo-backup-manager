#!/usr/bin/env python3
"""Live end-to-end check of every API against REAL Google, using phonesim as the phone.

Prerequisites: server running (make run), both accounts connected, `make build` done (bin/phonesim).
Usage:  python3 -I scripts/live_check.py [--server http://localhost:8080] [--skip-switch]

Each run creates a few small, uniquely-labelled test photos (so their bytes are new) under
testdata/phone/{DCIM/Camera,Documents/Bills,Pictures/LiveCheck}, uploads them to the real accounts,
and leaves them there (delete them by hand in Google Photos). The camera photo is also put in the \"🗑 To delete\" album. With the account-switch step, one photo
goes to the overflow account and the primary is made current again at the end.
Exit code 0 = every check passed.
"""
import argparse
import datetime
import json
import os
import shutil
import subprocess
import sys
import urllib.error
import urllib.request

from PIL import Image, ImageDraw

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PHONE = os.path.join(ROOT, "testdata", "phone")
PHONESIM = os.path.join(ROOT, "bin", "phonesim")
results = []


def check(name, ok, detail=""):
    results.append((name, ok))
    print(f"  {'PASS' if ok else 'FAIL'}  {name}" + (f"  — {detail}" if detail and not ok else ""))
    return ok


def api(method, path, body=None, expect=200):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(SERVER + path, data=data, method=method, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req) as r:
            status, text = r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        status, text = e.code, e.read().decode()
    if status != expect:
        raise AssertionError(f"{method} {path}: HTTP {status}, want {expect}: {text[:300]}")
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return text


def phonesim(*args):
    out = subprocess.run([PHONESIM, "-server", SERVER, "-dir", PHONE, *args], cwd=ROOT, capture_output=True, text=True)
    if out.returncode != 0:
        raise AssertionError("phonesim failed: " + out.stderr[-500:])
    return out.stdout


def photo(folder, name, label):
    d = os.path.join(PHONE, folder)
    os.makedirs(d, exist_ok=True)
    img = Image.new("RGB", (320, 240), (30, 110, 180))
    ImageDraw.Draw(img).text((10, 10), f"live check {label}", fill=(255, 255, 255))
    path = os.path.join(d, name)
    img.save(path, "JPEG", quality=80)
    return path


def device_id():
    with open(os.path.join(ROOT, "testdata", "phonesim-device.json")) as f:
        return json.load(f)["device_id"]


def file_id(dev, folder, name):
    """Find a phone file's id through the cleanup preview (it lists every eligible file with ids)."""
    plan = api("POST", "/v1/cleanup/plan", {"device_id": dev, "folders": [folder]})
    for f in plan["eligible"]:
        if f["display_name"] == name:
            return f["file_id"], f
    return None, None


def main():
    ts = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    print(f"live check {ts} against {SERVER}\n")

    print("1. accounts")
    accts = api("GET", "/v1/accounts")
    primary = next(a for a in accts if a["role"] == "primary")
    overflow = next((a for a in accts if a["role"] == "overflow"), None)
    check("two accounts connected with live storage", len(accts) >= 2 and all(a.get("storage_free") for a in accts),
          str([(a["email"], a.get("storage_error")) for a in accts]))
    check("oauth start redirects to Google", "accounts.google.com" in redirect_target("/oauth/start?role=overflow"))

    print("2. sync + upload new photos (real Google)")
    cam = f"live-{ts}.jpg"
    bill = f"live-bill-{ts}.jpg"
    photo("DCIM/Camera", cam, f"camera {ts}")
    photo("Documents/Bills", bill, f"bill {ts}")
    os.makedirs(os.path.join(PHONE, "Pictures/LiveCheck"), exist_ok=True)
    shutil.copyfile(os.path.join(PHONE, "DCIM/Camera", cam), os.path.join(PHONE, "Pictures/LiveCheck", f"copy-{cam}"))
    out = phonesim()
    check("2 new photos uploaded (duplicate copy not)", "planned=2" in out, out)
    check("both backed up and verified by Google", out.count("backed_up verified=true") == 2, out)
    check("camera photo went to DCIM › Camera album", 'album "DCIM › Camera"' in out, out)
    check("bill went to Documents › Bills album", 'album "Documents › Bills"' in out, out)

    print("3. already existing")
    out = phonesim()
    check("re-sync plans nothing", "planned=0" in out, out)
    dev = device_id()
    cam_id, cam_row = file_id(dev, "DCIM/Camera", cam)
    check("camera photo is in Google (cleanup lists it as verified)", cam_id is not None)
    res = api("POST", "/v1/uploads", {"device_id": dev, "items": [{"file_id": cam_id, "account_id": primary["id"], "upload_token": "x"}]})
    check("re-uploading a backed-up file is refused", res["results"][0]["status"] == "already_backed_up", str(res))
    dup_id, _ = file_id(dev, "Pictures/LiveCheck", f"copy-{cam}")
    check("byte-identical copy counts as backed up", dup_id is not None)

    print("4. rules + validation errors")
    rules = api("GET", "/v1/rules")
    check("rules readable", isinstance(rules, list))
    api("POST", "/v1/sync", {"device_id": 999999, "files": []}, expect=404)
    check("unknown device → 404", True)
    api("POST", "/v1/sync", {"device_id": dev, "bogus": 1}, expect=400)
    check("unknown JSON field → 400", True)
    res = api("POST", "/v1/uploads", {"device_id": dev, "items": [{"file_id": 999999, "account_id": primary["id"], "upload_token": "x"}]})
    check("upload of a file not on this device → invalid", res["results"][0]["status"] == "invalid")

    print("5. cleanup (delete from phone)")
    plan = api("POST", "/v1/cleanup/plan", {"device_id": dev, "folders": ["DCIM", "Documents"]})
    names = [f["display_name"] for f in plan["eligible"]]
    check("new camera photo offered for cleanup", cam in names)
    check("protected Documents bill NOT offered", bill not in names and plan["kept"].get("protected", 0) >= 1, str(plan["kept"]))
    os.remove(os.path.join(PHONE, "DCIM/Camera", cam))  # the phone deletes it
    done = api("POST", "/v1/cleanup/done", {"device_id": dev, "file_ids": [cam_id, 999999]})
    check("cleanup/done records the deletion, ignores bogus id", done == {"marked_removed": 1, "ignored": 1}, str(done))
    plan = api("POST", "/v1/cleanup/plan", {"device_id": dev, "folders": ["DCIM/Camera"]})
    check("deleted photo not offered again", cam not in [f["display_name"] for f in plan["eligible"]])

    print("6. restore (download back from Google)")
    rplan = api("POST", "/v1/restore/plan", {"device_id": dev, "folder": "DCIM/Camera"})
    cam_item = next((i for i in rplan["items"] if i["display_name"] == cam), None)
    check("restore plan offers the deleted photo", cam_item is not None, str([i["display_name"] for i in rplan["items"]]))
    out = phonesim("-restore", "DCIM/Camera")
    check("photo restored into DCIM/Camera", os.path.exists(os.path.join(PHONE, "DCIM/Camera", cam)), out)
    check("restore linked, nothing rejected", "rejected:[]" in out and "linked:" in out, out)
    out = phonesim()
    check("restored photo is NOT re-uploaded", "planned=0" in out, out)

    print("7. similar + To delete + cloud check")
    sim = api("GET", "/v1/similar")
    check("similar groups readable", "groups" in sim)
    td = api("POST", "/v1/cloud/to-delete", {"media_ids": [cam_item["media_id"]]})
    check("camera photo added to the 🗑 To delete album", td["accounts"] and td["accounts"][0]["added"] == 1 and td["accounts"][0]["album_url"], str(td))
    chk = api("POST", "/v1/cloud/check")
    check("cloud check: everything still in Google, no errors", chk["deleted_in_cloud"] == 0 and not chk["errors"] and chk["checked"] > 0, str(chk))

    print("8. dashboard + photos by date")
    dash = api("GET", "/v1/dashboard")
    p = next(a for a in dash["accounts"] if a["id"] == primary["id"])
    check("dashboard lists accounts with items and folders", p["items"] >= 2 and "Documents/Bills" in p["folders"], str(p))
    check("dashboard shows To delete alert", any(a["kind"] == "to_delete_waiting" for a in dash["alerts"]))
    month = datetime.datetime.now().strftime("%Y-%m")
    where = api("GET", f"/v1/photos/where?month={month}")
    check("photos/where answers for this month", isinstance(where["accounts"], list))
    api("GET", "/v1/photos/where?month=March", expect=400)
    check("bad month → 400", True)

    if overflow and not ARGS.skip_switch:
        print("9. account full → overflow, then back")
        api("POST", f"/v1/accounts/{primary['id']}/full")
        sw = f"live-switch-{ts}.jpg"
        photo("DCIM/Camera", sw, f"switch {ts}")
        out = phonesim()
        check(f"with primary full, new photo goes to overflow ({overflow['email']})", f"-> account {overflow['id']}" in out and "backed_up verified=true" in out, out)
        api("POST", f"/v1/accounts/{primary['id']}/current", expect=409)
        check("full account can't be made current (409)", True)
        api("POST", f"/v1/accounts/{primary['id']}/available")
        api("POST", f"/v1/accounts/{primary['id']}/current")
        dash = api("GET", "/v1/dashboard")
        p = next(a for a in dash["accounts"] if a["id"] == primary["id"])
        check("primary available and current again", p["current"] and not p["full"], str(p))
        o = next(a for a in dash["accounts"] if a["id"] == overflow["id"])
        check("overflow now holds the switched photo", o["items"] >= 1, str(o))

    failed = [n for n, ok in results if not ok]
    print(f"\n{len(results) - len(failed)}/{len(results)} checks passed")
    for n in failed:
        print("  FAILED:", n)
    return 1 if failed else 0


def redirect_target(path):
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *a, **k):
            return None
    opener = urllib.request.build_opener(NoRedirect)
    try:
        opener.open(SERVER + path)
    except urllib.error.HTTPError as e:
        return e.headers.get("Location", "")
    return ""


if __name__ == "__main__":
    p = argparse.ArgumentParser()
    p.add_argument("--server", default="http://localhost:8080")
    p.add_argument("--skip-switch", action="store_true", help="don't test the account-full switch")
    ARGS = p.parse_args()
    SERVER = ARGS.server
    try:
        sys.exit(main())
    except AssertionError as e:
        print("ABORTED:", e)
        sys.exit(1)
