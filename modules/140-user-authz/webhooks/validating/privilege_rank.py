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


# Shared rank for User admission and ClusterAuthorizationRule escalate admission.
#
# A Kubernetes username is the Dex email claim. Grants live on that string (CAR/AR/CRB), not on
# the User object. Rank is "do you already hold P": rank(actor) >= rank(target).
#
# Actor: only mapped accessLevel / well-known ClusterRoles count. An additionalRole such as
# user-authn:edit must not inflate the actor to SuperAdmin.
#
# Target: an unmapped additionalRole or CRB roleRef counts as SuperAdmin so a custom cluster-admin
# equivalent cannot be minted by a weaker requester.

from typing import Any, Iterable, Optional

ACCESS_LEVEL_RANK = {
    "User": 10,
    "PrivilegedUser": 20,
    "Editor": 30,
    "Admin": 40,
    "ClusterEditor": 50,
    "ClusterAdmin": 60,
    "SuperAdmin": 70,
}

SUPER_RANK = ACCESS_LEVEL_RANK["SuperAdmin"]
CLUSTER_ADMIN_RANK = ACCESS_LEVEL_RANK["ClusterAdmin"]
ADMIN_RANK = ACCESS_LEVEL_RANK["Admin"]

# Well-known ClusterRoles. Keys are asserted in privilege_rank_test.py.
# cluster-admin is the built-in Kubernetes role; the rest are rendered by Deckhouse templates.
ROLE_RANK = {
    "cluster-admin": SUPER_RANK,
    "user-authz:super-admin": SUPER_RANK,
    "d8:manage:all:manager": SUPER_RANK,
    "d8:manage:security:manager": CLUSTER_ADMIN_RANK,
    "user-authz:cluster-admin": CLUSTER_ADMIN_RANK,
    "d8:manage:permission:module:user-authz:edit": CLUSTER_ADMIN_RANK,
}

TARGET_UNKNOWN_ROLE_RANK = SUPER_RANK

PRIVILEGED_USERS = {
    "system:apiserver",
    "system:serviceaccount:d8-system:deckhouse",
    "system:serviceaccount:kube-system:clusterrole-aggregation-controller",
}

CAR_SNAP = "d8-user-authz-privilege-cluster-authorization-rules"
AR_SNAP = "d8-user-authz-privilege-authorization-rules"
CRB_SNAP = "d8-user-authz-privilege-cluster-role-bindings"

SA_PREFIX = "system:serviceaccount:"

CAR_JQ_FILTER = """
{
  "name": .metadata.name,
  "accessLevel": (.spec.accessLevel // ""),
  "additionalRoles": [.spec.additionalRoles[]? | .name],
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

def kubernetes_snapshots() -> str:
    """Informer stanzas for the privilege validating hook."""
    return f"""
- name: {CAR_SNAP}
  apiVersion: deckhouse.io/v1alpha1
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
"""


def _indent(text: str, n: int) -> str:
    pad = " " * n
    return "\n".join(pad + line if line else line for line in text.splitlines())


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


def access_level_rank(level: Optional[str], *, cap: Optional[int] = None) -> int:
    if not isinstance(level, str) or not level:
        return 0
    rank = ACCESS_LEVEL_RANK.get(level, 0)
    if cap is not None and rank > cap:
        return cap
    return rank


def role_rank(name: Optional[str], *, for_target: bool) -> int:
    if not isinstance(name, str) or not name:
        return 0
    if name in ROLE_RANK:
        return ROLE_RANK[name]
    return TARGET_UNKNOWN_ROLE_RANK if for_target else 0


def additional_roles_rank(names: Iterable, *, for_target: bool) -> int:
    best = 0
    for name in _list(names):
        best = max(best, role_rank(name, for_target=for_target))
    return best


def rank_label(rank: int) -> str:
    for name, value in sorted(ACCESS_LEVEL_RANK.items(), key=lambda kv: kv[1], reverse=True):
        if rank >= value:
            return name
    return "none"


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


def _fr_rank(fr: dict, *, namespaced: bool, crb: bool, for_target: bool) -> int:
    if crb:
        return role_rank(fr.get("role"), for_target=for_target)
    cap = ADMIN_RANK if namespaced else None
    return max(
        access_level_rank(fr.get("accessLevel"), cap=cap),
        additional_roles_rank(fr.get("additionalRoles"), for_target=for_target),
    )


def identity_rank(snapshots: Any, kind: str, name: str, *, for_target: bool) -> int:
    """
    Highest grant already attached to this token identity.

    kind is "user", "group", or "sa". User names that come from spec.email are matched
    lowercase against already-lowercase rule subjects (the issued username is lowercased).
    Group names and ServiceAccount keys are compared verbatim.
    """
    if not isinstance(name, str) or not name:
        return 0

    if kind == "user":
        key = "userSubjects"
        needle = name.lower() if for_target else name
    elif kind == "group":
        key = "groupSubjects"
        needle = name
    elif kind == "sa":
        key = "saSubjects"
        needle = name
    else:
        return 0

    best = 0
    for snap_name, namespaced, crb in (
        (CAR_SNAP, False, False),
        (AR_SNAP, True, False),
        (CRB_SNAP, False, True),
    ):
        for fr in iter_filter_results(snapshots, snap_name):
            subjects = _subjects(fr, key)
            # Incoming email is lowercased; a mixed-case *rule subject* never matches a token.
            if kind == "user" and for_target:
                match = needle in [s for s in subjects if s == s.lower()]
            else:
                match = needle in subjects
            if match:
                best = max(best, _fr_rank(fr, namespaced=namespaced, crb=crb, for_target=for_target))
    return best


def sa_key(username: str) -> Optional[str]:
    if not isinstance(username, str) or not username.startswith(SA_PREFIX):
        return None
    return username[len(SA_PREFIX):]


def actor_rank(user_info: Any, snapshots: Any) -> int:
    info = as_plain(user_info) or {}
    username = info.get("username") or ""
    groups = [g for g in _list(info.get("groups")) if isinstance(g, str)]

    if username in PRIVILEGED_USERS or "system:masters" in groups:
        return SUPER_RANK

    best = identity_rank(snapshots, "user", username, for_target=False)
    sa = sa_key(username)
    if sa:
        best = max(best, identity_rank(snapshots, "sa", sa, for_target=False))
    for group in groups:
        best = max(best, identity_rank(snapshots, "group", group, for_target=False))
    return best


def target_user_rank(snapshots: Any, email: str) -> int:
    return identity_rank(snapshots, "user", email, for_target=True)


def car_granted_rank(spec: Any) -> int:
    spec = as_plain(spec) or {}
    roles = []
    for item in _list(spec.get("additionalRoles")):
        item = as_plain(item) or {}
        if isinstance(item, dict):
            roles.append(item.get("name"))
        elif isinstance(item, str):
            roles.append(item)
    return max(
        access_level_rank(spec.get("accessLevel")),
        additional_roles_rank(roles, for_target=True),
    )


def escalate_denied(actor: int, target: int) -> bool:
    return target > actor
