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

config = """
configVersion: v1
beforeHelm: 1
kubernetes:
- name: "python_versions"
  apiVersion: "deckhouse.io/v1"
  kind: "Python"
  jqFilter: ".spec.version"
"""


def main(ctx: hook.Context):
    # Since we subscribed to deckhouse.io/v1 ApiVersion, we get .spec.version as an object with
    # fields 'major' and 'minor'.
    versions = ctx.snapshots["python_versions"]

    # DotMap simplifies access to nested fields, especially to inexisting ones.
    v = DotMap(ctx.values)
    v.zzPython.internal.pythonVersions = [f"{v.major}.{v.minor}" for v in versions]

    # We need values to be JSON serializable, so we convert DotMap back to dict.
    ctx.values = v.toDict()


if __name__ == "__main__":
    hook.run(main, config=config)
