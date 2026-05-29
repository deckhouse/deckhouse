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

"""Diff two werf build reports for the same Git SHA.

Usage:
    reproducibility_check.py <baseline.json> <rerun.json>

Edition is derived from the parent directory of the baseline report path
(e.g. 'build_report_FE/images_tags_werf.json' -> 'FE').

Exit code:
    0  all DockerImageDigest values match
    1  at least one image differs
    2  usage error
"""

import difflib
import json
import sys
from pathlib import Path


def detect_edition(report_path: Path) -> str:
    parent = report_path.parent.name
    if parent.startswith("build_report_"):
        return parent[len("build_report_"):]
    return parent or report_path.stem


def load_report(report_path: Path) -> dict:
    with report_path.open(encoding="utf-8") as fh:
        return json.load(fh)


def entry_lines(entry: dict) -> list:
    return json.dumps(entry, sort_keys=True, indent=2).splitlines()


# Deckhouse-specific: module images are rendered by .werf/defines/modules.tmpl as
# "<ModuleName>/<ImageName>". Image names whose first segment is one of these is
# a global meta-image, not a module (e.g. 'dev/install', 'base/vex').
_NON_MODULE_PREFIXES = {"dev", "base"}


def split_module(image_name: str) -> tuple:
    """Return (module, image_subname) for table display.

    Returns ('—', image_name) for global images that don't belong to a module.
    """
    if "/" not in image_name:
        return "—", image_name
    head, _, tail = image_name.partition("/")
    if head in _NON_MODULE_PREFIXES:
        return "—", image_name
    return head, tail


def main(argv: list) -> int:
    if len(argv) != 3:
        print(f"Usage: {argv[0]} <baseline.json> <rerun.json>", file=sys.stderr)
        return 2

    baseline_path = Path(argv[1])
    rerun_path = Path(argv[2])
    edition = detect_edition(baseline_path)

    baseline_images = load_report(baseline_path).get("Images") or {}
    rerun_images    = load_report(rerun_path).get("Images") or {}

    mismatched = []
    for name in sorted(set(baseline_images) | set(rerun_images)):
        b_digest = baseline_images.get(name, {}).get("DockerImageDigest", "")
        r_digest = rerun_images.get(name, {}).get("DockerImageDigest", "")
        if b_digest != r_digest:
            mismatched.append((name, b_digest, r_digest))

    print(f"# Build reproducibility check ({edition})")
    print()
    print(f"Baseline: {baseline_path}")
    print(f"Rerun:    {rerun_path}")
    print(f"Images in baseline: {len(baseline_images)}; in rerun: {len(rerun_images)}")
    print()

    if not mismatched:
        print(f"OK: all {len(baseline_images)} image digests match. Build is reproducible.")
        return 0

    print(f"Digest mismatch: {len(mismatched)} image(s) differ.")
    print()
    print("| Module | Image | Baseline digest | Rerun digest |")
    print("|---|---|---|---|")
    for name, b_digest, r_digest in mismatched:
        module, image = split_module(name)
        print(f"| `{module}` | `{image}` | `{b_digest or '—'}` | `{r_digest or '—'}` |")
    print()

    print("## Per-image diff")
    print()
    for name, _, _ in mismatched:
        b_entry = baseline_images.get(name, {})
        r_entry = rerun_images.get(name, {})
        print(f"### `{name}`")
        print()
        print("```diff")
        diff = difflib.unified_diff(
            entry_lines(b_entry),
            entry_lines(r_entry),
            fromfile=f"baseline:Images.{name}",
            tofile=f"rerun:Images.{name}",
            lineterm="",
        )
        for line in diff:
            print(line)
        print("```")
        print()

    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
