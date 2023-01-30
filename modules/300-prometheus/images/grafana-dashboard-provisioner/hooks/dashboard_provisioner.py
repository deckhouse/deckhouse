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

import os
import tempfile
import json
import shutil

def __config__():
    config = {
        "configVersion": "v1",
        "kubernetes": [
            {
                "name": "dashboard_resources",
                "apiVersion": "deckhouse.io/v1",
                "kind": "GrafanaDashboardDefinition",
                "includeSnapshotsFrom": [
                    "dashboard_resources"
                ],
                "jqFilter": '{"name": .metadata.name, "folder": .spec.folder, "definition": .spec.definition}'
            }
        ]
    }
    return json.dumps(config)

def __main__():
    tmp_dir = tempfile.mkdtemp(prefix="dashboard.")
    existing_uids_file = tempfile.mktemp(prefix="uids.")

    malformed_dashboards = ""
    for i in context.get("snapshots.dashboard_resources"):
        dashboard = context.get(f"snapshots.dashboard_resources.{i}.filterResult")
        title = json.loads(dashboard)["definition"]["title"]
        if not title:
            malformed_dashboards += f" {json.loads(dashboard)['name']}"
            continue

        title = slugify(title)

        if not "definition" in json.loads(dashboard) or not "uid" in json.loads(dashboard)["definition"]:
            print(f"ERROR: definition.uid is mandatory field")
            continue

        dashboard_uid = json.loads(dashboard)["definition"]["uid"]
        if dashboard_uid in open(existing_uids_file).read():
            print(f"ERROR: a dashboard with the same uid is already exist: {dashboard_uid}")
            continue
        else:
            with open(existing_uids_file, "a") as f:
                f.write(dashboard_uid+'\n')

        folder = json.loads(dashboard)["folder"]
        file = f"{folder}/{title}.json"

        if folder == "General":
            file = f"{title}.json"

        os.makedirs(f"{tmp_dir}/{folder}", exist_ok=True)
        with open(f"{tmp_dir}/{file}", "w") as f:
            json.dump(json.loads(dashboard)["definition"], f)

    if malformed_dashboards:
        print(f"Skipping malformed dashboards: {malformed_dashboards}")

    shutil.rmtree("/etc/grafana/dashboards/", ignore_errors=True)
    shutil.copytree(tmp_dir, "/etc/grafana/dashboards/")
    shutil.rmtree(tmp_dir)
    os.remove(existing_uids_file)

    with open("/tmp/ready", "w") as f:
        f.write("ok")

hook.run()
