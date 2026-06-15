#!/usr/bin/env python3

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

"""
Detect module images that actually changed in this PR.

Inputs (env):
  BUILD_REPORT_PATH   - path to werf build report (images_tags_werf.json)
                        from the **first successful Build FE run of this PR**
                        (NOT the current run — see security-scan-images.yml,
                        step "Find baseline Build FE run"). On that build,
                        werf cache could only contain layers from main, so
                        `Rebuilt: false` reliably means "identical to main".
  IMAGES_DIGESTS_PATH - path to images_digests.json extracted from the
                        assembled dev image of the **current** run
                        (/deckhouse/modules/images_digests.json).
  OUTPUT_OLD_DIGESTS  - where to write digests_old_main.json (default: ./digests_old_main.json).
  OUTPUT_CHANGED      - where to write changed_images.json (default: ./changed_images.json).
  GITHUB_OUTPUT       - if set, also emit `changed_count`, `matrix` and
                        `changed_compact` as job outputs.

Why "first successful build"? werf's cache is content-addressable and
shared across the PR's build history. On the very first build, the cache
contains only main-published layers, so `Rebuilt: false` truly means
"matches main". On any subsequent build, `Rebuilt: false` can also mean
"matches a layer this PR built earlier" — which would silently mask
images already modified by the PR. Anchoring the baseline to the first
successful build sidesteps that.

Algorithm:
  1. Read the baseline build report. werf's `Images` is a map keyed by
     werf image name in kebab-case (e.g. "node-manager/bashible-apiserver").
     Each entry has booleans `Final` / `Rebuilt` and a content
     `DockerImageDigest`. Keep only entries with `Final: true` and
     `Rebuilt: false` (= final, taken from main cache on the first PR
     build) -> "digests_old_main.json".
  2. Read images_digests.json from the assembled dev image of the current
     run. werf populates it with the same content `ImageDigest` values
     produced by the current build, keyed by {moduleCamel: {imageCamel: digest}}.
  3. Build a *set* of digests from digests_old_main.json. For every entry
     in images_digests.json, if its digest is not in that set, mark it as
     CHANGED. Comparing by digest (not by name) is robust:
       - it auto-handles the kebab<->camel name conversion, and
       - it correctly groups images whose content is bit-identical
         (e.g. csiExternalSnapshotter127..131 share one digest in the
         repo's real images_digests.json snapshots).
"""

import json
import os
import sys


def load_build_report(path: str) -> dict:
    with open(path) as fp:
        report = json.load(fp)
    images = report.get("Images")
    if isinstance(images, list):
        normalized = {}
        for entry in images:
            key = entry.get("WerfImageName") or entry.get("Name") or entry.get("Image")
            if key:
                normalized[key] = entry
        images = normalized
    if not isinstance(images, dict):
        raise SystemExit(f"unexpected build report shape at {path}: no Images map")
    return images


def collect_old_main_digests(images: dict) -> dict:
    """Return { werf_image_name: { "DockerImageDigest": "..." } } for every
    final image that werf reused from cache on the *first* PR build
    (Final == True and Rebuilt == False)."""
    out = {}
    for name, entry in images.items():
        if not isinstance(entry, dict):
            continue
        if not entry.get("Final"):
            continue
        if entry.get("Rebuilt") is True:
            continue
        digest = entry.get("DockerImageDigest")
        if not digest:
            continue
        out[name] = {"DockerImageDigest": digest}
    return out


def compute_changed(images_digests: dict, old_digest_set: set) -> list:
    changed = []
    for module, mod_images in (images_digests or {}).items():
        if not isinstance(mod_images, dict):
            continue
        for image, digest in mod_images.items():
            if not isinstance(digest, str):
                continue
            if digest in old_digest_set:
                continue
            changed.append({"module": module, "image": image, "digest": digest})
    changed.sort(key=lambda c: (c["module"], c["image"]))
    return changed


def emit_github_outputs(changed: list) -> None:
    out_path = os.environ.get("GITHUB_OUTPUT")
    if not out_path:
        return
    matrix = {"include": changed}
    compact = [f"{c['module']}.{c['image']}" for c in changed]
    with open(out_path, "a") as fp:
        fp.write(f"changed_count={len(changed)}\n")
        fp.write(f"matrix={json.dumps(matrix, separators=(',', ':'))}\n")
        fp.write(f"changed_compact={json.dumps(compact, separators=(',', ':'))}\n")


def main() -> int:
    build_report_path = os.environ.get("BUILD_REPORT_PATH", "images_tags_werf.json")
    images_digests_path = os.environ.get("IMAGES_DIGESTS_PATH", "images_digests.json")
    out_old = os.environ.get("OUTPUT_OLD_DIGESTS", "digests_old_main.json")
    out_changed = os.environ.get("OUTPUT_CHANGED", "changed_images.json")

    if not os.path.exists(build_report_path):
        # The baseline build_report_FE artifact (from the first successful
        # Build FE run of this PR) is gone — most likely older than the
        # GitHub Actions artifact retention window. Without it we cannot
        # compute a meaningful main-vs-PR digest diff, so we degrade
        # gracefully: emit empty outputs. Downstream CVE / rootless /
        # antivirus scanners treat an empty `changed_compact` as "no
        # filter" and fall back to scanning every image.
        print(
            "WARNING: baseline build report not found at "
            f"{build_report_path}.\n"
            "  The first successful Build FE artifact of this PR has likely\n"
            "  expired (GitHub Actions retention). A reliable diff against\n"
            "  main cannot be computed; emitting an empty changed_compact\n"
            "  filter. Downstream image scans will run in FULL-SCAN mode\n"
            "  (every image scanned)."
        )
        with open(out_old, "w") as fp:
            json.dump({}, fp)
        with open(out_changed, "w") as fp:
            json.dump([], fp)
        emit_github_outputs([])
        return 0

    images = load_build_report(build_report_path)
    old_main = collect_old_main_digests(images)

    with open(out_old, "w") as fp:
        json.dump(old_main, fp, indent=2, sort_keys=True)
    print(f"Baseline build report total entries: {len(images)}")
    print(f"digests_old_main.json entries:       {len(old_main)}  (kept Final=true && Rebuilt=false)")

    with open(images_digests_path) as fp:
        images_digests = json.load(fp)

    old_digest_set = {v["DockerImageDigest"] for v in old_main.values()}
    changed = compute_changed(images_digests, old_digest_set)

    total = sum(
        1
        for m in (images_digests or {}).values()
        if isinstance(m, dict)
        for _ in m.values()
    )
    with open(out_changed, "w") as fp:
        json.dump(changed, fp, indent=2)
    print(f"Module images in images_digests.json: {total}")
    print(f"Changed module images:                {len(changed)}")
    if changed:
        print("First 10 changed:")
        for c in changed[:10]:
            print(f"  {c['module']}.{c['image']}  {c['digest']}")

    emit_github_outputs(changed)
    return 0


if __name__ == "__main__":
    sys.exit(main())
