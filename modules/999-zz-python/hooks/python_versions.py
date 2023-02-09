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

import sys

from deckhouse import hook
from dotmap import DotMap

# Refer to Shell Operator doc
# https://github.com/flant/shell-operator/blob/main/HOOKS.md
config = """
configVersion: v1
beforeHelm: 10
kubernetes:
- name: python_versions
  apiVersion: "deckhouse.io/v1"
  kind: "Python"
  jqFilter: |
    .spec.version

  # We don't want to keep full custom resources in memory.
  keepFullObjectsInMemory: false

  # We need only snapshots, not to react to particular events.
  executeHookOnEvent: []
  executeHookOnSynchronization: false
"""


def main(ctx: hook.Context):

    # DotMap simplifies access to nested fields, especially to inexisting ones. Since we need values
    # to be JSON serializable, so we convert DotMap back to dict.
    versions = ctx.snapshots.get("python_versions", [])
    v = DotMap(ctx.values)
    v.zzPython.internal.pythonVersions = [parse_snap_version(v) for v in versions]
    ctx.values = v.toDict()


def parse_snap_version(snap):
    # Since we subscribed to deckhouse.io/v1 ApiVersion, we get .spec.version as an object with
    # fields 'major' and 'minor'.
    v = snap["filterResult"]
    major, minor = v["major"], v["minor"]
    return f"{major}.{minor}"


if __name__ == "__main__":
    hook.run(main, config=config)
