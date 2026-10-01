#!/bin/sh
set -eu

command -v rg >/dev/null 2>&1 || { echo 'privacy check: ripgrep (rg) is required' >&2; exit 2; }

scan() {
  rg --no-config --hidden --no-ignore \
  --glob '!.git/**' \
  --glob '!.specs/**' \
  --glob '!.tools/**' \
  --glob '!node_modules/**' \
  --glob '!.astro/**' \
  --glob '!src/content/chapters/**' \
  --glob '!dist/**' \
  --glob '!scripts/check-privacy-leaks.sh' "$@" .
}

inventory_status=0
scan --files --quiet || inventory_status=$?
case "$inventory_status" in
  0) ;;
  1) echo 'privacy check: no scannable files; no safety conclusion' >&2; exit 2 ;;
  *) echo 'privacy check: scanner failed during inventory' >&2; exit 2 ;;
esac

status=0
scan --files-with-matches '/Users/|/Volumes/|sk-[A-Za-z0-9_-]{12,}|-----BEGIN [A-Z ]*PRIVATE KEY-----' || status=$?
case "$status" in
  0) echo 'privacy check: possible local path or secret in listed files (values redacted)' >&2; exit 1 ;;
  1) echo 'privacy check: selected patterns not found (not a complete security audit)' ;;
  *) echo 'privacy check: scanner failed; no safety conclusion' >&2; exit 2 ;;
esac
