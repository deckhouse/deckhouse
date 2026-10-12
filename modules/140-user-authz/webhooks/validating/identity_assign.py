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


# can-assign kernel for identity admission.
#
# A Kubernetes username is the Dex email claim. Grants live on that string
# (CAR / AR / CRB), not on the User object. Creating a credential for a free
# string is regular CRUD. Touching an occupied string, or writing a CAR, is
# user-role assignment:
#
#   allow if covers(actor.rules, target.rules)
#        or every target role is inside the actor's labeled can-assign range
#
# Range labels are honored only on platform-owned ClusterRole names. Custom
# roles are cover-only (fail closed). An actor whose cluster-wide rules grant
# everything, such as a subject of the default cluster-admin ClusterRole, holds
# the SuperAdmin range.

from typing import Any, Iterable, List, NamedTuple, Optional, Sequence, Set, Tuple

LABEL_BASIC_MAX = "user-authz.deckhouse.io/can-assign-basic-max"
LABEL_SCOPE = "user-authz.deckhouse.io/can-assign-scope"
LABEL_SUBSYSTEM = "user-authz.deckhouse.io/can-assign-subsystem"
LABEL_MAX_LEVEL = "user-authz.deckhouse.io/can-assign-max-level"
ANNOTATION_ACCESS_LEVEL = "user-authz.deckhouse.io/access-level"

CAN_ASSIGN_LABELS = (LABEL_BASIC_MAX, LABEL_SCOPE, LABEL_SUBSYSTEM, LABEL_MAX_LEVEL)

CAR_SNAP = "d8-user-authz-assign-cluster-authorization-rules"
AR_SNAP = "d8-user-authz-assign-authorization-rules"
CRB_SNAP = "d8-user-authz-assign-cluster-role-bindings"
CROLE_SNAP = "d8-user-authz-assign-cluster-roles"
MT_STATE_SNAP = "d8-user-authz-assign-multitenancy-state"

SA_PREFIX = "system:serviceaccount:"

BASIC_LEVEL_ROLE = {
    "User": "user-authz:user",
    "PrivilegedUser": "user-authz:privileged-user",
    "Editor": "user-authz:editor",
    "Admin": "user-authz:admin",
    "ClusterEditor": "user-authz:cluster-editor",
    "ClusterAdmin": "user-authz:cluster-admin",
    "SuperAdmin": "user-authz:super-admin",
}

# The aggregated ClusterRole of the custom roles of each level (templates/cluster-roles-custom-aggregated.yaml).
# Every ClusterAuthorizationRule below SuperAdmin binds it next to user-authz:<level>, so it stands for
# that level: assigning a level assigns its custom extension too, the way car_target_roles counts a rule
# by its accessLevel alone.
CUSTOM_LEVEL_ROLE = {
    role + ":custom": level for level, role in BASIC_LEVEL_ROLE.items() if level != "SuperAdmin"
}

BASIC_ORDER = {
    "User": 10,
    "PrivilegedUser": 20,
    "Editor": 30,
    "Admin": 40,
    "ClusterEditor": 50,
    "ClusterAdmin": 60,
    "SuperAdmin": 70,
}

RBACV2_LEVELS = ("viewer", "user", "manager", "admin", "superadmin")
RBACV2_ORDER = {name: i for i, name in enumerate(RBACV2_LEVELS)}

DISASTER_NAMES = frozenset({
    "cluster-admin",
    "user-authz:super-admin",
})

EXEMPT_USERS = frozenset({
    "system:apiserver",
    "system:sudouser",
    "system:kube-controller-manager",
    "system:kube-scheduler",
    "system:volume-scheduler",
    "dhctl",
    "observability",
    "system:serviceaccount:d8-system:deckhouse",
    "system:serviceaccount:kube-system:clusterrole-aggregation-controller",
})

EXEMPT_GROUPS = frozenset({
    "system:masters",
    "system:serviceaccounts:kube-system",
})

AR_ACCESS_LEVEL_CAP = "Admin"

# Every verb on every resource of every API group and on every non-resource
# URL: the rules of the default cluster-admin ClusterRole.
FULL_ACCESS_RULES = (
    {"apiGroups": ["*"], "resources": ["*"], "verbs": ["*"]},
    {"nonResourceURLs": ["*"], "verbs": ["*"]},
)

# Name prefix of the ClusterRoleBindings rendered from a ClusterAuthorizationRule:
# user-authz:<rule>:<postfix>. A rule name cannot contain a colon.
RULE_BINDING_PREFIX = "user-authz:"

_ALL_NAMES = object()


class RoleDesc(NamedTuple):
    scope: str
    subsystem: str
    level: str


class AssignRange(NamedTuple):
    basic_max: Optional[str]
    scope: Optional[str]
    subsystems: Tuple[str, ...]
    max_level: Optional[str]


SUPERADMIN_RANGE = AssignRange(basic_max="SuperAdmin", scope="system", subsystems=(),
                               max_level="superadmin")


class CatalogEntry(NamedTuple):
    name: str
    rules: List[dict]
    labels: dict
    access_level: str


def as_plain(obj: Any) -> Any:
    if obj is None:
        return None
    if hasattr(obj, "toDict"):
        return obj.toDict()
    return obj


def _list(obj: Any) -> list:
    value = as_plain(obj)
    if not value:
        return []
    if isinstance(value, list):
        return value
    return list(value)


def _dict(obj: Any) -> dict:
    value = as_plain(obj)
    return value if isinstance(value, dict) else {}


def iter_filter_results(snapshots: Any, snap_name: str) -> Iterable[dict]:
    if snapshots is None:
        return
    snaps = snapshots.get(snap_name) if hasattr(snapshots, "get") else None
    if not snaps:
        return
    for item in snaps:
        item = as_plain(item) or {}
        if isinstance(item, dict):
            fr = as_plain(item.get("filterResult")) or {}
        else:
            fr = as_plain(getattr(item, "filterResult", None)) or {}
        if isinstance(fr, dict):
            yield fr


def _subjects(fr: dict, key: str) -> list:
    return [s for s in _list(fr.get(key)) if isinstance(s, str)]


def sa_key(username: str) -> Optional[str]:
    if not isinstance(username, str) or not username.startswith(SA_PREFIX):
        return None
    return username[len(SA_PREFIX):]


def is_exempt(user_info: Any) -> bool:
    info = _dict(user_info)
    username = info.get("username") or ""
    if username in EXEMPT_USERS:
        return True
    groups = [g for g in _list(info.get("groups")) if isinstance(g, str)]
    return bool(EXEMPT_GROUPS.intersection(groups))


def is_platform_range_role(name: str) -> bool:
    if not isinstance(name, str) or not name:
        return False
    if name == "cluster-admin":
        return True
    if name.startswith("user-authz:"):
        return True
    return name.startswith("d8:") and not name.startswith("d8:custom:")


def basic_role_for_level(level: Optional[str], *, cap: Optional[str] = None) -> Optional[str]:
    if not isinstance(level, str) or level not in BASIC_LEVEL_ROLE:
        return None
    if cap and BASIC_ORDER.get(level, 0) > BASIC_ORDER.get(cap, 0):
        level = cap
    return BASIC_LEVEL_ROLE[level]


def additional_role_names(spec: Any) -> List[str]:
    names = []
    for item in _list(_dict(spec).get("additionalRoles")):
        if isinstance(item, str):
            if item:
                names.append(item)
            continue
        item = _dict(item)
        name = item.get("name")
        if isinstance(name, str) and name:
            names.append(name)
    return names


def load_catalog(snapshots: Any) -> dict:
    catalog = {}
    for fr in iter_filter_results(snapshots, CROLE_SNAP):
        name = fr.get("name")
        if not isinstance(name, str) or not name:
            continue
        catalog[name] = CatalogEntry(
            name=name,
            rules=[r for r in _list(fr.get("rules")) if isinstance(r, dict)],
            labels=_dict(fr.get("labels")),
            access_level=fr.get("accessLevel") or "",
        )
    return catalog


def _fr_role_names(fr: dict, *, namespaced: bool) -> List[str]:
    names = []
    cap = AR_ACCESS_LEVEL_CAP if namespaced else None
    basic = basic_role_for_level(fr.get("accessLevel"), cap=cap)
    if basic:
        names.append(basic)
    # AuthorizationRule has no additionalRoles field; ignore a forged snapshot key.
    if namespaced:
        return names
    for role in _list(fr.get("additionalRoles")):
        if isinstance(role, str) and role:
            names.append(role)
    return names


def _match_user_subject(subjects: Sequence[str], email: str, *, lowercase_needle: bool) -> bool:
    if lowercase_needle:
        needle = email.lower()
        return needle in [s for s in subjects if s == s.lower()]
    return email in subjects


def collect_roles_for_identity(snapshots: Any, kind: str, name: str, *,
                               lowercase_user: bool,
                               include_namespaced: bool = True) -> List[str]:
    """Role names already attached to this token identity via CAR / AR / CRB."""
    if not isinstance(name, str) or not name:
        return []

    if kind == "user":
        key = "userSubjects"
    elif kind == "group":
        key = "groupSubjects"
        lowercase_user = False
    elif kind == "sa":
        key = "saSubjects"
        lowercase_user = False
    else:
        return []

    found: List[str] = []
    seen: Set[str] = set()

    def add(role: str) -> None:
        if role and role not in seen:
            seen.add(role)
            found.append(role)

    sources = [(CAR_SNAP, False)]
    if include_namespaced:
        sources.append((AR_SNAP, True))
    for snap_name, namespaced in sources:
        for fr in iter_filter_results(snapshots, snap_name):
            subjects = _subjects(fr, key)
            if kind == "user":
                ok = _match_user_subject(subjects, name, lowercase_needle=lowercase_user)
            else:
                ok = name in subjects
            if ok:
                for role in _fr_role_names(fr, namespaced=namespaced):
                    add(role)

    for fr in iter_filter_results(snapshots, CRB_SNAP):
        subjects = _subjects(fr, key)
        if kind == "user":
            ok = _match_user_subject(subjects, name, lowercase_needle=lowercase_user)
        else:
            ok = name in subjects
        if ok:
            role = fr.get("role")
            if isinstance(role, str):
                add(role)

    return found


def actor_roles(user_info: Any, snapshots: Any) -> List[str]:
    info = _dict(user_info)
    username = info.get("username") or ""
    groups = [g for g in _list(info.get("groups")) if isinstance(g, str)]
    found: List[str] = []
    seen: Set[str] = set()

    def add_all(roles: Iterable[str]) -> None:
        for role in roles:
            if role not in seen:
                seen.add(role)
                found.append(role)

    add_all(collect_roles_for_identity(
        snapshots, "user", username, lowercase_user=False, include_namespaced=False))
    sa = sa_key(username)
    if sa:
        add_all(collect_roles_for_identity(
            snapshots, "sa", sa, lowercase_user=False, include_namespaced=False))
    for group in groups:
        add_all(collect_roles_for_identity(
            snapshots, "group", group, lowercase_user=False, include_namespaced=False))
    return found


def multitenancy_enabled(snapshots: Any) -> bool:
    """Whether enableMultiTenancy is in effect.

    templates/namespace.yaml renders the d8-user-authz-multitenancy-state
    ConfigMap only when it is, so a missing ConfigMap reads as off, the same
    way multitenancy.py reads it.
    """
    return any(fr.get("enableMultiTenancy") is True for fr in iter_filter_results(snapshots, MT_STATE_SNAP))


def car_is_namespace_limited(fr: dict, *, multitenancy: bool) -> bool:
    """Whether a ClusterAuthorizationRule snapshot may leave out some namespaces.

    Any namespaceSelector counts, matchAny included, and so does any non-empty
    limitNamespaces, whether or not multi-tenancy is on. This holds on purpose
    even for entries that cover every namespace, such as ".*" with
    allowAccessToSystemNamespaces. The check does not read the patterns, so
    such a rule errs on the strict side.

    With multi-tenancy on, a rule without allowAccessToSystemNamespaces counts
    too, because the authorization webhook keeps it out of the system
    namespaces, and a rule that does not reach them must not assign a role
    that would. With multi-tenancy off, the authorization webhook is not
    installed, and multitenancy.py refuses all three fields, so a rule reaches
    every namespace.
    """
    if fr.get("namespaceSelector") is not None or bool(_list(fr.get("limitNamespaces"))):
        return True
    return multitenancy and fr.get("allowAccessToSystemNamespaces") is not True


def rule_of_binding(name: Any) -> str:
    """The rule named in a user-authz:<rule>:<postfix> binding name, or an empty string."""
    if not isinstance(name, str) or not name.startswith(RULE_BINDING_PREFIX):
        return ""
    rule, sep, _ = name[len(RULE_BINDING_PREFIX):].partition(":")
    return rule if sep else ""


def actor_cluster_wide_roles(user_info: Any, snapshots: Any) -> List[str]:
    """Roles the actor holds in every namespace and over cluster-scoped objects.

    They come from ClusterRoleBindings and from the ClusterAuthorizationRules
    that are not namespace-limited (car_is_namespace_limited). A
    namespace-limited rule still renders cluster-wide bindings, user-authz:<rule>:<postfix>, and the
    multi-tenancy authorization webhook keeps them inside the rule's
    namespaces, so those bindings do not count either.
    """
    info = _dict(user_info)
    username = info.get("username") or ""
    sa = sa_key(username) or ""
    groups = {g for g in _list(info.get("groups")) if isinstance(g, str)}

    def is_subject(fr: dict) -> bool:
        if username and username in _subjects(fr, "userSubjects"):
            return True
        if sa and sa in _subjects(fr, "saSubjects"):
            return True
        return any(g in groups for g in _subjects(fr, "groupSubjects"))

    found: List[str] = []
    seen: Set[str] = set()

    def add(role: Any) -> None:
        if isinstance(role, str) and role and role not in seen:
            seen.add(role)
            found.append(role)

    multitenancy = multitenancy_enabled(snapshots)
    limited: Set[str] = set()
    for fr in iter_filter_results(snapshots, CAR_SNAP):
        if car_is_namespace_limited(fr, multitenancy=multitenancy):
            name = fr.get("name")
            if isinstance(name, str) and name:
                limited.add(name)
            continue
        if is_subject(fr):
            for role in _fr_role_names(fr, namespaced=False):
                add(role)

    for fr in iter_filter_results(snapshots, CRB_SNAP):
        if is_subject(fr) and rule_of_binding(fr.get("name")) not in limited:
            add(fr.get("role"))
    return found


def actor_has_full_access(user_info: Any, snapshots: Any, catalog: dict) -> bool:
    """Whether the actor's cluster-wide roles grant every verb on everything."""
    rules = union_rules(actor_cluster_wide_roles(user_info, snapshots), catalog)
    return covers(rules, FULL_ACCESS_RULES)


def target_user_roles(snapshots: Any, email: str, groups: Optional[Iterable[str]] = None) -> List[str]:
    found: List[str] = []
    seen: Set[str] = set()

    def add_all(roles: Iterable[str]) -> None:
        for role in roles:
            if role not in seen:
                seen.add(role)
                found.append(role)

    add_all(collect_roles_for_identity(snapshots, "user", email, lowercase_user=True))
    for group in _list(groups):
        if isinstance(group, str) and group:
            add_all(collect_roles_for_identity(snapshots, "group", group, lowercase_user=False))
    return found


def target_group_roles(snapshots: Any, group_name: str) -> List[str]:
    return collect_roles_for_identity(snapshots, "group", group_name, lowercase_user=False)


def car_target_roles(spec: Any) -> List[str]:
    spec = _dict(spec)
    names = []
    basic = basic_role_for_level(spec.get("accessLevel"))
    if basic:
        names.append(basic)
    names.extend(additional_role_names(spec))
    out, seen = [], set()
    for name in names:
        if name not in seen:
            seen.add(name)
            out.append(name)
    return out


def describe_role(name: str, labels: Optional[dict] = None) -> Optional[RoleDesc]:
    labels = labels or {}
    if (name.startswith("d8:manage:permission:") or ":capability:" in name
            or name.startswith("d8:system-capability:")):
        return None

    rbac_scope = labels.get("rbac.deckhouse.io/scope") or labels.get("scope") or ""
    rbac_sub = labels.get("rbac.deckhouse.io/subsystem") or labels.get("subsystem") or ""

    parts = name.split(":")
    level = ""
    scope = rbac_scope
    subsystem = rbac_sub

    if name.startswith("d8:use:role:") and len(parts) >= 4:
        scope = "namespace"
        level = parts[3]
    elif name.startswith("d8:manage:all:") and len(parts) == 4:
        scope = "system"
        level = parts[3]
    elif name.startswith("d8:manage:") and len(parts) == 4:
        scope = "subsystem"
        subsystem = parts[2]
        level = parts[3]
    elif name.startswith("d8:subsystem:") and len(parts) >= 4:
        scope = "subsystem"
        subsystem = parts[2]
        level = parts[-1]
    elif name.startswith("d8:system:") and len(parts) == 3:
        scope = "system"
        level = parts[2]
    elif name.startswith("d8:namespace:") and len(parts) == 3:
        scope = "namespace"
        level = parts[2]
    elif name.startswith("d8:project:") and len(parts) == 3:
        scope = "project"
        level = parts[2]

    if level not in RBACV2_ORDER:
        return None
    if not scope:
        return None
    return RoleDesc(scope=scope, subsystem=subsystem, level=level)


def basic_level_of(name: str, entry: Optional[CatalogEntry]) -> Optional[str]:
    for level, role in BASIC_LEVEL_ROLE.items():
        if name == role:
            return level
    if name in CUSTOM_LEVEL_ROLE:
        return CUSTOM_LEVEL_ROLE[name]
    if (entry and entry.access_level in BASIC_ORDER
            and is_platform_range_role(name)):
        return entry.access_level
    return None


def is_disaster(name: str, entry: Optional[CatalogEntry]) -> bool:
    if name in DISASTER_NAMES:
        return True
    desc = describe_role(name, entry.labels if entry else None)
    return bool(desc and desc.level == "superadmin")


def _range_from_entry(entry: CatalogEntry) -> Optional[AssignRange]:
    if not is_platform_range_role(entry.name):
        return None
    labels = entry.labels
    basic_max = labels.get("can-assign-basic-max") or labels.get(LABEL_BASIC_MAX) or None
    scope = labels.get("can-assign-scope") or labels.get(LABEL_SCOPE) or None
    subsystem = labels.get("can-assign-subsystem") or labels.get(LABEL_SUBSYSTEM) or ""
    max_level = labels.get("can-assign-max-level") or labels.get(LABEL_MAX_LEVEL) or None
    if basic_max and basic_max not in BASIC_ORDER:
        basic_max = None
    if max_level and max_level not in RBACV2_ORDER:
        max_level = None
    if scope not in ("subsystem", "system", None):
        scope = None
    if not basic_max and not max_level:
        return None
    subsystems = (subsystem,) if subsystem and scope == "subsystem" else ()
    return AssignRange(basic_max=basic_max, scope=scope, subsystems=subsystems, max_level=max_level)


def merge_ranges(entries: Iterable[CatalogEntry]) -> AssignRange:
    basic_max = None
    max_level = None
    system = False
    subsystems: Set[str] = set()
    for entry in entries:
        rng = _range_from_entry(entry)
        if rng is None:
            continue
        if rng.basic_max and BASIC_ORDER[rng.basic_max] >= BASIC_ORDER.get(basic_max, 0):
            basic_max = rng.basic_max
        if rng.max_level and RBACV2_ORDER[rng.max_level] >= RBACV2_ORDER.get(max_level, -1):
            max_level = rng.max_level
        if rng.scope == "system":
            system = True
        subsystems.update(rng.subsystems)
    return AssignRange(
        basic_max=basic_max,
        scope="system" if system else ("subsystem" if subsystems else None),
        subsystems=tuple(sorted(subsystems)),
        max_level=max_level,
    )


def actor_range(actor_role_names: Sequence[str], catalog: dict, *,
                full_access: bool = False) -> AssignRange:
    """The actor's can-assign range.

    An actor with full access (actor_has_full_access) holds the SuperAdmin
    range on both ladders. Otherwise the range merges the can-assign labels
    of the actor's platform roles.
    """
    if full_access:
        return SUPERADMIN_RANGE
    entries = []
    for name in actor_role_names:
        entry = catalog.get(name)
        if entry:
            entries.append(entry)
    return merge_ranges(entries)


def role_in_range(name: str, entry: Optional[CatalogEntry], rng: AssignRange) -> bool:
    if is_disaster(name, entry):
        return rng.basic_max == "SuperAdmin" or rng.max_level == "superadmin"

    basic = basic_level_of(name, entry)
    if basic and rng.basic_max:
        if BASIC_ORDER[basic] <= BASIC_ORDER[rng.basic_max]:
            return True

    desc = describe_role(name, entry.labels if entry else None)
    if not desc or not rng.max_level:
        return False
    if RBACV2_ORDER[desc.level] > RBACV2_ORDER[rng.max_level]:
        return False
    if rng.scope == "system":
        return True
    if rng.scope == "subsystem":
        return desc.scope == "subsystem" and desc.subsystem in rng.subsystems
    return False


def _owner_verbs(rule: dict) -> list:
    return [v for v in _list(rule.get("verbs")) if isinstance(v, str)]


def _contains_star_or(items: Sequence[str], needle: str) -> bool:
    return "*" in items or needle in items


def _non_resource_covers(owner_url: str, servant_url: str) -> bool:
    if owner_url == "*":
        return True
    if owner_url.endswith("*"):
        return servant_url.startswith(owner_url[:-1])
    return owner_url == servant_url


def _resource_rule_covers(owner: dict, verb: str, group: str, resource: str, name: Any) -> bool:
    if _list(owner.get("nonResourceURLs")):
        return False
    if not _contains_star_or(_owner_verbs(owner), verb):
        return False
    groups = [g for g in _list(owner.get("apiGroups")) if isinstance(g, str)]
    if not groups:
        groups = [""]
    if not _contains_star_or(groups, group):
        return False
    resources = [r for r in _list(owner.get("resources")) if isinstance(r, str)]
    if not _contains_star_or(resources, resource):
        return False
    owner_names = [n for n in _list(owner.get("resourceNames")) if isinstance(n, str)]
    if name is _ALL_NAMES:
        return not owner_names
    if owner_names and name not in owner_names:
        return False
    return True


def _nonresource_rule_covers(owner: dict, verb: str, url: str) -> bool:
    urls = [u for u in _list(owner.get("nonResourceURLs")) if isinstance(u, str)]
    if not urls:
        return False
    if not _contains_star_or(_owner_verbs(owner), verb):
        return False
    return any(_non_resource_covers(u, url) for u in urls)


def _servant_atoms(rule: dict) -> List[tuple]:
    verbs = [v for v in _list(rule.get("verbs")) if isinstance(v, str)]
    urls = [u for u in _list(rule.get("nonResourceURLs")) if isinstance(u, str)]
    if urls:
        return [("non", verb, url) for verb in verbs for url in urls]
    groups = [g for g in _list(rule.get("apiGroups")) if isinstance(g, str)] or [""]
    resources = [r for r in _list(rule.get("resources")) if isinstance(r, str)]
    names = [n for n in _list(rule.get("resourceNames")) if isinstance(n, str)]
    name_atoms: List[Any] = names or [_ALL_NAMES]
    return [
        ("res", verb, group, resource, name)
        for verb in verbs
        for group in groups
        for resource in resources
        for name in name_atoms
    ]


def covers(owner_rules: Sequence[dict], servant_rules: Sequence[dict]) -> bool:
    """Conservative port of Kubernetes rbac.Covers: every servant atom must match an owner rule."""
    owners = [r for r in owner_rules if isinstance(r, dict)]
    for servant in servant_rules:
        if not isinstance(servant, dict):
            return False
        atoms = _servant_atoms(servant)
        if not atoms:
            continue
        for atom in atoms:
            if atom[0] == "non":
                ok = any(_nonresource_rule_covers(owner, atom[1], atom[2]) for owner in owners)
            else:
                ok = any(
                    _resource_rule_covers(owner, atom[1], atom[2], atom[3], atom[4])
                    for owner in owners
                )
            if not ok:
                return False
    return True


def union_rules(role_names: Sequence[str], catalog: dict) -> List[dict]:
    rules = []
    for name in role_names:
        entry = catalog.get(name)
        if entry:
            rules.extend(entry.rules)
    return rules


def _has_coverable_rules(rules: Sequence[dict]) -> bool:
    for rule in rules:
        if isinstance(rule, dict) and _servant_atoms(rule):
            return True
    return False


def can_assign(actor_role_names: Sequence[str], target_role_names: Sequence[str],
               catalog: dict, *, full_access: bool = False) -> Optional[List[str]]:
    """
    None means allow. A list is the leftover target roles the actor cannot assign.

    Disaster names (cluster-admin, SuperAdmin, rbacv2 superadmin) require the
    SuperAdmin range even when the live ClusterRole was rewritten to rules the
    actor already covers. An actor with full access holds that range. Cover is
    only used for non-disaster roles that still have atoms. A role missing from
    the catalog is leftover: range is not inferred from the name alone.
    """
    targets = [n for n in target_role_names if isinstance(n, str) and n]
    if not targets:
        return None

    rng = actor_range(actor_role_names, catalog, full_access=full_access)
    actor_rules = union_rules(actor_role_names, catalog)
    leftover = []
    for name in targets:
        entry = catalog.get(name)
        if is_disaster(name, entry):
            if role_in_range(name, entry, rng):
                continue
            leftover.append(name)
            continue
        if entry is None:
            leftover.append(name)
            continue
        if _has_coverable_rules(entry.rules) and covers(actor_rules, entry.rules):
            continue
        if role_in_range(name, entry, rng):
            continue
        leftover.append(name)
    if not leftover:
        return None
    return leftover


def range_summary(rng: AssignRange) -> str:
    parts = []
    if rng.basic_max:
        parts.append(f"basic<={rng.basic_max}")
    if rng.max_level:
        scope = rng.scope or "?"
        sub = ",".join(rng.subsystems) if rng.subsystems else "*"
        parts.append(f"{scope}/{sub}<={rng.max_level}")
    return ", ".join(parts) if parts else "none"


def deny_message(resource: str, field: str, value: str, leftover: Sequence[str],
                 rng: AssignRange) -> str:
    roles = ", ".join(leftover)
    return (
        f'{resource} "{field}" "{value}" already carries roles [{roles}]; '
        f"the requester's can-assign range is {range_summary(rng)} and does not cover them"
    )


def deny_car_message(name: str, leftover: Sequence[str], rng: AssignRange) -> str:
    roles = ", ".join(leftover)
    return (
        f'clusterauthorizationrules.deckhouse.io "{name}" grants roles [{roles}], '
        f"which are outside the requester's can-assign range ({range_summary(rng)})"
    )


def deny_label_message(name: str) -> str:
    return (
        f'clusterroles.rbac.authorization.k8s.io "{name}" cannot change '
        f"user-authz.deckhouse.io/can-assign-* labels; those labels are platform-owned"
    )


def can_assign_labels_changed(old_obj: Any, new_obj: Any) -> bool:
    old_labels = _dict(_dict(old_obj).get("metadata")).get("labels")
    new_labels = _dict(_dict(new_obj).get("metadata")).get("labels")
    old_labels = _dict(old_labels)
    new_labels = _dict(new_labels)
    for key in CAN_ASSIGN_LABELS:
        if old_labels.get(key) != new_labels.get(key):
            return True
    return False


def _indent(text: str, n: int) -> str:
    pad = " " * n
    return "\n".join(pad + line if line else line for line in text.splitlines())


CAR_JQ_FILTER = """
{
  "name": .metadata.name,
  "accessLevel": (.spec.accessLevel // ""),
  "additionalRoles": [.spec.additionalRoles[]? | .name],
  "namespaceSelector": (.spec.namespaceSelector // null),
  "limitNamespaces": [.spec.limitNamespaces[]?],
  "allowAccessToSystemNamespaces": (.spec.allowAccessToSystemNamespaces == true),
  "groupSubjects": [.spec.subjects[]? | select(.kind == "Group") | .name],
  "userSubjects": [.spec.subjects[]? | select(.kind == "User") | .name],
  "saSubjects": [.spec.subjects[]? | select(.kind == "ServiceAccount") | "\\(.namespace):\\(.name)"]
}
"""

AR_JQ_FILTER = """
{
  "name": .metadata.name,
  "namespace": .metadata.namespace,
  "accessLevel": (.spec.accessLevel // ""),
  "additionalRoles": [.spec.additionalRoles[]? | .name],
  "groupSubjects": [.spec.subjects[]? | select(.kind == "Group") | .name],
  "userSubjects": [.spec.subjects[]? | select(.kind == "User") | .name],
  "saSubjects": [.spec.subjects[]? | select(.kind == "ServiceAccount") | "\\(.namespace):\\(.name)"]
}
"""

CRB_JQ_FILTER = """
{
  "name": .metadata.name,
  "role": .roleRef.name,
  "groupSubjects": [.subjects[]? | select(.kind == "Group") | .name],
  "userSubjects": [.subjects[]? | select(.kind == "User") | .name],
  "saSubjects": [.subjects[]? | select(.kind == "ServiceAccount") | "\\(.namespace):\\(.name)"]
}
"""

CROLE_JQ_FILTER = """
{
  "name": .metadata.name,
  "rules": (.rules // []),
  "accessLevel": (.metadata.annotations["user-authz.deckhouse.io/access-level"] // ""),
  "labels": {
    "can-assign-basic-max": (.metadata.labels["user-authz.deckhouse.io/can-assign-basic-max"] // ""),
    "can-assign-scope": (.metadata.labels["user-authz.deckhouse.io/can-assign-scope"] // ""),
    "can-assign-subsystem": (.metadata.labels["user-authz.deckhouse.io/can-assign-subsystem"] // ""),
    "can-assign-max-level": (.metadata.labels["user-authz.deckhouse.io/can-assign-max-level"] // ""),
    "scope": (.metadata.labels["rbac.deckhouse.io/scope"] // ""),
    "subsystem": (.metadata.labels["rbac.deckhouse.io/subsystem"] // "")
  }
}
"""


def kubernetes_snapshots() -> str:
    # ClusterAuthorizationRule is read in v1: v1alpha1 has no namespaceSelector,
    # and the apiserver drops that field from objects served in v1alpha1.
    return f"""
- name: {CAR_SNAP}
  apiVersion: deckhouse.io/v1
  kind: ClusterAuthorizationRule
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  jqFilter: |-
{ _indent(CAR_JQ_FILTER.strip(), 4) }
- name: {AR_SNAP}
  apiVersion: deckhouse.io/v1alpha1
  kind: AuthorizationRule
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  jqFilter: |-
{ _indent(AR_JQ_FILTER.strip(), 4) }
- name: {CRB_SNAP}
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRoleBinding
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  jqFilter: |-
{ _indent(CRB_JQ_FILTER.strip(), 4) }
- name: {CROLE_SNAP}
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  jqFilter: |-
{ _indent(CROLE_JQ_FILTER.strip(), 4) }
- name: {MT_STATE_SNAP}
  apiVersion: v1
  kind: ConfigMap
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  namespace:
    nameSelector:
      matchNames:
      - d8-user-authz
  nameSelector:
    matchNames:
    - d8-user-authz-multitenancy-state
  jqFilter: |-
    {{"enableMultiTenancy": (.data.enableMultiTenancy == "true")}}
"""
