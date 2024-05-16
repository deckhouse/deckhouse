#!/usr/bin/env python3

# Copyright 2024 Flant JSC
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

from deckhouse import hook
from dotmap import DotMap

config = """
configVersion: v1
kubernetes:
  - name: groups
    apiVersion: deckhouse.io/v1alpha1
    kind: Group
    queue: "groups"
    group: main
    executeHookOnEvent: []
    executeHookOnSynchronization: false
    keepFullObjectsInMemory: false
    jqFilter: |
      {
        "name": .metadata.name,
        "groupName": .spec.name,
        "members": .spec.members
      }
  - name: users
    apiVersion: deckhouse.io/v1alpha1
    kind: User
    queue: "users"
    group: main
    executeHookOnEvent: []
    executeHookOnSynchronization: false
    keepFullObjectsInMemory: false
    jqFilter: |
      {
        "userName": .spec.name
      }
kubernetesValidating:
- name: groups-unique.deckhouse.io
  group: main
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE", "DELETE"]
    resources:   ["groups"]
    scope:       "Cluster"
"""


def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        binding_context.pprint(pformat="json")  # debug printing

        errmsg = validate(binding_context)
        if errmsg is None:
            ctx.output.validations.allow()
        else:
            ctx.output.validations.deny(errmsg)
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap) -> str | None:
    match ctx.review.request.operation:
        case "CREATE" | "UPDATE":
            return validate_creation_or_update(ctx)
        case "DELETE":
            return validate_delete(ctx)
        case _:
            raise Exception(f"Unknown operation {ctx.operation}")


def validate_creation_or_update(ctx: DotMap) -> str | None:
    obj_name = ctx.review.request.object.metadata.name
    group_name = ctx.review.request.object.spec.name

    if [obj.filterResult for obj in ctx.snapshots.groups if
        obj.filterResult.name != obj_name and obj.filterResult.groupName == group_name]:
        return f"groups.deckhouse.io \"{group_name}\" already exists"

    if group_name.startswith("system:"):
        return f"groups.deckhouse.io \"{group_name}\" must not start with the \"system:\" prefix"

    for member in ctx.review.request.object.spec.members:
        match member.kind:
            case "Group":
                if not is_exist(ctx.snapshots.groups, {"groupName": member.name}):
                    return f"groups.deckhouse.io \"{member.name}\" not exist"
            case "User":
                if not is_exist(ctx.snapshots.users, {"userName": member.name}):
                    return f"users.deckhouse.io \"{member.name}\" not exist"
            case _:
                raise Exception(f"Unknown member kind {member.kind}")

    return None


def is_exist(arr: list[DotMap], target: dict) -> bool:
    for obj in arr:
        for k, v in target:
            if obj.filterResult[k] != v:
                break  # go to next item in list
        else:
            return True

    return False


def validate_delete(ctx: DotMap) -> str | None:
    group_name = ctx.review.request.object.spec.name

    for group in ctx.snapshots.groups:
        for member in group.filterResult.members:
            if member.kind == "Group" and member.name == group_name:
                return f"groups.deckhouse.io \"{group.filterResult.name}\" contains groups.deckhouse.io \"{group_name}\""

    return None


if __name__ == "__main__":
    hook.run(main, config=config)
