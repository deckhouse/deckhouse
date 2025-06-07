#!/usr/bin/python3

# Copyright 2023 Flant JSC
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

import json
import os, time
from deckhouse import hook
from slugify import slugify

root_path = f"/etc/grafana/dashboards/"


def main(ctx: hook.Context):
    known_uids = set()
    malformed_dashboards = []
    dashboard_dict = {}
    ts = time.time()

    for i in ctx.snapshots.get("dashboard_resources", []):
        dashboard = i["filterResult"]
        dashboard_name = dashboard.get('name', '')
        try:
            definition = json.loads(dashboard["definition"])
        except Exception as e:
            print(f"ERROR: Dashboard '{dashboard_name}' contains errors: {e}.")
            malformed_dashboards.append(dashboard_name)
            continue

        title = definition.get("title")
        if not title:
            malformed_dashboards.append(dashboard_name)
            continue

        title = slugify(title)

        if not definition.get("uid"):
            print(f"ERROR: definition.uid is mandatory field missing in the dashboard {dashboard_name}")
            malformed_dashboards.append(dashboard_name)
            continue

        uid = definition["uid"]
        if uid in known_uids:
            print(f"ERROR: a dashboard with the same uid is already exist: {uid}")
            continue
        known_uids.add(uid)

        folder = dashboard.get("folder", "General")
        file = f"{title}.json"

        if folder not in dashboard_dict:
            dashboard_dict[folder] = {}

        dashboard_dict[folder][file] = definition

    if len(malformed_dashboards) > 0:
        print(f'WARN: Skipping malformed dashboards: {", ".join(malformed_dashboards)}')

    for folder, files in dashboard_dict.items():
        if folder == "General":
            # General folder can't be provisioned, see the link for more details
            # https://github.com/grafana/grafana/blob/3dde8585ff951d5e9a46cfd64d296fdab5acd9a2/docs/sources/http_api/folder.md#a-note-about-the-general-folder
            folder_path = root_path
        else:
            folder_path = os.path.join(root_path, folder)
            os.makedirs(folder_path, exist_ok=True)

        for file, definition in files.items():
            file_path = os.path.join(folder_path, file)
            with open(file_path, "w") as f:
                json.dump(definition, f)

    remove_outdated_files(root_path, ts)

    with open("/tmp/ready", "w") as f:
        f.write("ok")


# cleanup outdated files that were not touched (mtime < currenttime)
def remove_outdated_files(folder, ts):
    for root, _, files in os.walk(folder):
        for file_name in files:
            file_location = os.path.join(root, file_name)
            if os.stat(file_location).st_mtime < ts:
                try:
                    os.unlink(file_location)
                except Exception as e:
                    print('WARN: Failed to delete %s. Reason: %s' % (file_location, e))


if __name__ == "__main__":
    hook.run(main, configpath="dashboard_provisioner.yaml")
