#!/usr/bin/env python3

# Copyright 2021 Flant JSC
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
import os
from shell_operator import hook
from slugify import slugify

def main(ctx: hook.Context):
    dashboards = []
    known_uids = set()
    malformed_dashboards = []
    dashboard_dict = {}

    for i in ctx.snapshots.get("dashboard_resources", []):
        dashboard = i["filterResult"]
        definition = json.loads(dashboard["definition"])
        title = definition.get("title")
        if not title:
            malformed_dashboards.append(dashboard.get('name', ''))
            continue

        title = slugify(title)

        if not definition.get("uid"):
            print(f"ERROR: definition.uid is mandatory field")
            continue

        uid = definition["uid"]
        if uid in known_uids:
            print(f"ERROR: a dashboard with the same uid is already exist: {uid}")
            continue
        known_uids.add(uid)

        folder = dashboard.get("folder", "General")
        if folder == "General":
            print(f"ERROR: cannot provision dashboards to the 'General' folder")
        else:
            file = f"{folder}/{title}.json"

        if folder not in dashboard_dict:
            dashboard_dict[folder] = {}

        dashboard_dict[folder][file] = definition

    if len(malformed_dashboards) > 0:
        print(f'Skipping malformed dashboards: {", ".join(malformed_dashboards)}')

    for folder, files in dashboard_dict.items():
        folder_path = f"/etc/grafana/dashboards/"
        os.makedirs(folder_path, exist_ok=True)

        for file, definition in files.items():
            file_path = os.path.join(folder_path, file)
            with open(file_path, "w") as f:
                json.dump(definition, f)

            dashboards.append((folder_path, file))

    with open("/tmp/ready", "w") as f:
        f.write("ok")

if __name__ == "__main__":
    hook.run(main, configpath="dashboard_provisioner.yaml")
