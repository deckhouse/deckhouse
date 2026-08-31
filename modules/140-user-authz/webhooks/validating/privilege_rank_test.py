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

import unittest

import privilege_rank as rank


def snap(snap_name, **fr):
    return {snap_name: [{"filterResult": fr}]}


def car(user=None, group=None, level="SuperAdmin", roles=None, sa=None):
    return snap(rank.CAR_SNAP,
                name="rule",
                accessLevel=level,
                additionalRoles=roles or [],
                userSubjects=[user] if user else [],
                groupSubjects=[group] if group else [],
                saSubjects=[sa] if sa else [])


def merge(*snaps):
    out = {}
    for s in snaps:
        for k, v in s.items():
            out.setdefault(k, []).extend(v)
    return out


class TestPrivilegeRank(unittest.TestCase):
    def test_access_level_order(self):
        self.assertLess(rank.ACCESS_LEVEL_RANK["ClusterAdmin"], rank.SUPER_RANK)
        self.assertEqual(rank.access_level_rank("SuperAdmin"), rank.SUPER_RANK)
        self.assertEqual(rank.access_level_rank("Admin", cap=rank.ADMIN_RANK), rank.ADMIN_RANK)

    def test_role_rank_keys_are_the_documented_set(self):
        self.assertEqual(frozenset(rank.ROLE_RANK), frozenset({
            "cluster-admin",
            "user-authz:super-admin",
            "d8:manage:all:manager",
            "d8:manage:security:manager",
            "user-authz:cluster-admin",
            "d8:manage:permission:module:user-authz:edit",
        }))

    def test_unknown_role_inflates_target_only(self):
        self.assertEqual(rank.role_rank("cluster-write-all", for_target=True), rank.SUPER_RANK)
        self.assertEqual(rank.role_rank("d8:manage:permission:module:user-authn:edit",
                                        for_target=False), 0)

    def test_actor_system_masters_is_super(self):
        self.assertEqual(
            rank.actor_rank({"username": "kube", "groups": ["system:masters"]}, {}),
            rank.SUPER_RANK)

    def test_actor_from_car_access_level(self):
        snapshots = merge(car(user="alice@corp", level="ClusterAdmin"), snap(rank.AR_SNAP))
        self.assertEqual(
            rank.actor_rank({"username": "alice@corp", "groups": []}, snapshots),
            rank.CLUSTER_ADMIN_RANK)

    def test_actor_user_authn_edit_additional_role_does_not_inflate(self):
        snapshots = merge(
            car(user="eve@corp", level="User",
                roles=["d8:manage:permission:module:user-authn:edit"]),
            snap(rank.AR_SNAP))
        self.assertEqual(
            rank.actor_rank({"username": "eve@corp", "groups": []}, snapshots),
            rank.ACCESS_LEVEL_RANK["User"])

    def test_target_email_lowercased_against_lowercase_subject(self):
        snapshots = merge(car(user="admin@corp", level="SuperAdmin"),
                          snap(rank.AR_SNAP))
        self.assertEqual(rank.target_user_rank(snapshots, "Admin@Corp"), rank.SUPER_RANK)

    def test_target_ignores_mixed_case_subject(self):
        snapshots = merge(car(user="Admin@Corp", level="SuperAdmin"),
                          snap(rank.AR_SNAP))
        self.assertEqual(rank.target_user_rank(snapshots, "admin@corp"), 0)

    def test_target_unknown_additional_role_is_super(self):
        snapshots = merge(car(user="joe@corp", level="User", roles=["cluster-write-all"]),
                          snap(rank.AR_SNAP))
        self.assertEqual(rank.target_user_rank(snapshots, "joe@corp"), rank.SUPER_RANK)

    def test_car_granted_rank_additional_roles(self):
        self.assertEqual(
            rank.car_granted_rank({"accessLevel": "ClusterAdmin",
                                   "additionalRoles": [{"name": "cluster-admin"}]}),
            rank.SUPER_RANK)

    def test_escalate_denied_only_when_target_higher(self):
        self.assertTrue(rank.escalate_denied(rank.CLUSTER_ADMIN_RANK, rank.SUPER_RANK))
        self.assertFalse(rank.escalate_denied(rank.CLUSTER_ADMIN_RANK, rank.CLUSTER_ADMIN_RANK))
        self.assertFalse(rank.escalate_denied(rank.SUPER_RANK, rank.CLUSTER_ADMIN_RANK))

if __name__ == "__main__":
    unittest.main()
