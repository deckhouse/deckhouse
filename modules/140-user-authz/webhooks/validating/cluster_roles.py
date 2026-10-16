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

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional
from dotmap import DotMap


# Validates RBAC v2 framework objects (ClusterRoles):
#   - the "d8:" name prefix is reserved for Deckhouse, users may only create "d8:custom:*" objects;
#   - the built-in framework kinds (role/capability) cannot be created by users,
#     they must use custom-role/custom-capability;
#   - the rbac.deckhouse.io/capability values of the built-in capabilities cannot be claimed by users,
#     their values must start with "custom.";
#   - custom-role must not define its own rules (aggregation only) and must define aggregationRule;
#   - custom roles/capabilities must not aggregate the system lineages together with
#     the namespace/project lineages (privilege escalation across scopes).

import re


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
# Built-in system-side lineages: the system lineage plus one lineage per subsystem the platform ships.
# A module the DKP subsystem table puts in none of them may ship a subsystem of its own with its
# d8:subsystem:<name>:<level> roles; the webhook learns those from the subsystem roles in the cluster
# (SUBSYSTEM_ROLES_SNAPSHOT). Any other lineage is neutral: it only pulls user-created custom
# capabilities, which the user could bind directly anyway.
SYSTEM_LINEAGES = {
    "system",
    "iam",
    "security",
    "cluster",
    "delivery",
    "network",
    "storage",
    "observability",
    "managed-services",
}
# The lineages of the legacy scheme: the subsystems that the role model merged into cluster or renamed,
# and all, the lineage of the d8:manage:all:* and d8:use:role:* roles of that scheme. No role of the
# model aggregates them, but a module that has not migrated yet still labels its system capabilities
# with the old subsystems and its subsystem and use roles with all, and the compatibility aliases of
# the old roles collect them, so they are not neutral. They stay on the system side for as long as the
# platform keeps the legacy names alive, and go together with templates/rbacv2-compat/.
LEGACY_SYSTEM_LINEAGES = {
    "deckhouse",
    "infrastructure",
    "kubernetes",
    "networking",
    "all",
}

# The context binding of templates/rbacv2-cluster-roles-validation-webhook.yaml that lists the subsystem
# roles in the cluster, {"subsystem": <the rbac.deckhouse.io/subsystem label>} per role.
SUBSYSTEM_ROLES_SNAPSHOT = "subsystem_roles"


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


def _old_label(request, key: str) -> Optional[str]:
    """The value of a label on the object before an UPDATE; None on any other operation or without it."""
    if request.operation != "UPDATE":
        return None
    old_meta = _as_dict(request.oldObject).get("metadata") or {}
    return (old_meta.get("labels") or {}).get(key)


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


def _system_lineages(ctx: DotMap) -> set:
    """
    The system-side lineages: the system lineage, the subsystems the platform ships, the subsystems of
    the legacy scheme, and every subsystem a module ships its d8:subsystem:<name>:<level> roles for.
    Only Deckhouse creates roles of kind role -- users may not claim the kind -- so the snapshot names
    the subsystems of the platform and its modules, and nothing a user made up.
    """
    lineages = SYSTEM_LINEAGES | LEGACY_SYSTEM_LINEAGES
    for role in ctx.snapshots.get(SUBSYSTEM_ROLES_SNAPSHOT, []) or []:
        subsystem = role.filterResult.subsystem
        if isinstance(subsystem, str) and subsystem:
            lineages.add(subsystem)
    return lineages


def _cluster_level_selector(selectors, system_lineages) -> Optional[str]:
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
            if m is not None and m.group(1) in system_lineages:
                return f'the "{m.group(1)}" lineage'

    return None


def _platform_lineages_on(labels, system_lineages) -> set:
    """Lineages of the platform whose aggregation label the object carries on itself."""
    found = set()
    for key in labels:
        m = AGGREGATE_LABEL_RE.match(key)
        if m is None:
            continue
        lineage = m.group(1)
        if lineage in TENANT_LINEAGES or lineage in system_lineages:
            found.add(lineage)
    return found


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


def validation_error(ctx: DotMap) -> Optional[str]:
    request = ctx.review.request
    obj = _as_dict(request.object)

    name = obj.get("metadata", {}).get("name", "")
    labels = obj.get("metadata", {}).get("labels") or {}
    kind_label = labels.get(KIND_LABEL, "")
    rules = obj.get("rules") or []
    selectors = (obj.get("aggregationRule") or {}).get("clusterRoleSelectors") or []
    system_lineages = _system_lineages(ctx)

    # The d8: name prefix is reserved; users may only create objects under d8:custom:.
    # Commander materializes its global roles as d8:commander:* on managed clusters
    # (e2e SSA-patches d8:commander:commander-admin); that prefix is Commander's, not a tenant's.
    if name.startswith("d8:") and not name.startswith("d8:custom:") and not name.startswith("d8:commander:"):
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

    # The capability label is what a role names a capability by, and a delegatable role may select by
    # nothing else. The aggregation controller pours the rules of every ClusterRole that carries the
    # value into each role that selects it, whatever its name or kind. Deckhouse writes its own
    # capabilities through the service account this webhook does not see, so a built-in value on an
    # object written here would pass someone's rules off as that capability, and only the custom.
    # values are left to users. An UPDATE that leaves the value the object already had passes, so an
    # object written before this check existed, such as one a GitOps tool applies again, keeps working,
    # while adding a reserved value or changing to one is refused.
    capability = labels.get(CAPABILITY_SELECTOR_LABEL)
    if (
        capability is not None
        and not str(capability).startswith("custom.")
        and capability != _old_label(request, CAPABILITY_SELECTOR_LABEL)
    ):
        return (
            f'ClusterRole "{name}" must not carry "{CAPABILITY_SELECTOR_LABEL}: {capability}": '
            'the values without the "custom." prefix are reserved for Deckhouse built-in capabilities.'
        )

    # The delegatable label is judged on its own, before anything below, because the grant registry
    # judges it that way: modules/160-multitenancy-manager/templates/cluster-objects-controller/
    # grantable-resources.yaml excludes every ClusterRole without this label and admits every
    # ClusterRole with it, whatever else the object carries. A role that claimed the label without
    # the framework labels was therefore bindable in every project and validated by nothing at all.
    if labels.get(DELEGATABLE_LABEL) == "true" and not _aggregates_only_tenant_capabilities(selectors):
        return (
            f'ClusterRole "{name}" must not be labeled "{DELEGATABLE_LABEL}: true": '
            "a role bindable inside a project may aggregate namespace and project capabilities only."
        )

    # A label of a platform lineage on the object itself is what makes the aggregation controller pour
    # its rules into the platform role of that lineage -- d8:namespace:admin selects exactly
    # rbac.deckhouse.io/aggregate-to-namespace-as: admin. The API server does not care what else the
    # object carries, so without this check any ClusterRole with such a label, whatever its name and
    # with no framework kind at all, joins every holder of that role in the cluster. Only an object
    # that declares itself a custom capability may carry one; the checks below then hold it to the
    # rest of the contract.
    platform_lineages = _platform_lineages_on(labels, system_lineages)
    if platform_lineages and kind_label != "custom-capability":
        return (
            f'ClusterRole "{name}" carries the aggregation label of the '
            f'"{", ".join(sorted(platform_lineages))}" lineage but is not labeled '
            f'"{KIND_LABEL}: custom-capability": only a custom capability may aggregate into a platform role.'
        )

    # The rest applies to anything that presents itself as a custom object -- by its framework label
    # or by its name. Judging by the label alone let a "d8:custom:" role skip every rule about what
    # such a name is allowed to mean.
    if kind_label in ("custom-role", "custom-capability") or name.startswith("d8:custom:"):
        if kind_label in ("custom-role", "custom-capability") and not name.startswith("d8:custom:"):
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

        # A capability is the leaf of the model: it carries rules and is aggregated by roles. The
        # built-in ones are held to that by the platform contract test, and a custom one is held to it
        # here, or the rule would depend on who created the object rather than on what it does.
        if kind_label == "custom-capability" and selectors:
            return (
                f'ClusterRole "{name}" with "{KIND_LABEL}: custom-capability" must not define '
                "aggregationRule: a capability carries rules and is aggregated by roles, it does not "
                "aggregate other capabilities."
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
                elif lineage in system_lineages:
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
            offending = _cluster_level_selector(selectors, system_lineages)
            if offending:
                return (
                    f'ClusterRole "{name}" with "{SCOPE_LABEL}: {scope or "<unset>"}" must not aggregate {offending}: '
                    "a role granted in a namespace or a project may only aggregate namespace and project capabilities."
                )

    return None

# The template calls validate(ctx) and expects a (message, allowed) pair, while the check above keeps
# its original shape and returns the reason or None.
def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    message = validation_error(ctx)
    return (message, False) if message else (None, True)
