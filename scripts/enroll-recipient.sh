#!/usr/bin/env bash
# Usage: scripts/enroll-recipient.sh <age1... recipient> "<owner/device, date>"
# Confirm the key with its owner over a second channel before running this.
source "$(dirname "$0")/_p123.sh"
[ $# -ge 1 ] || { echo "usage: $0 <recipient> [comment]" >&2; exit 2; }
p123 recipients add "$1" --comment "${2:-}"
echo "Now run: scripts/rotate-recipients.sh (needs an enrolled identity), then test decryption with the new identity."
