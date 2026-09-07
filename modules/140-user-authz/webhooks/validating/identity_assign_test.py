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


def entry(name, rules=None, labels=None, access_level="", heritage=True):
    labels = dict(labels or {})
    if heritage:
        labels.setdefault("heritage", "deckhouse")
    return assign.CatalogEntry(name=name, rules=rules or [], labels=labels,
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

    def test_range_ignores_forged_can_assign_labels_without_heritage(self):
        cat = default_catalog()
        cat["d8:manage:pwned:manager"] = entry(
            "d8:manage:pwned:manager", rules=[], labels=SUPER_LABELS, heritage=False)
        rng = assign.actor_range(["d8:manage:pwned:manager"], cat)
        self.assertIsNone(rng.basic_max)
        self.assertIsNone(rng.max_level)

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


class TestUnknownRoles(unittest.TestCase):
    def test_role_absent_from_catalog_is_leftover_below_superadmin(self):
        cat = default_catalog()
        self.assertEqual(assign.can_assign(["user-authz:cluster-admin"], ["gone:role"], cat),
                         ["gone:role"])
        self.assertEqual(assign.can_assign(["d8:manage:all:manager"], ["gone:role"], cat),
                         ["gone:role"])

    def test_superadmin_range_covers_a_role_absent_from_catalog(self):
        # A ClusterRoleBinding outliving its ClusterRole still names a human
        # subject; that must not lock SuperAdmin out of every open provider.
        cat = default_catalog()
        self.assertIsNone(assign.can_assign(["user-authz:super-admin"], ["gone:role"], cat))
        self.assertIsNone(assign.can_assign(["cluster-admin"], ["gone:role", "user-authz:super-admin"], cat))

    def test_forged_superadmin_range_does_not_cover_unknown_roles(self):
        cat = default_catalog()
        cat["d8:system:pwned"] = entry("d8:system:pwned", rules=[], labels=SUPER_LABELS, heritage=False)
        self.assertEqual(assign.can_assign(["d8:system:pwned"], ["gone:role"], cat), ["gone:role"])


# --- DexProvider: identity space -> targets (specs/002-dexprovider-identity-gate) ---

def car(name, level, users=(), groups=(), additional=()):
    return {"filterResult": {
        "name": name, "accessLevel": level, "additionalRoles": list(additional),
        "userSubjects": list(users), "groupSubjects": list(groups), "saSubjects": [],
    }}


def crb(name, role, users=(), groups=()):
    return {"filterResult": {
        "name": name, "role": role,
        "userSubjects": list(users), "groupSubjects": list(groups), "saSubjects": [],
    }}


def dex_snaps():
    return {
        assign.CAR_SNAP: [
            car("super", "SuperAdmin", users=["root@corp"], groups=["superadmins"]),
            car("cadmins", "ClusterAdmin", users=["alice@corp"]),
            car("boss", "Editor", users=["Boss@Contractor.Example"]),
            car("trap", "SuperAdmin", groups=["gate-trap"]),
            car("parent-super", "SuperAdmin", groups=["parent"]),
        ],
        assign.AR_SNAP: [{"filterResult": {
            "name": "ns-admin", "namespace": "app", "accessLevel": "SuperAdmin",
            "additionalRoles": [], "userSubjects": ["dev@contractor.example"],
            "groupSubjects": [], "saSubjects": [],
        }}],
        assign.CRB_SNAP: [
            crb("sa-binding", "user-authz:user", users=["system:serviceaccount:d8-system:x"]),
            crb("masters", "cluster-admin", groups=["system:masters"]),
            crb("devs-view", "d8:manage:security:viewer", groups=["devs"]),
        ],
        assign.CROLE_SNAP: [],
        assign.USER_SNAP: [],
        assign.GROUP_SNAP: [{"filterResult": {
            "name": "parent", "members": [{"kind": "Group", "name": "child"}],
        }}],
    }


def oidc(**extra):
    spec = {"type": "OIDC", "displayName": "corp",
            "oidc": {"issuer": "https://idp", "clientID": "a", "clientSecret": "s",
                     "scopes": ["openid", "email", "groups"]}}
    for k, v in extra.items():
        if k == "allowedIdentities":
            spec["allowedIdentities"] = v
        else:
            spec["oidc"][k] = v
    return spec


class TestDexIdentityFilters(unittest.TestCase):
    def test_email_axis_open_without_block(self):
        self.assertIsNone(assign.dex_email_filter(oidc()))

    def test_email_axis_open_with_empty_lists(self):
        self.assertIsNone(assign.dex_email_filter(
            oidc(allowedIdentities={"emails": [], "emailDomains": []})))

    def test_email_filter_lowercases_and_splits_axes(self):
        emails, domains = assign.dex_email_filter(oidc(allowedIdentities={
            "emails": ["Alice@Corp", " bob@corp "], "emailDomains": ["Contractor.Example"]}))
        self.assertEqual(emails, frozenset({"alice@corp", "bob@corp"}))
        self.assertEqual(domains, frozenset({"contractor.example"}))

    def test_email_filter_ignores_non_strings(self):
        emails, domains = assign.dex_email_filter(oidc(allowedIdentities={
            "emails": ["a@corp", 5, None, ""], "emailDomains": [{"x": 1}]}))
        self.assertEqual(emails, frozenset({"a@corp"}))
        self.assertEqual(domains, frozenset())

    def test_group_axis_open_for_ldap(self):
        self.assertIsNone(assign.dex_group_filter({"type": "LDAP", "ldap": {"host": "x"}}))

    def test_group_axis_open_for_oidc_without_allowed_groups(self):
        self.assertIsNone(assign.dex_group_filter(oidc()))

    def test_group_axis_closed_by_oidc_allowed_groups(self):
        self.assertEqual(assign.dex_group_filter(oidc(allowedGroups=["devs", "ops"])),
                         frozenset({"devs", "ops"}))

    def test_group_axis_closed_by_gitlab_and_crowd_groups(self):
        self.assertEqual(assign.dex_group_filter(
            {"type": "Gitlab", "gitlab": {"groups": ["g1"]}}), frozenset({"g1"}))
        self.assertEqual(assign.dex_group_filter(
            {"type": "Crowd", "crowd": {"groups": ["c1"]}}), frozenset({"c1"}))

    def test_group_axis_bitbucket_teams_unless_team_groups(self):
        self.assertEqual(assign.dex_group_filter(
            {"type": "BitbucketCloud", "bitbucketCloud": {"teams": ["t1"]}}), frozenset({"t1"}))
        self.assertIsNone(assign.dex_group_filter(
            {"type": "BitbucketCloud",
             "bitbucketCloud": {"teams": ["t1"], "includeTeamGroups": True}}))

    def test_group_axis_github_needs_teams_on_every_org(self):
        closed = {"type": "Github", "github": {"orgs": [
            {"name": "acme", "teams": ["dev", "ops"]}, {"name": "lab", "teams": ["x"]}]}}
        self.assertEqual(assign.dex_group_filter(closed),
                         frozenset({"acme:dev", "acme:ops", "lab:x"}))
        open_org = {"type": "Github", "github": {"orgs": [
            {"name": "acme", "teams": ["dev"]}, {"name": "lab"}]}}
        self.assertIsNone(assign.dex_group_filter(open_org))
        self.assertIsNone(assign.dex_group_filter({"type": "Github", "github": {"orgs": []}}))

    def test_group_axis_saml_needs_filter_groups(self):
        self.assertIsNone(assign.dex_group_filter(
            {"type": "SAML", "saml": {"allowedGroups": ["a"]}}))
        self.assertEqual(assign.dex_group_filter(
            {"type": "SAML", "saml": {"allowedGroups": ["a"], "filterGroups": True}}),
            frozenset({"a"}))

    def test_group_axis_universal_block(self):
        self.assertEqual(assign.dex_group_filter(
            {"type": "LDAP", "ldap": {}, "allowedIdentities": {"groups": ["g"]}}),
            frozenset({"g"}))

    def test_group_axis_universal_intersects_connector_filter(self):
        spec = oidc(allowedGroups=["devs", "ops"], allowedIdentities={"groups": ["ops", "qa"]})
        self.assertEqual(assign.dex_group_filter(spec), frozenset({"ops"}))


class TestDexTargetRoles(unittest.TestCase):
    def test_open_provider_targets_every_human_grant(self):
        targets = assign.dex_target_roles({"type": "LDAP", "ldap": {"host": "x"}}, dex_snaps())
        self.assertIn("user-authz:super-admin", targets)
        self.assertIn("user-authz:cluster-admin", targets)
        self.assertIn("user-authz:editor", targets)
        self.assertIn("d8:manage:security:viewer", targets)

    def test_open_provider_skips_system_prefixed_subjects(self):
        # kube-apiserver refuses OIDC usernames and groups with the system: prefix,
        # so no provider can assert them.
        targets = assign.dex_target_roles({"type": "LDAP", "ldap": {"host": "x"}}, dex_snaps())
        self.assertNotIn("user-authz:user", targets)
        self.assertNotIn("cluster-admin", targets)

    def test_groups_closed_email_open_still_reaches_user_grants(self):
        targets = assign.dex_target_roles(oidc(allowedGroups=["devs"]), dex_snaps())
        self.assertIn("user-authz:super-admin", targets)
        self.assertIn("d8:manage:security:viewer", targets)
        self.assertNotIn("cluster-admin", targets)

    def test_email_closed_groups_open_still_reaches_group_grants(self):
        targets = assign.dex_target_roles(
            oidc(allowedIdentities={"emailDomains": ["contractor.example"]}), dex_snaps())
        self.assertIn("user-authz:editor", targets)
        self.assertIn("user-authz:super-admin", targets)
        self.assertNotIn("user-authz:cluster-admin", targets)

    def test_both_closed_targets_only_listed_identities(self):
        targets = assign.dex_target_roles(oidc(
            allowedIdentities={"emailDomains": ["contractor.example"], "groups": ["devs"]}),
            dex_snaps())
        self.assertEqual(sorted(targets),
                         ["d8:manage:security:viewer", "user-authz:admin", "user-authz:editor"])

    def test_namespaced_rule_is_capped_at_admin(self):
        targets = assign.dex_target_roles(oidc(
            allowedIdentities={"emails": ["dev@contractor.example"], "groups": ["none"]}),
            dex_snaps())
        self.assertEqual(targets, ["user-authz:admin"])

    def test_both_closed_with_no_grants_is_empty(self):
        targets = assign.dex_target_roles(oidc(
            allowedIdentities={"emailDomains": ["nobody.example"], "groups": ["none"]}),
            dex_snaps())
        self.assertEqual(targets, [])

    def test_email_match_is_case_insensitive(self):
        targets = assign.dex_target_roles(oidc(
            allowedIdentities={"emails": ["boss@contractor.example"], "groups": ["none"]}),
            dex_snaps())
        self.assertEqual(targets, ["user-authz:editor"])

    def test_domain_match_is_exact(self):
        targets = assign.dex_target_roles(oidc(
            allowedIdentities={"emailDomains": ["example"], "groups": ["none"]}),
            dex_snaps())
        self.assertEqual(targets, [])

    def test_group_trap_reaches_superadmin(self):
        targets = assign.dex_target_roles(oidc(
            allowedIdentities={"emailDomains": ["nobody.example"], "groups": ["gate-trap"]}),
            dex_snaps())
        self.assertEqual(targets, ["user-authz:super-admin"])

    def test_parent_group_grant_is_not_inherited_by_idp_groups(self):
        # research R8: Group nesting is applied to local Users only; a token from an
        # external IdP carries exactly the groups the IdP asserted.
        targets = assign.dex_target_roles(oidc(
            allowedIdentities={"emailDomains": ["nobody.example"], "groups": ["child"]}),
            dex_snaps())
        self.assertEqual(targets, [])

    def test_local_user_group_membership_is_not_inherited_by_email(self):
        snaps = dex_snaps()
        snaps[assign.USER_SNAP] = [{"filterResult": {
            "name": "boss", "email": "boss@contractor.example", "groups": ["superadmins"]}}]
        targets = assign.dex_target_roles(oidc(
            allowedIdentities={"emails": ["boss@contractor.example"], "groups": ["none"]}),
            snaps)
        self.assertEqual(targets, ["user-authz:editor"])


class TestDexBenignUpdate(unittest.TestCase):
    def test_identical_is_benign(self):
        self.assertTrue(assign.dex_benign_update(oidc(), oidc()))

    def test_credential_rotation_is_benign(self):
        self.assertTrue(assign.dex_benign_update(oidc(clientSecret="old"), oidc(clientSecret="new")))
        old = {"type": "LDAP", "ldap": {"host": "h", "bindPW": "a"}}
        new = {"type": "LDAP", "ldap": {"host": "h", "bindPW": "b"}}
        self.assertTrue(assign.dex_benign_update(old, new))

    def test_display_name_and_enabled_are_benign(self):
        new = oidc(); new["displayName"] = "renamed"
        self.assertTrue(assign.dex_benign_update(oidc(), new))
        old = oidc(); old["enabled"] = False
        new = oidc(); new["enabled"] = True
        self.assertTrue(assign.dex_benign_update(old, new))

    def test_list_order_and_null_noise_are_benign(self):
        new = oidc(scopes=["groups", "email", "openid"])
        self.assertTrue(assign.dex_benign_update(oidc(), new))
        new = oidc(insecureSkipVerify=None)
        new["oidc"]["allowedGroups"] = []
        self.assertTrue(assign.dex_benign_update(oidc(), new))

    def test_narrowing_is_benign(self):
        self.assertTrue(assign.dex_benign_update(
            oidc(), oidc(allowedIdentities={"emailDomains": ["a"], "groups": ["g"]})))
        self.assertTrue(assign.dex_benign_update(
            oidc(allowedIdentities={"emailDomains": ["a", "b"]}),
            oidc(allowedIdentities={"emailDomains": ["a"]})))
        self.assertTrue(assign.dex_benign_update(
            oidc(allowedGroups=["a", "b"]), oidc(allowedGroups=["a"])))
        saml_off = {"type": "SAML", "saml": {"ssoURL": "u", "allowedGroups": ["a"]}}
        saml_on = {"type": "SAML", "saml": {"ssoURL": "u", "allowedGroups": ["a"], "filterGroups": True}}
        self.assertTrue(assign.dex_benign_update(saml_off, saml_on))

    def test_widening_is_not_benign(self):
        self.assertFalse(assign.dex_benign_update(
            oidc(allowedIdentities={"emailDomains": ["a"]}),
            oidc(allowedIdentities={"emailDomains": ["a", "b"]})))
        self.assertFalse(assign.dex_benign_update(
            oidc(allowedIdentities={"emailDomains": ["a"], "groups": ["g"]}),
            oidc(allowedIdentities={"groups": ["g"]})))
        self.assertFalse(assign.dex_benign_update(
            oidc(allowedGroups=["a"]), oidc(allowedGroups=["a", "b"])))
        self.assertFalse(assign.dex_benign_update(oidc(allowedGroups=["a"]), oidc()))

    def test_source_or_other_connector_changes_are_not_benign(self):
        self.assertFalse(assign.dex_benign_update(oidc(), oidc(issuer="https://other")))
        self.assertFalse(assign.dex_benign_update(oidc(), oidc(promptType="consent")))
        self.assertFalse(assign.dex_benign_update(oidc(), oidc(insecureSkipEmailVerified=True)))
        self.assertFalse(assign.dex_benign_update(
            oidc(), {"type": "LDAP", "displayName": "corp", "ldap": {"host": "h"}}))

    def test_narrowing_plus_source_change_is_not_benign(self):
        self.assertFalse(assign.dex_benign_update(
            oidc(), oidc(issuer="https://other",
                         allowedIdentities={"emailDomains": ["a"], "groups": ["g"]})))


class TestDexDenyMessage(unittest.TestCase):
    def test_message_names_provider_leftover_range_and_remedy(self):
        rng = assign.AssignRange(basic_max="ClusterAdmin", scope=None, subsystems=(), max_level=None)
        msg = assign.deny_dex_message("corp", ["user-authz:super-admin"], rng)
        self.assertTrue(msg.startswith('dexproviders.deckhouse.io "corp": the provider can assert '
                                       'identities that already carry roles [user-authz:super-admin]'))
        self.assertIn("basic<=ClusterAdmin", msg)
        self.assertIn("spec.allowedIdentities", msg)
        self.assertIn("SuperAdmin", msg)


if __name__ == "__main__":
    unittest.main()
