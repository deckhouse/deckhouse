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


# ClusterRoleBinding guard for RBAC v2 (card 14 / ADR-2).
#
# A ClusterRoleBinding grants its role in EVERY namespace of the cluster. Two families of built-in
# roles must therefore never be bound through a ClusterRoleBinding:
#
#   1. Scoped roles — d8:namespace:* and d8:project:* (and their d8:custom:namespace:* /
#      d8:custom:project:* variants). They are meant to be granted in a bounded set of namespaces
#      (a namespace via RoleBinding, or a whole project via ProjectRoleBinding/ClusterProjectRoleBinding).
#      Binding them cluster-wide would silently escalate a namespace/project-scoped role to every
#      namespace.
#
#   2. Capabilities — any d8:*-capability:* (namespace/project/system/subsystem, built-in or the
#      d8:custom:*-capability:* variants). Capabilities are aggregation building blocks: they exist to
#      be composed into the d8:<scope>:<level> roles via aggregationRule, not to be bound directly.
#      A ClusterRoleBinding to a capability bypasses the role model, so it is refused regardless of the
#      capability scope. (The previous bash implementation refused only the namespace/project
#      capabilities and let d8:system-capability:* / d8:subsystem-capability:* through — an
#      inconsistent gap this hook closes.)
#
# Plain system-side ROLES (d8:system:* and d8:subsystem:*) are cluster-scoped by design and remain
# bindable via ClusterRoleBinding. Everything outside the d8: namespace is untouched.
#
# The decision is made purely from roleRef on the reviewed object (name-based, snapshot-free): the
# earlier snapshot-driven variant failed OPEN whenever the shell-operator snapshot was empty. roleRef
# is immutable on a (Cluster)RoleBinding, so validating CREATE is sufficient — an accepted binding can
# never later mutate its roleRef into a forbidden one.

from typing import Optional

from deckhouse import hook
from dotmap import DotMap

# Scoped built-in roles (and their custom variants) that must be granted in a bounded scope, never
# cluster-wide. Matched as name prefixes on a ClusterRole roleRef.
#
# `d8:use:role:` is the LEGACY name of the namespace lineage (renamed to d8:namespace:*). It is kept
# alive for one release by the compatibility aliases in templates/rbacv2-compat/ and grants
# namespace-scoped rules, so — exactly like d8:namespace:* — it must not be bound cluster-wide via a
# ClusterRoleBinding. (The legacy system-side alias d8:manage:* maps to the cluster-scoped
# system/subsystem roles and is intentionally NOT listed: a CRB to it is legitimate.) Remove the
# d8:use:role: prefix together with the aliases next release.
SCOPED_ROLE_PREFIXES = (
    "d8:namespace:",
    "d8:project:",
    "d8:use:role:",
    "d8:custom:namespace:",
    "d8:custom:project:",
)

# Any capability, in any scope, built-in or custom, is identified by this marker in the role name
# (e.g. d8:namespace-capability:kubernetes:view_resources, d8:system-capability:prometheus:view,
# d8:custom:project-capability:team:deploy). Capabilities are building blocks and are never bound
# directly via a ClusterRoleBinding.
CAPABILITY_MARKER = "-capability:"

# Identities that legitimately manage bindings; excluded at the API server via matchConditions and
# repeated here as defense-in-depth for clusters where matchConditions are unavailable.
PRIVILEGED_USERS = {
    "system:apiserver",
    "system:serviceaccount:d8-system:deckhouse",
}

CONFIG = """
configVersion: v1
kubernetesValidating:
- name: role-validating.deckhouse.io
  group: main
  matchConditions:
  - expression: ("system:apiserver" != request.userInfo.username)
    name: exclude-kube-apiserver
  - expression: ("system:serviceaccount:d8-system:deckhouse" != request.userInfo.username)
    name: exclude-deckhouse
  rules:
  - apiGroups:   ["rbac.authorization.k8s.io"]
    apiVersions: ["*"]
    operations:  ["CREATE"]
    resources:   ["clusterrolebindings"]
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


def forbidden_reason(role_name: str) -> Optional[str]:
    """Returns why a ClusterRole may not be bound cluster-wide, or None when the binding is allowed."""
    if CAPABILITY_MARKER in role_name:
        return "capability"
    for prefix in SCOPED_ROLE_PREFIXES:
        if role_name.startswith(prefix):
            return "scoped role"
    return None


def validate(ctx: DotMap) -> Optional[str]:
    request = ctx.review.request
    if request.userInfo.username in PRIVILEGED_USERS:
        return None

    obj = request.object
    if hasattr(obj, "toDict"):
        obj = obj.toDict()

    role_ref = obj.get("roleRef") or {}
    if role_ref.get("kind") != "ClusterRole":
        return None

    role_name = role_ref.get("name") or ""
    kind = forbidden_reason(role_name)
    if kind is None:
        return None

    return (
        f"ClusterRole '{role_name}' is a {kind} and cannot be granted through a ClusterRoleBinding "
        "(it would apply in every namespace). Use a RoleBinding to grant it in a namespace, or a "
        "ProjectRoleBinding/ClusterProjectRoleBinding to grant it across a project."
    )


if __name__ == "__main__":
    hook.run(main, config=CONFIG)
