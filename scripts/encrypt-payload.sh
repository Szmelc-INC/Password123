#!/usr/bin/env bash
# Usage: scripts/encrypt-payload.sh <plaintext-outside-repo.json> <lesson-id>
source "$(dirname "$0")/_p123.sh"
[ $# -eq 2 ] || { echo "usage: $0 <plaintext-file> <lesson-id>" >&2; exit 2; }
umask 077
p123 encrypt "$1" --lesson "$2"
