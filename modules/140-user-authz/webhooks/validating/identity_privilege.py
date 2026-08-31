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


# Privilege-equivalence admission for User objects and ClusterAuthorizationRule writes.
#
# Lives in user-authz: the grants are this module's objects. Validating rules name Users; if
# user-authn is disabled those rules never match.

from typing import Optional

from deckhouse import hook
from dotmap import DotMap

import privilege_rank as rank

MATCH_CONDITIONS = """
  matchConditions:
  - expression: ("system:apiserver" != request.userInfo.username)
    name: exclude-kube-apiserver
  - expression: ("system:serviceaccount:d8-system:deckhouse" != request.userInfo.username)
    name: exclude-deckhouse
  - expression: ("system:serviceaccount:kube-system:clusterrole-aggregation-controller" != request.userInfo.username)
    name: exclude-aggregation-controller
"""

CONFIG = f"""
configVersion: v1
kubernetesValidating:
- name: d8-user-authz-identity-privilege.deckhouse.io
  includeSnapshotsFrom: ["{rank.CAR_SNAP}", "{rank.AR_SNAP}", "{rank.CRB_SNAP}"]
{MATCH_CONDITIONS}
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE", "DELETE"]
    resources:   ["users"]
    scope:       "Cluster"
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["clusterauthorizationrules"]
    scope:       "Cluster"
kubernetes:
{rank.kubernetes_snapshots()}
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


def validate(ctx: DotMap) -> Optional[str]:
    req = ctx.review.request
    username = req.userInfo.username
    if username in rank.PRIVILEGED_USERS:
        return None

    actor = rank.actor_rank(req.userInfo, ctx.snapshots)
    kind = (req.kind.kind or "").lower()

    if kind == "user":
        return validate_user(req, ctx.snapshots, actor)
    if kind == "clusterauthorizationrule":
        return validate_car(req, actor)
    return None


def _spec_field(obj, field):
    if not obj:
        return None
    spec = obj.spec
    if not spec:
        return None
    return spec[field]


def validate_user(req, snapshots, actor: int) -> Optional[str]:
    if req.operation == "DELETE":
        email = _spec_field(req.oldObject, "email")
    else:
        email = _spec_field(req.object, "email")
        old_email = _spec_field(req.oldObject, "email")
        if isinstance(old_email, str) and isinstance(email, str):
            # Changing onto a more privileged email is the same as creating that identity.
            old_rank = rank.target_user_rank(snapshots, old_email)
            new_rank = rank.target_user_rank(snapshots, email)
            if rank.escalate_denied(actor, max(old_rank, new_rank)):
                return _identity_deny("users.deckhouse.io", ".spec.email", email, actor,
                                      max(old_rank, new_rank))
            return None

    if not isinstance(email, str) or not email:
        return None
    target = rank.target_user_rank(snapshots, email)
    if rank.escalate_denied(actor, target):
        return _identity_deny("users.deckhouse.io", ".spec.email", email, actor, target)
    return None


def validate_car(req, actor: int) -> Optional[str]:
    spec = req.object.spec if req.object else None
    granted = rank.car_granted_rank(spec)
    if not rank.escalate_denied(actor, granted):
        return None
    name = ""
    if req.object and req.object.metadata:
        name = req.object.metadata.name or ""
    return (
        f'clusterauthorizationrules.deckhouse.io "{name}" grants {rank.rank_label(granted)}, '
        f"which is higher than the requester's {rank.rank_label(actor)}"
    )


def _identity_deny(resource: str, field: str, value: str, actor: int, target: int) -> str:
    return (
        f'{resource} "{field}" "{value}" already carries {rank.rank_label(target)} privileges; '
        f"the requester holds {rank.rank_label(actor)} and cannot create, update, or delete that User"
    )


if __name__ == "__main__":
    hook.run(main, config=CONFIG)
