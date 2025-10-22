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

from typing import Optional, Self

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
    obj_name = ctx.review.request.object.metadata.name
    group_name = ctx.review.request.object.spec.name
    warnings = []

    group_tree = GroupTree.from_binding_context(
        target_group=ctx.review.request.object.spec,
        all_groups=ctx.snapshots.groups
    )
    found_loop, loop_path = group_tree.detect_cycle()
    if found_loop:
        return (
            f"Invalid group hierarchy: cycle detected! Path: groups.deckhouse.io({loop_path}). Groups must form a "
            "tree without circular references."
        ), warnings

    if [obj.filterResult for obj in ctx.snapshots.groups if
        obj.filterResult.name != obj_name and obj.filterResult.groupName == group_name]:
        return f"groups.deckhouse.io \"{group_name}\" already exists", warnings

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
                    f"groups.deckhouse.io \"{group.filterResult.name}\" contains groups.deckhouse.io \"{group_name}\"")

    return None, warnings


class Group:
    """
    Represents a single group node in a hierarchy.

    Attributes:
        name (str): The name of the group.
        subgroups (list[Group]): List of child groups.
    """
    def __init__(self, name):
        """
        Initialize a Group instance.

        Args:
            name (str): The name of the group.
        """
        self.name = name
        self.subgroups = []

    def add_subgroup(self, group: Self):
        """
        Add a subgroup to this group.

        Args:
            group (Group): The child group to add.
        """
        self.subgroups.append(group)


class GroupTree(list[Group]):
    """
    Represents a forest of groups (one or more root nodes).

    Inherits from list[Group] to behave like a list of Group objects,
    but also provides convenient methods for building and analyzing the hierarchy.
    """
    def __init__(self, groups: list[Group]):
        """
        Initialize a GroupTree from a list of Group objects.

        Args:
            groups (list[Group]): Initial groups to include in the forest.
        """
        super().__init__()
        self.extend(groups)

    @classmethod
    def from_binding_context(cls, target_group: DotMap, all_groups: DotMap):
        """
        Build a GroupTree from a binding context (target group + all known groups).

        This method:
            - Creates Group objects for each group in all_groups and the target_group.
            - Links subgroups according to the 'members' field (only if kind == 'Group').
            - Determines root nodes (groups that are not children of any other group).
            - If no roots are found, returns a tree containing only the target group
              (indicating a potential cycle in the graph).

        Args:
            target_group (dotmap.DotMap): The group that is being targeted or reviewed.
            all_groups (dotmap.DotMap): List of all available groups in the context.

        Returns:
            GroupTree: A GroupTree instance containing root groups.
        """
        name_to_group = {g.filterResult.name: Group(g.filterResult.name) for g in all_groups}
        name_to_group[target_group.name] = Group(target_group.name)

        # searching/adding all exists group's subgroups
        for obj in all_groups:
            group = name_to_group[obj.filterResult.name]
            for member in obj.filterResult.members:
                if member.kind == "Group" and member.name in name_to_group:
                    group.add_subgroup(name_to_group[member.name])

        # searching/adding target group's subgroups
        root_group = name_to_group[target_group.name]
        for member in target_group.members:
            if member.kind == "Group" and member.name in name_to_group:
                root_group.add_subgroup(name_to_group[member.name])

        # looking for root nodes
        all_children = {child.name for g in name_to_group.values() for child in g.subgroups}
        # find root nodes: groups that are never listed as a child of another group
        roots = [g for g in name_to_group.values() if g.name not in all_children]

        # DFS helper to collect all reachable groups starting from given roots
        reachable = set()
        def dfs(group):
            if group.name in reachable:
                return
            reachable.add(group.name)
            for sub in group.subgroups:
                dfs(sub)

        # Explore from all discovered roots
        for r in roots:
            dfs(r)

        # Any groups that are not reachable from roots are assumed to be cyclic
        cyclic_groups = [g for g in name_to_group.values() if g.name not in reachable]

        if cyclic_groups:
            # Add cyclic groups as separate "roots" so they can still be validated later
            roots.extend(cyclic_groups)

        # # Fallback: if no roots exist at all, return a tree with the target group only
        if not roots:
            return cls([root_group])

        return cls(roots)

    def detect_cycle(self) -> tuple[bool, str]:
        """
        Detect cycles in a forest (list of Group roots).

        Returns:
            tuple[bool, str]:
                - bool: True if a cycle is detected, otherwise False
                - str:  Path of the cycle as a string, e.g. `"A" -> "B" -> "C" -> "A"`

        Algorithm:
            - Performs a DFS traversal on each root in the forest.
            - Uses `visited` to track fully explored nodes.
            - Uses `stack` to track the current recursion path (active nodes).
            - If a node appears in `stack` again, a cycle is detected.
            - Builds a human-readable cycle path when a cycle is found.
        """
        visited = set()
        stack = set()

        def dfs(node: Group, path: list) -> tuple[bool, str]:
            """
            Depth-First Search helper for cycle detection.

            Args:
                node (Group): current node being traversed
                path (list): the current traversal path

            Returns:
                tuple[bool, str]:
                    - True and cycle path if a cycle is detected
                    - False and empty string otherwise
            """
            if node in stack:
                cycle_path = " -> ".join(f'"{g.name}"' for g in path + [node])
                return True, cycle_path

            if node in visited:
                return False, ""

            stack.add(node)
            path.append(node)

            for child in node.subgroups:
                loop_found, loop_path = dfs(child, path)
                if loop_found:
                    return True, loop_path

            path.pop()
            stack.remove(node)
            visited.add(node)
            return False, ""

        for root in self:
            found, cycle = dfs(root, [])
            if found:
                return True, cycle

        return False, ""


if __name__ == "__main__":
    hook.run(main, config=config)
