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
        # The default Kubernetes ClusterRole: every rule, no can-assign labels.
        entry("cluster-admin", rules=STAR_ALL),
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


GITOPS = "system:serviceaccount:gitops:deployer"
GITOPS_INFO = {"username": GITOPS, "groups": ["system:serviceaccounts", "system:serviceaccounts:gitops"]}


def car_fr(name, *, users=(), groups=(), sas=(), access_level="", additional_roles=(),
           namespace_selector=None, limit_namespaces=(), allow_system=False):
    return {"filterResult": {
        "name": name,
        "accessLevel": access_level,
        "additionalRoles": list(additional_roles),
        "namespaceSelector": namespace_selector,
        "limitNamespaces": list(limit_namespaces),
        "allowAccessToSystemNamespaces": allow_system,
        "userSubjects": list(users),
        "groupSubjects": list(groups),
        "saSubjects": list(sas),
    }}


def crb_fr(name, role, *, users=(), groups=(), sas=()):
    return {"filterResult": {
        "name": name,
        "role": role,
        "userSubjects": list(users),
        "groupSubjects": list(groups),
        "saSubjects": list(sas),
    }}


def mt_state(enabled):
    """The multi-tenancy state snapshot: the ConfigMap exists only while multi-tenancy is on."""
    return [{"filterResult": {"enableMultiTenancy": True}}] if enabled else []


def assign_snaps(cars=(), crbs=(), multitenancy=True):
    return {
        assign.CAR_SNAP: list(cars),
        assign.AR_SNAP: [],
        assign.CRB_SNAP: list(crbs),
        assign.CROLE_SNAP: [],
        assign.MT_STATE_SNAP: mt_state(multitenancy),
    }


OPS_INFO = {"username": "ops@corp", "groups": []}


def open_car(*, allow_system=True, **kwargs):
    """A rule granting cluster-admin to ops@corp."""
    return car_fr("admins", users=["ops@corp"], access_level="User", additional_roles=["cluster-admin"],
                  allow_system=allow_system, **kwargs)


class TestFullAccess(unittest.TestCase):
    def full_access(self, snaps, info=None, cat=None):
        return assign.actor_has_full_access(info or GITOPS_INFO, snaps, cat or default_catalog())

    def test_namespace_limited_car(self):
        limited = assign.car_is_namespace_limited
        limits = {
            "limitNamespaces": {"limit_namespaces": ["app-.*"]},
            "labelSelector": {"namespace_selector": {"labelSelector": {"matchLabels": {"team": "a"}}}},
            "matchAny": {"namespace_selector": {"matchAny": True}},
            "empty selector": {"namespace_selector": {}},
        }
        for multitenancy in (True, False):
            for name, limit in limits.items():
                for allow_system in (True, False):
                    with self.subTest(limit=name, allow_system=allow_system, multitenancy=multitenancy):
                        fr = car_fr("limited", allow_system=allow_system, **limit)["filterResult"]
                        self.assertTrue(limited(fr, multitenancy=multitenancy))
        self.assertFalse(limited(car_fr("open", allow_system=True)["filterResult"], multitenancy=True))
        self.assertFalse(limited(car_fr("open", allow_system=True)["filterResult"], multitenancy=False))
        # The authorization webhook keeps a rule out of the system namespaces only with multi-tenancy on.
        self.assertTrue(limited(car_fr("no-system")["filterResult"], multitenancy=True))
        self.assertFalse(limited(car_fr("no-system")["filterResult"], multitenancy=False))

    def test_limit_that_covers_every_namespace_still_counts(self):
        # The check does not read the patterns on purpose, so a rule that reaches every namespace
        # through limitNamespaces errs on the strict side.
        for pattern in (".*", "^.*$", ".+"):
            with self.subTest(pattern=pattern):
                fr = car_fr("all", allow_system=True, limit_namespaces=[pattern])["filterResult"]
                self.assertTrue(assign.car_is_namespace_limited(fr, multitenancy=True))

    def test_multitenancy_state(self):
        self.assertTrue(assign.multitenancy_enabled(assign_snaps(multitenancy=True)))
        self.assertFalse(assign.multitenancy_enabled(assign_snaps(multitenancy=False)))
        self.assertFalse(assign.multitenancy_enabled({}))
        self.assertFalse(assign.multitenancy_enabled(
            {assign.MT_STATE_SNAP: [{"filterResult": {"enableMultiTenancy": False}}]}))

    def test_cluster_admin_crb_is_full_access(self):
        snaps = assign_snaps(crbs=[crb_fr("gitops", "cluster-admin", sas=["gitops:deployer"])])
        self.assertTrue(self.full_access(snaps))

    def test_cluster_admin_crb_on_a_group_is_full_access(self):
        snaps = assign_snaps(crbs=[crb_fr("platform", "cluster-admin", groups=["platform"])])
        self.assertTrue(self.full_access(snaps, {"username": "ops@corp", "groups": ["platform"]}))

    def test_car_without_namespace_limits_is_full_access(self):
        self.assertTrue(self.full_access(assign_snaps(cars=[open_car()]), OPS_INFO))

    def test_car_without_system_namespaces_is_full_access_without_multitenancy(self):
        # Without multi-tenancy the authorization webhook is not installed and the rule reaches every
        # namespace; multitenancy.py refuses allowAccessToSystemNamespaces, so no rule can set it.
        car = open_car(allow_system=False)
        self.assertTrue(self.full_access(assign_snaps(cars=[car], multitenancy=False), OPS_INFO))
        self.assertFalse(self.full_access(assign_snaps(cars=[car], multitenancy=True), OPS_INFO))

    def test_limited_car_is_not_full_access_without_multitenancy(self):
        car = open_car(limit_namespaces=["app-.*"])
        self.assertFalse(self.full_access(assign_snaps(cars=[car], multitenancy=False), OPS_INFO))

    def test_car_kept_out_of_system_namespaces_is_not_full_access(self):
        snaps = assign_snaps(
            cars=[car_fr("admins", users=["ops@corp"], access_level="User",
                         additional_roles=["cluster-admin"])],
            crbs=[crb_fr("user-authz:admins:additional-role:cluster-admin", "cluster-admin",
                         users=["ops@corp"])])
        self.assertFalse(self.full_access(snaps, {"username": "ops@corp", "groups": []}))

    def test_car_with_limit_namespaces_is_not_full_access(self):
        snaps = assign_snaps(
            cars=[car_fr("limited", sas=["gitops:deployer"], access_level="User",
                         additional_roles=["cluster-admin"], limit_namespaces=["app-.*"])],
            crbs=[
                crb_fr("user-authz:limited:additional-role:cluster-admin", "cluster-admin",
                       sas=["gitops:deployer"]),
                crb_fr("user-authz:limited:user", "user-authz:user", sas=["gitops:deployer"]),
            ])
        self.assertFalse(self.full_access(snaps))

    def test_car_with_namespace_selector_is_not_full_access(self):
        snaps = assign_snaps(
            cars=[car_fr("selected", groups=["platform"], additional_roles=["cluster-admin"],
                         namespace_selector={"labelSelector": {"matchLabels": {"team": "a"}}})],
            crbs=[crb_fr("user-authz:selected:additional-role:cluster-admin", "cluster-admin",
                         groups=["platform"])])
        self.assertFalse(self.full_access(snaps, {"username": "ops@corp", "groups": ["platform"]}))

    def test_plain_crb_counts_next_to_a_limited_car(self):
        snaps = assign_snaps(
            cars=[car_fr("limited", sas=["gitops:deployer"], additional_roles=["cluster-admin"],
                         limit_namespaces=["app-.*"])],
            crbs=[
                crb_fr("user-authz:limited:additional-role:cluster-admin", "cluster-admin",
                       sas=["gitops:deployer"]),
                crb_fr("gitops", "cluster-admin", sas=["gitops:deployer"]),
            ])
        self.assertTrue(self.full_access(snaps))

    def test_limited_car_does_not_hide_bindings_of_another_rule(self):
        snaps = assign_snaps(
            cars=[car_fr("limited", sas=["gitops:deployer"], limit_namespaces=["app-.*"])],
            crbs=[crb_fr("user-authz:limited-open:additional-role:cluster-admin", "cluster-admin",
                         sas=["gitops:deployer"])])
        self.assertTrue(self.full_access(snaps))

    def test_rules_short_of_everything_are_not_full_access(self):
        cat = default_catalog()
        cat["read-everything"] = entry("read-everything", rules=[
            {"apiGroups": ["*"], "resources": ["*"], "verbs": ["get", "list", "watch"]},
            {"nonResourceURLs": ["*"], "verbs": ["*"]},
        ])
        for role in ("user-authz:cluster-admin", "d8:manage:security:manager", "read-everything"):
            with self.subTest(role=role):
                snaps = assign_snaps(crbs=[crb_fr("b", role, sas=["gitops:deployer"])])
                self.assertFalse(self.full_access(snaps, cat=cat))

    def test_full_access_adds_up_over_bindings(self):
        cat = default_catalog()
        cat["non-resource-all"] = entry("non-resource-all", rules=[{"nonResourceURLs": ["*"], "verbs": ["*"]}])
        snaps = assign_snaps(crbs=[
            crb_fr("resources", "user-authz:cluster-admin", sas=["gitops:deployer"]),
            crb_fr("urls", "non-resource-all", groups=["system:serviceaccounts:gitops"]),
        ])
        self.assertTrue(self.full_access(snaps, cat=cat))

    def test_bindings_of_others_do_not_count(self):
        snaps = assign_snaps(
            cars=[car_fr("admins", users=["ops@corp"], additional_roles=["cluster-admin"])],
            crbs=[crb_fr("platform", "cluster-admin", groups=["platform"], sas=["gitops:other"])])
        self.assertFalse(self.full_access(snaps))

    def test_role_missing_from_catalog_is_not_full_access(self):
        cat = default_catalog()
        del cat["cluster-admin"]
        snaps = assign_snaps(crbs=[crb_fr("gitops", "cluster-admin", sas=["gitops:deployer"])])
        self.assertFalse(self.full_access(snaps, cat=cat))

    def test_full_access_is_the_superadmin_range(self):
        self.assertEqual(
            assign.actor_range([], default_catalog(), full_access=True),
            assign.AssignRange(basic_max="SuperAdmin", scope="system", subsystems=(),
                               max_level="superadmin"))

    def test_full_access_assigns_disaster_roles(self):
        actor = ["cluster-admin"]
        cat = default_catalog()
        for target in ("user-authz:super-admin", "cluster-admin", "d8:system:superadmin",
                       "d8:subsystem:security:superadmin"):
            with self.subTest(target=target):
                self.assertEqual(assign.can_assign(actor, [target], cat), [target])
                self.assertIsNone(assign.can_assign(actor, [target], cat, full_access=True))


if __name__ == "__main__":
    unittest.main()
