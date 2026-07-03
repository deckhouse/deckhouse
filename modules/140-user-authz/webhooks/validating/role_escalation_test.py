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

import role_escalation
from deckhouse import hook, tests
from dotmap import DotMap


def binding_context(kind, name, rules, username="namespace-admin", namespace=None, operation="CREATE"):
    metadata = {"name": name}
    if namespace is not None:
        metadata["namespace"] = namespace
    object_ = {
        "apiVersion": "rbac.authorization.k8s.io/v1",
        "kind": kind,
        "metadata": metadata,
        "rules": rules,
    }
    return DotMap(
        {
            "binding": "rbacv2-role-escalation.deckhouse.io",
            "review": {
                "request": {
                    "uid": "5c2c5f30-5a8e-4a4e-9d52-25d44a3677b1",
                    "operation": operation,
                    "userInfo": {"username": username},
                    "object": object_,
                }
            },
        }
    )


def rule(groups, resources, verbs):
    return {"apiGroups": groups, "resources": resources, "verbs": verbs}


PRB_WRITE = rule(["deckhouse.io"], ["projectrolebindings"], ["create"])
CONFIGMAP_RULE = rule([""], ["configmaps"], ["get", "list", "create", "update", "delete"])


def expected_deny(kind, name):
    # mirrors role_escalation.validate's message so the assertion stays in sync with the source.
    resources = ", ".join(sorted(role_escalation.PROTECTED_RESOURCES))
    return (
        f'{kind} "{name}" must not grant mutating access to Deckhouse project-management '
        f'resources ({resources} in group "{role_escalation.PROTECTED_GROUP}"). '
        "These permissions are conferred only by the built-in d8:project:* roles and their "
        "capabilities; granting them through a custom Role/ClusterRole would escalate "
        "namespace privileges to project-management privileges."
    )


class TestRoleEscalationValidation(unittest.TestCase):
    def run_hook(self, ctx):
        return hook.testrun(role_escalation.main, [ctx])

    def test_namespace_admin_role_with_projectrolebindings_write_is_denied(self):
        out = self.run_hook(binding_context("Role", "escalate", [PRB_WRITE], namespace="team-a"))
        tests.assert_validation_deny(self, out, expected_deny("Role", "escalate"))

    def test_normal_namespaced_role_is_allowed(self):
        out = self.run_hook(binding_context("Role", "app-editor", [CONFIGMAP_RULE], namespace="team-a"))
        tests.assert_validation_allowed(self, out, None)

    def test_clusterrole_with_projects_write_is_denied(self):
        out = self.run_hook(
            binding_context("ClusterRole", "d8:custom:namespace-capability:bad",
                            [rule(["deckhouse.io"], ["projects"], ["update"])])
        )
        tests.assert_validation_deny(self, out, expected_deny("ClusterRole", "d8:custom:namespace-capability:bad"))

    def test_read_only_on_protected_resources_is_allowed(self):
        # get/list/watch are not escalation; the project view capabilities legitimately read these.
        out = self.run_hook(
            binding_context("Role", "viewer",
                            [rule(["deckhouse.io"], ["projectrolebindings"], ["get", "list", "watch"])],
                            namespace="team-a")
        )
        tests.assert_validation_allowed(self, out, None)

    def test_wildcard_group_and_resource_with_write_is_denied(self):
        out = self.run_hook(
            binding_context("Role", "wild", [rule(["*"], ["*"], ["*"])], namespace="team-a")
        )
        tests.assert_validation_deny(self, out, expected_deny("Role", "wild"))

    def test_deckhouse_group_wildcard_resource_write_is_denied(self):
        out = self.run_hook(
            binding_context("Role", "wild-deckhouse",
                            [rule(["deckhouse.io"], ["*"], ["patch"])], namespace="team-a")
        )
        tests.assert_validation_deny(self, out, expected_deny("Role", "wild-deckhouse"))

    def test_subresource_on_protected_is_denied(self):
        out = self.run_hook(
            binding_context("Role", "subres",
                            [rule(["deckhouse.io"], ["clusterprojectrolebindings/status"], ["update"])],
                            namespace="team-a")
        )
        tests.assert_validation_deny(self, out, expected_deny("Role", "subres"))

    def test_protected_resource_in_other_group_is_allowed(self):
        # a "projects" resource in some unrelated group is not a Deckhouse project-management object.
        out = self.run_hook(
            binding_context("Role", "other-projects",
                            [rule(["example.com"], ["projects"], ["create"])], namespace="team-a")
        )
        tests.assert_validation_allowed(self, out, None)

    def test_update_operation_is_validated(self):
        out = self.run_hook(
            binding_context("Role", "escalate", [PRB_WRITE], namespace="team-a", operation="UPDATE")
        )
        tests.assert_validation_deny(self, out, expected_deny("Role", "escalate"))

    def test_deckhouse_sa_is_allowed_to_author_protected_rules(self):
        # the built-in roles are authored by Deckhouse / the aggregation controller; they bypass.
        out = self.run_hook(
            binding_context("ClusterRole", "d8:project-capability:multitenancy-manager:manage_rbac",
                            [PRB_WRITE], username="system:serviceaccount:d8-system:deckhouse")
        )
        tests.assert_validation_allowed(self, out, None)


if __name__ == "__main__":
    unittest.main()
