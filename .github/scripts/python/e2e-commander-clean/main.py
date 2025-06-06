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

import os
import time
import requests
import argparse
from datetime import datetime, timedelta, timezone
from typing import List, Dict

# Constants and configuration
E2E_COMMANDER_HOST = os.environ['E2E_COMMANDER_HOST']
E2E_COMMANDER_TOKEN = os.environ['E2E_COMMANDER_TOKEN']
HOURS_TO_REMOVE = 24 * 2  # 2 days
COMMANDER_TIME_FORMAT = "%Y-%m-%dT%H:%M:%S.%f%z"
HEADERS = {'X-Auth-Token': E2E_COMMANDER_TOKEN}


class DeletionError(Exception):
    """Exception raised when cluster deletion fails."""

    def __init__(self, deletion_failed=None, timeout=None):
        self.deletion_failed = deletion_failed or []
        self.timeout = timeout or []
        super().__init__("Failed to delete clusters")


def get_clusters() -> List[Dict]:
    """Fetch all clusters from the Commander API."""
    response = requests.get(f"https://{E2E_COMMANDER_HOST}/api/v1/clusters", headers=HEADERS)
    response.raise_for_status()
    return response.json()


def get_cluster_status(cluster_id: str) -> str:
    """Get the current status of a cluster by ID."""
    url = f"https://{E2E_COMMANDER_HOST}/api/v1/clusters/{cluster_id}"
    response = requests.get(url, headers=HEADERS)
    response.raise_for_status()
    return response.json()["status"]


def wait_for_cluster_deletion(clusters: List[Dict[str, str]]) -> None:
    """Poll the status of clusters until deletion is confirmed or timeout."""
    sleep_time = 30
    max_attempts = 60
    attempt = 0
    deletion_failed = []

    while clusters and attempt < max_attempts:
        attempt += 1
        print(f"\nWaiting for clusters to be deleted, attempt {attempt}/{max_attempts}")
        print("=" * 40)

        clusters_copy = clusters.copy()
        for cluster in clusters_copy:
            try:
                status = get_cluster_status(cluster["id"])
            except Exception as e:
                print(f"Error fetching status for {cluster['name']}: {e}")
                continue

            print(f"- {cluster['name']} --- {status}")

            if status == "deleted":
                clusters.remove(cluster)
            elif status == "deletion_failed":
                deletion_failed.append(cluster)
                clusters.remove(cluster)

        if not clusters:
            break
        time.sleep(sleep_time)

    if deletion_failed or clusters:
        raise DeletionError(deletion_failed=deletion_failed, timeout=clusters)


def delete_clusters_by_list(clusters_to_delete: List[Dict[str, str]]) -> None:
    """Delete specified list of clusters and verify their deletion."""
    for cluster in clusters_to_delete:
        cluster_id = cluster["id"]
        url = f"https://{E2E_COMMANDER_HOST}/api/v1/clusters/{cluster_id}"
        try:
            requests.delete(url, headers=HEADERS)
        except Exception as e:
            print(f"Error deleting {cluster['name']} ({cluster_id}): {e}")
            continue

    try:
        wait_for_cluster_deletion(clusters_to_delete)
    except DeletionError as e:
        if e.deletion_failed:
            print("\nDeletion failed for clusters:")
            for cluster in e.deletion_failed:
                print(f"- {cluster['name']}")
        if e.timeout:
            print("\nTimeout while deleting clusters:")
            for cluster in e.timeout:
                print(f"- {cluster['name']}")
        exit(1)
    else:
        print("All specified clusters were successfully deleted.")


def remove_old_clusters(clusters: List[Dict]) -> None:
    """Remove clusters older than a specified number of hours."""
    print(f"Removing all clusters older than {HOURS_TO_REMOVE} hours")
    expire_time = datetime.now(timezone.utc) - timedelta(hours=HOURS_TO_REMOVE)
    clusters_to_delete = []

    for cluster in clusters:
        try:
            created_at = datetime.strptime(cluster["created_at"], COMMANDER_TIME_FORMAT)
        except Exception as e:
            print(f"Failed to parse created_at for cluster {cluster['name']}: {e}")
            continue

        if created_at < expire_time:
            print(f"Cluster {cluster['name']} was created more than {HOURS_TO_REMOVE} hours ago. Deleting.")
            clusters_to_delete.append({"id": cluster["id"], "name": cluster["name"]})
        else:
            print(f"Cluster {cluster['name']} is younger than {HOURS_TO_REMOVE} hours. Skipping.")

    delete_clusters_by_list(clusters_to_delete)


def remove_clusters_by_pr(clusters: List[Dict], pr_number: str) -> None:
    """Remove clusters created from a specific pull request."""
    print(f"Removing all clusters created in PR: {pr_number}")
    clusters_to_delete = []

    for cluster in clusters:
        pr_tag = cluster.get("values", {}).get("branch")
        if pr_tag == f"pr{pr_number}":
            print(f"Cluster {cluster['name']} was created in PR {pr_number}. Deleting.")
            clusters_to_delete.append({"id": cluster["id"], "name": cluster["name"]})

    if clusters_to_delete:
        delete_clusters_by_list(clusters_to_delete)
    else:
        print("No clusters found for the specified PR.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="E2E Cluster Autocleaner")
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--auto", action="store_true", help=f"Delete clusters older than {HOURS_TO_REMOVE} hours")
    group.add_argument("--pr", type=str, help="Delete clusters created by the specified pull request")
    args = parser.parse_args()

    clusters_data = get_clusters()

    if args.auto:
        remove_old_clusters(clusters_data)
    elif args.pr is not None:
        remove_clusters_by_pr(clusters_data, args.pr)
