#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

docker run --rm -it \
  --network host \
  -v "$PWD:/work" \
  -w /work \
  d8_playwrite \
  bash -lc "get_screenshots.sh -v"
