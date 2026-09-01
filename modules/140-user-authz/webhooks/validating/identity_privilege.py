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


# Identity can-assign admission: User, Group, ClusterAuthorizationRule
# (including DELETE), UserOperation, DexProvider, and can-assign-* labels
# on ClusterRole.
# Kernel is identity_assign.py.

from typing import Any, List, Optional

from deckhouse import hook
from dotmap import DotMap

import identity_assign as assign

MATCH_CONDITIONS = """
  matchConditions:
  - expression: ("system:apiserver" != request.userInfo.username)
    name: exclude-kube-apiserver
  - expression: ("system:sudouser" != request.userInfo.username)
    name: exclude-sudouser
  - expression: ("system:kube-controller-manager" != request.userInfo.username)
    name: exclude-kube-controller-manager
  - expression: ("system:kube-scheduler" != request.userInfo.username)
    name: exclude-kube-scheduler
  - expression: ("system:volume-scheduler" != request.userInfo.username)
    name: exclude-volume-scheduler
  - expression: ("dhctl" != request.userInfo.username)
    name: exclude-dhctl
  - expression: ("observability" != request.userInfo.username)
    name: exclude-observability
  - expression: ("system:serviceaccount:d8-system:deckhouse" != request.userInfo.username)
    name: exclude-deckhouse
  - expression: ("system:serviceaccount:kube-system:clusterrole-aggregation-controller" != request.userInfo.username)
    name: exclude-aggregation-controller
  - expression: '!("system:masters" in request.userInfo.groups)'
    name: exclude-system-masters
  - expression: '!("system:serviceaccounts:kube-system" in request.userInfo.groups)'
    name: exclude-kube-system-sas
"""

CONFIG = f"""
configVersion: v1
kubernetesValidating:
- name: d8-user-authz-identity-assign.deckhouse.io
  includeSnapshotsFrom:
    - "{assign.CAR_SNAP}"
    - "{assign.AR_SNAP}"
    - "{assign.CRB_SNAP}"
    - "{assign.CROLE_SNAP}"
    - "{assign.USER_SNAP}"
    - "{assign.GROUP_SNAP}"
{MATCH_CONDITIONS}
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE", "DELETE"]
    resources:   ["users"]
    scope:       "Cluster"
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE", "DELETE"]
    resources:   ["groups"]
    scope:       "Cluster"
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE", "DELETE"]
    resources:   ["clusterauthorizationrules"]
    scope:       "Cluster"
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE"]
    resources:   ["useroperations"]
    scope:       "Cluster"
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["dexproviders"]
    scope:       "Cluster"
  - apiGroups:   ["rbac.authorization.k8s.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["clusterroles"]
    scope:       "Cluster"
kubernetes:
{assign.kubernetes_snapshots()}
"""


def main(ctx: hook.Context):
    try:
        binding_context = DotMap(ctx.binding_context)
        errmsg = validate(binding_context)
        if errmsg:
            ctx.output.validations.deny(errmsg)
        else:
            ctx.output.validations.allow()
    except Exception as e:
        ctx.output.validations.error(str(e))


def _spec(obj: Any) -> dict:
    if not obj:
        return {}
    return assign._dict(assign.as_plain(obj).get("spec") if isinstance(assign.as_plain(obj), dict)
                        else getattr(obj, "spec", None))


def _meta_name(obj: Any) -> str:
    if not obj:
        return ""
    plain = assign.as_plain(obj)
    if isinstance(plain, dict):
        return ((plain.get("metadata") or {}).get("name")) or ""
    meta = getattr(obj, "metadata", None)
    return (getattr(meta, "name", None) or "") if meta else ""


def _spec_groups(spec: dict) -> List[str]:
    return [g for g in assign._list(spec.get("groups")) if isinstance(g, str) and g]


def validate(ctx: DotMap) -> Optional[str]:
    req = ctx.review.request
    if assign.is_exempt(req.userInfo):
        return None

    kind = (req.kind.kind or "").lower()
    if kind == "clusterrole":
        return validate_clusterrole(req)

    catalog = assign.load_catalog(ctx.snapshots)
    actor = assign.actor_roles(req.userInfo, ctx.snapshots)

    if kind == "user":
        return validate_user(req, ctx.snapshots, actor, catalog)
    if kind == "group":
        return validate_group(req, ctx.snapshots, actor, catalog)
    if kind == "clusterauthorizationrule":
        return validate_car(req, actor, catalog)
    if kind == "useroperation":
        return validate_useroperation(req, ctx.snapshots, actor, catalog)
    if kind == "dexprovider":
        return validate_dexprovider(req, ctx.snapshots, actor, catalog)
    return None


def validate_user(req, snapshots, actor: List[str], catalog: dict) -> Optional[str]:
    new_spec = _spec(req.object)
    old_spec = _spec(req.oldObject)

    emails = []
    groups = []
    if req.operation == "DELETE":
        emails.append(old_spec.get("email"))
        groups.extend(_spec_groups(old_spec))
    else:
        emails.append(new_spec.get("email"))
        groups.extend(_spec_groups(new_spec))
        if req.operation == "UPDATE":
            emails.append(old_spec.get("email"))
            groups.extend(_spec_groups(old_spec))

    user_names = [_meta_name(req.object), _meta_name(req.oldObject)]
    extra_groups = list(groups)
    for user_name in user_names:
        extra_groups.extend(assign.membership_groups(snapshots, user_name=user_name))
    for email in emails:
        if isinstance(email, str) and email:
            extra_groups.extend(assign.membership_groups(snapshots, email=email))

    targets: List[str] = []
    seen = set()
    display_email = ""
    for email in emails:
        if not isinstance(email, str) or not email:
            continue
        if not display_email:
            display_email = email
        for role in assign.target_user_roles(snapshots, email, extra_groups):
            if role not in seen:
                seen.add(role)
                targets.append(role)

    if not targets:
        return None
    rng = assign.actor_range(actor, catalog)
    leftover = assign.can_assign(actor, targets, catalog)
    if leftover is None:
        return None
    return assign.deny_message("users.deckhouse.io", ".spec.email", display_email, leftover, rng)


def validate_group(req, snapshots, actor: List[str], catalog: dict) -> Optional[str]:
    names = []
    new_spec = _spec(req.object)
    old_spec = _spec(req.oldObject)
    if req.operation != "DELETE":
        names.append(new_spec.get("name"))
    if req.operation in ("UPDATE", "DELETE"):
        names.append(old_spec.get("name"))

    targets: List[str] = []
    seen = set()
    display = ""
    for name in names:
        if not isinstance(name, str) or not name:
            continue
        if not display:
            display = name
        for role in assign.target_group_roles(snapshots, name):
            if role not in seen:
                seen.add(role)
                targets.append(role)

    if not targets:
        return None
    rng = assign.actor_range(actor, catalog)
    leftover = assign.can_assign(actor, targets, catalog)
    if leftover is None:
        return None
    return assign.deny_message("groups.deckhouse.io", ".spec.name", display, leftover, rng)


def validate_car(req, actor: List[str], catalog: dict) -> Optional[str]:
    obj = req.oldObject if req.operation == "DELETE" else req.object
    spec = _spec(obj)
    targets = assign.car_target_roles(spec)
    leftover = assign.can_assign(actor, targets, catalog)
    if leftover is None:
        return None
    rng = assign.actor_range(actor, catalog)
    return assign.deny_car_message(_meta_name(obj) or "obj", leftover, rng)


def validate_clusterrole(req) -> Optional[str]:
    if not assign.can_assign_labels_changed(req.oldObject, req.object):
        return None
    return assign.deny_label_message(_meta_name(req.object) or "obj")


def validate_useroperation(req, snapshots, actor: List[str], catalog: dict) -> Optional[str]:
    username = (assign._dict(req.userInfo).get("username")) or ""
    if username == assign.USER_API_SA:
        return None

    spec = _spec(req.object)
    user_name = spec.get("user") if isinstance(spec.get("user"), str) else ""
    target = assign._dict(spec.get("target"))
    email = target.get("email") if isinstance(target.get("email"), str) else ""
    extra = assign.membership_groups(snapshots, user_name=user_name, email=email)
    rec = assign.user_record(snapshots, name=user_name, email=email)
    if rec and not email:
        rec_email = rec.get("email")
        if isinstance(rec_email, str):
            email = rec_email

    targets = assign.target_user_roles(snapshots, email, extra) if email else []
    if not email and extra:
        for group in extra:
            for role in assign.target_group_roles(snapshots, group):
                if role not in targets:
                    targets.append(role)
    if not targets:
        return None
    leftover = assign.can_assign(actor, targets, catalog)
    if leftover is None:
        return None
    rng = assign.actor_range(actor, catalog)
    display = email or user_name or _meta_name(req.object) or "obj"
    return assign.deny_uo_message(display, leftover, rng)


def validate_dexprovider(req, snapshots, actor: List[str], catalog: dict) -> Optional[str]:
    spec = _spec(req.object)
    targets = assign.dex_target_roles(spec, snapshots)
    leftover = assign.can_assign(actor, targets, catalog)
    if leftover is None:
        return None
    rng = assign.actor_range(actor, catalog)
    return assign.deny_dex_message(_meta_name(req.object) or "obj", leftover, rng)


if __name__ == "__main__":
    hook.run(main, config=CONFIG)
