#
# THIS FILE IS GENERATED, PLEASE DO NOT EDIT.
#

# Copyright 2025 Flant JSC
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

#!/usr/bin/env python3

import argparse
import os
import time
import requests
from packaging.version import Version
from requests.exceptions import RequestException


# ENV
DD_URL = os.getenv("DEFECTDOJO_URL")
DD_API_KEY = os.getenv("DEFECTDOJO_API_KEY")
REGISTRY_URL = os.getenv("REGISTRY_URL")
REGISTRY_USERNAME = os.getenv("REGISTRY_USERNAME")
REGISTRY_PASSWORD = os.getenv("REGISTRY_PASSWORD")

HEADERS = {"Authorization": f"Token {DD_API_KEY}"}

MAX_RETRIES = 8
RETRY_BACKOFF = 2  # seconds


def get_registry_tags():
    url = f"https://{REGISTRY_URL}/v2/deckhouse/fe/tags/list"

    def retryable_get(*args, **kwargs):
        for attempt in range(1, MAX_RETRIES + 1):
            try:
                return requests.get(*args, timeout=80, **kwargs)
            except RequestException as e:
                if attempt == MAX_RETRIES:
                    raise RuntimeError(f"❌ Registry request failed after {MAX_RETRIES} attempts: {e}")
                print(f"⚠️ Registry request failed (attempt {attempt}), retrying in {RETRY_BACKOFF}s...")
                time.sleep(RETRY_BACKOFF)

    response = retryable_get(
        url,
        headers={"User-Agent": "python-cve-checker/1.0", "Accept": "application/json"}
    )

    if response.status_code != 401 or "WWW-Authenticate" not in response.headers:
        response.raise_for_status()
        return [t for t in response.json().get("tags", []) if isinstance(t, str)]

    auth_header = response.headers["WWW-Authenticate"]
    auth_parts = dict(part.strip().split("=") for part in auth_header[7:].split(","))
    realm = auth_parts["realm"].strip('"')
    service = auth_parts.get("service", "").strip('"')
    scope = auth_parts.get("scope", "").strip('"')

    token_params = {"service": service}
    if scope:
        token_params["scope"] = scope

    token_response = retryable_get(
        realm,
        params=token_params,
        auth=(REGISTRY_USERNAME, REGISTRY_PASSWORD),
        headers={"User-Agent": "python-cve-checker/1.0", "Accept": "application/json"}
    )
    token_response.raise_for_status()
    token = token_response.json().get("token")
    if not token:
        raise RuntimeError("Bearer token not returned by registry")

    final_response = retryable_get(
        url,
        headers={"Authorization": f"Bearer {token}", "User-Agent": "python-cve-checker/1.0", "Accept": "application/json"}
    )
    final_response.raise_for_status()
    return [t for t in final_response.json().get("tags", []) if isinstance(t, str)]


def filter_patch_tags(tags, minor_version):
    version_prefix = f"v{minor_version}."
    filtered = [t for t in tags if t.startswith(version_prefix) and t.count(".") == 2]
    return sorted(filtered, key=lambda t: Version(t.lstrip("v")))


def get_defectdojo_findings(version_tag):
    findings = []
    limit = 100
    offset = 0

    while True:
        params = {
            "tags": f"image_release_tag:{version_tag}",
            "verified": "true",
            "limit": limit,
            "offset": offset,
        }
        response = requests.get(f"https://{DD_URL}/api/v2/findings/", headers=HEADERS, params=params)
        response.raise_for_status()
        data = response.json()
        results = data.get("results", [])
        findings.extend([(f["title"], f["severity"], f["active"]) for f in results])

        if not data.get("next"):
            break
        offset += limit

    return findings


def diff_findings(curr, prev):
    prev_dict = {title: (sev, active) for title, sev, active in prev}
    curr_dict = {title: (sev, active) for title, sev, active in curr}

    added = []
    fixed = []
    unchanged = []

    for title in curr_dict:
        curr_sev, curr_active = curr_dict[title]
        if title not in prev_dict:
            added.append((title, curr_sev))
        else:
            prev_sev, prev_active = prev_dict[title]
            if curr_active:
                unchanged.append((title, curr_sev))
            else:
                fixed.append((title, curr_sev))

    for title in prev_dict:
        if title not in curr_dict:
            fixed.append((title, prev_dict[title][0]))

    return added, fixed, unchanged


def auto_compare(tags):
    def is_valid_patch(t):
        return isinstance(t, str) and t.startswith("v") and t.count(".") == 2

    def get_minor(t):
        return ".".join(t.lstrip("v").split(".")[:2])

    valid_tags = [t for t in tags if is_valid_patch(t)]
    minors = sorted(set(get_minor(t) for t in valid_tags), key=Version)

    if len(minors) < 2:
        raise RuntimeError("❌ Not enough minor versions to compare")

    latest_minor = minors[-1]
    prev_minor = minors[-2]

    latest_patches = filter_patch_tags(valid_tags, latest_minor)
    prev_patches = filter_patch_tags(valid_tags, prev_minor)

    if not latest_patches or not prev_patches:
        raise RuntimeError("❌ Cannot determine latest patches for two minor versions")

    return latest_patches[-1], prev_patches[-1]


def resolve_tags(tags, version, prev_version):
    def find_latest_patch(minor): return filter_patch_tags(tags, minor)[-1]

    patch_tags = [t for t in tags if isinstance(t, str) and t.count(".") == 2 and t.startswith("v")]
    if not patch_tags:
        raise RuntimeError("❌ No valid patch tags found")

    if version.count(".") == 2:
        curr_tag = f"v{version}"
    else:
        curr_tag = find_latest_patch(version)

    if prev_version:
        if prev_version.count(".") == 2:
            prev_tag = f"v{prev_version}"
        else:
            prev_tag = find_latest_patch(prev_version)
    else:
        minor = ".".join(curr_tag.lstrip("v").split(".")[:2])
        patches = filter_patch_tags(tags, minor)
        idx = patches.index(curr_tag)
        if idx == 0:
            raise RuntimeError("❌ No previous patch version available.")
        prev_tag = patches[idx - 1]

    return curr_tag, prev_tag


def main():
    """
    Compare CVEs between two Deckhouse releases using DefectDojo.

    Usage examples:
      python compare_cve_between_releases.py --version 1.70
          → Compare last two patch versions of minor 1.70

      python compare_cve_between_releases.py --version 1.70 --prev-version 1.69
          → Compare latest patch of 1.70 vs latest patch of 1.69

      python compare_cve_between_releases.py --version 1.70.12 --prev-version 1.69.4
          → Compare exact patch versions 1.70.12 vs 1.69.4

      python compare_cve_between_releases.py --auto-compare-minors
          → Automatically detect two most recent minor releases and compare their latest patches
    """
    parser = argparse.ArgumentParser(
        description="Compare CVEs between two Deckhouse image releases using DefectDojo.\n\n"
                    "Examples:\n"
                    "  python compare_cve_between_releases.py --version 1.70\n"
                    "  python compare_cve_between_releases.py --version 1.70 --prev-version 1.69\n"
                    "  python compare_cve_between_releases.py --version 1.70.12 --prev-version 1.69.4\n"
                    "  python compare_cve_between_releases.py --auto-compare-minors",
        formatter_class=argparse.RawTextHelpFormatter
    )
    parser.add_argument("--version", help="X.Y or X.Y.Z")
    parser.add_argument("--prev-version", help="Optional previous version in X.Y or X.Y.Z format")
    parser.add_argument("--auto-compare-minors", action="store_true", help="Compare latest patches of last two minor releases")
    args = parser.parse_args()

    tags = get_registry_tags()

    if args.auto_compare_minors:
        curr_tag, prev_tag = auto_compare(tags)
    else:
        if not args.version:
            print("❌ Provide --version or use --auto-compare-minors")
            return
        curr_tag, prev_tag = resolve_tags(tags, args.version, args.prev_version)

    print(f"🟢 Current version: {curr_tag}")
    print(f"🟡 Previous version: {prev_tag}")

    curr_findings = get_defectdojo_findings(curr_tag)
    prev_findings = get_defectdojo_findings(prev_tag)

    added, fixed, unchanged = diff_findings(curr_findings, prev_findings)

    print("\n🆕 New vulnerabilities:")
    for title, sev in added:
        print(f"  [{sev}] {title}")

    print("\n✅ Fixed vulnerabilities:")
    for title, sev in fixed:
        print(f"  [{sev}] {title}")

    print("\n🔄 Still present vulnerabilities:")
    for title, sev in unchanged:
        print(f"  [{sev}] {title}")


if __name__ == "__main__":
    main()
