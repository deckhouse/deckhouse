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

from typing import List

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

        print("request", request.pprint(pformat="json"))  # debug printing

        errmsg = validate(request)
        if errmsg is None:
            ctx.output.validations.allow()
        else:
            ctx.output.validations.deny(errmsg)
    except Exception as e:
        print("validating error", str(e))  # debug printing
        ctx.output.validations.error(str(e))


def validate(request: DotMap) -> str | None:
    match request.operation:
        case "CREATE":
            return validate_creation(request.object)
        case "UPDATE":
            return validate_update(request.object)
        case _:
            raise Exception(f"Unknown operation {request.operation}")


def validate_creation(obj):
    # Validate name
    name = obj.metadata.name
    name_segments = name.split("-")
    if not validate_name_schema(name_segments):
        return f"Name must comply with schema python-$major-$minor, got {name}"

    # Validate version
    spec_version = parse_version(obj.spec.version)
    if not validate_version(name_segments, spec_version):
        return f"Name must comply with spec.version, got {name} and {DotMap(spec_version).pprint(pformat='json')}"

    return None


def validate_update(obj):
    # Validates version
    name = obj.metadata.name
    name_segments = name.split("-")
    spec_version = parse_version(obj.spec.version)
    if not validate_version(name_segments, spec_version):
        return f"Name must comply with spec.version, got {name} and {DotMap(spec_version).pprint(pformat='json')}"
    return None


def validate_name_schema(name_segments: List[str]) -> bool:
    # Validate name structure
    return len(name_segments) == 3 or name_segments[0] == "python"


def validate_version(name_segments: List[str], spec_version: dict) -> bool:
    # Validate version structure and numbers
    name_major, name_minor = name_segments[1], name_segments[2]
    spec_major, spec_minor = spec_version["major"], spec_version["minor"]
    return name_major == spec_major and name_minor == spec_minor


def parse_version(version: str | dict | DotMap) -> dict:
    # v1beta1 and v1
    if isinstance(version, dict):
        return version

    # v1alpha1
    major, minor = version.split(".")
    return {
        "major": major,  # str
        "minor": minor,  # str
    }


if __name__ == "__main__":
    hook.run(main, config=config)
