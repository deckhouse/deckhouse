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
from deckhouse import hook

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
    for crd in iter_yamls(find_crds_root(__file__)):
        ctx.kubernetes.create_or_update(crd)


def iter_yamls(root_path: str):
    if not os.path.exists(root_path):
        return

    for dirpath, dirnames, filenames in os.walk(top=root_path):
        for filename in filenames:
            if not filename.endswith(".yaml"):
                # Wee only seek manifests
                continue
            if filename.startswith("doc-"):
                # Skip dedicated doc yamls, common for Deckhouse internal modules
                continue

            crd_path = os.path.join(dirpath, filename)
            with open(crd_path, "r", encoding="utf-8") as f:
                for manifest in yaml.safe_load_all(f):
                    if manifest is None:
                        continue
                    yield manifest

        for dirname in dirnames:
            subroot = os.path.join(dirpath, dirname)
            for manifest in iter_yamls(subroot):
                yield manifest


def find_crds_root(hookpath):
    hooks_root = os.path.dirname(hookpath)
    module_root = os.path.dirname(hooks_root)
    crds_root = os.path.join(module_root, "crds")
    return crds_root


if __name__ == "__main__":
    hook.run(main, config=config)
