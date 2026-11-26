# Shared helper for the thin wrapper scripts: build p123 once into .tmp/ and run it.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${P123_BIN:-}"
if [ -z "$BIN" ]; then
  BIN="$ROOT/.tmp/p123"
  mkdir -p "$ROOT/.tmp"
  (cd "$ROOT" && go build -trimpath -o "$BIN" ./cmd/p123)
fi
p123() { "$BIN" --repo "$ROOT" "$@"; }
