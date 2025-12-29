#!/usr/bin/env python3
import json
import os
from datetime import datetime, timezone
from email.utils import format_datetime
from xml.etree.ElementTree import Element, SubElement, ElementTree

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
DATA_DIR = os.path.join(ROOT, "data")
SITE_PATH = os.path.join(DATA_DIR, "site.json")
ENTRIES_PATH = os.path.join(DATA_DIR, "entries.json")
OUTPUT_PATH = os.path.join(ROOT, "feed.xml")


def load_json(path, default):
    if not os.path.exists(path):
        return default
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def parse_iso8601(value):
    if value.endswith("Z"):
        value = value.replace("Z", "+00:00")
    return datetime.fromisoformat(value)


def rfc2822(dt):
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return format_datetime(dt)


def main():
    site = load_json(SITE_PATH, {})
    entries = load_json(ENTRIES_PATH, [])

    rss = Element("rss", version="2.0")
    channel = SubElement(rss, "channel")

    SubElement(channel, "title").text = site.get("title", "Wapuugotchi RSS")
    SubElement(channel, "link").text = site.get("link", "")
    SubElement(channel, "description").text = site.get("description", "")

    if entries:
        latest = max(parse_iso8601(e["created_at"]) for e in entries)
        SubElement(channel, "lastBuildDate").text = rfc2822(latest)

    entries_sorted = sorted(entries, key=lambda e: e["created_at"], reverse=True)
    for entry in entries_sorted:
        item = SubElement(channel, "item")
        SubElement(item, "title").text = entry.get("title", "")
        SubElement(item, "link").text = entry.get("link", site.get("link", ""))
        guid = SubElement(item, "guid")
        guid.text = entry.get("id", "")
        guid.set("isPermaLink", "false")
        created_at = parse_iso8601(entry["created_at"])
        SubElement(item, "pubDate").text = rfc2822(created_at)
        SubElement(item, "description").text = entry.get("content", "")

    tree = ElementTree(rss)
    tree.write(OUTPUT_PATH, encoding="utf-8", xml_declaration=True)


if __name__ == "__main__":
    main()
