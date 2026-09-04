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
import shutil
import subprocess
import unittest

import identity_assign as assign
import identity_privilege
import yaml
from deckhouse import hook, tests
from dotmap import DotMap
from identity_assign_test import STAR, STAR_ALL, CAR_EDIT, USERS_EDIT


HELPDESK = "eve@corp"
CLUSTER_ADMIN = "alice@corp"
SUPERADMIN = "root@corp"
PRIVILEGED_EMAIL = "root@corp"
PRIVILEGED_GROUP = "superadmins"
SECURITY = "sec@corp"


def clusterrole(name, rules, labels=None):
    return {"filterResult": {
        "name": name,
        "rules": rules,
        "accessLevel": "",
        "labels": labels or {},
    }}


def default_croles():
    return [
        clusterrole("user-authz:user", USERS_EDIT),
        clusterrole("user-authz:admin", USERS_EDIT + CAR_EDIT),
        clusterrole("user-authz:cluster-admin", STAR,
                    {"can-assign-basic-max": "ClusterAdmin"}),
        clusterrole("user-authz:super-admin", STAR_ALL, {
            "can-assign-basic-max": "SuperAdmin",
            "can-assign-scope": "system",
            "can-assign-max-level": "superadmin",
        }),
        clusterrole("cluster-admin", STAR_ALL, {
            "can-assign-basic-max": "SuperAdmin",
            "can-assign-max-level": "superadmin",
        }),
        clusterrole("d8:manage:security:manager", CAR_EDIT + USERS_EDIT, {
            "can-assign-basic-max": "ClusterAdmin",
            "can-assign-scope": "subsystem",
            "can-assign-subsystem": "security",
            "can-assign-max-level": "admin",
        }),
        clusterrole("d8:manage:all:manager", STAR_ALL, {
            "can-assign-basic-max": "ClusterAdmin",
            "can-assign-scope": "system",
            "can-assign-max-level": "admin",
        }),
        clusterrole("d8:manage:permission:module:user-authz:edit", CAR_EDIT, {
            "can-assign-basic-max": "ClusterAdmin",
        }),
        clusterrole("d8:manage:permission:module:user-authn:edit", USERS_EDIT),
        clusterrole("d8:subsystem:security:admin", CAR_EDIT),
        clusterrole("d8:subsystem:security:superadmin", STAR_ALL),
        clusterrole("d8:system:superadmin", STAR_ALL),
        clusterrole("pwn", []),
        clusterrole("d8:custom:evil", []),
    ]


def snapshots(privileged_email=PRIVILEGED_EMAIL, privileged_group=PRIVILEGED_GROUP,
              cluster_admin=CLUSTER_ADMIN):
    return {
        assign.CAR_SNAP: [
            {"filterResult": {
                "name": "super",
                "accessLevel": "SuperAdmin",
                "additionalRoles": [],
                "userSubjects": [privileged_email],
                "groupSubjects": [privileged_group],
                "saSubjects": [],
            }},
            {"filterResult": {
                "name": "cadmins",
                "accessLevel": "ClusterAdmin",
                "additionalRoles": [],
                "userSubjects": [cluster_admin, HELPDESK],
                "groupSubjects": [],
                "saSubjects": [],
            }},
        ],
        assign.AR_SNAP: [],
        assign.CRB_SNAP: [],
        assign.CROLE_SNAP: default_croles(),
        assign.USER_SNAP: [],
        assign.GROUP_SNAP: [],
    }


def ctx(kind, operation, spec, old_spec=None, username=HELPDESK, groups=None,
        extra_snaps=None, labels=None, old_labels=None, api_group="deckhouse.io"):
    obj = None if spec is None and kind != "ClusterRole" else {
        "apiVersion": "v1",
        "kind": kind,
        "metadata": {"name": "obj", "labels": labels or {}},
        "spec": spec or {},
    }
    if kind == "ClusterRole":
        obj = {
            "apiVersion": "rbac.authorization.k8s.io/v1",
            "kind": "ClusterRole",
            "metadata": {"name": (spec or {}).get("name", "obj"), "labels": labels or {}},
            "rules": (spec or {}).get("rules") or [],
        }
        old = None
        if old_spec is not None or old_labels is not None:
            old = {
                "apiVersion": "rbac.authorization.k8s.io/v1",
                "kind": "ClusterRole",
                "metadata": {"name": (old_spec or spec or {}).get("name", "obj"),
                             "labels": old_labels or {}},
                "rules": (old_spec or {}).get("rules") or [],
            }
    else:
        old = None if old_spec is None else {
            "apiVersion": "deckhouse.io/v1",
            "kind": kind,
            "metadata": {"name": "obj"},
            "spec": old_spec,
        }
    snaps = snapshots()
    if extra_snaps:
        for k, v in extra_snaps.items():
            snaps.setdefault(k, []).extend(v)
    return DotMap({
        "binding": "d8-user-authz-identity-assign.deckhouse.io",
        "review": {
            "request": {
                "uid": "00000000-0000-0000-0000-000000000001",
                "kind": {"group": api_group, "version": "v1", "kind": kind},
                "operation": operation,
                "userInfo": {"username": username, "groups": groups or []},
                "object": obj,
                "oldObject": old,
            }
        },
        "snapshots": snaps,
        "type": "Validating",
    })


def isolated_helpdesk_ctx(*args, **kwargs):
    kwargs.setdefault("username", HELPDESK)
    context = ctx(*args, **kwargs)
    context.snapshots[assign.CAR_SNAP][1].filterResult.userSubjects = [CLUSTER_ADMIN]
    return context


class TestIdentityAssignHook(unittest.TestCase):
    def run_hook(self, context):
        return hook.testrun(identity_privilege.main, [context])

    def test_helpdesk_cannot_create_user_for_superadmin_email(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "CREATE", {"email": PRIVILEGED_EMAIL, "password": "x"}))
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_helpdesk_cannot_delete_superadmin_user(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "DELETE", None, old_spec={"email": PRIVILEGED_EMAIL}))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_helpdesk_can_create_ordinary_user(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "CREATE", {"email": "dev@corp", "password": "x"}))
        tests.assert_validation_allowed(self, out, None)

    def test_clusteradmin_cannot_recreate_superadmin_user(self):
        out = self.run_hook(ctx(
            "User", "CREATE", {"email": PRIVILEGED_EMAIL}, username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_clusteradmin_can_manage_another_clusteradmin_user(self):
        out = self.run_hook(ctx(
            "User", "DELETE", None, old_spec={"email": CLUSTER_ADMIN}, username=CLUSTER_ADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_superadmin_can_delete_superadmin_user(self):
        out = self.run_hook(ctx(
            "User", "DELETE", None, old_spec={"email": PRIVILEGED_EMAIL}, username=SUPERADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_annotation_does_not_bypass_can_assign(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "CREATE", {"email": PRIVILEGED_EMAIL}))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_clusteradmin_cannot_write_superadmin_car(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "SuperAdmin",
             "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_clusteradmin_can_write_clusteradmin_car(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "ClusterAdmin",
             "subjects": [{"kind": "User", "name": "peer@corp"}]},
            username=CLUSTER_ADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_clusteradmin_cannot_attach_cluster_admin_additional_role(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "User",
             "additionalRoles": [{"apiGroup": "rbac.authorization.k8s.io",
                                  "kind": "ClusterRole", "name": "cluster-admin"}],
             "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_deckhouse_sa_is_allowed(self):
        out = self.run_hook(ctx(
            "User", "DELETE", None, old_spec={"email": PRIVILEGED_EMAIL},
            username="system:serviceaccount:d8-system:deckhouse"))
        tests.assert_validation_allowed(self, out, None)

    def test_system_masters_is_allowed(self):
        out = self.run_hook(ctx(
            "User", "CREATE", {"email": PRIVILEGED_EMAIL},
            username="kubernetes-admin", groups=["system:masters"]))
        tests.assert_validation_allowed(self, out, None)

    def test_apiserver_is_allowed(self):
        out = self.run_hook(ctx(
            "User", "DELETE", None, old_spec={"email": PRIVILEGED_EMAIL},
            username="system:apiserver"))
        tests.assert_validation_allowed(self, out, None)

    def test_authn_only_can_create_ordinary_user(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "CREATE", {"email": "dev@corp", "password": "x"}))
        tests.assert_validation_allowed(self, out, None)

    def test_authn_only_cannot_create_user_whose_email_has_user_level_car(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "CREATE", {"email": "granted-user@corp"},
            extra_snaps={assign.CAR_SNAP: [{"filterResult": {
                "name": "plain",
                "accessLevel": "User",
                "additionalRoles": [],
                "userSubjects": ["granted-user@corp"],
                "groupSubjects": [],
                "saSubjects": [],
            }}]}))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_group_car_does_not_protect_user_without_membership(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "DELETE", None, old_spec={"email": "admin@deckhouse.io"}))
        tests.assert_validation_allowed(self, out, None)

    def test_group_membership_protects_user_without_spec_groups(self):
        extra = {
            assign.USER_SNAP: [{"filterResult": {
                "name": "admin", "email": "admin@deckhouse.io", "groups": [],
            }}],
            assign.GROUP_SNAP: [{"filterResult": {
                "name": PRIVILEGED_GROUP, "members": ["admin"],
            }}],
        }
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "DELETE", None, old_spec={"email": "admin@deckhouse.io"},
            extra_snaps=extra))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_nested_group_membership_protects_user_delete(self):
        extra = {
            assign.USER_SNAP: [{"filterResult": {
                "name": "admin", "email": "admin@deckhouse.io", "groups": [],
            }}],
            assign.GROUP_SNAP: [
                {"filterResult": {
                    "name": "inner",
                    "members": [{"kind": "User", "name": "admin"}],
                }},
                {"filterResult": {
                    "name": PRIVILEGED_GROUP,
                    "members": [{"kind": "Group", "name": "inner"}],
                }},
            ],
        }
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "DELETE", None, old_spec={"email": "admin@deckhouse.io"},
            extra_snaps=extra))
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_user_spec_groups_superadmin_is_denied(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "CREATE",
            {"email": "admin@deckhouse.io", "groups": [PRIVILEGED_GROUP]}))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_clusteradmin_cannot_write_all_manager_additional_role(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "User",
             "additionalRoles": [{"name": "d8:manage:all:manager"}],
             "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_clusteradmin_can_write_user_authz_edit_additional_role(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "User",
             "additionalRoles": [{"name": "d8:manage:permission:module:user-authz:edit"}],
             "subjects": [{"kind": "User", "name": "peer@corp"}]},
            username=CLUSTER_ADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_clusteradmin_cannot_write_unknown_additional_role(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "User",
             "additionalRoles": [{"name": "cluster-write-all"}],
             "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_clusteradmin_cannot_delete_superadmin_car(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "DELETE", None,
            old_spec={"accessLevel": "SuperAdmin",
                      "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_clusteradmin_cannot_downgrade_superadmin_car(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "UPDATE",
            {"accessLevel": "User",
             "subjects": [{"kind": "User", "name": "eve@corp"}]},
            old_spec={"accessLevel": "SuperAdmin",
                      "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_clusteradmin_cannot_repoint_superadmin_car(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "UPDATE",
            {"accessLevel": "SuperAdmin",
             "subjects": [{"kind": "User", "name": "other@corp"}]},
            old_spec={"accessLevel": "SuperAdmin",
                      "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_clusteradmin_can_delete_clusteradmin_car(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "DELETE", None,
            old_spec={"accessLevel": "ClusterAdmin",
                      "subjects": [{"kind": "User", "name": "peer@corp"}]},
            username=CLUSTER_ADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_superadmin_can_delete_superadmin_car(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "DELETE", None,
            old_spec={"accessLevel": "SuperAdmin",
                      "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username=SUPERADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_security_manager_cannot_delete_superadmin_car(self):
        extra = {assign.CRB_SNAP: [{"filterResult": {
            "name": "sec",
            "role": "d8:manage:security:manager",
            "userSubjects": [SECURITY],
            "groupSubjects": [],
            "saSubjects": [],
        }}]}
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "DELETE", None,
            old_spec={"accessLevel": "SuperAdmin",
                      "subjects": [{"kind": "User", "name": "peer@corp"}]},
            username=SECURITY, extra_snaps=extra))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_security_manager_can_delete_clusteradmin_car(self):
        extra = {assign.CRB_SNAP: [{"filterResult": {
            "name": "sec",
            "role": "d8:manage:security:manager",
            "userSubjects": [SECURITY],
            "groupSubjects": [],
            "saSubjects": [],
        }}]}
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "DELETE", None,
            old_spec={"accessLevel": "ClusterAdmin",
                      "subjects": [{"kind": "User", "name": "peer@corp"}]},
            username=SECURITY, extra_snaps=extra))
        tests.assert_validation_allowed(self, out, None)

    def test_deckhouse_sa_can_delete_superadmin_car(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "DELETE", None,
            old_spec={"accessLevel": "SuperAdmin",
                      "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username="system:serviceaccount:d8-system:deckhouse"))
        tests.assert_validation_allowed(self, out, None)

    def test_clusteradmin_cannot_delete_car_with_cluster_admin_additional_role(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "DELETE", None,
            old_spec={"accessLevel": "User",
                      "additionalRoles": [{"name": "cluster-admin"}],
                      "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_superadmin_can_write_superadmin_car(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "SuperAdmin",
             "subjects": [{"kind": "User", "name": "peer@corp"}]},
            username=SUPERADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_superadmin_group_member_can_write_superadmin_car(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "SuperAdmin",
             "subjects": [{"kind": "Group", "name": "more-admins"}]},
            username="someone@corp", groups=[PRIVILEGED_GROUP]))
        tests.assert_validation_allowed(self, out, None)

    def test_admin_ar_cannot_write_clusteradmin_car(self):
        extra = {assign.AR_SNAP: [{"filterResult": {
            "name": "ns-admin",
            "namespace": "app",
            "accessLevel": "Admin",
            "additionalRoles": [],
            "userSubjects": [HELPDESK],
            "groupSubjects": [],
            "saSubjects": [],
        }}]}
        out = self.run_hook(isolated_helpdesk_ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "ClusterAdmin",
             "subjects": [{"kind": "User", "name": "x@corp"}]},
            extra_snaps=extra))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_admin_ar_cannot_write_admin_car(self):
        extra = {assign.AR_SNAP: [{"filterResult": {
            "name": "ns-admin",
            "namespace": "app",
            "accessLevel": "Admin",
            "additionalRoles": [],
            "userSubjects": [HELPDESK],
            "groupSubjects": [],
            "saSubjects": [],
        }}]}
        out = self.run_hook(isolated_helpdesk_ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "Admin",
             "subjects": [{"kind": "User", "name": "x@corp"}]},
            extra_snaps=extra))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_emptied_superadmin_role_does_not_admit_superadmin_car(self):
        extra = {
            assign.CRB_SNAP: [{"filterResult": {
                "name": "sec",
                "role": "d8:manage:security:manager",
                "userSubjects": [SECURITY],
                "groupSubjects": [],
                "saSubjects": [],
            }}],
            assign.CROLE_SNAP: [clusterrole("user-authz:super-admin", [], {
                "can-assign-basic-max": "SuperAdmin",
                "can-assign-scope": "system",
                "can-assign-max-level": "superadmin",
            })],
        }
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "SuperAdmin",
             "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username=SECURITY, extra_snaps=extra))
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_crb_security_manager_can_write_clusteradmin_car(self):
        extra = {assign.CRB_SNAP: [{"filterResult": {
            "name": "sec",
            "role": "d8:manage:security:manager",
            "userSubjects": [SECURITY],
            "groupSubjects": [],
            "saSubjects": [],
        }}]}
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "ClusterAdmin",
             "subjects": [{"kind": "User", "name": "peer@corp"}]},
            username=SECURITY, extra_snaps=extra))
        tests.assert_validation_allowed(self, out, None)

    def test_crb_security_manager_cannot_write_superadmin_car(self):
        extra = {assign.CRB_SNAP: [{"filterResult": {
            "name": "sec",
            "role": "d8:manage:security:manager",
            "userSubjects": [SECURITY],
            "groupSubjects": [],
            "saSubjects": [],
        }}]}
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "SuperAdmin",
             "subjects": [{"kind": "User", "name": "peer@corp"}]},
            username=SECURITY, extra_snaps=extra))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_group_occupied_name_denied_for_helpdesk(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "Group", "CREATE", {"name": PRIVILEGED_GROUP, "members": []}))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_group_ordinary_name_allowed(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "Group", "CREATE", {"name": "devs", "members": []}))
        tests.assert_validation_allowed(self, out, None)

    def test_group_member_change_on_superadmin_group_denied(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "Group", "UPDATE",
            {"name": PRIVILEGED_GROUP, "members": [{"kind": "User", "name": HELPDESK}]},
            old_spec={"name": PRIVILEGED_GROUP, "members": []}))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_forged_custom_range_labels_are_ignored(self):
        extra = {
            assign.CRB_SNAP: [{"filterResult": {
                "name": "pwn",
                "role": "pwn",
                "userSubjects": [HELPDESK],
                "groupSubjects": [],
                "saSubjects": [],
            }}],
            assign.CROLE_SNAP: [clusterrole("pwn", [], {
                "can-assign-max-level": "superadmin",
                "can-assign-basic-max": "SuperAdmin",
            })],
        }
        out = self.run_hook(isolated_helpdesk_ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "SuperAdmin",
             "subjects": [{"kind": "User", "name": "x@corp"}]},
            extra_snaps=extra))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_clusteradmin_cannot_relabel_own_role_to_superadmin(self):
        out = self.run_hook(ctx(
            "ClusterRole", "UPDATE",
            {"name": "user-authz:cluster-admin", "rules": STAR},
            old_spec={"name": "user-authz:cluster-admin", "rules": STAR},
            username=CLUSTER_ADMIN,
            labels={"user-authz.deckhouse.io/can-assign-basic-max": "SuperAdmin"},
            old_labels={"user-authz.deckhouse.io/can-assign-basic-max": "ClusterAdmin"},
            api_group="rbac.authorization.k8s.io"))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_deckhouse_can_write_can_assign_labels(self):
        out = self.run_hook(ctx(
            "ClusterRole", "UPDATE",
            {"name": "d8:manage:security:manager", "rules": []},
            old_spec={"name": "d8:manage:security:manager", "rules": []},
            username="system:serviceaccount:d8-system:deckhouse",
            labels={"user-authz.deckhouse.io/can-assign-basic-max": "ClusterAdmin"},
            old_labels={},
            api_group="rbac.authorization.k8s.io"))
        tests.assert_validation_allowed(self, out, None)

    def test_helpdesk_cannot_reset_superadmin_via_useroperation(self):
        extra = {
            assign.USER_SNAP: [{"filterResult": {
                "name": "root", "email": PRIVILEGED_EMAIL, "groups": [],
            }}],
        }
        out = self.run_hook(isolated_helpdesk_ctx(
            "UserOperation", "CREATE",
            {"user": "root", "type": "ResetPassword", "initiatorType": "admin"},
            extra_snaps=extra))
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_helpdesk_initiator_self_does_not_bypass_useroperation(self):
        extra = {
            assign.USER_SNAP: [{"filterResult": {
                "name": "root", "email": PRIVILEGED_EMAIL, "groups": [],
            }}],
        }
        out = self.run_hook(isolated_helpdesk_ctx(
            "UserOperation", "CREATE",
            {"user": "root", "type": "ResetPassword", "initiatorType": "self"},
            extra_snaps=extra))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_helpdesk_can_reset_ordinary_user(self):
        extra = {
            assign.USER_SNAP: [{"filterResult": {
                "name": "dev", "email": "dev@corp", "groups": [],
            }}],
        }
        out = self.run_hook(isolated_helpdesk_ctx(
            "UserOperation", "CREATE",
            {"user": "dev", "type": "ResetPassword"},
            extra_snaps=extra))
        tests.assert_validation_allowed(self, out, None)

    def test_useroperation_target_email_superadmin_denied(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "UserOperation", "CREATE",
            {"type": "Lock", "target": {"connectorID": "ldap", "email": PRIVILEGED_EMAIL}}))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_useroperation_membership_superadmin_denied(self):
        extra = {
            assign.USER_SNAP: [{"filterResult": {
                "name": "admin", "email": "admin@deckhouse.io", "groups": [],
            }}],
            assign.GROUP_SNAP: [{"filterResult": {
                "name": PRIVILEGED_GROUP, "members": ["admin"],
            }}],
        }
        out = self.run_hook(isolated_helpdesk_ctx(
            "UserOperation", "CREATE",
            {"user": "admin", "type": "ResetPassword"},
            extra_snaps=extra))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_user_api_sa_can_reset_superadmin(self):
        extra = {
            assign.USER_SNAP: [{"filterResult": {
                "name": "root", "email": PRIVILEGED_EMAIL, "groups": [],
            }}],
        }
        out = self.run_hook(ctx(
            "UserOperation", "CREATE",
            {"user": "root", "type": "ResetPassword", "initiatorType": "self"},
            username=assign.USER_API_SA, extra_snaps=extra))
        tests.assert_validation_allowed(self, out, None)

    def test_superadmin_can_reset_superadmin_via_useroperation(self):
        extra = {
            assign.USER_SNAP: [{"filterResult": {
                "name": "root", "email": PRIVILEGED_EMAIL, "groups": [],
            }}],
        }
        out = self.run_hook(ctx(
            "UserOperation", "CREATE",
            {"user": "root", "type": "ResetPassword"},
            username=SUPERADMIN, extra_snaps=extra))
        tests.assert_validation_allowed(self, out, None)

    def test_clusteradmin_cannot_create_open_oidc_dexprovider(self):
        out = self.run_hook(ctx(
            "DexProvider", "CREATE",
            {"type": "OIDC", "displayName": "corp", "oidc": {"issuer": "https://idp"}},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_helpdesk_cannot_create_open_oidc_dexprovider(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "DexProvider", "CREATE",
            {"type": "OIDC", "displayName": "corp", "oidc": {"allowedGroups": ["devs"]}}))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_saml_filtered_ordinary_group_denied_when_superadmin_email_exists(self):
        out = self.run_hook(ctx(
            "DexProvider", "CREATE",
            {"type": "SAML", "displayName": "corp",
             "saml": {"filterGroups": True, "allowedGroups": ["devs"]}},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_saml_filtered_ordinary_group_allowed_if_superadmin_is_group_only(self):
        context = ctx(
            "DexProvider", "CREATE",
            {"type": "SAML", "displayName": "corp",
             "saml": {"filterGroups": True, "allowedGroups": ["devs"]}},
            username=CLUSTER_ADMIN)
        context.snapshots[assign.CAR_SNAP][0].filterResult.userSubjects = []
        out = self.run_hook(context)
        tests.assert_validation_allowed(self, out, None)

    def test_saml_filtered_superadmin_group_denied(self):
        out = self.run_hook(ctx(
            "DexProvider", "CREATE",
            {"type": "SAML", "displayName": "corp",
             "saml": {"filterGroups": True, "allowedGroups": [PRIVILEGED_GROUP]}},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_superadmin_can_create_open_oidc_dexprovider(self):
        out = self.run_hook(ctx(
            "DexProvider", "CREATE",
            {"type": "OIDC", "displayName": "corp"},
            username=SUPERADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_github_closed_teams_denied_when_superadmin_email_exists(self):
        out = self.run_hook(ctx(
            "DexProvider", "CREATE",
            {"type": "Github", "displayName": "gh",
             "github": {"orgs": [{"name": "acme", "teams": ["devs"]}]}},
            username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_clusteradmin_can_rename_closed_saml_without_new_targets(self):
        spec = {"type": "SAML", "displayName": "corp-2",
                "saml": {"filterGroups": True, "allowedGroups": ["devs"],
                         "ssoURL": "https://idp/sso"}}
        old = {"type": "SAML", "displayName": "corp",
               "saml": {"filterGroups": True, "allowedGroups": ["devs"],
                        "ssoURL": "https://idp/sso"}}
        out = self.run_hook(ctx(
            "DexProvider", "UPDATE", spec, old_spec=old, username=CLUSTER_ADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_clusteradmin_can_rotate_open_oidc_secret(self):
        spec = {"type": "OIDC", "displayName": "corp",
                "oidc": {"issuer": "https://idp", "clientID": "a", "clientSecret": "new"}}
        old = {"type": "OIDC", "displayName": "corp",
               "oidc": {"issuer": "https://idp", "clientID": "a", "clientSecret": "old"}}
        out = self.run_hook(ctx(
            "DexProvider", "UPDATE", spec, old_spec=old, username=CLUSTER_ADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_clusteradmin_cannot_repoint_open_oidc_issuer(self):
        spec = {"type": "OIDC", "displayName": "corp",
                "oidc": {"issuer": "https://evil", "clientID": "a", "clientSecret": "x"}}
        old = {"type": "OIDC", "displayName": "corp",
               "oidc": {"issuer": "https://idp", "clientID": "a", "clientSecret": "x"}}
        out = self.run_hook(ctx(
            "DexProvider", "UPDATE", spec, old_spec=old, username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_clusteradmin_cannot_repoint_closed_saml_sso(self):
        spec = {"type": "SAML", "displayName": "corp",
                "saml": {"filterGroups": True, "allowedGroups": ["devs"],
                         "ssoURL": "https://evil/sso"}}
        old = {"type": "SAML", "displayName": "corp",
               "saml": {"filterGroups": True, "allowedGroups": ["devs"],
                        "ssoURL": "https://idp/sso"}}
        out = self.run_hook(ctx(
            "DexProvider", "UPDATE", spec, old_spec=old, username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_clusteradmin_cannot_replace_closed_saml_ca(self):
        spec = {"type": "SAML", "displayName": "corp",
                "saml": {"filterGroups": True, "allowedGroups": ["devs"],
                         "ssoURL": "https://idp/sso", "rootCAData": "new"}}
        old = {"type": "SAML", "displayName": "corp",
               "saml": {"filterGroups": True, "allowedGroups": ["devs"],
                        "ssoURL": "https://idp/sso", "rootCAData": "old"}}
        out = self.run_hook(ctx(
            "DexProvider", "UPDATE", spec, old_spec=old, username=CLUSTER_ADMIN))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_superadmin_can_repoint_open_oidc_issuer(self):
        spec = {"type": "OIDC", "displayName": "corp",
                "oidc": {"issuer": "https://evil"}}
        old = {"type": "OIDC", "displayName": "corp",
               "oidc": {"issuer": "https://idp"}}
        out = self.run_hook(ctx(
            "DexProvider", "UPDATE", spec, old_spec=old, username=SUPERADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_clusteradmin_cannot_open_closed_saml(self):
        context = ctx(
            "DexProvider", "UPDATE",
            {"type": "OIDC", "displayName": "corp", "oidc": {"issuer": "https://idp"}},
            old_spec={"type": "SAML", "displayName": "corp",
                      "saml": {"filterGroups": True, "allowedGroups": ["devs"]}},
            username=CLUSTER_ADMIN)
        context.snapshots[assign.CAR_SNAP][0].filterResult.userSubjects = []
        out = self.run_hook(context)
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertIn("user-authz:super-admin", out.validations.data[0]["message"])

    def test_clusterrole_unrelated_label_change_allowed(self):
        out = self.run_hook(ctx(
            "ClusterRole", "UPDATE",
            {"name": "custom", "rules": []},
            old_spec={"name": "custom", "rules": []},
            username=CLUSTER_ADMIN,
            labels={"foo": "bar"},
            old_labels={"foo": "baz"},
            api_group="rbac.authorization.k8s.io"))
        tests.assert_validation_allowed(self, out, None)


class TestIdentityAssignConfigContract(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.config = yaml.safe_load(identity_privilege.CONFIG)
        cls.validating = cls.config["kubernetesValidating"][0]

    def test_wave2_resources(self):
        resources = []
        for rule in self.validating["rules"]:
            resources.extend(rule["resources"])
        self.assertEqual(sorted(set(resources)), [
            "clusterauthorizationrules", "clusterroles", "dexproviders",
            "groups", "useroperations", "users",
        ])

    def test_useroperations_are_create_only(self):
        ops = []
        for rule in self.validating["rules"]:
            if "useroperations" in rule["resources"]:
                ops.extend(rule["operations"])
        self.assertEqual(sorted(set(ops)), ["CREATE"])

    def test_car_operations_include_delete(self):
        car_ops = []
        for rule in self.validating["rules"]:
            if "clusterauthorizationrules" in rule["resources"]:
                car_ops.extend(rule["operations"])
        self.assertEqual(sorted(set(car_ops)), ["CREATE", "DELETE", "UPDATE"])

    def test_snapshots_include_crb_and_clusterroles(self):
        kinds = [b["kind"] for b in self.config["kubernetes"]]
        self.assertEqual(kinds, [
            "ClusterAuthorizationRule", "AuthorizationRule",
            "ClusterRoleBinding", "ClusterRole", "User", "Group",
        ])


@unittest.skipUnless(shutil.which("jq"), "jq is required to execute the hook's jqFilter programs")
class TestAssignSnapshotJQFilters(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        config = yaml.safe_load(identity_privilege.CONFIG)
        cls.filters = {b["name"]: b["jqFilter"] for b in config["kubernetes"]}

    def run_filter(self, snapshot_name, obj):
        result = subprocess.run(
            ["jq", "-c", self.filters[snapshot_name]],
            input=json.dumps(obj), capture_output=True, text=True, check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def test_car_filter_keeps_level_roles_and_subjects(self):
        out = self.run_filter(assign.CAR_SNAP, {
            "metadata": {"name": "admin-rule"},
            "spec": {
                "accessLevel": "SuperAdmin",
                "additionalRoles": [{"name": "cluster-admin"}],
                "subjects": [
                    {"kind": "Group", "name": "g"},
                    {"kind": "ServiceAccount", "name": "sa", "namespace": "ns"},
                    {"kind": "User", "name": "u@x"},
                ],
            },
        })
        self.assertEqual(out["additionalRoles"], ["cluster-admin"])
        self.assertEqual(out["saSubjects"], ["ns:sa"])

    def test_crb_filter(self):
        out = self.run_filter(assign.CRB_SNAP, {
            "metadata": {"name": "b"},
            "roleRef": {"name": "d8:manage:security:manager"},
            "subjects": [{"kind": "User", "name": "sec@corp"}],
        })
        self.assertEqual(out["role"], "d8:manage:security:manager")
        self.assertEqual(out["userSubjects"], ["sec@corp"])

    def test_clusterrole_filter_keeps_can_assign_labels(self):
        out = self.run_filter(assign.CROLE_SNAP, {
            "metadata": {
                "name": "d8:manage:security:manager",
                "labels": {
                    "user-authz.deckhouse.io/can-assign-basic-max": "ClusterAdmin",
                    "user-authz.deckhouse.io/can-assign-max-level": "admin",
                },
            },
            "rules": CAR_EDIT,
        })
        self.assertEqual(out["labels"]["can-assign-basic-max"], "ClusterAdmin")
        self.assertEqual(out["rules"], CAR_EDIT)

    def test_user_filter(self):
        out = self.run_filter(assign.USER_SNAP, {
            "metadata": {"name": "admin"},
            "spec": {"email": "admin@deckhouse.io", "groups": ["legacy"]},
        })
        self.assertEqual(out["email"], "admin@deckhouse.io")
        self.assertEqual(out["groups"], ["legacy"])

    def test_group_filter(self):
        out = self.run_filter(assign.GROUP_SNAP, {
            "metadata": {"name": "g1"},
            "spec": {"name": "superadmins", "members": [
                {"kind": "User", "name": "admin"},
                {"kind": "Group", "name": "other"},
            ]},
        })
        self.assertEqual(out["name"], "superadmins")
        self.assertEqual(out["members"], [
            {"kind": "User", "name": "admin"},
            {"kind": "Group", "name": "other"},
        ])


if __name__ == "__main__":
    unittest.main()
