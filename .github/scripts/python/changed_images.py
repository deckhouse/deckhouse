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
Detect final werf images that should be scanned for the current PR.

Algorithm:
  1. Get PR commits.
  2. Read current images_tags_werf.json.
  3. Keep only entries with:
       Final == true
       Commit in PR commits
  4. Convert werf image name into module/image fields for matrix output.
  5. Read images_digests.json and map changed digests to scanner keys.
  6. Write changed_images.json and GitHub Actions outputs.

changed_compact must use keys from images_digests.json, for example:
  nodeManager.bashibleApiserver

It must not be built from WerfImageName, for example:
  node-manager.bashible-apiserver

The AV scanner and rootless scanner match ONLY_IMAGES against keys from
images_digests.json.
"""

import json
import os
import subprocess
import sys
from typing import Any, Optional


NON_MODULE_WERF_IMAGE_NAMES = {
    "dev",
    "dev-prebuild",
    "dev-vex-artifact",
    "release-channel-version",
    "deckhouse-main",
    "deckhouse-install",
    "deckhouse-install-standalone",
    "install",
    "install-standalone",
    "install-vex-artifact",
}

NON_MODULE_WERF_IMAGE_PREFIXES = (
    "dev/",
)


def run(cmd: list[str]) -> str:
    return subprocess.check_output(cmd, text=True).strip()


def load_json(path: str) -> Any:
    with open(path) as fp:
        return json.load(fp)


def load_build_report(path: str) -> dict:
    report = load_json(path)

    images = report.get("Images")
    if isinstance(images, list):
        normalized = {}

        for entry in images:
            if not isinstance(entry, dict):
                continue

            key = entry.get("WerfImageName") or entry.get("Name") or entry.get("Image")
            if key:
                normalized[key] = entry

        images = normalized

    if not isinstance(images, dict):
        raise SystemExit(f"unexpected build report shape at {path}: no Images map")

    return images


def load_images_digests(path: str) -> dict:
    data = load_json(path)

    if not isinstance(data, dict):
        raise SystemExit(f"unexpected images digests shape at {path}: root is not object")

    return data


def is_zero_sha(value: str) -> bool:
    return bool(value) and set(value) == {"0"}


def get_before_commit_from_event() -> Optional[str]:
    before = os.environ.get("GITHUB_EVENT_BEFORE")
    if before:
        return before

    event_path = os.environ.get("GITHUB_EVENT_PATH")
    if not event_path or not os.path.exists(event_path):
        return None

    try:
        event = load_json(event_path)
    except (OSError, json.JSONDecodeError):
        return None

    before = event.get("before")
    if isinstance(before, str) and before:
        return before

    return None


def get_commits_from_range(revision_range: str) -> set[str]:
    commits = run(["git", "log", "--format=%H", revision_range])
    return set(commits.splitlines()) if commits else set()


def get_pr_commits() -> set[str]:
    base_ref = os.environ.get("GITHUB_BASE_REF")

    if base_ref:
        if not base_ref.startswith("origin/"):
            base_ref = f"origin/{base_ref}"

        merge_base = run(["git", "merge-base", base_ref, "HEAD"])
        return get_commits_from_range(f"{merge_base}..HEAD")

    before = get_before_commit_from_event()
    if before and not is_zero_sha(before):
        return get_commits_from_range(f"{before}..HEAD")

    head = run(["git", "rev-parse", "HEAD"])
    return {head}


def split_werf_image_name(name: str) -> tuple[str, str]:
    if "/" not in name:
        return name, name

    module, image = name.split("/", 1)
    return module, image


def normalize_digest(value: str) -> Optional[str]:
    if not value:
        return None

    marker = "sha256:"
    index = value.find(marker)
    if index == -1:
        return None

    return value[index:]


def extract_digest(value: Any) -> Optional[str]:
    if isinstance(value, str):
        return normalize_digest(value)

    if not isinstance(value, dict):
        return None

    digest_keys = (
        "digest",
        "Digest",
        "dockerImageDigest",
        "DockerImageDigest",
        "docker_image_digest",
        "imageDigest",
        "ImageDigest",
    )

    for key in digest_keys:
        digest = value.get(key)
        if isinstance(digest, str):
            normalized = normalize_digest(digest)
            if normalized:
                return normalized

    return None


def build_compact_keys_by_digest(images_digests: dict) -> dict[str, list[str]]:
    compact_keys_by_digest: dict[str, list[str]] = {}

    for module, images in images_digests.items():
        if not isinstance(module, str):
            continue

        if not isinstance(images, dict):
            continue

        for image, value in images.items():
            if not isinstance(image, str):
                continue

            digest = extract_digest(value)
            if not digest:
                continue

            compact_key = f"{module}.{image}"
            compact_keys_by_digest.setdefault(digest, []).append(compact_key)

    for keys in compact_keys_by_digest.values():
        keys.sort()

    return compact_keys_by_digest


def is_non_module_werf_image(name: str) -> bool:
    if name in NON_MODULE_WERF_IMAGE_NAMES:
        return True

    return name.startswith(NON_MODULE_WERF_IMAGE_PREFIXES)


def requires_compact_key(name: str) -> bool:
    if "/" not in name:
        return False

    if is_non_module_werf_image(name):
        return False

    return True


def compute_changed(
    images: dict,
    pr_commits: set[str],
    compact_keys_by_digest: dict[str, list[str]],
) -> list:
    changed = []

    for name, entry in images.items():
        if not isinstance(entry, dict):
            continue

        if entry.get("Final") is not True:
            continue

        commit = entry.get("Commit")
        if not commit or commit not in pr_commits:
            continue

        digest = normalize_digest(str(entry.get("DockerImageDigest", "")))
        if not digest:
            continue

        werf_image_name = entry.get("WerfImageName") or name
        module, image = split_werf_image_name(werf_image_name)

        compact_keys = compact_keys_by_digest.get(digest, [])

        changed.append({
            "module": module,
            "image": image,
            "digest": digest,
            "commit": commit,
            "werf_image_name": werf_image_name,
            "compact_keys": compact_keys,
        })

    changed.sort(key=lambda c: (c["module"], c["image"]))
    return changed


def get_missing_compact_key_images(changed: list) -> list:
    missing = []

    for item in changed:
        werf_image_name = item.get("werf_image_name", "")
        compact_keys = item.get("compact_keys", [])

        if requires_compact_key(werf_image_name) and not compact_keys:
            missing.append(item)

    return missing


def build_changed_compact(changed: list) -> list[str]:
    compact = set()

    for item in changed:
        for compact_key in item.get("compact_keys", []):
            compact.add(compact_key)

    return sorted(compact)


def build_matrix(changed: list) -> dict:
    include = []

    for item in changed:
        include.append({
            "module": item["module"],
            "image": item["image"],
            "digest": item["digest"],
            "commit": item["commit"],
            "werf_image_name": item["werf_image_name"],
        })

    return {"include": include}


def emit_github_outputs(changed: list) -> None:
    out_path = os.environ.get("GITHUB_OUTPUT")
    if not out_path:
        return

    matrix = build_matrix(changed)
    compact = build_changed_compact(changed)

    with open(out_path, "a") as fp:
        fp.write(f"changed_count={len(changed)}\n")
        fp.write(f"matrix={json.dumps(matrix, separators=(',', ':'))}\n")
        fp.write(f"changed_compact={json.dumps(compact, separators=(',', ':'))}\n")


def print_changed(changed: list) -> None:
    if not changed:
        return

    print("Images for scan:")

    for item in changed:
        compact_keys = item.get("compact_keys") or ["<no compact key>"]
        print(
            f"  {item['werf_image_name']}  "
            f"{','.join(compact_keys)}  "
            f"{item['commit']}  "
            f"{item['digest']}"
        )


def main() -> int:
    build_report_path = os.environ.get("BUILD_REPORT_PATH", "images_tags_werf.json")
    images_digests_path = os.environ.get("IMAGES_DIGESTS_PATH", "images_digests.json")
    out_changed = os.environ.get("OUTPUT_CHANGED", "changed_images.json")

    if not os.path.exists(build_report_path):
        raise SystemExit(f"ERROR: build report not found: {build_report_path}")

    if not os.path.exists(images_digests_path):
        raise SystemExit(f"ERROR: images digests not found: {images_digests_path}")

    pr_commits = get_pr_commits()
    print(f"PR commits: {len(pr_commits)}")

    images = load_build_report(build_report_path)
    print(f"Build report total entries: {len(images)}")

    images_digests = load_images_digests(images_digests_path)
    compact_keys_by_digest = build_compact_keys_by_digest(images_digests)
    compact_keys_count = sum(len(keys) for keys in compact_keys_by_digest.values())
    print(f"Images digests compact keys: {compact_keys_count}")

    changed = compute_changed(images, pr_commits, compact_keys_by_digest)

    missing_compact = get_missing_compact_key_images(changed)
    if missing_compact:
        print("WARNING: failed to map changed module images to images_digests.json keys:")
        for item in missing_compact:
            print(f"  {item['werf_image_name']}  {item['digest']}  {item['commit']}")
        print("WARNING: unmapped images will be skipped by scanners that use changed_compact")

    changed_compact = build_changed_compact(changed)

    with open(out_changed, "w") as fp:
        json.dump(changed, fp, indent=2)

    print(f"Changed final images: {len(changed)}")
    print(f"Changed compact keys: {len(changed_compact)}")

    print_changed(changed)
    emit_github_outputs(changed)

    return 0


if __name__ == "__main__":
    sys.exit(main())
