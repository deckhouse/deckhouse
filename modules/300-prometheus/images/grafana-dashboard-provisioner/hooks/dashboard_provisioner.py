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

import os
import tempfile
import json
import shutil
import re
from shell_operator import hook

def slugify(value):
    value = value.lower()
    value = re.sub(r"[^\w\s-]", "", value).strip()
    value = re.sub(r"[-\s]+", "-", value)
    return value

def main(ctx: hook.Context):
    tmp_dir = tempfile.mkdtemp(prefix="dashboard.")
    known_uids = set()
    malformed_dashboards = ""

    for i in ctx.snapshots.get("dashboard_resources", []):
        dashboard = i["filterResult"]
        definition_str = dashboard["definition"]
        definition = json.loads(definition_str)
        title = definition.get("title")
        if not title:
            malformed_dashboards += f" {dashboard.get('name', '')}"
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
        file = f"{folder}/{title}.json"

        os.makedirs(f"{tmp_dir}/{folder}", exist_ok=True)
        with open(f"{tmp_dir}/{file}", "w") as f:
            json.dump(definition, f)

    if malformed_dashboards:
        print(f"Skipping malformed dashboards: {malformed_dashboards}")

    shutil.rmtree("/etc/grafana/dashboards/", ignore_errors=True)
    shutil.move(tmp_dir, "/etc/grafana/dashboards/")
    shutil.rmtree(tmp_dir)

    with open("/tmp/ready", "w") as f:
        f.write("ok")

if __name__ == "__main__":
    hook.run(main, configpath="dashboard_provisioner.yaml")
