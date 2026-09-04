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


# This hook guards the authorization rules owned by this module against a local identity silently
# landing in one of them.
#
# A local identity name reaches the token Dex issues, and kube-apiserver maps it to a Kubernetes
# identity with an empty prefix, so there is no namespacing between locally managed identities and
# identities asserted by an external identity provider:
#
# - Group.spec.name becomes the group name in the "groups" claim, byte for byte.
# - User.spec.email becomes the username, lowercased on the way. The AuthorizationRule and
#   ClusterAuthorizationRule CRDs say so themselves: "Use the user's `email` as the username to
#   grant privileges to the specific user".
#
# Creating such an object with a name that an existing rule already lists as a subject therefore
# grants that rule's privileges to it, with nothing in either object recording that it happened.
#
# The check lives in this module rather than in user-authn on purpose. It needs the authorization
# rules, which this module owns, and it only needs the incoming object from the admission request.
# A hook in user-authn would have to open informers on the CRDs of this module, and a missing CRD
# makes informer creation fail (shell-operator, pkg/kube_events_manager/resource_informer.go), which
# would take the whole hook down in a cluster where user-authz is disabled. Here the dependency is
# the other way round and is harmless: the validating rules below name resources owned by
# user-authn, and if that module is disabled its CRDs are absent, the rules simply never match.

from typing import Callable, NamedTuple, Optional

from deckhouse import hook
from dotmap import DotMap

CLUSTER_RULES_SNAPSHOT_NAME = "d8-user-authz-collision-cluster-authorization-rules"
NAMESPACED_RULES_SNAPSHOT_NAME = "d8-user-authz-collision-authorization-rules"

CONFIG = f"""
configVersion: v1
kubernetesValidating:
- name: d8-user-authz-group-authorization-rule-collision.deckhouse.io
  includeSnapshotsFrom: ["{CLUSTER_RULES_SNAPSHOT_NAME}", "{NAMESPACED_RULES_SNAPSHOT_NAME}"]
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE", "DELETE"]
    resources:   ["groups"]
    scope:       "Cluster"
- name: d8-user-authz-user-authorization-rule-collision.deckhouse.io
  includeSnapshotsFrom: ["{CLUSTER_RULES_SNAPSHOT_NAME}", "{NAMESPACED_RULES_SNAPSHOT_NAME}"]
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE", "DELETE"]
    resources:   ["users"]
    scope:       "Cluster"
kubernetes:
- name: {CLUSTER_RULES_SNAPSHOT_NAME}
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterAuthorizationRule
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  jqFilter: |
    {{
      "name": .metadata.name,
      "groupSubjects": [.spec.subjects[]? | select(.kind == "Group") | .name],
      "userSubjects": [.spec.subjects[]? | select(.kind == "User") | .name]
    }}
- name: {NAMESPACED_RULES_SNAPSHOT_NAME}
  apiVersion: deckhouse.io/v1alpha1
  kind: AuthorizationRule
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  jqFilter: |
    {{
      "name": .metadata.name,
      "namespace": .metadata.namespace,
      "groupSubjects": [.spec.subjects[]? | select(.kind == "Group") | .name],
      "userSubjects": [.spec.subjects[]? | select(.kind == "User") | .name]
    }}
"""

# Acknowledges that a collision with an authorization rule is intentional. The prefix is
# user-authz because the constraint and its enforcement belong to this module: with user-authz
# disabled there are no rules and the annotation means nothing.
COLLISION_ANNOTATION = "user-authz.deckhouse.io/allow-authorization-rule-collision"


class IdentityKind(NamedTuple):
    """
    Everything this hook needs to know about one of the two validated kinds.

    Attributes:
        resource: the resource name used in the messages.
        spec_field: the name of the spec field holding the identity.
        subjects_key: the jqFilter output key holding the matching rule subjects.
        token_identity: maps the spec field value to the identity that actually reaches the
                        token, which is what an authorization rule subject is matched against.
    """
    resource: str
    spec_field: str
    subjects_key: str
    token_identity: Callable[[str], str]

    @property
    def field_path(self) -> str:
        return f".spec.{self.spec_field}"


# Group.spec.name reaches the "groups" claim byte for byte: nothing between the Group object and
# the Password object Dex serves normalises it (modules/150-user-authn/hooks/get_dex_user_crds.go,
# makeUserGroupsMap and newPasswordObject), so the comparison is exact in both directions.
#
# User.spec.email does not: Deckhouse lowercases it before it reaches the Password object
# (modules/150-user-authn/hooks/get_dex_user_crds.go:276), so the username in the token is
# unconditionally spec.email.lower().
IDENTITY_KINDS = {
    "group": IdentityKind(resource="groups.deckhouse.io", spec_field="name",
                          subjects_key="groupSubjects", token_identity=lambda name: name),
    "user": IdentityKind(resource="users.deckhouse.io", spec_field="email",
                         subjects_key="userSubjects", token_identity=str.lower),
}


def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        errmsg, warnings = validate(binding_context)
        if errmsg is None:
            ctx.output.validations.allow(*warnings)
        else:
            ctx.output.validations.deny(errmsg)
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap) -> tuple[Optional[str], list[str]]:
    req = ctx.review.request
    identity = IDENTITY_KINDS.get(req.kind.kind.lower())
    if identity is None:
        return None, []

    if req.operation == "DELETE":
        return warn_rule_outlives_identity(ctx, identity,
                                           spec_value(req.oldObject, identity.spec_field))

    return validate_identity(ctx, identity,
                             name=spec_value(req.object, identity.spec_field),
                             old_name=spec_value(req.oldObject, identity.spec_field))


def spec_value(obj: Optional[DotMap], field: str):
    """
    Read a spec field of an admission review object.

    `object` is null on DELETE and `oldObject` is null on CREATE, so both have to be treated as
    absent rather than dereferenced.
    """
    if not obj:
        return None

    return obj.spec[field]


def validate_identity(ctx: DotMap, identity: IdentityKind,
                      name, old_name) -> tuple[Optional[str], list[str]]:
    warnings = []

    if not isinstance(name, str) or not name:
        # The field is required by both CRDs, so an absent value is not something this hook has an
        # opinion about — the owning module's own schema and webhooks reject it.
        return None, warnings

    granting_rule = granting_rule_for(ctx, identity, name)
    if granting_rule is None:
        return None, warnings

    collision = (f"{identity.resource} \"{identity.field_path}\" \"{name}\" "
                 f"is already granted privileges by {granting_rule}")

    # An already existing collision is only reported: the privileges are effective anyway, and
    # denying updates would lock such an object out of any further modification. The comparison is
    # on the token identity, so rewriting an already colliding email in a different case does not
    # count as introducing it. An UPDATE whose oldObject is missing counts as new, so an unreadable
    # previous value fails closed.
    name_is_new = (ctx.review.request.operation == "CREATE"
                   or not isinstance(old_name, str)
                   or identity.token_identity(old_name) != identity.token_identity(name))
    if name_is_new and not is_collision_acknowledged(ctx.review.request.object):
        return (
            f"{collision}; use a different value or set the "
            f"\"{COLLISION_ANNOTATION}: true\" annotation to confirm the collision"
        ), warnings

    warnings.append(f"{collision}; it inherits the privileges granted by that rule")
    return None, warnings


def warn_rule_outlives_identity(ctx: DotMap, identity: IdentityKind,
                                name) -> tuple[Optional[str], list[str]]:
    """
    Report that deleting the object does not revoke the rule.

    The name stays granted, so recreating a Group or a User with the same name restores the
    privileges, and until then the name is free for anyone allowed to create one.
    """
    if not isinstance(name, str) or not name:
        return None, []

    granting_rule = granting_rule_for(ctx, identity, name)
    if granting_rule is None:
        return None, []

    return None, [f"{granting_rule} still grants privileges to "
                  f"\"{identity.field_path}\" \"{name}\""]


def granting_rule_for(ctx: DotMap, identity: IdentityKind, name: str) -> Optional[str]:
    """
    Find an authorization rule that already grants privileges to the given identity name.

    A rule subject is matched against the identity that ends up in the token, not against the spec
    field verbatim, and the two differ for User.spec.email. The asymmetry is deliberate and both
    directions matter:

    - A mixed-case *incoming* email is a real collision. The username the API server sees is
      unconditionally spec.email.lower(), so "Privileged@Example.com" inherits everything a rule
      grants to "privileged@example.com". Comparing verbatim would miss it and make the check
      bypassable by case alone.
    - A mixed-case *rule subject* is not a collision. Subjects reach the RoleBinding verbatim
      (modules/140-user-authz/templates/cluster-role-bindings.yaml:38,
      modules/140-user-authz/hooks/handle_manage_bindings.go:256) and RBAC matches them exactly, so
      a subject that is not already lowercase can never match an issued token, and reporting it
      would be a false positive.

    Group.spec.name has no such normalisation anywhere on its way to the "groups" claim, so both
    sides are compared verbatim for groups.

    Returns:
        Optional[str]: description of the rule granting the privileges, or None when there is none.
                       The first rule found for a name wins, so the reported rule is stable.
    """
    privileged = {}

    for rule in ctx.snapshots.get(CLUSTER_RULES_SNAPSHOT_NAME, []):
        rule_name = f"clusterauthorizationrules.deckhouse.io \"{rule.filterResult.name}\""
        for subject in rule.filterResult[identity.subjects_key]:
            privileged.setdefault(subject, rule_name)

    for rule in ctx.snapshots.get(NAMESPACED_RULES_SNAPSHOT_NAME, []):
        rule_name = (f"authorizationrules.deckhouse.io "
                     f"\"{rule.filterResult.namespace}/{rule.filterResult.name}\"")
        for subject in rule.filterResult[identity.subjects_key]:
            privileged.setdefault(subject, rule_name)

    return privileged.get(identity.token_identity(name))


def is_collision_acknowledged(obj: DotMap) -> bool:
    return obj.metadata.annotations.get(COLLISION_ANNOTATION, "").lower() == "true"


if __name__ == "__main__":
    hook.run(main, config=CONFIG)
