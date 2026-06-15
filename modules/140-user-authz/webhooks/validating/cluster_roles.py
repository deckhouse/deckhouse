#!/usr/bin/python3

# Copyright 2026 Flant JSC
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


# Validates RBAC v2 framework objects (ClusterRoles):
#   - the "d8:" name prefix is reserved for Deckhouse, users may only create "d8:custom:*" objects;
#   - the built-in framework kinds (role/capability) cannot be created by users,
#     they must use custom-role/custom-capability;
#   - custom-role must not define its own rules (aggregation only) and must define aggregationRule;
#   - custom roles/capabilities must not aggregate the system lineages together with
#     the namespace/project lineages (privilege escalation across scopes).

import re
from typing import Optional

from deckhouse import hook
from dotmap import DotMap

KIND_LABEL = "rbac.deckhouse.io/kind"
AGGREGATE_LABEL_RE = re.compile(r"^rbac\.deckhouse\.io/aggregate-to-(.+)-as$")
TENANT_LINEAGES = {"namespace", "project"}
# Built-in system-side lineages: the system lineage plus one lineage per subsystem.
# Unknown (custom) lineages are neutral: they only pull user-created custom
# capabilities, which the user could bind directly anyway.
SYSTEM_LINEAGES = {
    "system",
    "deckhouse",
    "infrastructure",
    "kubernetes",
    "networking",
    "observability",
    "security",
    "storage",
}

CONFIG = """
configVersion: v1
kubernetesValidating:
- name: rbacv2-cluster-roles.deckhouse.io
  group: main
  matchConditions:
  - expression: ("system:apiserver" != request.userInfo.username)
    name: exclude-kube-apiserver
  - expression: ("system:serviceaccount:d8-system:deckhouse" != request.userInfo.username)
    name: exclude-deckhouse
  - expression: ("system:serviceaccount:kube-system:clusterrole-aggregation-controller" != request.userInfo.username)
    name: exclude-aggregation-controller
  rules:
  - apiGroups:   ["rbac.authorization.k8s.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["clusterroles"]
    scope:       "Cluster"
"""


def main(ctx: hook.Context):
    try:
        binding_context = DotMap(ctx.binding_context)
        error_message = validate(binding_context)
        if error_message:
            ctx.output.validations.deny(error_message)
        else:
            ctx.output.validations.allow()
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap) -> Optional[str]:
    obj = ctx.review.request.object
    if hasattr(obj, "toDict"):
        obj = obj.toDict()

    name = obj.get("metadata", {}).get("name", "")
    labels = obj.get("metadata", {}).get("labels") or {}
    kind_label = labels.get(KIND_LABEL, "")
    rules = obj.get("rules") or []
    selectors = (obj.get("aggregationRule") or {}).get("clusterRoleSelectors") or []

    # The d8: name prefix is reserved; users may only create objects under d8:custom:.
    if name.startswith("d8:") and not name.startswith("d8:custom:"):
        return (
            'ClusterRole names with the "d8:" prefix are reserved for Deckhouse. '
            'Use the "d8:custom:" prefix for custom roles and capabilities.'
        )

    # Built-in framework kinds cannot be claimed by users.
    if kind_label in ("role", "capability"):
        return (
            f'The label "{KIND_LABEL}: {kind_label}" is reserved for Deckhouse built-in objects. '
            'Use "custom-role" or "custom-capability" instead.'
        )

    if kind_label in ("custom-role", "custom-capability"):
        if not name.startswith("d8:custom:"):
            return (
                f'ClusterRole "{name}" labeled "{KIND_LABEL}: {kind_label}" '
                'must be named with the "d8:custom:" prefix.'
            )

        # Custom roles aggregate capabilities and must not carry their own rules.
        if kind_label == "custom-role":
            if rules:
                return (
                    f'ClusterRole "{name}" with "{KIND_LABEL}: custom-role" must not define rules. '
                    "Move the rules to a custom-capability and aggregate it."
                )
            if not selectors:
                return (
                    f'ClusterRole "{name}" with "{KIND_LABEL}: custom-role" '
                    "must define aggregationRule.clusterRoleSelectors."
                )

        # Forbid aggregating the system-side lineages together with the namespace/project lineages.
        system_side, tenant_side = None, None
        for selector in selectors:
            for key in (selector.get("matchLabels") or {}):
                m = AGGREGATE_LABEL_RE.match(key)
                if m is None:
                    continue
                lineage = m.group(1)
                if lineage in TENANT_LINEAGES:
                    tenant_side = lineage
                elif lineage in SYSTEM_LINEAGES:
                    system_side = lineage
        if system_side and tenant_side:
            return (
                f'ClusterRole "{name}" must not aggregate the "{system_side}" lineage together with '
                f'the "{tenant_side}" lineage: mixing system and namespace/project scopes is forbidden.'
            )

    return None


if __name__ == "__main__":
    hook.run(main, config=CONFIG)
