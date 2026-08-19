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

import json
import typing

from identity_collision import (
    CLUSTER_RULES_SNAPSHOT_NAME,
    NAMESPACED_RULES_SNAPSHOT_NAME,
    COLLISION_ANNOTATION,
)

# The rules every scenario is validated against. "privileged" identities are subjects of a
# ClusterAuthorizationRule, "team-*" ones of a namespaced AuthorizationRule.
CLUSTER_RULE_GROUP_SUBJECT = "privileged-group"
CLUSTER_RULE_USER_SUBJECT = "privileged@example.com"
NAMESPACED_RULE_GROUP_SUBJECT = "team-devs"
NAMESPACED_RULE_USER_SUBJECT = "team-lead@example.com"

CLUSTER_RULE_DESCRIPTION = 'clusterauthorizationrules.deckhouse.io "admin-rule"'
NAMESPACED_RULE_DESCRIPTION = 'authorizationrules.deckhouse.io "team-a/team-rule"'

# A subject an administrator wrote in mixed case. It reaches the RoleBinding verbatim and RBAC
# matches subjects exactly, so it can never match an issued token and must not be reported.
MIXED_CASE_RULE_USER_SUBJECT = "Legacy@Example.com"
MIXED_CASE_RULE_GROUP_SUBJECT = "Legacy-Group"


def _snapshots(with_rules: bool = True,
               cluster_rule_group_subjects: typing.Optional[list] = None,
               cluster_rule_user_subjects: typing.Optional[list] = None) -> dict:
    if not with_rules:
        return {CLUSTER_RULES_SNAPSHOT_NAME: [], NAMESPACED_RULES_SNAPSHOT_NAME: []}

    if cluster_rule_group_subjects is None:
        cluster_rule_group_subjects = [CLUSTER_RULE_GROUP_SUBJECT]
    if cluster_rule_user_subjects is None:
        cluster_rule_user_subjects = [CLUSTER_RULE_USER_SUBJECT]

    return {
        CLUSTER_RULES_SNAPSHOT_NAME: [
            {
                "filterResult": {
                    "name": "admin-rule",
                    "groupSubjects": cluster_rule_group_subjects,
                    "userSubjects": cluster_rule_user_subjects,
                }
            }
        ],
        NAMESPACED_RULES_SNAPSHOT_NAME: [
            {
                "filterResult": {
                    "name": "team-rule",
                    "namespace": "team-a",
                    "groupSubjects": [NAMESPACED_RULE_GROUP_SUBJECT],
                    "userSubjects": [NAMESPACED_RULE_USER_SUBJECT],
                }
            }
        ],
    }


def _binding_context(kind: str, resource: str, webhook: str, operation: str,
                     spec: typing.Optional[dict], old_spec: typing.Optional[dict],
                     acknowledged: bool, with_rules: bool,
                     cluster_rule_group_subjects: typing.Optional[list] = None,
                     cluster_rule_user_subjects: typing.Optional[list] = None) -> str:
    metadata = {"name": "test-object"}
    if acknowledged:
        metadata["annotations"] = {COLLISION_ANNOTATION: "true"}

    obj = None if spec is None else {
        "apiVersion": "deckhouse.io/v1",
        "kind": kind,
        "metadata": metadata,
        "spec": spec,
    }
    old_obj = None if old_spec is None else {
        "apiVersion": "deckhouse.io/v1",
        "kind": kind,
        "metadata": metadata,
        "spec": old_spec,
    }

    return json.dumps({
        "binding": webhook,
        "review": {
            "request": {
                "uid": "b4e3d1c0-0000-0000-0000-000000000000",
                "kind": {"group": "deckhouse.io", "version": "v1", "kind": kind},
                "resource": {"group": "deckhouse.io", "version": "v1", "resource": resource},
                "name": "test-object",
                "operation": operation,
                "userInfo": {"username": "kubernetes-admin", "groups": ["system:masters"]},
                "object": obj,
                "oldObject": old_obj,
                "dryRun": False,
            }
        },
        "snapshots": _snapshots(with_rules, cluster_rule_group_subjects,
                                cluster_rule_user_subjects),
        "type": "Validating",
    })


def prepare_group_binding_context(group_name: str, operation: str = "CREATE",
                                  old_group_name: typing.Optional[str] = None,
                                  acknowledged: bool = False,
                                  with_rules: bool = True,
                                  with_old_object: bool = True,
                                  cluster_rule_group_subjects: typing.Optional[list] = None) -> str:
    if operation == "DELETE":
        return _binding_context(
            kind="Group", resource="groups",
            webhook="d8-user-authz-group-authorization-rule-collision.deckhouse.io",
            operation=operation, spec=None, old_spec={"name": group_name, "members": []},
            acknowledged=acknowledged, with_rules=with_rules,
            cluster_rule_group_subjects=cluster_rule_group_subjects,
        )

    # with_old_object=False models an UPDATE whose oldObject the hook cannot read.
    old_spec = None if operation == "CREATE" or not with_old_object else {
        "name": old_group_name, "members": []}
    return _binding_context(
        kind="Group", resource="groups",
        webhook="d8-user-authz-group-authorization-rule-collision.deckhouse.io",
        operation=operation,
        spec={"name": group_name, "members": []},
        old_spec=old_spec,
        acknowledged=acknowledged, with_rules=with_rules,
        cluster_rule_group_subjects=cluster_rule_group_subjects,
    )


def prepare_user_binding_context(email: str, operation: str = "CREATE",
                                 old_email: typing.Optional[str] = None,
                                 acknowledged: bool = False,
                                 with_rules: bool = True,
                                 with_old_object: bool = True,
                                 cluster_rule_user_subjects: typing.Optional[list] = None) -> str:
    if operation == "DELETE":
        return _binding_context(
            kind="User", resource="users",
            webhook="d8-user-authz-user-authorization-rule-collision.deckhouse.io",
            operation=operation, spec=None, old_spec={"email": email},
            acknowledged=acknowledged, with_rules=with_rules,
            cluster_rule_user_subjects=cluster_rule_user_subjects,
        )

    old_spec = None if operation == "CREATE" or not with_old_object else {"email": old_email}
    return _binding_context(
        kind="User", resource="users",
        webhook="d8-user-authz-user-authorization-rule-collision.deckhouse.io",
        operation=operation, spec={"email": email}, old_spec=old_spec,
        acknowledged=acknowledged, with_rules=with_rules,
        cluster_rule_user_subjects=cluster_rule_user_subjects,
    )


def prepare_cluster_authorization_rule(subjects: typing.Optional[list]) -> dict:
    """
    A ClusterAuthorizationRule object as the API server stores it, to feed a jqFilter.

    subjects=None omits `spec.subjects` entirely, which is what the `[]?` guard in both filters
    exists for.
    """
    spec = {"accessLevel": "Editor"}
    if subjects is not None:
        spec["subjects"] = subjects

    return {
        "apiVersion": "deckhouse.io/v1alpha1",
        "kind": "ClusterAuthorizationRule",
        "metadata": {"name": "admin-rule"},
        "spec": spec,
    }


def prepare_authorization_rule(subjects: typing.Optional[list]) -> dict:
    """A namespaced AuthorizationRule object, to feed a jqFilter."""
    rule = prepare_cluster_authorization_rule(subjects)
    rule["kind"] = "AuthorizationRule"
    rule["metadata"] = {"name": "team-rule", "namespace": "team-a"}
    return rule


MIXED_KIND_SUBJECTS = [
    {"kind": "Group", "name": CLUSTER_RULE_GROUP_SUBJECT},
    {"kind": "ServiceAccount", "name": "builder", "namespace": "ci"},
    {"kind": "User", "name": CLUSTER_RULE_USER_SUBJECT},
]
