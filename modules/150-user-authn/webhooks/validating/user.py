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
  - name: users
    apiVersion: deckhouse.io/v1
    kind: User
    queue: "users"
    group: main
    executeHookOnEvent: []
    executeHookOnSynchronization: false
    keepFullObjectsInMemory: false
    jqFilter: |
      {
        "name": .metadata.name,
        "userID": .spec.userID,
        "email": .spec.email,
        "groups": .spec.groups
      }
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
        "members": .spec.members
      }
kubernetesValidating:
- name: users-unique.deckhouse.io
  group: main
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE", "DELETE"]
    resources:   ["users"]
    scope:       "Cluster"
"""


def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        binding_context.pprint(pformat="json")  # debug printing
        validate(binding_context, ctx.output.validations)
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap, output: hook.ValidationsCollector):
    match ctx.review.request.operation:
        case "CREATE" | "UPDATE":
            validate_creation_or_update(ctx, output)
        case "DELETE":
            validate_delete(ctx, output)
        case _:
            raise Exception(f"Unknown operation {ctx.operation}")


def validate_creation_or_update(ctx: DotMap, output: hook.ValidationsCollector):
    operation = ctx.review.request.operation
    user_name = ctx.review.request.object.metadata.name
    user_id = ctx.review.request.object.spec.userID
    email = ctx.review.request.object.spec.email
    groups = ctx.review.request.object.spec.groups

    user_with_the_same_email = [obj.filterResult for obj in ctx.snapshots.users if obj.filterResult.name != user_name and obj.filterResult.email == email]
    if user_with_the_same_email:
        output.deny(f"users.deckhouse.io \"{user_name}\", user \"{user_with_the_same_email[0]}\" is already using email \"{email}\"")
        return

    if operation == "CREATE" and groups:
        output.deny("\".spec.groups\" is deprecated, use the \"Group\" object.")
        return

    if operation == "UPDATE" and groups:
        snapshot_user = next((user.filterResult for user in ctx.snapshots.users if user.filterResult.name == user_name), None)
        if snapshot_user and set(snapshot_user.groups) - set(groups):
            output.deny("\".spec.groups\" is deprecated, modification is forbidden, only removal of all elements is allowed")
            return

    if email.startswith("system:"):
        output.deny(f"users.deckhouse.io \"{user_name}\", \".spec.email\" must not start with the \"system:\" prefix")
        return

    if user_id:
        output.allow("\".spec.userID\" is deprecated and shouldn't be set manually (if set, its value is ignored)")
        return

    output.allow()


def validate_delete(ctx: DotMap, output: hook.ValidationsCollector):
    user_name = ctx.review.request.object.metadata.name

    for group in ctx.snapshots.groups:
        for member in group.filterResult.members:
            if member.kind == "User" and member.name == user_name:
                output.deny(f"groups.deckhouse.io \"{group.filterResult.name}\" contains users.deckhouse.io \"{user_name}\"")
                return

    output.allow()


if __name__ == "__main__":
    hook.run(main, config=config)
