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


def load_digests(report_path: Path) -> dict:
    with report_path.open(encoding="utf-8") as fh:
        data = json.load(fh)
    return {
        name: info.get("DockerImageDigest", "")
        for name, info in (data.get("Images") or {}).items()
    }


def normalized_json(report_path: Path) -> list:
    data = json.loads(report_path.read_text(encoding="utf-8"))
    return json.dumps(data, sort_keys=True, indent=2).splitlines()


def main(argv: list) -> int:
    if len(argv) != 3:
        print(f"Usage: {argv[0]} <baseline.json> <rerun.json>", file=sys.stderr)
        return 2

    baseline_path = Path(argv[1])
    rerun_path = Path(argv[2])
    edition = detect_edition(baseline_path)

    baseline = load_digests(baseline_path)
    rerun = load_digests(rerun_path)

    mismatched = []
    for image in sorted(set(baseline) | set(rerun)):
        b = baseline.get(image, "")
        r = rerun.get(image, "")
        if b != r:
            mismatched.append((image, b, r))

    print(f"# Build reproducibility check ({edition})")
    print()
    print(f"Baseline: {baseline_path}")
    print(f"Rerun:    {rerun_path}")
    print(f"Images in baseline: {len(baseline)}; in rerun: {len(rerun)}")
    print()

    if not mismatched:
        print(f"OK: all {len(baseline)} image digests match. Build is reproducible.")
        return 0

    print(f"Digest mismatch: {len(mismatched)} image(s) differ.")
    print()
    print("| Image | Baseline digest | Rerun digest |")
    print("|---|---|---|")
    for image, baseline_digest, rerun_digest in mismatched:
        print(f"| `{image}` | `{baseline_digest or '—'}` | `{rerun_digest or '—'}` |")
    print()

    print("## JSON diff")
    print()
    print("```diff")
    diff = difflib.unified_diff(
        normalized_json(baseline_path),
        normalized_json(rerun_path),
        fromfile=str(baseline_path),
        tofile=str(rerun_path),
        lineterm="",
    )
    for line in diff:
        print(line)
    print("```")
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
