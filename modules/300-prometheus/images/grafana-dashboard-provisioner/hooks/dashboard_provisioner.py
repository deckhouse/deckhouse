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
from shell_operator import hook
from slugify import slugify

def main(ctx: hook.Context):
    dashboards = []
    known_uids = set()
    malformed_dashboards = []

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
            file = f"{title}.json"
        else:
            file = f"{folder}/{title}.json"

        dashboards.append((file, json.dumps(definition)))

    if len(malformed_dashboards) > 0:
        print(f'Skipping malformed dashboards: {", ".join(malformed_dashboards)}')

    for file, contents in dashboards:
        path = f"/etc/grafana/dashboards/{file}"
        with open(path, "w") as f:
            f.write(contents)
        if not file.startswith("General/"):
            folder = file.split("/")[:-1]
            folder_path = f"/etc/grafana/dashboards/{'/'.join(folder)}"
            if not any(file.startswith(folder_path) for file, _ in dashboards):
                dashboards.append((folder_path, ""))

    with open("/tmp/ready", "w") as f:
        f.write("ok")

if __name__ == "__main__":
    hook.run(main, configpath="dashboard_provisioner.yaml")
