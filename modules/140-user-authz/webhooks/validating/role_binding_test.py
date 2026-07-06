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

import role_binding
from deckhouse import hook, tests
from dotmap import DotMap


def binding_context(role_name, role_kind="ClusterRole", username="alice@example.com", operation="CREATE"):
    return DotMap(
        {
            "binding": "role-validating.deckhouse.io",
            "review": {
                "request": {
                    "uid": "0b6e6c2e-6f4b-4b1a-9a2e-2f6a1e3d4c5b",
                    "operation": operation,
                    "userInfo": {"username": username},
                    "object": {
                        "apiVersion": "rbac.authorization.k8s.io/v1",
                        "kind": "ClusterRoleBinding",
                        "metadata": {"name": "t-crb"},
                        "roleRef": {
                            "apiGroup": "rbac.authorization.k8s.io",
                            "kind": role_kind,
                            "name": role_name,
                        },
                        "subjects": [{"kind": "User", "name": "u@example.com", "apiGroup": "rbac.authorization.k8s.io"}],
                    },
                }
            },
        }
    )


class TestRoleBindingValidation(unittest.TestCase):
    def run_hook(self, ctx):
        return hook.testrun(role_binding.main, [ctx])

    def assert_denied(self, role_name, kind_word):
        out = self.run_hook(binding_context(role_name))
        # message stays in sync with role_binding.validate
        expected = (
            f"ClusterRole '{role_name}' is a {kind_word} and cannot be granted through a ClusterRoleBinding "
            "(it would apply in every namespace). Use a RoleBinding to grant it in a namespace, or a "
            "ProjectRoleBinding/ClusterProjectRoleBinding to grant it across a project."
        )
        tests.assert_validation_deny(self, out, expected)

    # --- scoped roles: denied cluster-wide -------------------------------------------------------
    def test_namespace_role_denied(self):
        self.assert_denied("d8:namespace:admin", "scoped role")

    def test_project_role_denied(self):
        self.assert_denied("d8:project:viewer", "scoped role")

    def test_custom_namespace_role_denied(self):
        self.assert_denied("d8:custom:namespace:team-x", "scoped role")

    def test_custom_project_role_denied(self):
        self.assert_denied("d8:custom:project:team-x", "scoped role")

    def test_legacy_use_role_denied(self):
        # d8:use:role:* is the legacy name of the namespace lineage (kept as a compat alias); a CRB to
        # it grants namespace rules cluster-wide, so it must be denied like d8:namespace:*.
        self.assert_denied("d8:use:role:admin", "scoped role")

    def test_legacy_use_role_kubernetes_denied(self):
        self.assert_denied("d8:use:role:viewer:kubernetes", "scoped role")

    # --- capabilities: denied cluster-wide regardless of scope -----------------------------------
    def test_namespace_capability_denied(self):
        self.assert_denied("d8:namespace-capability:kubernetes:view_resources", "capability")

    def test_project_capability_denied(self):
        self.assert_denied("d8:project-capability:multitenancy-manager:manage_rbac", "capability")

    def test_system_capability_denied(self):
        # the gap the bash implementation missed: a system-side capability bound directly via CRB.
        self.assert_denied("d8:system-capability:prometheus:view", "capability")

    def test_subsystem_capability_denied(self):
        self.assert_denied("d8:subsystem-capability:observability:view_common_resources", "capability")

    def test_custom_namespace_capability_denied(self):
        self.assert_denied("d8:custom:namespace-capability:team:read", "capability")

    def test_custom_project_capability_denied(self):
        self.assert_denied("d8:custom:project-capability:team:deploy", "capability")

    def test_custom_system_capability_denied(self):
        self.assert_denied("d8:custom:system-capability:team:peek", "capability")

    # --- system/subsystem ROLES: cluster-scoped by design, allowed -------------------------------
    def test_system_role_allowed(self):
        out = self.run_hook(binding_context("d8:system:viewer"))
        tests.assert_validation_allowed(self, out, None)

    def test_subsystem_role_allowed(self):
        out = self.run_hook(binding_context("d8:subsystem:observability:manager"))
        tests.assert_validation_allowed(self, out, None)

    def test_legacy_manage_alias_allowed(self):
        # d8:manage:* is the legacy name of the cluster-scoped system/subsystem lineage (compat alias);
        # a ClusterRoleBinding to it is legitimate, unlike the namespace-scoped d8:use:role:*.
        out = self.run_hook(binding_context("d8:manage:observability:manager"))
        tests.assert_validation_allowed(self, out, None)

    # --- non-d8 and non-ClusterRole refs: untouched ----------------------------------------------
    def test_ordinary_clusterrole_allowed(self):
        out = self.run_hook(binding_context("cluster-admin"))
        tests.assert_validation_allowed(self, out, None)

    def test_custom_user_clusterrole_allowed(self):
        out = self.run_hook(binding_context("my-team:read-only"))
        tests.assert_validation_allowed(self, out, None)

    def test_role_ref_kind_role_is_ignored(self):
        # roleRef.kind=Role is not a ClusterRole binding target we gate here.
        out = self.run_hook(binding_context("d8:namespace:admin", role_kind="Role"))
        tests.assert_validation_allowed(self, out, None)

    # --- bypass identities ------------------------------------------------------------------------
    def test_deckhouse_sa_bypasses(self):
        out = self.run_hook(binding_context("d8:namespace:admin", username="system:serviceaccount:d8-system:deckhouse"))
        tests.assert_validation_allowed(self, out, None)

    def test_apiserver_bypasses(self):
        out = self.run_hook(binding_context("d8:project:admin", username="system:apiserver"))
        tests.assert_validation_allowed(self, out, None)


if __name__ == "__main__":
    unittest.main()
