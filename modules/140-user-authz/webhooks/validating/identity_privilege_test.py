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
# distributed under the License as distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import json
import shutil
import subprocess
import unittest

import identity_privilege
import privilege_rank as rank
import yaml
from deckhouse import hook, tests
from dotmap import DotMap


HELPDESK = "eve@corp"
CLUSTER_ADMIN = "alice@corp"
SUPERADMIN = "root@corp"
PRIVILEGED_EMAIL = "root@corp"
PRIVILEGED_GROUP = "superadmins"


def snapshots(privileged_email=PRIVILEGED_EMAIL, privileged_group=PRIVILEGED_GROUP,
              cluster_admin=CLUSTER_ADMIN):
    return {
        rank.CAR_SNAP: [
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
        rank.AR_SNAP: [],
        rank.CRB_SNAP: [],
    }


def ctx(kind, operation, spec, old_spec=None, username=HELPDESK, groups=None,
        extra_snaps=None):
    obj = None if spec is None else {
        "apiVersion": "deckhouse.io/v1",
        "kind": kind,
        "metadata": {"name": "obj"},
        "spec": spec,
    }
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
    # Helpdesk default: only the ClusterAdmin CAR lists eve, so actor rank is ClusterAdmin
    # unless we override snapshots. Isolated helpdesk is username not in any high grant.
    return DotMap({
        "binding": "d8-user-authz-identity-privilege.deckhouse.io",
        "review": {
            "request": {
                "uid": "00000000-0000-0000-0000-000000000001",
                "kind": {"group": "deckhouse.io", "version": "v1", "kind": kind},
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
    """eve is not a subject of the ClusterAdmin CAR."""
    kwargs.setdefault("username", HELPDESK)
    context = ctx(*args, **kwargs)
    context.snapshots[rank.CAR_SNAP][1].filterResult.userSubjects = [CLUSTER_ADMIN]
    return context


class TestIdentityPrivilege(unittest.TestCase):
    def run_hook(self, context):
        return hook.testrun(identity_privilege.main, [context])

    def test_helpdesk_cannot_create_user_for_superadmin_email(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "CREATE", {"email": PRIVILEGED_EMAIL, "password": "x"}))
        tests.assert_validation_deny(self, out, (
            f'users.deckhouse.io ".spec.email" "{PRIVILEGED_EMAIL}" already carries SuperAdmin '
            f"privileges; the requester holds none and cannot create, update, or delete that User"))

    def test_helpdesk_cannot_delete_superadmin_user(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "DELETE", None, old_spec={"email": PRIVILEGED_EMAIL}))
        tests.assert_validation_deny(self, out, (
            f'users.deckhouse.io ".spec.email" "{PRIVILEGED_EMAIL}" already carries SuperAdmin '
            f"privileges; the requester holds none and cannot create, update, or delete that User"))

    def test_helpdesk_can_create_ordinary_user(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "CREATE", {"email": "dev@corp", "password": "x"}))
        tests.assert_validation_allowed(self, out, None)

    def test_clusteradmin_cannot_recreate_superadmin_user(self):
        out = self.run_hook(ctx(
            "User", "CREATE", {"email": PRIVILEGED_EMAIL}, username=CLUSTER_ADMIN))
        tests.assert_validation_deny(self, out, (
            f'users.deckhouse.io ".spec.email" "{PRIVILEGED_EMAIL}" already carries SuperAdmin '
            f"privileges; the requester holds ClusterAdmin and cannot create, update, or delete "
            f"that User"))

    def test_clusteradmin_can_manage_another_clusteradmin_user(self):
        out = self.run_hook(ctx(
            "User", "DELETE", None, old_spec={"email": CLUSTER_ADMIN}, username=CLUSTER_ADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_superadmin_can_delete_superadmin_user(self):
        out = self.run_hook(ctx(
            "User", "DELETE", None, old_spec={"email": PRIVILEGED_EMAIL}, username=SUPERADMIN))
        tests.assert_validation_allowed(self, out, None)

    def test_annotation_does_not_bypass_privilege_check(self):
        out = self.run_hook(isolated_helpdesk_ctx(
            "User", "CREATE",
            {"email": PRIVILEGED_EMAIL},
        ))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_clusteradmin_cannot_write_superadmin_car(self):
        out = self.run_hook(ctx(
            "ClusterAuthorizationRule", "CREATE",
            {"accessLevel": "SuperAdmin",
             "subjects": [{"kind": "User", "name": "eve@corp"}]},
            username=CLUSTER_ADMIN))
        tests.assert_validation_deny(self, out, (
            'clusterauthorizationrules.deckhouse.io "obj" grants SuperAdmin, '
            "which is higher than the requester's ClusterAdmin"))

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
        tests.assert_validation_deny(self, out, (
            'clusterauthorizationrules.deckhouse.io "obj" grants SuperAdmin, '
            "which is higher than the requester's ClusterAdmin"))

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


@unittest.skipUnless(shutil.which("jq"), "jq is required to execute the hook's jqFilter programs")
class TestPrivilegeSnapshotJQFilters(unittest.TestCase):
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
        out = self.run_filter(rank.CAR_SNAP, {
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
        self.assertEqual(out, {
            "name": "admin-rule",
            "accessLevel": "SuperAdmin",
            "additionalRoles": ["cluster-admin"],
            "groupSubjects": ["g"],
            "userSubjects": ["u@x"],
            "saSubjects": ["ns:sa"],
        })

    def test_filters_tolerate_missing_subjects(self):
        out = self.run_filter(rank.CAR_SNAP, {
            "metadata": {"name": "empty"},
            "spec": {"accessLevel": "User"},
        })
        self.assertEqual(out["userSubjects"], [])
        self.assertEqual(out["additionalRoles"], [])


if __name__ == "__main__":
    unittest.main()
