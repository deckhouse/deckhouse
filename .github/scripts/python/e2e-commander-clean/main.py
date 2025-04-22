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

import requests
import os
from datetime import datetime, timedelta, timezone
import time

e2e_commander_host = os.environ['E2E_COMMANDER_HOST']
e2e_commander_token = os.environ['E2E_COMMANDER_TOKEN']
HOURS_TO_REMOVE = 24 * 2 # 2 days
COMMANDER_TIME_FORMAT = "%Y-%m-%dT%H:%M:%S.%f%z"
commander_headers = {
        'X-Auth-Token': e2e_commander_token,
    }

class DeletionError(Exception):
    """
    Exception that occurs when cluster deletion fails.
    """
    def __init__(self, deletion_failed=None, timeout=None):
        self.deletion_failed = deletion_failed or []
        self.timeout = timeout or []
        super().__init__("Failed to delete clusters")

def get_clusters():
    cls = requests.get(f"https://{e2e_commander_host}/api/v1/clusters", headers = commander_headers)
    return cls


def get_cluster_status(cluster_id: str):
    url = f"https://{e2e_commander_host}/api/v1/clusters/{cluster_id}"
    response = requests.get(url=url, headers=commander_headers).json()
    return response["status"]

def get_clusters_delete_status(clusters: list[dict[str: str, str: str]]):
    sleep_time = 30
    attempt = 0
    max_attempt = 60
    deletion_failed_clusters = []
    while len(clusters) > 0:
        attempt += 1
        print(f"\nWait to cluster delete, attempt {attempt}/{max_attempt}")
        print("=" * 40)
        for cluster in clusters:
            try:
                status = get_cluster_status(cluster["id"])
            except Exception as e:
                print(e)
                print("Error getting cluster status, continue...")
                continue
            print(f'-  {cluster["name"]} --- {status}')
            if status == "deleted":
                clusters.remove({"id": cluster_id, "name": cluster["name"]})
            elif status == "deletion_failed":
                deletion_failed_clusters.append({"id": cluster_id, "name": cluster["name"]})
                clusters.remove({"id": cluster_id, "name": cluster["name"]})
            else:
                continue
        if (len(deletion_failed_clusters) > 0 and len(clusters) == 0) or attempt >= max_attempt:
            raise DeletionError(deletion_failed=deletion_failed_clusters, timeout=clusters)
        time.sleep(sleep_time)

if __name__ == "__main__":

    expire_time = datetime.now(timezone.utc) - timedelta(hours=HOURS_TO_REMOVE)
    clusters = get_clusters().json()
    clusters_to_delete = []
    for i in clusters:
        cluster_id = i["id"]
        cluster_name = i["name"]
        created_at = datetime.strptime(i["created_at"], COMMANDER_TIME_FORMAT)
        if created_at < expire_time:
            print(f"Cluster {cluster_name} created more than {HOURS_TO_REMOVE} hours ago, deleting")
            url = f"https://{e2e_commander_host}/api/v1/clusters/{cluster_id}"
            requests.delete(url=url, headers=commander_headers)
            clusters_to_delete.append({"id": cluster_id, "name": cluster_name})
        else:
            print(f"Cluster {cluster_name} created less than {HOURS_TO_REMOVE} hours ago, skip")
            skip_delete = True

    try:
        get_clusters_delete_status(clusters_to_delete)
    except DeletionError as e:
        if len(e.deletion_failed) > 0:
            print("\nError deleting clusters, were not deleted:")
            for i in e.deletion_failed:
                print(f"-  {i['name']}")
        if len(e.timeout) > 0:
            print("\nTimeout deleting clusters, were not deleted:")
            for i in e.timeout:
                print(f"-  {i['name']}")
        exit(1)
    else:
        print("All clusters were successfully removed.")
