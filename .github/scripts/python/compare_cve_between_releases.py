#!/usr/bin/env python3

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

import argparse
import os
import requests
from packaging.version import Version
from dotenv import load_dotenv

load_dotenv()

# ENV
DD_URL = os.getenv("DEFECTDOJO_URL")
DD_API_KEY = os.getenv("DEFECTDOJO_API_KEY")
REGISTRY_URL = os.getenv("REGISTRY_URL")
REGISTRY_IMAGE = os.getenv("REGISTRY_IMAGE")
REGISTRY_USERNAME = os.getenv("REGISTRY_USERNAME")
REGISTRY_PASSWORD = os.getenv("REGISTRY_PASSWORD")

HEADERS = {"Authorization": f"Token {DD_API_KEY}"}


def get_registry_tags():
    url = f"https://{REGISTRY_URL}/v2/{REGISTRY_IMAGE}/tags/list"
    response = requests.get(
        url,
        headers={"User-Agent": "python-cve-checker/1.0", "Accept": "application/json"},
        timeout=10
    )

    if response.status_code != 401 or "WWW-Authenticate" not in response.headers:
        response.raise_for_status()
        return response.json().get("tags", [])

    auth_header = response.headers["WWW-Authenticate"]
    if not auth_header.startswith("Bearer"):
        raise RuntimeError("Unsupported authentication scheme (expected Bearer)")

    auth_parts = {}
    for part in auth_header[len("Bearer "):].split(","):
        key, value = part.strip().split("=")
        auth_parts[key] = value.strip('"')

    realm = auth_parts["realm"]
    service = auth_parts.get("service")
    scope = auth_parts.get("scope")

    token_params = {"service": service}
    if scope:
        token_params["scope"] = scope

    token_response = requests.get(
        realm,
        params=token_params,
        auth=(REGISTRY_USERNAME, REGISTRY_PASSWORD),
        headers={"User-Agent": "python-cve-checker/1.0", "Accept": "application/json"},
        timeout=10
    )
    token_response.raise_for_status()
    token = token_response.json().get("token")
    if not token:
        raise RuntimeError("Bearer token not returned by registry")

    final_response = requests.get(
        url,
        headers={
            "Authorization": f"Bearer {token}",
            "User-Agent": "python-cve-checker/1.0",
            "Accept": "application/json"
        },
        timeout=30
    )
    final_response.raise_for_status()
    return final_response.json().get("tags", [])


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
        response = requests.get(f"{DD_URL}/api/v2/findings/", headers=HEADERS, params=params)
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


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True, help="X.Y or X.Y.Z")
    parser.add_argument("--prev-version", help="Optional previous version in X.Y.Z format")
    args = parser.parse_args()

    tags = get_registry_tags()

    if args.version.count(".") == 1:
        patch_tags = filter_patch_tags(tags, args.version)
        if not patch_tags:
            print(f"❌ No tags found for minor version {args.version}")
            return
        curr_tag = patch_tags[-1]

        if args.prev_version:
            prev_tag = f"v{args.prev_version}"
            if prev_tag not in patch_tags:
                print(f"❌ Provided prev-version {prev_tag} not found in registry.")
                return
        else:
            if len(patch_tags) < 2:
                print("❌ Not enough patch versions for comparison.")
                return
            prev_tag = patch_tags[-2]

    else:
        curr_tag = f"v{args.version}"
        minor = ".".join(args.version.split(".")[:2])
        patch_tags = filter_patch_tags(tags, minor)

        if curr_tag not in patch_tags:
            print(f"❌ Current tag {curr_tag} not found in registry.")
            return

        if args.prev_version:
            prev_tag = f"v{args.prev_version}"
            if prev_tag not in patch_tags:
                print(f"❌ Provided prev-version {prev_tag} not found in registry.")
                return
        else:
            idx = patch_tags.index(curr_tag)
            if idx == 0:
                print("❌ No previous patch version available.")
                return
            prev_tag = patch_tags[idx - 1]

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