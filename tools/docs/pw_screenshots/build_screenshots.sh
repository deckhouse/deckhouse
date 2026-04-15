#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

docker run --rm -it --network host \
  -v "$REPO_ROOT:/work" \
  -w /work/tools/docs/pw_screenshots \
  d8_playwrite \
  bash -lc "pytest get_screenshots.py -v"
