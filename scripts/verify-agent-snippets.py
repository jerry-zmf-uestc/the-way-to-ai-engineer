#!/usr/bin/env python3
# coding: utf-8
"""Check Agent source excerpts. This is source parity, not a teaching-quality score."""
import json
import re
import sys
from pathlib import Path

root = Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else Path(__file__).resolve().parents[1]
manifest = json.loads((root / "book/chapters.json").read_text())

def lines(text):
    # Only indentation, blank lines and full-line comments may differ.
    return [line.lstrip().rstrip() for line in text.splitlines()
            if line.strip() and not line.lstrip().startswith("//")]

def matches(block, source, language="go"):
    if block in source:
        return True
    if language == "python":
        return False  # Python indentation is semantic: current excerpts require exact source.
    # Multiline raw/template strings have meaningful indentation: require exact parity.
    if any(line.count("`") % 2 for line in block.splitlines()):
        return False
    wanted, actual = lines(block), lines(source)
    return any(actual[n:n + len(wanted)] == wanted for n in range(len(actual) - len(wanted) + 1))

count = 0
for entry in manifest.values():
    if entry["part"] != "Part 3 · Golang Agent" or not entry["published"]:
        continue
    path = root / entry["path"]
    text = path.read_text()
    go = [file for lab in entry["labs"] for file in (root / lab).rglob("*.go")
          if not file.name.endswith("_test.go")]
    go += [file for file in (root / "labs/internal").rglob("*.go")
           if not file.name.endswith("_test.go")]
    py = [file for lab in entry["labs"] for file in (root / lab).rglob("*.py")]
    js = [file for lab in entry["labs"] for file in (root / lab).rglob("*.html")]
    for language, block in re.findall(r"```(go|javascript|python)\n(.*?)\n```", text, re.S):
        candidates = go if language == "go" else (py if language == "python" else js)
        if not any(matches(block, file.read_text(), language) for file in candidates):
            sys.exit(f"{entry['path']}: source excerpt differs: {block.splitlines()[0]}")
        count += 1
print(f"Agent excerpts: {count} blocks match associated source (indent/comments may be elided; semantic teaching review separate)")
