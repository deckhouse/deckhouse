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
# Marks a ClusterRole as bindable inside a project: the grant registry for cluster roles excludes
# everything without it, so a role may reach a project namespace only when it carries the label.
DELEGATABLE_LABEL = "rbac.deckhouse.io/delegatable"
# Selector label of a capability. Its value is "<scope>-capability.<module>.<name>", and a custom
# capability prefixes that with "custom.".
CAPABILITY_SELECTOR_LABEL = "rbac.deckhouse.io/capability"
# Capability scopes whose rules a project may legitimately receive. A system/subsystem capability
# must never travel into a project through a delegatable custom role.
TENANT_CAPABILITY_SCOPES = {"namespace-capability", "project-capability"}
SCOPE_LABEL = "rbac.deckhouse.io/scope"
SUBSYSTEM_LABEL = "rbac.deckhouse.io/subsystem"
# The segment a custom object's name must carry for a given scope: "d8:custom:<segment>:<name>".
# A subsystem role is the exception -- its segment is the subsystem id, so it is checked separately.
NAME_SEGMENT_BY_SCOPE = {"namespace": "namespace", "project": "project", "system": "system"}
# Scopes of a role that is granted inside a namespace or a project. The containment goes one way:
# such a role must not reach for cluster-level capabilities, whereas a system/subsystem role may
# well include namespace-level ones -- a platform administrator holds those anyway.
TENANT_ROLE_SCOPES = {"namespace", "project"}
# Administrators may override the DISPLAY title/description of a built-in role by
# setting these annotations on it. The "d8:" prefix is otherwise reserved, so we allow an UPDATE to a
# built-in role iff it touches ONLY annotations under this prefix (never rules/aggregation/labels).
CUSTOM_META_PREFIX = "custom.meta.deckhouse.io/"
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


def _as_dict(obj) -> dict:
    if hasattr(obj, "toDict"):
        obj = obj.toDict()
    return obj or {}


def _only_custom_meta_annotation_change(old: dict, new: dict) -> bool:
    """True when old→new differ ONLY in custom.meta.deckhouse.io/* annotations: rules, aggregationRule,
    labels and every non-custom.meta annotation are byte-for-byte unchanged. This lets a platform admin
    set a display title/description on a built-in d8: role without being able to change its
    permissions through the same reserved-prefix bypass."""
    if (old.get("rules") or []) != (new.get("rules") or []):
        return False
    if (old.get("aggregationRule") or {}) != (new.get("aggregationRule") or {}):
        return False
    old_meta = old.get("metadata") or {}
    new_meta = new.get("metadata") or {}
    if (old_meta.get("labels") or {}) != (new_meta.get("labels") or {}):
        return False
    old_ann = old_meta.get("annotations") or {}
    new_ann = new_meta.get("annotations") or {}
    for key in set(old_ann) | set(new_ann):
        if key.startswith(CUSTOM_META_PREFIX):
            continue
        if old_ann.get(key) != new_ann.get(key):
            return False
    return True


def _capability_scope(value: str) -> str:
    """Scope segment of a capability label value; "" when the value has an unexpected shape."""
    if value.startswith("custom."):
        value = value[len("custom.") :]
    return value.split(".", 1)[0]


def _name_segment(name: str) -> str:
    """The <segment> of "d8:custom:<segment>:<name>"; "" when the name has fewer parts."""
    parts = name.split(":")

    return parts[2] if len(parts) > 3 else ""


def _expected_name_segment(kind_label: str, scope: str, subsystem: str) -> Optional[str]:
    """
    The name segment the object must carry, or None when nothing can be required of it.

    The name is what people read in a binding, in an audit log and in the console, while the scope
    label is what the checks judge by, so the two must not tell different stories. Only a declared
    scope is enforced: an object that claims no scope claims nothing, and it is already judged by
    the stricter tenant rules.
    """
    if not scope:
        return None

    if kind_label == "custom-capability":
        return f"{scope}-capability"

    if scope == "subsystem":
        # The segment is the subsystem id itself ("d8:custom:security:auditor"), so it can only be
        # checked against the label that names it.
        return subsystem or None

    return NAME_SEGMENT_BY_SCOPE.get(scope)


def _cluster_level_selector(selectors) -> Optional[str]:
    """
    The first aggregated thing that belongs to the cluster side, described for the error message,
    or None when everything aggregated stays within a namespace or a project.

    Two selector shapes mean the same thing here: the built-in roles aggregate a whole lineage
    ("aggregate-to-<lineage>-as"), while a role assembled from individual capabilities names each
    of them by the capability label.
    """
    for selector in selectors:
        for key, value in (selector.get("matchLabels") or {}).items():
            if key == CAPABILITY_SELECTOR_LABEL:
                if _capability_scope(value) not in TENANT_CAPABILITY_SCOPES:
                    return f'capability "{value}"'
                continue

            m = AGGREGATE_LABEL_RE.match(key)
            if m is not None and m.group(1) in SYSTEM_LINEAGES:
                return f'the "{m.group(1)}" lineage'

    return None


def _aggregates_only_tenant_capabilities(selectors) -> bool:
    """
    Whether every aggregated capability is one a project may receive.

    Fail-closed on purpose: anything the check cannot read as a tenant-scoped capability disqualifies
    the role. A selector by matchExpressions, by some other label, or with a value of an unexpected
    shape could pull in a system capability, and the whole point of the label is that binding such a
    role inside a project is refused.
    """
    if not selectors:
        return False

    for selector in selectors:
        if selector.get("matchExpressions"):
            return False

        match_labels = selector.get("matchLabels") or {}
        if not match_labels:
            return False

        for key, value in match_labels.items():
            if key == CAPABILITY_SELECTOR_LABEL:
                if _capability_scope(value) not in TENANT_CAPABILITY_SCOPES:
                    return False
                continue

            m = AGGREGATE_LABEL_RE.match(key)
            if m is None or m.group(1) not in TENANT_LINEAGES:
                return False

    return True


def validate(ctx: DotMap) -> Optional[str]:
    request = ctx.review.request
    obj = _as_dict(request.object)

    name = obj.get("metadata", {}).get("name", "")
    labels = obj.get("metadata", {}).get("labels") or {}
    kind_label = labels.get(KIND_LABEL, "")
    rules = obj.get("rules") or []
    selectors = (obj.get("aggregationRule") or {}).get("clusterRoleSelectors") or []

    # The d8: name prefix is reserved; users may only create objects under d8:custom:.
    if name.startswith("d8:") and not name.startswith("d8:custom:"):
        # Exception: allow an UPDATE of a built-in role that changes ONLY its
        # custom.meta.deckhouse.io/* annotations (display title/description) — no privilege change.
        if request.operation == "UPDATE" and _only_custom_meta_annotation_change(
            _as_dict(request.oldObject), obj
        ):
            return None
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
            # After creation .rules belongs to the aggregation controller, which fills it from the
            # aggregated capabilities. Every later write therefore carries rules the user did not
            # author -- a plain "kubectl label" already sends them back -- so on UPDATE we object
            # only when the rules actually change. Comparing against the old object keeps the
            # guarantee (a user still cannot smuggle rules in) without making an aggregated role
            # uneditable.
            if rules and (
                request.operation != "UPDATE" or rules != (_as_dict(request.oldObject).get("rules") or [])
            ):
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

        # The name must not contradict the declared scope.
        expected_segment = _expected_name_segment(kind_label, labels.get(SCOPE_LABEL, ""), labels.get(SUBSYSTEM_LABEL, ""))
        if expected_segment is not None and _name_segment(name) != expected_segment:
            return (
                f'ClusterRole "{name}" with "{SCOPE_LABEL}: {labels.get(SCOPE_LABEL)}" must be named '
                f'"d8:custom:{expected_segment}:<name>": the name and the scope must not disagree.'
            )

        # A role granted inside a namespace or a project must stay within that world. The check
        # above does not cover it: it only fires when the two sides are aggregated TOGETHER, so a
        # tenant-scoped role built purely from cluster-level capabilities passed unnoticed. An
        # unlabeled custom role is treated as tenant-scoped, because that is what it is bindable as.
        scope = labels.get(SCOPE_LABEL, "")
        if scope in TENANT_ROLE_SCOPES or (kind_label == "custom-role" and not scope):
            offending = _cluster_level_selector(selectors)
            if offending:
                return (
                    f'ClusterRole "{name}" with "{SCOPE_LABEL}: {scope or "<unset>"}" must not aggregate {offending}: '
                    "a role granted in a namespace or a project may only aggregate namespace and project capabilities."
                )

        # The delegatable label is what lets a role be bound inside a project, so it may only be
        # claimed by a role built entirely from namespace/project capabilities. The check above is
        # not enough: it only reads the "aggregate-to-<lineage>-as" selectors, while a role
        # assembled from individual capabilities selects them by the capability label instead.
        if labels.get(DELEGATABLE_LABEL) == "true" and not _aggregates_only_tenant_capabilities(selectors):
            return (
                f'ClusterRole "{name}" must not be labeled "{DELEGATABLE_LABEL}: true": '
                "a role bindable inside a project may aggregate namespace and project capabilities only."
            )

    return None


if __name__ == "__main__":
    hook.run(main, config=CONFIG)
