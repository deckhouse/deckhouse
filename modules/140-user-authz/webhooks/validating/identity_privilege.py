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

import sys
from typing import Any, List, Optional

from dotmap import DotMap

# The ValidationWebhook renders identity_assign.py and this file into one hook, so the kernel's
# names are defined in this very module. `assign` keeps them under the prefix they have when the
# kernel is a module of its own.
assign = sys.modules[__name__]

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


def validation_error(ctx: DotMap) -> Optional[str]:
    req = ctx.review.request
    if assign.is_exempt(req.userInfo):
        return None

    kind = (req.kind.kind or "").lower()
    if kind == "clusterrole":
        return validate_clusterrole(req)

    catalog = assign.load_catalog(ctx.snapshots)
    actor = assign.actor_roles(req.userInfo, ctx.snapshots)
    full_access = assign.actor_has_full_access(req.userInfo, ctx.snapshots, catalog)

    if kind == "user":
        return validate_user(req, ctx.snapshots, actor, catalog, full_access)
    if kind == "group":
        return validate_group(req, ctx.snapshots, actor, catalog, full_access)
    if kind == "clusterauthorizationrule":
        return validate_car(req, ctx.snapshots, actor, catalog, full_access)
    if kind == "useroperation":
        return validate_useroperation(req, ctx.snapshots, actor, catalog, full_access)
    if kind == "dexprovider":
        return validate_dexprovider(req, ctx.snapshots, actor, catalog, full_access)
    if kind == "moduleconfig":
        return validate_moduleconfig(req, ctx.snapshots, actor, catalog, full_access)
    return None


def validate_user(req, snapshots, actor: List[str], catalog: dict,
                  full_access: bool) -> Optional[str]:
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
    rng = assign.actor_range(actor, catalog, full_access=full_access)
    leftover = assign.can_assign(actor, targets, catalog, full_access=full_access)
    if leftover is None:
        return None
    return assign.deny_message("users.deckhouse.io", ".spec.email", display_email, leftover, rng)


def validate_group(req, snapshots, actor: List[str], catalog: dict,
                   full_access: bool) -> Optional[str]:
    names = []
    new_spec = _spec(req.object)
    old_spec = _spec(req.oldObject)
    if req.operation != "DELETE":
        names.append(new_spec.get("name"))
    if req.operation in ("UPDATE", "DELETE"):
        names.append(old_spec.get("name"))

    # A member gets the roles of the group and of every group above it.
    names = [name for name in names if isinstance(name, str) and name]
    display = names[0] if names else ""
    targets = assign.target_groups_roles(snapshots, assign._ancestor_groups(assign._group_by_name(snapshots), names))

    if not targets:
        return None
    rng = assign.actor_range(actor, catalog, full_access=full_access)
    leftover = assign.can_assign(actor, targets, catalog, full_access=full_access)
    if leftover is not None:
        return assign.deny_message("groups.deckhouse.io", ".spec.name", display, leftover, rng)

    # A new member already let into the system namespaces by another rule takes the rules of the group
    # there too, which is a ClusterAdmin assignment, as for a ClusterAuthorizationRule (validate_car). An
    # actor who may assign ClusterAdmin skips the walk.
    # A rename gives every member the rules of the new name, as an addition would.
    group_name = new_spec.get("name")
    added = set(assign._member_refs(new_spec))
    if req.operation == "UPDATE" and old_spec.get("name") == group_name:
        added -= set(assign._member_refs(old_spec))
    if req.operation == "DELETE" or not isinstance(group_name, str) or not group_name or not added:
        return None
    system_role = assign.BASIC_LEVEL_ROLE[assign.SYSTEM_NAMESPACES_LEVEL]
    if assign.can_assign(actor, [system_role], catalog, full_access=full_access) is None:
        return None
    if assign.group_members_reach_system_namespaces(snapshots, group_name, sorted(added)):
        return assign.deny_group_system_namespaces_message(display, rng)
    return None


def validate_car(req, snapshots, actor: List[str], catalog: dict, full_access: bool) -> Optional[str]:
    new_spec = _spec(req.object)
    old_spec = _spec(req.oldObject)
    if req.operation == "DELETE":
        specs = [old_spec]
        obj = req.oldObject
    elif req.operation == "UPDATE":
        specs = [old_spec, new_spec]
        obj = req.object
    else:
        specs = [new_spec]
        obj = req.object
    targets = []
    for spec in specs:
        for name in assign.car_target_roles(spec):
            if name not in targets:
                targets.append(name)
    leftover = assign.can_assign(actor, targets, catalog, full_access=full_access)
    if leftover is not None:
        rng = assign.actor_range(actor, catalog, full_access=full_access)
        return assign.deny_car_message(_meta_name(obj) or "obj", leftover, rng)
    # Opening the system namespaces is a ClusterAdmin assignment, whatever the access level of the rule:
    # by its own fields, or because a request it applies to, the requester's own among them, is already
    # let into them by another rule, which the authorization webhook joins with this one. An actor who may
    # assign ClusterAdmin skips the walk.
    system_role = assign.BASIC_LEVEL_ROLE[assign.SYSTEM_NAMESPACES_LEVEL]
    if assign.can_assign(actor, [system_role], catalog, full_access=full_access) is None:
        return None
    if any(assign.car_reaches_system_namespaces(spec, snapshots)
           or assign.car_shares_subjects_with_a_reaching_rule(spec, snapshots, req.userInfo)
           for spec in specs):
        rng = assign.actor_range(actor, catalog, full_access=full_access)
        return assign.deny_car_system_namespaces_message(_meta_name(obj) or "obj", rng)
    return None


def validate_moduleconfig(req, snapshots, actor: List[str], catalog: dict,
                          full_access: bool) -> Optional[str]:
    """Turning multi-tenancy off opens the system namespaces to every ClusterAuthorizationRule."""
    obj = req.oldObject if req.operation == "DELETE" else req.object
    if _meta_name(obj) != "user-authz":
        return None
    old_settings = _spec(req.oldObject).get("settings")
    new_settings = _spec(req.object).get("settings")
    if not assign.turns_multitenancy_off(req.operation, old_settings, new_settings, snapshots):
        return None
    system_role = assign.BASIC_LEVEL_ROLE[assign.SYSTEM_NAMESPACES_LEVEL]
    if assign.can_assign(actor, [system_role], catalog, full_access=full_access) is None:
        return None
    rng = assign.actor_range(actor, catalog, full_access=full_access)
    return assign.deny_multitenancy_off_message(rng)


def validate_clusterrole(req) -> Optional[str]:
    if assign.can_assign_labels_changed(req.oldObject, req.object):
        return assign.deny_label_message(_meta_name(req.object) or "obj")
    if assign.claims_platform_ownership(req.object):
        return assign.deny_heritage_message(_meta_name(req.object) or "obj")
    return None


def validate_useroperation(req, snapshots, actor: List[str], catalog: dict,
                           full_access: bool) -> Optional[str]:
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
    leftover = assign.can_assign(actor, targets, catalog, full_access=full_access)
    if leftover is None:
        return None
    rng = assign.actor_range(actor, catalog, full_access=full_access)
    display = email or user_name or _meta_name(req.object) or "obj"
    return assign.deny_uo_message(display, leftover, rng)


def validate_dexprovider(req, snapshots, actor: List[str], catalog: dict,
                         full_access: bool) -> Optional[str]:
    """Gate a provider by the identity space it can assert.

    Targets are the roles already granted to identities inside that space
    (spec.allowedIdentities plus the connector's own group filters); an open
    axis reaches every grant on subjects of that kind. Credential rotation,
    display fields, enabling and narrowing the space are admitted without a
    check; any other change is measured as a fresh connection.
    """
    new_spec = _spec(req.object)
    if req.operation == "UPDATE" and assign.dex_benign_update(_spec(req.oldObject), new_spec):
        return None
    targets = assign.dex_target_roles(new_spec, snapshots)
    if not targets:
        return None
    leftover = assign.can_assign(actor, targets, catalog, full_access=full_access)
    if leftover is None:
        return None
    rng = assign.actor_range(actor, catalog, full_access=full_access)
    return assign.deny_dex_message(_meta_name(req.object) or "obj", leftover, rng)


# The template calls validate(ctx) and expects a (message, allowed) pair. The checks above answer
# with a deny message or None.
def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    message = validation_error(ctx)
    if message:
        return message, False
    return None, True
