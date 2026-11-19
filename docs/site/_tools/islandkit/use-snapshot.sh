#!/bin/sh

# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Puts the committed islandKit snapshot where the islandKit stage puts a fresh
# render.
#
# The islandKit stage needs a token for the private npm registry. A build
# without one (DOCS_ISLANDKIT_SNAPSHOT=1, see docs/site/README.md, Site header)
# takes its output - for now, the header - from the snapshot instead: the
# fragments, their index, the bundle and the compiled styles, written by
# snapshot.mjs. A local snapshot may lag behind the pinned version, and that is
# fine for editing pages; a pipeline build of the site does not let it.
#
# Usage: use-snapshot.sh site
#        use-snapshot.sh module <EDITION>

set -eu

snapshot="$(cd "$(dirname "$0")" && pwd)/snapshot"
mode="${1:-}"

fail() {
  echo "islandKit snapshot: $1" >&2
  exit 1
}

case "${mode}" in
  site)
    fragments="${snapshot}/site/_generated"
    ;;
  module)
    edition="${2:-}"
    [ -n "${edition}" ] || fail "the module mode needs an edition."
    fragments="${snapshot}/module/${edition}"
    ;;
  *)
    fail "unknown mode '${mode}', expected 'site' or 'module <EDITION>'."
    ;;
esac

index="${snapshot}/${mode}/islandkit_header.json"
[ -f "${index}" ] || fail "there is no snapshot in docs/site/_tools/islandkit/snapshot. Create it with 'make islandkit-snapshot' (needs access to the registry), or build with FOX_NPM_TOKEN."
[ -d "${fragments}" ] || fail "the snapshot has nothing for '${mode} ${edition:-}'. Update it with 'make islandkit-snapshot'."

mkdir -p /data
cp -R "${fragments}" /data/_generated
cp "${index}" /data/islandkit_header.json
cp -R "${snapshot}/assets" /data/islandkit-assets
cp -R "${snapshot}/styles" /data/deckhouse-islands

version="$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' "${index}")"
pinned="$(sed -n 's/.*"islandkit": *"\([^"]*\)".*/\1/p' "$(dirname "$0")/versions.json")"
echo "islandKit: taken from the committed snapshot, version ${version} (no registry token)."
if [ "${version}" != "${pinned}" ]; then
  echo "islandKit: the pinned version is ${pinned}, so this is not what the site will publish." >&2
fi
