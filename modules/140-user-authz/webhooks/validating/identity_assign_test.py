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

import identity_assign as assign


STAR = [{"apiGroups": ["*"], "resources": ["*"], "verbs": ["*"]}]
STAR_ALL = STAR + [{"nonResourceURLs": ["*"], "verbs": ["*"]}]
CAR_EDIT = [{"apiGroups": ["deckhouse.io"], "resources": ["clusterauthorizationrules"],
             "verbs": ["create", "update", "patch", "delete"]}]
USERS_EDIT = [{"apiGroups": ["deckhouse.io"], "resources": ["users", "groups"],
               "verbs": ["create", "update", "patch", "delete"]}]


def entry(name, rules=None, labels=None, access_level=""):
    return assign.CatalogEntry(name=name, rules=rules or [], labels=labels or {},
                               access_level=access_level)


def catalog(*entries):
    return {e.name: e for e in entries}


SECURITY_LABELS = {
    "can-assign-basic-max": "ClusterAdmin",
    "can-assign-scope": "subsystem",
    "can-assign-subsystem": "security",
    "can-assign-max-level": "admin",
}

CLUSTER_ADMIN_LABELS = {"can-assign-basic-max": "ClusterAdmin"}
SUPER_LABELS = {
    "can-assign-basic-max": "SuperAdmin",
    "can-assign-scope": "system",
    "can-assign-max-level": "superadmin",
}


def default_catalog():
    return catalog(
        entry("user-authz:user", rules=USERS_EDIT),
        entry("user-authz:admin", rules=USERS_EDIT + CAR_EDIT),
        entry("user-authz:cluster-admin", rules=STAR, labels=CLUSTER_ADMIN_LABELS),
        entry("user-authz:super-admin", rules=STAR_ALL, labels=SUPER_LABELS),
        entry("cluster-admin", rules=STAR_ALL, labels=SUPER_LABELS),
        entry("d8:manage:security:manager", rules=CAR_EDIT + USERS_EDIT, labels=SECURITY_LABELS),
        entry("d8:manage:all:manager", rules=STAR_ALL, labels={
            "can-assign-basic-max": "ClusterAdmin",
            "can-assign-scope": "system",
            "can-assign-max-level": "admin",
        }),
        entry("d8:manage:permission:module:user-authz:edit", rules=CAR_EDIT,
              labels={"can-assign-basic-max": "ClusterAdmin"}),
        entry("d8:manage:permission:module:user-authn:edit", rules=USERS_EDIT),
        entry("d8:subsystem:security:admin", rules=CAR_EDIT),
        entry("d8:subsystem:security:superadmin", rules=STAR_ALL),
        entry("d8:system:superadmin", rules=STAR_ALL),
        entry("d8:namespace:admin", rules=USERS_EDIT),
    )


class TestCovers(unittest.TestCase):
    def test_star_covers_specific(self):
        self.assertTrue(assign.covers(STAR, CAR_EDIT))

    def test_specific_does_not_cover_star(self):
        self.assertFalse(assign.covers(CAR_EDIT, STAR))

    def test_cluster_admin_does_not_cover_superadmin_nonresource(self):
        self.assertFalse(assign.covers(STAR, STAR_ALL))

    def test_empty_target_is_covered(self):
        self.assertTrue(assign.covers(CAR_EDIT, []))

    def test_resource_names_all_not_covered_by_named(self):
        owner = [{"apiGroups": ["deckhouse.io"], "resources": ["moduleconfigs"],
                  "resourceNames": ["user-authz"], "verbs": ["update"]}]
        servant = [{"apiGroups": ["deckhouse.io"], "resources": ["moduleconfigs"],
                    "verbs": ["update"]}]
        self.assertFalse(assign.covers(owner, servant))

    def test_nonresource_prefix(self):
        owner = [{"nonResourceURLs": ["/healthz*"], "verbs": ["get"]}]
        servant = [{"nonResourceURLs": ["/healthz/foo"], "verbs": ["get"]}]
        self.assertTrue(assign.covers(owner, servant))


class TestDescribeAndRange(unittest.TestCase):
    def test_describe_main_and_rbacv2_names(self):
        self.assertEqual(assign.describe_role("d8:manage:security:manager").scope, "subsystem")
        self.assertEqual(assign.describe_role("d8:manage:security:manager").subsystem, "security")
        self.assertEqual(assign.describe_role("d8:manage:all:manager").scope, "system")
        self.assertEqual(assign.describe_role("d8:subsystem:security:admin").level, "admin")
        self.assertEqual(assign.describe_role("d8:system:superadmin").level, "superadmin")
        self.assertIsNone(assign.describe_role("d8:manage:permission:module:user-authz:edit"))

    def test_custom_labels_are_ignored(self):
        cat = default_catalog()
        cat["pwn"] = entry("pwn", labels={"can-assign-max-level": "superadmin"})
        leftover = assign.can_assign(["pwn"], ["user-authz:super-admin"], cat)
        self.assertEqual(leftover, ["user-authz:super-admin"])

    def test_d8_custom_labels_are_ignored(self):
        cat = default_catalog()
        cat["d8:custom:evil"] = entry(
            "d8:custom:evil",
            labels={"can-assign-basic-max": "SuperAdmin", "can-assign-max-level": "superadmin"},
        )
        leftover = assign.can_assign(["d8:custom:evil"], ["user-authz:super-admin"], cat)
        self.assertEqual(leftover, ["user-authz:super-admin"])

    def test_security_assigns_clusteradmin_not_superadmin(self):
        cat = default_catalog()
        actor = ["d8:manage:security:manager"]
        self.assertIsNone(assign.can_assign(actor, ["user-authz:cluster-admin"], cat))
        self.assertEqual(assign.can_assign(actor, ["user-authz:super-admin"], cat),
                         ["user-authz:super-admin"])
        self.assertIsNone(assign.can_assign(actor, ["d8:subsystem:security:admin"], cat))
        self.assertEqual(assign.can_assign(actor, ["d8:subsystem:security:superadmin"], cat),
                         ["d8:subsystem:security:superadmin"])
        self.assertEqual(assign.can_assign(actor, ["d8:system:superadmin"], cat),
                         ["d8:system:superadmin"])

    def test_empty_target_allows(self):
        self.assertIsNone(assign.can_assign([], [], default_catalog()))

    def test_emptied_superadmin_rules_are_not_covered(self):
        cat = default_catalog()
        cat["user-authz:super-admin"] = entry(
            "user-authz:super-admin", rules=[], labels=SUPER_LABELS)
        leftover = assign.can_assign(
            ["d8:manage:security:manager"], ["user-authz:super-admin"], cat)
        self.assertEqual(leftover, ["user-authz:super-admin"])

    def test_emptied_cluster_admin_rules_are_not_covered(self):
        cat = default_catalog()
        cat["cluster-admin"] = entry("cluster-admin", rules=[], labels=SUPER_LABELS)
        leftover = assign.can_assign(
            ["d8:manage:security:manager"], ["cluster-admin"], cat)
        self.assertEqual(leftover, ["cluster-admin"])

    def test_rewritten_superadmin_rules_are_not_covered(self):
        shrunk = [{"apiGroups": ["rbac.authorization.k8s.io"],
                   "resources": ["clusterroles"], "verbs": ["update"]}]
        cat = default_catalog()
        cat["d8:manage:security:manager"] = entry(
            "d8:manage:security:manager",
            rules=CAR_EDIT + USERS_EDIT + shrunk,
            labels=SECURITY_LABELS,
        )
        cat["user-authz:super-admin"] = entry(
            "user-authz:super-admin", rules=shrunk, labels=SUPER_LABELS)
        leftover = assign.can_assign(
            ["d8:manage:security:manager"], ["user-authz:super-admin"], cat)
        self.assertEqual(leftover, ["user-authz:super-admin"])

    def test_rewritten_cluster_admin_rules_are_not_covered(self):
        shrunk = [{"apiGroups": ["rbac.authorization.k8s.io"],
                   "resources": ["clusterroles"], "verbs": ["update"]}]
        cat = default_catalog()
        cat["d8:manage:security:manager"] = entry(
            "d8:manage:security:manager",
            rules=CAR_EDIT + USERS_EDIT + shrunk,
            labels=SECURITY_LABELS,
        )
        cat["cluster-admin"] = entry("cluster-admin", rules=shrunk, labels=SUPER_LABELS)
        leftover = assign.can_assign(
            ["d8:manage:security:manager"], ["cluster-admin"], cat)
        self.assertEqual(leftover, ["cluster-admin"])

    def test_missing_catalog_role_is_not_assigned_by_range(self):
        leftover = assign.can_assign(
            ["d8:manage:security:manager"],
            ["d8:manage:security:user"],
            default_catalog())
        self.assertEqual(leftover, ["d8:manage:security:user"])

    def test_security_assigns_present_security_user_by_range(self):
        cat = default_catalog()
        cat["d8:manage:security:user"] = entry("d8:manage:security:user", rules=USERS_EDIT)
        self.assertIsNone(assign.can_assign(
            ["d8:manage:security:manager"], ["d8:manage:security:user"], cat))

    def test_access_level_annotation_ignored_on_custom(self):
        cat = default_catalog()
        cat["pwn"] = entry("pwn", rules=STAR_ALL, access_level="User")
        leftover = assign.can_assign(
            ["d8:manage:security:manager"], ["pwn"], cat)
        self.assertEqual(leftover, ["pwn"])

    def test_unknown_additional_role_fail_closed(self):
        leftover = assign.can_assign(
            ["user-authz:cluster-admin"], ["cluster-write-all"], default_catalog())
        self.assertEqual(leftover, ["cluster-write-all"])

    def test_superadmin_cover_allows_disaster(self):
        self.assertIsNone(assign.can_assign(
            ["user-authz:super-admin"], ["cluster-admin"], default_catalog()))

    def test_authn_edit_does_not_assign_occupied(self):
        leftover = assign.can_assign(
            ["d8:manage:permission:module:user-authn:edit"],
            ["user-authz:super-admin"], default_catalog())
        self.assertEqual(leftover, ["user-authz:super-admin"])

    def test_platform_owned_predicate(self):
        self.assertTrue(assign.is_platform_range_role("d8:manage:security:manager"))
        self.assertTrue(assign.is_platform_range_role("user-authz:cluster-admin"))
        self.assertTrue(assign.is_platform_range_role("cluster-admin"))
        self.assertFalse(assign.is_platform_range_role("d8:custom:x"))
        self.assertFalse(assign.is_platform_range_role("pwn"))


class TestIdentityCollection(unittest.TestCase):
    def test_actor_from_crb_only(self):
        snaps = {
            assign.CAR_SNAP: [],
            assign.AR_SNAP: [],
            assign.CRB_SNAP: [{"filterResult": {
                "name": "sec",
                "role": "d8:manage:security:manager",
                "userSubjects": ["sec@corp"],
                "groupSubjects": [],
                "saSubjects": [],
            }}],
            assign.CROLE_SNAP: [],
        }
        self.assertEqual(
            assign.actor_roles({"username": "sec@corp", "groups": []}, snaps),
            ["d8:manage:security:manager"])

    def test_actor_roles_ignore_authorization_rules(self):
        snaps = {
            assign.CAR_SNAP: [],
            assign.AR_SNAP: [{"filterResult": {
                "name": "ns-admin",
                "namespace": "app",
                "accessLevel": "Admin",
                "additionalRoles": ["cluster-admin"],
                "userSubjects": ["eve@corp"],
                "groupSubjects": [],
                "saSubjects": [],
            }}],
            assign.CRB_SNAP: [],
            assign.CROLE_SNAP: [],
        }
        self.assertEqual(assign.actor_roles({"username": "eve@corp", "groups": []}, snaps), [])

    def test_target_user_includes_spec_groups(self):
        snaps = {
            assign.CAR_SNAP: [{"filterResult": {
                "name": "g",
                "accessLevel": "SuperAdmin",
                "additionalRoles": [],
                "userSubjects": [],
                "groupSubjects": ["superadmins"],
                "saSubjects": [],
            }}],
            assign.AR_SNAP: [],
            assign.CRB_SNAP: [],
            assign.CROLE_SNAP: [],
        }
        self.assertEqual(
            assign.target_user_roles(snaps, "admin@deckhouse.io", ["superadmins"]),
            ["user-authz:super-admin"])
        self.assertEqual(assign.target_user_roles(snaps, "admin@deckhouse.io", []), [])

    def test_target_email_lowercase_against_lowercase_subject(self):
        snaps = {
            assign.CAR_SNAP: [{"filterResult": {
                "name": "u",
                "accessLevel": "SuperAdmin",
                "additionalRoles": [],
                "userSubjects": ["admin@corp"],
                "groupSubjects": [],
                "saSubjects": [],
            }}],
            assign.AR_SNAP: [],
            assign.CRB_SNAP: [],
            assign.CROLE_SNAP: [],
        }
        self.assertEqual(assign.target_user_roles(snaps, "Admin@Corp"), ["user-authz:super-admin"])

    def test_exempt_groups(self):
        self.assertTrue(assign.is_exempt({"username": "x", "groups": ["system:masters"]}))
        self.assertTrue(assign.is_exempt(
            {"username": "system:serviceaccount:kube-system:foo",
             "groups": ["system:serviceaccounts:kube-system"]}))
        self.assertFalse(assign.is_exempt({"username": "eve@corp", "groups": []}))

    def test_membership_from_group_snapshot(self):
        snaps = {
            assign.USER_SNAP: [{"filterResult": {
                "name": "admin", "email": "admin@deckhouse.io", "groups": [],
            }}],
            assign.GROUP_SNAP: [{"filterResult": {
                "name": "superadmins", "members": ["admin"],
            }}],
            assign.CAR_SNAP: [],
            assign.AR_SNAP: [],
            assign.CRB_SNAP: [],
        }
        self.assertEqual(assign.membership_groups(snaps, email="admin@deckhouse.io"),
                         ["superadmins"])

    def test_membership_walks_nested_groups(self):
        snaps = {
            assign.USER_SNAP: [{"filterResult": {
                "name": "admin", "email": "admin@deckhouse.io", "groups": [],
            }}],
            assign.GROUP_SNAP: [
                {"filterResult": {
                    "name": "inner",
                    "members": [{"kind": "User", "name": "admin"}],
                }},
                {"filterResult": {
                    "name": "superadmins",
                    "members": [{"kind": "Group", "name": "inner"}],
                }},
            ],
            assign.CAR_SNAP: [],
            assign.AR_SNAP: [],
            assign.CRB_SNAP: [],
        }
        self.assertEqual(assign.membership_groups(snaps, email="admin@deckhouse.io"),
                         ["inner", "superadmins"])

    def test_membership_nested_group_cycle_stops(self):
        snaps = {
            assign.GROUP_SNAP: [
                {"filterResult": {
                    "name": "a",
                    "members": [{"kind": "Group", "name": "b"},
                                {"kind": "User", "name": "admin"}],
                }},
                {"filterResult": {
                    "name": "b",
                    "members": [{"kind": "Group", "name": "a"}],
                }},
            ],
        }
        self.assertEqual(assign.groups_containing_user(snaps, "admin"), ["a", "b"])

    def test_occupied_roles_ignore_system_subjects(self):
        snaps = {
            assign.CAR_SNAP: [{"filterResult": {
                "name": "human",
                "accessLevel": "SuperAdmin",
                "additionalRoles": [],
                "userSubjects": [],
                "groupSubjects": ["superadmins"],
                "saSubjects": [],
            }}],
            assign.AR_SNAP: [],
            assign.CRB_SNAP: [{"filterResult": {
                "name": "k8s",
                "role": "cluster-admin",
                "userSubjects": [],
                "groupSubjects": ["system:masters"],
                "saSubjects": [],
            }}],
        }
        self.assertEqual(assign.occupied_grant_roles(snaps), ["user-authz:super-admin"])

    def test_oidc_is_not_claims_closed(self):
        self.assertFalse(assign.dex_claims_closed({"type": "OIDC", "oidc": {"allowedGroups": ["devs"]}}))

    def test_saml_filter_groups_is_closed(self):
        self.assertTrue(assign.dex_claims_closed({
            "type": "SAML", "saml": {"filterGroups": True, "allowedGroups": ["devs"]},
        }))
        self.assertFalse(assign.dex_claims_closed({
            "type": "SAML", "saml": {"allowedGroups": ["devs"]},
        }))

    def test_open_oidc_targets_occupied_roles(self):
        snaps = {
            assign.CAR_SNAP: [{"filterResult": {
                "name": "g",
                "accessLevel": "SuperAdmin",
                "additionalRoles": [],
                "userSubjects": [],
                "groupSubjects": ["superadmins"],
                "saSubjects": [],
            }}],
            assign.AR_SNAP: [],
            assign.CRB_SNAP: [],
        }
        self.assertEqual(
            assign.dex_target_roles({"type": "OIDC"}, snaps),
            ["user-authz:super-admin"])

    def test_closed_saml_ignores_unlisted_group_grants(self):
        snaps = {
            assign.CAR_SNAP: [{"filterResult": {
                "name": "g",
                "accessLevel": "SuperAdmin",
                "additionalRoles": [],
                "userSubjects": [],
                "groupSubjects": ["superadmins"],
                "saSubjects": [],
            }}],
            assign.AR_SNAP: [],
            assign.CRB_SNAP: [],
        }
        spec = {"type": "SAML", "saml": {"filterGroups": True, "allowedGroups": ["devs"]}}
        self.assertEqual(assign.dex_target_roles(spec, snaps), [])

    def test_closed_saml_includes_user_subject_grants(self):
        snaps = {
            assign.CAR_SNAP: [{"filterResult": {
                "name": "g",
                "accessLevel": "SuperAdmin",
                "additionalRoles": [],
                "userSubjects": ["root@corp"],
                "groupSubjects": ["superadmins"],
                "saSubjects": [],
            }}],
            assign.AR_SNAP: [],
            assign.CRB_SNAP: [],
        }
        spec = {"type": "SAML", "saml": {"filterGroups": True, "allowedGroups": ["devs"]}}
        self.assertEqual(assign.dex_target_roles(spec, snaps), ["user-authz:super-admin"])

    def test_dex_trust_anchor_ignores_secret_and_display_name(self):
        old = {"type": "OIDC", "displayName": "corp",
               "oidc": {"issuer": "https://idp", "clientID": "a", "clientSecret": "old"}}
        new = {"type": "OIDC", "displayName": "corp-2",
               "oidc": {"issuer": "https://idp", "clientID": "a", "clientSecret": "new"}}
        self.assertEqual(assign.dex_trust_anchor(old), assign.dex_trust_anchor(new))

    def test_dex_trust_anchor_changes_with_issuer(self):
        old = {"type": "OIDC", "oidc": {"issuer": "https://idp"}}
        new = {"type": "OIDC", "oidc": {"issuer": "https://evil"}}
        self.assertNotEqual(assign.dex_trust_anchor(old), assign.dex_trust_anchor(new))

    def test_user_record_name_does_not_fallback_to_email(self):
        snaps = {
            assign.USER_SNAP: [
                {"filterResult": {"name": "other", "email": "eve@corp", "groups": ["superadmins"]}},
                {"filterResult": {"name": "eve", "email": "eve-real@corp", "groups": []}},
            ],
        }
        rec = assign.user_record(snaps, name="eve", email="eve@corp")
        self.assertEqual(rec["name"], "eve")
        self.assertEqual(rec["email"], "eve-real@corp")
        self.assertIsNone(assign.user_record(snaps, name="missing", email="eve@corp"))
        self.assertEqual(assign.user_record(snaps, email="eve@corp")["name"], "other")


if __name__ == "__main__":
    unittest.main()
