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

def get_clusters():
    cls = requests.get(f"https://{e2e_commander_host}/api/v1/clusters", headers = commander_headers)
    return cls


def get_cluster_status(cluster_id: str):
    url = f"https://{e2e_commander_host}/api/v1/clusters/{cluster_id}"
    response = requests.get(url=url, headers=commander_headers).json()
    return response["status"]

def delete_cluster(cluster_id: str, cluster_name: str):
    cluster_is_deleted = False
    sleep_time = 0
    url = f"https://${e2e_commander_host}/api/v1/clusters/${cluster_id}"
    # requests.delete(url=url, headers=commander_headers)
    while not cluster_is_deleted:
        time.sleep(sleep_time)
        sleep_time = 10
        cluster_status = get_cluster_status(cluster_id)
        if cluster_status == "deleted":
            print(f"-  Cluster {cluster_name} deleted")
            cluster_is_deleted = True
        elif cluster_status == "deletion_failed":
            print(f"-  Cluster {cluster_name}: deletion_failed")
        else:
            print(f"-  Cluster {cluster_name}: {cluster_status}")
            continue

if __name__ == "__main__":

    expire_time = datetime.now(timezone.utc) - timedelta(hours=HOURS_TO_REMOVE)
    clusters = get_clusters().json()
    for i in clusters:
        cluster_id = i["id"]
        cluster_name = i["name"]
        print(i["created_at"])
        created_at = datetime.strptime(i["created_at"], COMMANDER_TIME_FORMAT)
        print(created_at)
        print(datetime.now(timezone.utc))
        if created_at < expire_time:
            print(f"Cluster {cluster_name} created more than {HOURS_TO_REMOVE} hours ago, deleting")
            delete_cluster(cluster_id, cluster_name)
        else:
            print(f"Cluster {cluster_name} created less than {HOURS_TO_REMOVE} hours ago, skip")
            skip_delete = True



