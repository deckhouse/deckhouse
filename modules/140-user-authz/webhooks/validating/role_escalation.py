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


# Anti-escalation guard for namespaced Role and ClusterRole objects (RBAC v2).
#
# A namespace administrator holds the d8:namespace-capability:kubernetes:manage_security capability,
# which grants full CRUD on namespaced `roles`/`rolebindings` but NOT `escalate`/`bind`/`*`. Native
# Kubernetes RBAC already prevents creating a (Cluster)Role with permissions the requester does not
# already hold (no `escalate` verb is granted anywhere in the RBAC v2 templates), so the primary
# escalation path is closed by the API server itself.
#
# This webhook is a precise defense-in-depth layer: it forbids non-privileged users from authoring a
# Role/ClusterRole whose rules grant mutating access to the Deckhouse project-management API
# resources (group deckhouse.io: projects, projecttemplates, projectrolebindings,
# clusterprojectrolebindings, and the future projectnamespaces). Those rights must be conferred only
# by the built-in d8:project:* roles and their capabilities (created by Deckhouse / the aggregation
# controller, which are excluded via matchConditions). Ordinary namespaced RBAC (configmaps,
# secrets, the user's own workloads) is untouched.

from typing import Optional

from deckhouse import hook
from dotmap import DotMap

PROTECTED_GROUP = "deckhouse.io"
# Project-management resources whose mutation confers project-level powers. projectnamespaces does
# not have a CRD yet and is listed here defensively for forward compatibility.
PROTECTED_RESOURCES = {
    "projects",
    "projecttemplates",
    "projectrolebindings",
    "clusterprojectrolebindings",
    "projectnamespaces",
}
MUTATING_VERBS = {"create", "update", "patch", "delete", "deletecollection"}

# Privileged identities that legitimately author the built-in d8:project:* roles/capabilities. They
# are excluded at the API server via matchConditions; the same check is repeated in-hook as
# defense-in-depth for clusters where admission matchConditions are unavailable.
PRIVILEGED_USERS = {
    "system:apiserver",
    "system:serviceaccount:d8-system:deckhouse",
    "system:serviceaccount:kube-system:clusterrole-aggregation-controller",
}

CONFIG = """
configVersion: v1
kubernetesValidating:
- name: rbacv2-role-escalation.deckhouse.io
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
    resources:   ["roles", "clusterroles"]
    scope:       "*"
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


def rule_grants_protected(rule: dict) -> bool:
    groups = set(rule.get("apiGroups") or [])
    # subresources (e.g. projectrolebindings/status) escalate just as well; match on the base name.
    resources = {str(r).split("/", 1)[0] for r in (rule.get("resources") or [])}
    verbs = {str(v).lower() for v in (rule.get("verbs") or [])}

    group_match = "*" in groups or PROTECTED_GROUP in groups
    resource_match = "*" in resources or bool(resources & PROTECTED_RESOURCES)
    verb_match = "*" in verbs or bool(verbs & MUTATING_VERBS)
    return group_match and resource_match and verb_match


def validate(ctx: DotMap) -> Optional[str]:
    username = ctx.review.request.userInfo.username
    if username in PRIVILEGED_USERS:
        return None

    obj = ctx.review.request.object
    if hasattr(obj, "toDict"):
        obj = obj.toDict()

    kind = obj.get("kind") or "Role"
    name = obj.get("metadata", {}).get("name", "")
    rules = obj.get("rules") or []

    for rule in rules:
        if rule_grants_protected(rule):
            return (
                f'{kind} "{name}" must not grant mutating access to Deckhouse project-management '
                f'resources ({", ".join(sorted(PROTECTED_RESOURCES))} in group "{PROTECTED_GROUP}"). '
                "These permissions are conferred only by the built-in d8:project:* roles and their "
                "capabilities; granting them through a custom Role/ClusterRole would escalate "
                "namespace privileges to project-management privileges."
            )

    return None


if __name__ == "__main__":
    hook.run(main, config=CONFIG)
