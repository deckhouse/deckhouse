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
import hashlib
import json
import os
import sys
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


def retryable_get(url: str, headers: dict, params: dict = None, max_retries: int = MAX_RETRIES) -> requests.Response:
    """
    Performs a GET request with retry logic to handle rate limiting and temporary errors.
    
    Args:
        url: URL for the request
        headers: HTTP headers
        params: Query parameters
        max_retries: Maximum number of retry attempts
    
    Returns:
        Response object
    
    Raises:
        RuntimeError: If all retry attempts are exhausted
    """
    sleep = RETRY_BACKOFF
    
    for attempt in range(1, max_retries + 1):
        try:
            response = requests.get(url, headers=headers, params=params, timeout=60)
            
            # Successful request
            if response.status_code == 200:
                return response
            
            # Rate limiting
            if response.status_code == 429:
                retry_after = int(response.headers.get("Retry-After", sleep))
                print(f"⚠️ Rate limited. Waiting {retry_after}s (attempt {attempt}/{max_retries})")
                time.sleep(retry_after + 1)
                sleep *= 2
                continue
            
            # Other HTTP errors
            response.raise_for_status()
            
        except requests.Timeout:
            if attempt == max_retries:
                raise RuntimeError(f"❌ Request timeout after {max_retries} attempts: {url}")
            print(f"⚠️ Request timeout (attempt {attempt}), retrying in {sleep}s...")
            time.sleep(sleep)
            sleep *= 2
            continue
        except RequestException as e:
            if attempt == max_retries:
                raise RuntimeError(f"❌ Request failed after {max_retries} attempts: {e}")
            print(f"⚠️ Request failed (attempt {attempt}), retrying in {sleep}s...")
            time.sleep(sleep)
            sleep *= 2
            continue
    
    raise RuntimeError(f"❌ Max retries ({max_retries}) exceeded for {url}")


def get_deckhouse_products(product_type_name: str = "DKP") -> list[dict]:
    """
    Get a list of all Deckhouse products (modules) from DefectDojo.
    
    Internally gets the product type ID by name and then retrieves all products of that type.
    
    Args:
        product_type_name: Product type name (default: "DKP")
    
    Returns:
        List of dictionaries with 'id' and 'name' fields, where each element represents a Deckhouse module.
        Example: [{"id": 100, "name": "000-common"}, {"id": 101, "name": "002-deckhouse"}, ...]
    
    Raises:
        RuntimeError: If product type is not found or on request errors
    """
    # Step 1: Get product type ID
    print(f"🔍 Finding Product Type ID for '{product_type_name}'...")
    params = {"name": product_type_name}
    response = retryable_get(f"{DD_URL}/api/v2/product_types/", HEADERS, params)
    data = response.json()
    results = data.get("results", [])
    
    if not results:
        raise RuntimeError(f"❌ Product Type '{product_type_name}' not found in DefectDojo.")
    
    product_type_id = results[0]["id"]
    print(f"✅ Found Product Type ID: {product_type_id}")
    
    # Step 2: Get list of products by type ID
    products = []
    limit = 1000
    offset = 0
    
    print("🔍 Fetching Deckhouse products from DefectDojo...")
    
    while True:
        params = {
            "prod_type": product_type_id,
            "limit": limit,
            "offset": offset
        }
        
        response = retryable_get(f"{DD_URL}/api/v2/products/", HEADERS, params)
        data = response.json()
        results = data.get("results", [])
        
        if not results:
            break
        
        products.extend([{"id": p["id"], "name": p["name"]} for p in results])
        
        # Check if there are more pages
        if len(results) < limit:
            break
        
        offset += limit
    
    print(f"✅ Found {len(products)} Deckhouse modules")
    return products


def get_engagements_for_product_versions(product_id: int, curr_tag: str, prev_tag: str) -> tuple[dict | None, dict | None]:
    """
    Get engagements for current and previous product versions by 'image_release_tag' tags.
    
    Searches for both engagements for the product and validates tag and version correctness.
    
    Args:
        product_id: Product (module) ID in DefectDojo
        curr_tag: Current version tag (e.g., "v1.73.0")
        prev_tag: Previous version tag (e.g., "v1.72.10")
    
    Returns:
        Tuple of two dictionaries: (curr_engagement, prev_engagement)
        If an engagement is not found, None is returned instead of a dictionary.
        Each engagement contains fields: id, name, tags and other data from DefectDojo API.
    """
    def _find_engagement_for_version(product_id: int, version_tag: str) -> dict | None:
        """Internal function to find engagement by version with validation."""
        print(f"   🔍 _find_engagement_for_version: product_id={product_id}, version_tag={version_tag}")
        tag_to_find = f"image_release_tag:{version_tag}"
        
        params = {
            "product": product_id,
            "tag": tag_to_find,
            "active": "true",
            "ordering": "-updated",
            "limit": 1
        }
        
        response = retryable_get(f"{DD_URL}/api/v2/engagements/", HEADERS, params)
        data = response.json()
        results = data.get("results", [])
        
        if not results:
            print(f"⚠️ No engagement found for version {version_tag}")
            print(f"   Searched in product ID {product_id}")
            print(f"   Missing tag: 'image_release_tag:{version_tag}'")
            return None
        
        engagement = results[0]
        
        # CRITICAL CHECK 1: Ensure the found engagement actually has the correct tag
        engagement_tags = engagement.get("tags", [])
        tag_list = [str(tag) for tag in engagement_tags] if engagement_tags else []
        
        if tag_to_find not in tag_list:
            print(f"⚠️ Found engagement '{engagement.get('name', 'N/A')}' (ID: {engagement.get('id')})")
            print(f"   but it does NOT have the required tag '{tag_to_find}'")
            print(f"   Engagement tags: {tag_list}")
            return None
        
        # CRITICAL CHECK 2: Verify that the version in the engagement name matches the requested one
        engagement_name = engagement.get("name", "")
        version_without_prefix = version_tag.lstrip("v")
        
        if version_without_prefix not in engagement_name:
            print(f"⚠️ WARNING: Engagement '{engagement_name}' has correct tag '{tag_to_find}'")
            print(f"   but name doesn't contain version '{version_tag}'. Continuing anyway.")
        
        return engagement
    
    print(f"   🔍 Searching for current engagement: {curr_tag}")
    try:
        curr_engagement = _find_engagement_for_version(product_id, curr_tag)
        print(f"   ✅ Current engagement search completed")
    except Exception as e:
        print(f"   ❌ Error searching for current engagement {curr_tag}: {e}")
        curr_engagement = None
    
    print(f"   🔍 Searching for previous engagement: {prev_tag}")
    try:
        prev_engagement = _find_engagement_for_version(product_id, prev_tag)
        print(f"   ✅ Previous engagement search completed")
    except Exception as e:
        print(f"   ❌ Error searching for previous engagement {prev_tag}: {e}")
        prev_engagement = None
    
    # If at least one engagement is not found - return None
    if curr_engagement is None or prev_engagement is None:
        return None, None
    
    return curr_engagement, prev_engagement


def get_findings_for_engagement(engagement_id: int) -> list[dict]:
    """
    Get all verified and active findings for an engagement.
    
    Args:
        engagement_id: Engagement ID
    
    Returns:
        List of finding dictionaries
    
    Raises:
        RuntimeError: On request errors
    """
    findings = []
    limit = 1000
    offset = 0
    
    while True:
        params = {
            "test__engagement": engagement_id,
            "verified": "true",
            "active": "true",
            "limit": limit,
            "offset": offset
        }
        
        response = retryable_get(f"{DD_URL}/api/v2/findings/", HEADERS, params)
        data = response.json()
        results = data.get("results", [])
        
        if not results:
            break
        
        # CRITICAL: Filter int values IMMEDIATELY, BEFORE adding to the list
        for item in results:
            if not isinstance(item, dict):
                print(f"⚠️ WARNING: Skipping malformed item in API response (type: {type(item).__name__}). Item: {item}")
                continue
            findings.append(item)
        
        # Check if there are more pages
        if len(results) < limit:
            break
        
        offset += limit
    
    return findings


def stable_id(finding: dict) -> str:
    """
    Generates a stable identifier for a finding.
    
    Field priority:
    1. unique_id_from_tool
    2. cve
    3. component_name + component_version
    4. Fallback: hash of title, file_path, line, cwe
    
    Args:
        finding: Finding dict from DefectDojo API
    
    Returns:
        Unique identifier in "prefix:value" format
    """
    # PROTECTION: Check input type
    if not isinstance(finding, dict):
        return f"invalid_type:{type(finding).__name__}:{finding}"
    
    # Priority 1: unique_id_from_tool
    if finding.get("unique_id_from_tool"):
        return f"uid:{finding['unique_id_from_tool']}"
    
    # Priority 2: CVE
    if finding.get("cve"):
        return f"cve:{finding['cve']}"
    
    # Priority 3: component + version
    comp = finding.get("component_name")
    ver = finding.get("component_version")
    if comp and ver:
        return f"comp:{comp}:{ver}"
    
    # Fallback: hash based on available fields
    basis = "|".join([
        str(finding.get("title", "")),
        str(finding.get("file_path", "")),
        str(finding.get("line", "")),
        str(finding.get("cwe", ""))
    ])
    
    return f"hash:{hashlib.sha1(basis.encode()).hexdigest()}"


def diff_findings(curr_findings: list[dict], prev_findings: list[dict], module_name: str) -> tuple[list[tuple[str, str, str, str]], list[tuple[str, str, str, str]], list[tuple[str, str]]]:
    """
    Compares two lists of findings and returns new, fixed, and unchanged vulnerabilities.
    
    Args:
        curr_findings: List of findings for current version
        prev_findings: List of findings for previous version
        module_name: Module name for vulnerability association
    
    Returns:
        Tuple of three lists: (added, fixed, unchanged)
        added and fixed: each element is a tuple (title, severity, module_name, status)
        unchanged: each element is a tuple (title, severity) - for backward compatibility
    """
    # Create dictionaries using stable_id as the key
    prev_dict = {stable_id(f): f for f in prev_findings}
    curr_dict = {stable_id(f): f for f in curr_findings}

    added = []
    fixed = []
    unchanged = []

    # Find new and unchanged vulnerabilities
    for finding_id, finding in curr_dict.items():
        if finding_id not in prev_dict:
            # New vulnerability
            added.append((finding.get("title", "Unknown"), finding.get("severity", "Unknown"), module_name, "New"))
        else:
            # Vulnerability was present before
            unchanged.append((finding.get("title", "Unknown"), finding.get("severity", "Unknown")))

    # Find fixed vulnerabilities
    for finding_id, finding in prev_dict.items():
        if finding_id not in curr_dict:
            fixed.append((finding.get("title", "Unknown"), finding.get("severity", "Unknown"), module_name, "Fixed"))

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
            patch_count = len(patches)
            raise RuntimeError(
                f"❌ No previous patch available for minor {minor}. "
                f"Only {patch_count} patch(es) found: {', '.join(patches)}. "
                f"Use --prev-version to specify a different version or choose a minor with multiple patches."
            )
        prev_tag = patches[idx - 1]

    return curr_tag, prev_tag


def severity_sort_key(severity: str) -> int:
    """
    Returns a numeric key for sorting by severity.
    Higher priority = smaller number (Critical comes first).
    """
    severity_order = {
        "Critical": 0,
        "High": 1,
        "Medium": 2,
        "Low": 3,
        "Info": 4,
        "Unknown": 5
    }
    return severity_order.get(severity, 99)


def generate_reports(curr_tag: str, prev_tag: str, module_results: dict, 
                     total_vulnerabilities: list[tuple[str, str, str, str]], 
                     total_unchanged: list[tuple[str, str]]) -> None:
    """
    Generates reports in JSON and Markdown formats.
    
    Args:
        curr_tag: Current version tag
        prev_tag: Previous version tag
        module_results: Results by module
        total_vulnerabilities: List of vulnerabilities (title, severity, module_name, status)
        total_unchanged: List of unchanged vulnerabilities
    """
    # Calculate statistics
    total_new_count = sum(1 for _, _, _, status in total_vulnerabilities if status == "New")
    total_fixed_count = sum(1 for _, _, _, status in total_vulnerabilities if status == "Fixed")
    
    # Split for JSON (backward compatibility)
    new_list = [(title, sev) for title, sev, _, status in total_vulnerabilities if status == "New"]
    fixed_list = [(title, sev) for title, sev, _, status in total_vulnerabilities if status == "Fixed"]
    
    # Generate report.json
    json_report = {
        "comparison": {
            "current_version": curr_tag,
            "previous_version": prev_tag
        },
        "summary": {
            "total_new": total_new_count,
            "total_fixed": total_fixed_count,
            "total_unchanged": len(total_unchanged)
        },
        "modules": module_results,
        "details": {
            "new": [{"title": title, "severity": sev} for title, sev in new_list],
            "fixed": [{"title": title, "severity": sev} for title, sev in fixed_list],
            "unchanged": [{"title": title, "severity": sev} for title, sev in total_unchanged]
        }
    }
    
    with open("report.json", "w", encoding="utf-8") as f:
        json.dump(json_report, f, indent=2, ensure_ascii=False)
    
    print("\n✅ Generated report.json")
    
    # Generate report.md
    md_lines = [
        f"# CVE Comparison Report",
        f"",
        f"## Overview",
        f"",
        f"- **Current Version:** `{curr_tag}`",
        f"- **Previous Version:** `{prev_tag}`",
        f"- **Modules Scanned:** {len(module_results)}",
        f"",
        f"## Summary",
        f"",
        f"| Metric | Count |",
        f"|--------|-------|",
        f"| 🆕 New Vulnerabilities | {total_new_count} |",
        f"| ✅ Fixed Vulnerabilities | {total_fixed_count} |",
        f"| 🔄 Still Present | {len(total_unchanged)} |",
        f"",
        f"## Results by Module",
        f"",
        f"| Module | New | Fixed | Still Present |",
        f"|--------|-----|-------|---------------|"
    ]
    
    for module_name, results in sorted(module_results.items()):
        md_lines.append(
            f"| {module_name} | {results['new']} | {results['fixed']} | {results['still']} |"
        )
    
    # Split vulnerabilities by status
    new_vulns = [(title, severity, module_name) for title, severity, module_name, status in total_vulnerabilities if status == "New"]
    fixed_vulns = [(title, severity, module_name) for title, severity, module_name, status in total_vulnerabilities if status == "Fixed"]
    
    # New Vulnerabilities table
    md_lines.extend([
        f"",
        f"## 🆕 New Vulnerabilities ({len(new_vulns)})",
        f""
    ])
    
    if new_vulns:
        md_lines.append("| Module | Title | Severity |")
        md_lines.append("|--------|-------|----------|")
        
        # Sort: severity → module → title
        sorted_new = sorted(new_vulns, key=lambda x: (severity_sort_key(x[1]), x[2], x[0]))
        
        for title, severity, module_name in sorted_new:
            md_lines.append(f"| {module_name} | {title} | {severity} |")
    else:
        md_lines.append("_No new vulnerabilities found._")
    
    # Fixed Vulnerabilities table
    md_lines.extend([
        f"",
        f"## ✅ Fixed Vulnerabilities ({len(fixed_vulns)})",
        f""
    ])
    
    if fixed_vulns:
        md_lines.append("| Module | Title | Severity |")
        md_lines.append("|--------|-------|----------|")
        
        # Sort: severity → module → title
        sorted_fixed = sorted(fixed_vulns, key=lambda x: (severity_sort_key(x[1]), x[2], x[0]))
        
        for title, severity, module_name in sorted_fixed:
            md_lines.append(f"| {module_name} | {title} | {severity} |")
    else:
        md_lines.append("_No vulnerabilities were fixed._")
    
    with open("report.md", "w", encoding="utf-8") as f:
        f.write("\n".join(md_lines))
    
    print("✅ Generated report.md")


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

    # Determine versions for comparison
    tags = get_registry_tags()

    if args.auto_compare_minors:
        curr_tag, prev_tag = auto_compare(tags)
    else:
        if not args.version:
            print("❌ Provide --version or use --auto-compare-minors")
            sys.exit(1)
        curr_tag, prev_tag = resolve_tags(tags, args.version, args.prev_version)

    print(f"🟢 Current version: {curr_tag}")
    print(f"🟡 Previous version: {prev_tag}")

    # Get list of all Deckhouse modules
    products = get_deckhouse_products("DKP")
    
    if not products:
        print("❌ No Deckhouse products found in DefectDojo")
        sys.exit(1)
    
    print(f"\n📦 Found {len(products)} modules to scan")

    # Initialize aggregating data structures
    total_vulnerabilities = []  # List of tuples (title, severity, module_name, status)
    total_unchanged = []
    module_results = {}
    skipped_modules = []  # Modules skipped due to missing image_release_tag

    # Main loop: process each module
    for product in products:
        module_name = product["name"]
        product_id = product["id"]
        
        print(f"\n🔍 Processing module: {module_name}")
        
        try:
            # Find engagements for both versions
            curr_engagement, prev_engagement = get_engagements_for_product_versions(product_id, curr_tag, prev_tag)
            
            # If engagements are not found - skip the module
            if curr_engagement is None or prev_engagement is None:
                print(f"   ⚠️ WARNING: engagement(s) not found for module {module_name}")
                if curr_engagement is None:
                    print(f"      Missing current engagement for version {curr_tag}")
                if prev_engagement is None:
                    print(f"      Missing previous engagement for version {prev_tag}")
                skipped_modules.append(module_name)  # Save skipped module name
                continue  # Skip the module and continue processing
            
            print(f"   Current engagement: {curr_engagement['name']} (ID: {curr_engagement['id']})")
            print(f"   Previous engagement: {prev_engagement['name']} (ID: {prev_engagement['id']})")
            
            # Get findings for both engagements
            curr_findings = get_findings_for_engagement(curr_engagement["id"])
            prev_findings = get_findings_for_engagement(prev_engagement["id"])
            
            print(f"   Current findings: {len(curr_findings)}")
            print(f"   Previous findings: {len(prev_findings)}")
            
            # Perform diff
            added, fixed, unchanged = diff_findings(curr_findings, prev_findings, module_name)
            
            # Aggregate results
            total_vulnerabilities.extend(added)  # added contains (title, severity, module_name, "New")
            total_vulnerabilities.extend(fixed)  # fixed contains (title, severity, module_name, "Fixed")
            total_unchanged.extend(unchanged)
            
            # Save module results
            module_results[module_name] = {
                "new": len(added),
                "fixed": len(fixed),
                "still": len(unchanged),
                "curr_engagement_id": curr_engagement["id"],
                "prev_engagement_id": prev_engagement["id"]
            }
            
            print(f"   ✅ New: {len(added)}, Fixed: {len(fixed)}, Still: {len(unchanged)}")
            
        except Exception as e:
            print(f"   ❌ Error processing {module_name}: {e}")
            sys.exit(1)

    # Output aggregated results
    print("\n" + "="*60)
    print("📊 AGGREGATED RESULTS ACROSS ALL MODULES")
    print("="*60)
    # Calculate statistics from total_vulnerabilities
    total_new_count = sum(1 for _, _, _, status in total_vulnerabilities if status == "New")
    total_fixed_count = sum(1 for _, _, _, status in total_vulnerabilities if status == "Fixed")
    
    print(f"\nTotal modules processed: {len(module_results)}")
    print(f"Total new vulnerabilities: {total_new_count}")
    print(f"Total fixed vulnerabilities: {total_fixed_count}")
    print(f"Total still present: {len(total_unchanged)}")

    print("\n🆕 New vulnerabilities:")
    new_vulns = [v for v in total_vulnerabilities if v[3] == "New"]
    if new_vulns:
        for title, sev, _, _ in new_vulns[:20]:  # Show first 20
            print(f"  [{sev}] {title}")
        if len(new_vulns) > 20:
            print(f"  ... and {len(new_vulns) - 20} more (see report.md for full list)")
    else:
        print("  None")

    print("\n✅ Fixed vulnerabilities:")
    fixed_vulns = [v for v in total_vulnerabilities if v[3] == "Fixed"]
    if fixed_vulns:
        for title, sev, _, _ in fixed_vulns[:20]:  # Show first 20
            print(f"  [{sev}] {title}")
        if len(fixed_vulns) > 20:
            print(f"  ... and {len(fixed_vulns) - 20} more (see report.md for full list)")
    else:
        print("  None")

    print("\n🔄 Still present vulnerabilities:")
    if total_unchanged:
        print(f"  Total: {len(total_unchanged)} vulnerabilities")
    else:
        print("  None")

    # Output list of skipped modules
    if skipped_modules:
        print(f"\n⚠️ WARNING: {len(skipped_modules)} module(s) skipped due to missing engagements:")
        for module in skipped_modules:
            print(f"  - {module}")
        print(f"   These modules are excluded from comparison but do not cause job failure.")
    else:
        print(f"\n✅ All modules processed successfully")

    # Generate reports
    print("\n" + "="*60)
    print("📄 GENERATING REPORTS")
    print("="*60)
    generate_reports(curr_tag, prev_tag, module_results, total_vulnerabilities, total_unchanged)
    
    # Check 1: If ALL modules are skipped - this is a critical error (non-existent reports)
    if len(skipped_modules) == len(products):
        print(f"\n❌ CRITICAL ERROR: No engagements found for any module.")
        print(f"   This means the requested versions ({curr_tag} vs {prev_tag}) do not exist in DefectDojo.")
        print(f"   Cannot perform comparison - reports do not exist.")
        sys.exit(1)
    
    # Check 2: If there are new vulnerabilities - fail the job (security regression)
    if total_new_count > 0:
        print(f"\n❌ Script failed: Found {total_new_count} new vulnerability/vulnerabilities in the new version.")
        print(f"   This indicates security regression.")
        sys.exit(1)
    
    # Successful completion
    if skipped_modules:
        print(f"\n⚠️ Warning: {len(skipped_modules)} module(s) could not be processed (missing engagements).")
        print(f"   Results are available for {len(module_results)} processed module(s).")
    else:
        print(f"\n✅ All modules processed successfully")
    print("\n✅ Comparison completed successfully!")


if __name__ == "__main__":
    main()
