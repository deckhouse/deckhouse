#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

docker run --rm -it --network host \
  -v "$REPO_ROOT:/work" \
  -w /work/tools/docs/pw_screenshots \
  d8_playwrite \
  bash -lc "pytest get_screenshots.py -v && if [ -d /work/tools/docs/pw_screenshots/screenshots ]; then chown -R $(id -u):$(id -g) /work/tools/docs/pw_screenshots/screenshots; fi"

SRC_SCREENSHOTS="$REPO_ROOT/tools/docs/pw_screenshots/screenshots"
DST_IMAGES="$REPO_ROOT/docs/site/images/gs/installer"

mkdir -p "$DST_IMAGES"
cp -a "$SRC_SCREENSHOTS/." "$DST_IMAGES/"
rm -rf "$SRC_SCREENSHOTS"
