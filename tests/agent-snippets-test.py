#!/usr/bin/env python3
# coding: utf-8
"""Negative checks for source parity: mutation, string content and multiline literals."""
import json
import subprocess
import sys
import tempfile
from pathlib import Path

checker = Path(__file__).resolve().parents[1] / "scripts/verify-agent-snippets.py"
cases = [
    ("exact", 'func answer() string {\n\treturn "known"\n}', 'func answer() string {\n\treturn "known"\n}', True),
    ("indent only", 'if stop {\n\treturn "known"\n}', '\t\tif stop {\n\t\t\treturn "known"\n\t\t}', True),
    ("wrong literal", 'return "forged"', 'return "known"', False),
    ("wrong signature", 'type Reply struct { Value int64 }', 'type Reply struct { Value int }', False),
    ("raw string indentation", 's := `line\n next`', 's := `line\n  next`', False),
]
cases += [("python exact", "    x = 1\n    return x", "def f():\n    x = 1\n    return x", True, "python"),
          ("python indent changed", "    x = 1\nreturn x", "def f():\n    x = 1\n    return x", False, "python")]
for name, excerpt, source, expected, *optional in cases:
    language = optional[0] if optional else "go"
    with tempfile.TemporaryDirectory(prefix="agent-snippet-test-") as directory:
        root = Path(directory)
        (root / "book").mkdir()
        (root / "labs/sample").mkdir(parents=True)
        (root / "book/page.md").write_text("```" + language + "\n" + excerpt + "\n```\n")
        (root / ("labs/sample/main.py" if language=="python" else "labs/sample/main.go")).write_text(source)
        (root / "book/chapters.json").write_text(json.dumps({"sample": {"part": "Part 3 · Golang Agent", "published": True, "path": "book/page.md", "labs": ["labs/sample"]}}))
        result = subprocess.run([sys.executable, str(checker), str(root)], text=True, capture_output=True)
        if (result.returncode == 0) != expected:
            raise SystemExit(f"{name}: incorrect result {result.stdout}{result.stderr}")
        print("PASS:", name)
