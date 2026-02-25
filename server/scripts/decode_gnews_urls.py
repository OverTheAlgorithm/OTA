"""
Google News URL decoder script.
Reads JSON array of Google News URLs from stdin,
outputs JSON array of {original, decoded, ok} to stdout.

Usage:
    echo '["https://news.google.com/rss/articles/CBMi..."]' | python decode_gnews_urls.py

Dependencies:
    pip install googlenewsdecoder
"""

import json
import sys

from googlenewsdecoder import new_decoderv1


def decode_urls(urls):
    results = []
    for url in urls:
        try:
            r = new_decoderv1(url)
            if r.get("status") and r.get("decoded_url"):
                decoded = r["decoded_url"]
                # Validate: must start with http:// or https://
                if decoded.startswith("http://") or decoded.startswith("https://"):
                    results.append({"original": url, "decoded": decoded, "ok": True})
                else:
                    results.append({"original": url, "decoded": url, "ok": False})
            else:
                results.append({"original": url, "decoded": url, "ok": False})
        except Exception:
            results.append({"original": url, "decoded": url, "ok": False})
    return results


def main():
    raw = sys.stdin.read().strip()
    if not raw:
        json.dump([], sys.stdout)
        return

    try:
        urls = json.loads(raw)
    except json.JSONDecodeError:
        json.dump({"error": "invalid JSON input"}, sys.stdout)
        sys.exit(1)

    if not isinstance(urls, list):
        json.dump({"error": "expected JSON array"}, sys.stdout)
        sys.exit(1)

    results = decode_urls(urls)
    json.dump(results, sys.stdout, ensure_ascii=False)


if __name__ == "__main__":
    main()
