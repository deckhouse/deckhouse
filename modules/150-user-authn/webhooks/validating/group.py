#!/usr/bin/python3

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

from collections import deque
from typing import Optional

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
    apiVersion: deckhouse.io/v1
    kind: User
    queue: "users"
    group: main
    executeHookOnEvent: []
    executeHookOnSynchronization: false
    keepFullObjectsInMemory: false
    jqFilter: |
      {
        "userName": .metadata.name
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
        errmsg, warnings = validate(binding_context)
        if errmsg is None:
            ctx.output.validations.allow(*warnings)
        else:
            ctx.output.validations.deny(errmsg)
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap) -> tuple[Optional[str], list[str]]:
    operation = ctx.review.request.operation
    if operation == "CREATE" or operation == "UPDATE":
        return validate_creation_or_update(ctx)
    elif operation == "DELETE":
        return validate_delete(ctx)
    else:
        raise Exception(f"Unknown operation {ctx.operation}")


def validate_creation_or_update(ctx: DotMap) -> tuple[Optional[str], list[str]]:
    request = ctx.review.request
    obj_name = request.object.metadata.name
    group_name = request.object.spec.name
    warnings = []

    # Checked before the cycles: the hierarchy is keyed by spec.name and would merge a duplicate with the stored group.
    if [obj.filterResult for obj in ctx.snapshots.groups if
        obj.filterResult.name != obj_name and obj.filterResult.groupName == group_name]:
        return f"groups.deckhouse.io \"{group_name}\" already exists", warnings

    # The snapshot may already hold the written group; the review carries its versions before and after the write.
    stored_groups = [(g.filterResult.groupName, g.filterResult.members) for g in ctx.snapshots.groups
                     if g.filterResult.name != obj_name]
    groups_before = list(stored_groups)
    if request.operation == "UPDATE":
        groups_before.append((request.oldObject.spec.name, request.oldObject.spec.members))
    hierarchy_before = build_hierarchy(groups_before)
    hierarchy = build_hierarchy(stored_groups + [(group_name, request.object.spec.members)])

    created_cycle = find_created_cycle(hierarchy_before, hierarchy, group_name)
    if created_cycle:
        return (
            f"Invalid group hierarchy: cycle detected! Path: groups.deckhouse.io({format_path(created_cycle)}). "
            "Groups must form a tree without circular references."
        ), warnings

    existing_cycle = find_cycle(hierarchy, group_name)
    if existing_cycle:
        warnings.append(
            f"Group hierarchy already has a cycle. Path: groups.deckhouse.io({format_path(existing_cycle)}). "
            "Groups must form a tree without circular references. To break the cycle, remove one of these groups "
            "from the members of the previous one."
        )

    if group_name.startswith("system:"):
        return f"groups.deckhouse.io \"{group_name}\" must not start with the \"system:\" prefix", warnings

    for member in ctx.review.request.object.spec.members:
        if member.kind == "Group":
            if not is_exist(ctx.snapshots.groups, {"groupName": member.name}):
                warnings.append(f"groups.deckhouse.io \"{member.name}\" not exist")
        elif member.kind == "User":
            if not is_exist(ctx.snapshots.users, {"userName": member.name}):
                warnings.append(f"users.deckhouse.io \"{member.name}\" not exist")
        else:
            raise Exception(f"Unknown member kind {member.kind}")

    return None, warnings


def is_exist(arr: list[DotMap], target: dict) -> bool:
    for obj in arr:
        for k, v in target.items():
            if obj.filterResult[k] != v:
                break  # go to next item in list
        else:
            return True

    return False


def validate_delete(ctx: DotMap) -> tuple[Optional[str], list[str]]:
    group_name = ctx.review.request.oldObject.spec.name
    warnings = []

    for group in ctx.snapshots.groups:
        for member in group.filterResult.members:
            if member.kind == "Group" and member.name == group_name:
                warnings.append(
                    f"groups.deckhouse.io \"{group.filterResult.groupName}\" contains groups.deckhouse.io \"{group_name}\"")

    return None, warnings


def build_hierarchy(groups: list[tuple[str, list[DotMap]]]) -> dict[str, list[str]]:
    """
    Map each group to the groups among its members.

    Groups are keyed by spec.name: nested members, tokens and RBAC subjects use it, not metadata.name. A member that
    names a group that does not exist is left out, since it cannot be part of a cycle.

    Args:
        groups: pairs of spec.name and spec.members.

    Returns:
        dict[str, list[str]]: nested group names by group name, in the order of the members.
    """
    hierarchy = {name: [] for name, _ in groups}
    for name, members in groups:
        for member in members or []:
            if member.kind == "Group" and member.name in hierarchy and member.name not in hierarchy[name]:
                hierarchy[name].append(member.name)
    return hierarchy


def find_created_cycle(before: dict[str, list[str]], after: dict[str, list[str]], group_name: str) -> list[str]:
    """
    Find a cycle that a write creates.

    A write adds only memberships of the written group and, when its name is new, memberships that point at that
    name. So every cycle the write creates passes through the written group and leaves it through one of its own
    memberships: any of them when the name is new, otherwise one the group gains. The search looks for the shortest
    way back to the written group from the nested groups of those memberships.

    Args:
        before: the hierarchy with the written group as it was, or without it on CREATE.
        after: the hierarchy with the written group as it is written.
        group_name: spec.name of the written group.

    Returns:
        list[str]: group names along the cycle, starting and ending with the written group, or an empty list.
    """
    children_before = before.get(group_name)
    gained = [child for child in after[group_name] if children_before is None or child not in children_before]
    path_back = find_path(after, gained, group_name)
    return [group_name, *path_back] if path_back else []


def find_path(hierarchy: dict[str, list[str]], sources: list[str], target: str) -> list[str]:
    """
    Find the shortest chain of nested groups that leads from any of the sources to target.

    Returns:
        list[str]: group names from a source to target, both included, or an empty list.
    """
    previous = dict.fromkeys(sources)
    queue = deque(previous)
    while queue:
        name = queue.popleft()
        if name == target:
            path = []
            while name is not None:
                path.append(name)
                name = previous[name]
            return path[::-1]
        for child in hierarchy[name]:
            if child not in previous:
                previous[child] = name
                queue.append(child)
    return []


def find_cycle(hierarchy: dict[str, list[str]], group_name: str) -> list[str]:
    """
    Find any cycle of the hierarchy.

    The depth-first search starts at the written group, so a cycle among its nested groups is found first.

    Args:
        hierarchy: nested group names by group name.
        group_name: spec.name of the written group.

    Returns:
        list[str]: group names along the cycle, the first one repeated at the end, or an empty list.
    """
    finished = set()
    for root in [group_name, *(name for name in hierarchy if name != group_name)]:
        if root in finished:
            continue
        path = [root]
        on_path = {root}
        children = [iter(hierarchy[root])]
        while path:
            child = next(children[-1], None)
            if child is None:
                children.pop()
                on_path.remove(path[-1])
                finished.add(path.pop())
            elif child in on_path:
                return [*path[path.index(child):], child]
            elif child not in finished:
                path.append(child)
                on_path.add(child)
                children.append(iter(hierarchy[child]))
    return []


def format_path(names: list[str]) -> str:
    return " -> ".join(f'"{name}"' for name in names)


if __name__ == "__main__":
    hook.run(main, config=config)
