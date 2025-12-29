#!/usr/bin/env python3
import json
import os
import urllib.request
from datetime import datetime, timezone

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
DATA_DIR = os.path.join(ROOT, "data")
ENTRIES_PATH = os.path.join(DATA_DIR, "entries.json")
STATE_PATH = os.path.join(DATA_DIR, "state.json")
SITE_PATH = os.path.join(DATA_DIR, "site.json")

WP_URL = "https://api.wordpress.org/core/version-check/1.7/"


def load_json(path, default):
    if not os.path.exists(path):
        return default
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def save_json(path, payload):
    with open(path, "w", encoding="utf-8") as f:
        json.dump(payload, f, ensure_ascii=False, indent=2)


def iso_now():
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def fetch_latest_version():
    with urllib.request.urlopen(WP_URL, timeout=20) as resp:
        data = json.loads(resp.read().decode("utf-8"))
    offers = data.get("offers", [])
    if not offers:
        return None
    return offers[0].get("version")


def main():
    state = load_json(STATE_PATH, {"wordpress_latest": None})
    entries = load_json(ENTRIES_PATH, [])
    site = load_json(SITE_PATH, {})

    latest = fetch_latest_version()
    if not latest:
        return

    if state.get("wordpress_latest") == latest:
        return

    state["wordpress_latest"] = latest

    entry = {
        "id": f"wordpress-{latest}",
        "title": f"WordPress {latest} verfuegbar",
        "link": site.get("link", ""),
        "content": f"Neue WordPress Version {latest} wurde entdeckt.",
        "created_at": iso_now(),
    }
    entries.append(entry)

    save_json(STATE_PATH, state)
    save_json(ENTRIES_PATH, entries)


if __name__ == "__main__":
    main()
