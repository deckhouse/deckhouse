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

from deckhouse_sdk import hook
from dotmap import DotMap

config = """
configVersion: v1
kubernetesValidating:
- name: python-crd-name.deckhouse.io
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["v1alpha1", "v1beta1", "v1"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["pythons"]
    scope:       "Cluster"
"""


def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        request = DotMap(ctx.binding_context).review.request
        obj = request.object

        # Validate name structure
        name = obj.metadata.name
        name_segments = name.split("-")
        if len(name_segments) != 3 or name_segments[0] != "python":
            ctx.output.validations.forbid(
                f"Name must comply with schema python-$major-$minor, got {name}"
            )
            return

        # Validate the same version in spec and name
        name_major, name_minor = name_segments[1], name_segments[2]
        spec_version = parse_version(obj.spec.version)
        spec_major, spec_minor = spec_version["major"], spec_version["minor"]
        if name_major != spec_major or name_minor != spec_minor:
            ctx.output.validations.forbid(
                f"Name must comply with spec.version, got {name} and {spec_version}"
            )
            return

        ctx.output.validations.allow()
    except Exception as e:
        print("validating error", str(e))  # debug printing
        ctx.output.validations.error(str(e))
        return


def parse_version(version: str | dict) -> dict:
    if isinstance(version, dict):
        # v1beta1 and v1
        return version

    # v1alpha1
    major, minor = version.split(".")
    return {
        "major": major,  # str
        "minor": minor,  # str
    }


if __name__ == "__main__":
    hook.run(main, config=config)
