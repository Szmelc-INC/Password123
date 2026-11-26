#!/usr/bin/env bash
# Wrapper kept for the layout in ARCHITECTURE.md; the logic lives in p123.
source "$(dirname "$0")/_p123.sh"
p123 validate --skip-vectors "$@"
