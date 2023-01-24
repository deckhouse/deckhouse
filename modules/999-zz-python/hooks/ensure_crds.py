#!/usr/bin/env python3
#
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

import os

import yaml
from deckhouse_sdk import hook

# we expect structure
# modules/
#   999-your-module-name/
#       crds/
#           crd1.yaml
#           crd2.yaml
#           subdir/
#               crd3.yaml
#       hooks/
#           ensure_crds.py # this file


config = """
configVersion: v1
onStartup: 5
"""


def main(ctx: hook.Context):
    for crd in walk_crds(find_crds_root(__file__)):
        # TODO take conversions into account
        ctx.kubernetes.create_or_update(crd)


def walk_crds(crds_root):
    if not os.path.exists(crds_root):
        return

    for dirpath, _, filenames in os.walk(top=crds_root):
        for filename in filenames:
            if not filename.endswith(".yaml"):
                # Wee only seek manifests
                continue
            if filename.startswith("doc-"):
                # Skip dedicated doc yamls, common for Deckhouse internal modules
                continue
            crd_path = os.path.join(dirpath, filename)
            for manifest in yaml.safe_load_all(open(crd_path, "r", encoding="utf-8")):
                if manifest is None:
                    continue
                yield manifest


def find_crds_root(hookpath):
    hooks_dir = os.path.dirname(hookpath)
    module_dir = os.path.dirname(hooks_dir)
    crds_root = os.path.join(module_dir, "crds")
    return crds_root


if __name__ == "__main__":
    hook.run(main, config=config)
