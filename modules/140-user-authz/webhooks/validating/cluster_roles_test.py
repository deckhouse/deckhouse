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

import cluster_roles
from deckhouse import hook, tests
from dotmap import DotMap


def binding_context(name, labels=None, rules=None, selector_labels=None, annotations=None):
    object_ = {
        "apiVersion": "rbac.authorization.k8s.io/v1",
        "kind": "ClusterRole",
        "metadata": {"name": name},
    }
    if labels is not None:
        object_["metadata"]["labels"] = labels
    if annotations is not None:
        object_["metadata"]["annotations"] = annotations
    if rules is not None:
        object_["rules"] = rules
    if selector_labels is not None:
        object_["aggregationRule"] = {
            "clusterRoleSelectors": [{"matchLabels": ml} for ml in selector_labels]
        }
    return DotMap(
        {
            "binding": "rbacv2-cluster-roles.deckhouse.io",
            "review": {
                "request": {
                    "uid": "5c2c5f30-5a8e-4a4e-9d52-25d44a3677b1",
                    "operation": "CREATE",
                    "userInfo": {"username": "kubernetes-admin"},
                    "object": object_,
                }
            },
        }
    )


SOME_RULES = [{"apiGroups": [""], "resources": ["pods"], "verbs": ["get", "list"]}]

RESERVED_PREFIX_MSG = (
    'ClusterRole names with the "d8:" prefix are reserved for Deckhouse. '
    'Use the "d8:custom:" prefix for custom roles and capabilities.'
)


def _cr(name, labels=None, annotations=None, rules=None, aggregation=None):
    obj = {
        "apiVersion": "rbac.authorization.k8s.io/v1",
        "kind": "ClusterRole",
        "metadata": {"name": name},
    }
    if labels is not None:
        obj["metadata"]["labels"] = labels
    if annotations is not None:
        obj["metadata"]["annotations"] = annotations
    if rules is not None:
        obj["rules"] = rules
    if aggregation is not None:
        obj["aggregationRule"] = aggregation
    return obj


def update_binding_context(old_object, new_object, username="kubernetes-admin"):
    return DotMap(
        {
            "binding": "rbacv2-cluster-roles.deckhouse.io",
            "review": {
                "request": {
                    "uid": "5c2c5f30-5a8e-4a4e-9d52-25d44a3677b1",
                    "operation": "UPDATE",
                    "userInfo": {"username": username},
                    "object": new_object,
                    "oldObject": old_object,
                }
            },
        }
    )


class TestClusterRolesValidation(unittest.TestCase):
    def run_hook(self, ctx):
        return hook.testrun(cluster_roles.main, [ctx])

    def test_plain_cluster_role_is_allowed(self):
        out = self.run_hook(binding_context("my-own-role", rules=SOME_RULES))
        tests.assert_validation_allowed(self, out, None)

    def test_d8_prefix_is_reserved(self):
        for name in ["d8:namespace:admin", "d8:system:manager", "d8:whatever"]:
            with self.subTest(name=name):
                out = self.run_hook(binding_context(name, rules=SOME_RULES))
                tests.assert_validation_deny(
                    self,
                    out,
                    'ClusterRole names with the "d8:" prefix are reserved for Deckhouse. '
                    'Use the "d8:custom:" prefix for custom roles and capabilities.',
                )

    def test_builtin_kind_labels_are_reserved(self):
        for kind in ["role", "capability"]:
            with self.subTest(kind=kind):
                out = self.run_hook(
                    binding_context("my-role", labels={"rbac.deckhouse.io/kind": kind})
                )
                tests.assert_validation_deny(
                    self,
                    out,
                    f'The label "rbac.deckhouse.io/kind: {kind}" is reserved for Deckhouse '
                    'built-in objects. Use "custom-role" or "custom-capability" instead.',
                )

    def test_custom_kind_requires_custom_name_prefix(self):
        out = self.run_hook(
            binding_context(
                "my-role",
                labels={"rbac.deckhouse.io/kind": "custom-role"},
                selector_labels=[{"rbac.deckhouse.io/aggregate-to-namespace-as": "viewer"}],
            )
        )
        tests.assert_validation_deny(
            self,
            out,
            'ClusterRole "my-role" labeled "rbac.deckhouse.io/kind: custom-role" '
            'must be named with the "d8:custom:" prefix.',
        )

    def test_custom_role_must_not_define_rules(self):
        out = self.run_hook(
            binding_context(
                "d8:custom:namespace:developer",
                labels={"rbac.deckhouse.io/kind": "custom-role"},
                rules=SOME_RULES,
                selector_labels=[{"rbac.deckhouse.io/aggregate-to-namespace-as": "viewer"}],
            )
        )
        tests.assert_validation_deny(
            self,
            out,
            'ClusterRole "d8:custom:namespace:developer" with "rbac.deckhouse.io/kind: custom-role" '
            "must not define rules. Move the rules to a custom-capability and aggregate it.",
        )

    def test_custom_role_must_define_aggregation(self):
        out = self.run_hook(
            binding_context(
                "d8:custom:namespace:developer",
                labels={"rbac.deckhouse.io/kind": "custom-role"},
            )
        )
        tests.assert_validation_deny(
            self,
            out,
            'ClusterRole "d8:custom:namespace:developer" with "rbac.deckhouse.io/kind: custom-role" '
            "must define aggregationRule.clusterRoleSelectors.",
        )

    def test_custom_role_lineage_mixing_is_forbidden(self):
        out = self.run_hook(
            binding_context(
                "d8:custom:namespace:developer",
                labels={"rbac.deckhouse.io/kind": "custom-role"},
                selector_labels=[
                    {"rbac.deckhouse.io/aggregate-to-namespace-as": "admin"},
                    {"rbac.deckhouse.io/aggregate-to-system-as": "manager"},
                ],
            )
        )
        tests.assert_validation_deny(
            self,
            out,
            'ClusterRole "d8:custom:namespace:developer" must not aggregate the "system" lineage '
            'together with the "namespace" lineage: mixing system and namespace/project scopes is forbidden.',
        )

    def test_custom_capability_lineage_mixing_is_forbidden(self):
        out = self.run_hook(
            binding_context(
                "d8:custom:namespace-capability:foo",
                labels={"rbac.deckhouse.io/kind": "custom-capability"},
                rules=SOME_RULES,
                selector_labels=[
                    {"rbac.deckhouse.io/aggregate-to-project-as": "admin"},
                    {"rbac.deckhouse.io/aggregate-to-networking-as": "manager"},
                ],
            )
        )
        tests.assert_validation_deny(
            self,
            out,
            'ClusterRole "d8:custom:namespace-capability:foo" must not aggregate the "networking" lineage '
            'together with the "project" lineage: mixing system and namespace/project scopes is forbidden.',
        )

    def test_valid_custom_role_is_allowed(self):
        # The documented pattern: a custom role pulls a custom lineage with its own
        # capabilities plus a base built-in tenant lineage.
        out = self.run_hook(
            binding_context(
                "d8:custom:namespace:developer",
                labels={"rbac.deckhouse.io/kind": "custom-role"},
                selector_labels=[
                    {"rbac.deckhouse.io/aggregate-to-mycustom-as": "manager"},
                    {"rbac.deckhouse.io/aggregate-to-namespace-as": "viewer"},
                ],
            )
        )
        tests.assert_validation_allowed(self, out, None)

    def test_valid_custom_capability_is_allowed(self):
        out = self.run_hook(
            binding_context(
                "d8:custom:namespace-capability:view-logs",
                labels={
                    "rbac.deckhouse.io/kind": "custom-capability",
                    "rbac.deckhouse.io/aggregate-to-namespace-as": "viewer",
                },
                rules=SOME_RULES,
            )
        )
        tests.assert_validation_allowed(self, out, None)

    def test_custom_meta_annotations_are_allowed_on_custom_role(self):
        # Administrators may override how a role is displayed via the
        # custom.meta.deckhouse.io/{title,description} annotations (UI priority
        # custom.meta > <lang>.meta > name). The webhook must not reject them.
        out = self.run_hook(
            binding_context(
                "d8:custom:namespace:developer",
                labels={"rbac.deckhouse.io/kind": "custom-role"},
                annotations={
                    "custom.meta.deckhouse.io/title": "Developer (custom)",
                    "custom.meta.deckhouse.io/description": "Custom developer role",
                    "en.meta.deckhouse.io/title": "Developer",
                    "ru.meta.deckhouse.io/title": "Разработчик",
                },
                selector_labels=[{"rbac.deckhouse.io/aggregate-to-namespace-as": "viewer"}],
            )
        )
        tests.assert_validation_allowed(self, out, None)

    def test_custom_meta_annotations_are_allowed_on_custom_capability(self):
        out = self.run_hook(
            binding_context(
                "d8:custom:namespace-capability:view-logs",
                labels={
                    "rbac.deckhouse.io/kind": "custom-capability",
                    "rbac.deckhouse.io/aggregate-to-namespace-as": "viewer",
                },
                annotations={
                    "custom.meta.deckhouse.io/title": "View logs (custom)",
                    "custom.meta.deckhouse.io/description": "Read pod logs",
                },
                rules=SOME_RULES,
            )
        )
        tests.assert_validation_allowed(self, out, None)

    def test_custom_meta_annotations_are_allowed_on_plain_role(self):
        out = self.run_hook(
            binding_context(
                "my-own-role",
                rules=SOME_RULES,
                annotations={"custom.meta.deckhouse.io/title": "My role"},
            )
        )
        tests.assert_validation_allowed(self, out, None)

    def test_admin_can_aggregate_disabled_role_into_custom_role(self):
        # A role annotated rbac.deckhouse.io/disabled-for-direct-use-in-projects must not be
        # granted directly via a (Cluster)ProjectRoleBinding, but an administrator may still
        # aggregate it into their own custom role. This webhook validates only the custom-role
        # object (name/kind/rules/selectors/lineage) and never inspects the disabled annotation on
        # the aggregated targets, so the aggregation is allowed.
        out = self.run_hook(
            binding_context(
                "d8:custom:project:auditor",
                labels={"rbac.deckhouse.io/kind": "custom-role"},
                selector_labels=[
                    {"rbac.deckhouse.io/aggregate-to-project-as": "view"},
                ],
            )
        )
        tests.assert_validation_allowed(self, out, None)

    # An administrator may override a BUILT-IN role's display name by setting only the
    # custom.meta.deckhouse.io/* annotations on it, despite the reserved "d8:" prefix.
    def test_admin_can_set_custom_meta_on_builtin_role(self):
        old = _cr(
            "d8:namespace:admin",
            labels={"rbac.deckhouse.io/kind": "role"},
            annotations={"en.meta.deckhouse.io/title": "Namespace Administrator"},
        )
        new = _cr(
            "d8:namespace:admin",
            labels={"rbac.deckhouse.io/kind": "role"},
            annotations={
                "en.meta.deckhouse.io/title": "Namespace Administrator",
                "custom.meta.deckhouse.io/title": "NS Owner",
                "custom.meta.deckhouse.io/description": "Renamed by the platform admin",
            },
        )
        out = self.run_hook(update_binding_context(old, new))
        tests.assert_validation_allowed(self, out, None)

    def test_builtin_role_update_changing_rules_is_denied(self):
        old = _cr("d8:namespace:admin", rules=[])
        new = _cr(
            "d8:namespace:admin",
            rules=SOME_RULES,
            annotations={"custom.meta.deckhouse.io/title": "sneaky"},
        )
        out = self.run_hook(update_binding_context(old, new))
        tests.assert_validation_deny(self, out, RESERVED_PREFIX_MSG)

    def test_builtin_role_update_changing_aggregation_is_denied(self):
        old = _cr(
            "d8:namespace:admin",
            aggregation={"clusterRoleSelectors": [{"matchLabels": {"a": "1"}}]},
        )
        new = _cr(
            "d8:namespace:admin",
            aggregation={"clusterRoleSelectors": [{"matchLabels": {"a": "2"}}]},
            annotations={"custom.meta.deckhouse.io/title": "sneaky"},
        )
        out = self.run_hook(update_binding_context(old, new))
        tests.assert_validation_deny(self, out, RESERVED_PREFIX_MSG)

    def test_builtin_role_update_changing_non_custom_annotation_is_denied(self):
        old = _cr("d8:namespace:admin", annotations={"en.meta.deckhouse.io/title": "A"})
        new = _cr(
            "d8:namespace:admin",
            annotations={
                "en.meta.deckhouse.io/title": "B",
                "custom.meta.deckhouse.io/title": "x",
            },
        )
        out = self.run_hook(update_binding_context(old, new))
        tests.assert_validation_deny(self, out, RESERVED_PREFIX_MSG)

    def test_builtin_role_update_changing_labels_is_denied(self):
        old = _cr("d8:namespace:admin", labels={"rbac.deckhouse.io/kind": "role"})
        new = _cr(
            "d8:namespace:admin",
            labels={"rbac.deckhouse.io/kind": "role", "evil": "1"},
            annotations={"custom.meta.deckhouse.io/title": "x"},
        )
        out = self.run_hook(update_binding_context(old, new))
        tests.assert_validation_deny(self, out, RESERVED_PREFIX_MSG)


DELEGATABLE = {"rbac.deckhouse.io/kind": "custom-role", "rbac.deckhouse.io/delegatable": "true"}
CAPABILITY = "rbac.deckhouse.io/capability"

DELEGATABLE_DENY_MSG = (
    'ClusterRole "d8:custom:project:role-a" must not be labeled "rbac.deckhouse.io/delegatable: true": '
    "a role bindable inside a project may aggregate namespace and project capabilities only."
)


class TestDelegatableCustomRole(unittest.TestCase):
    """
    The delegatable label is what lets a role reach a project namespace, so it may only be claimed
    by a role assembled from namespace/project capabilities. The lineage check next to it does not
    cover this: it reads the "aggregate-to-<lineage>-as" selectors, while a role built from
    individual capabilities selects them by the capability label instead.
    """

    def run_hook(self, ctx):
        return hook.testrun(cluster_roles.main, [ctx])

    def _run(self, selector_values, labels=None):
        return self.run_hook(
            binding_context(
                "d8:custom:project:role-a",
                labels=labels if labels is not None else DELEGATABLE,
                selector_labels=[{CAPABILITY: value} for value in selector_values],
            )
        )

    def test_namespace_capabilities_may_be_delegatable(self):
        out = self._run(["namespace-capability.kubernetes.view_logs", "namespace-capability.cert-manager.view"])
        tests.assert_validation_allowed(self, out, None)

    def test_project_capabilities_may_be_delegatable(self):
        out = self._run(["project-capability.multitenancy-manager.view"])
        tests.assert_validation_allowed(self, out, None)

    def test_custom_tenant_capabilities_may_be_delegatable(self):
        out = self._run(["custom.project-capability.extra-a1b2c3", "custom.namespace-capability.extra-d4e5f6"])
        tests.assert_validation_allowed(self, out, None)

    def test_system_capability_may_not_be_delegatable(self):
        out = self._run(["namespace-capability.kubernetes.view_logs", "system-capability.deckhouse.manage"])
        tests.assert_validation_deny(self, out, DELEGATABLE_DENY_MSG)

    def test_subsystem_capability_may_not_be_delegatable(self):
        out = self._run(["subsystem-capability.security.view"])
        tests.assert_validation_deny(self, out, DELEGATABLE_DENY_MSG)

    def test_custom_system_capability_may_not_be_delegatable(self):
        out = self._run(["custom.system-capability.hh-bqjxg0"])
        tests.assert_validation_deny(self, out, DELEGATABLE_DENY_MSG)

    def test_unreadable_capability_value_may_not_be_delegatable(self):
        out = self._run(["whatever"])
        tests.assert_validation_deny(self, out, DELEGATABLE_DENY_MSG)

    def test_tenant_lineage_selector_may_be_delegatable(self):
        out = self.run_hook(
            binding_context(
                "d8:custom:project:role-a",
                labels=DELEGATABLE,
                selector_labels=[{"rbac.deckhouse.io/aggregate-to-namespace-as": "admin"}],
            )
        )
        tests.assert_validation_allowed(self, out, None)

    def test_system_lineage_selector_may_not_be_delegatable(self):
        out = self.run_hook(
            binding_context(
                "d8:custom:project:role-a",
                labels=DELEGATABLE,
                selector_labels=[{"rbac.deckhouse.io/aggregate-to-security-as": "viewer"}],
            )
        )
        tests.assert_validation_deny(self, out, DELEGATABLE_DENY_MSG)

    def test_selector_by_another_label_may_not_be_delegatable(self):
        out = self.run_hook(
            binding_context(
                "d8:custom:project:role-a",
                labels=DELEGATABLE,
                selector_labels=[{"some.other/label": "value"}],
            )
        )
        tests.assert_validation_deny(self, out, DELEGATABLE_DENY_MSG)

    def test_same_role_without_the_label_is_untouched(self):
        out = self._run(
            ["system-capability.deckhouse.manage"],
            labels={"rbac.deckhouse.io/kind": "custom-role"},
        )
        tests.assert_validation_allowed(self, out, None)


AGGREGATED_RULES = [{"apiGroups": [""], "resources": ["pods"], "verbs": ["get"]}]
CUSTOM_ROLE_LABELS = {"rbac.deckhouse.io/kind": "custom-role"}
AGGREGATION = {"clusterRoleSelectors": [{"matchLabels": {"rbac.deckhouse.io/capability": "namespace-capability.kubernetes.view_logs"}}]}

NO_RULES_MSG = (
    'ClusterRole "d8:custom:project:role-a" with "rbac.deckhouse.io/kind: custom-role" must not define rules. '
    "Move the rules to a custom-capability and aggregate it."
)


class TestAggregatedCustomRoleIsEditable(unittest.TestCase):
    """
    After creation .rules belongs to the aggregation controller, so every later write carries rules
    the user never authored — a plain "kubectl label" sends them straight back. Objecting to those
    would make an aggregated custom role uneditable, which is why UPDATE compares with the old
    object instead of just looking at the presence of rules.
    """

    def run_hook(self, ctx):
        return hook.testrun(cluster_roles.main, [ctx])

    def _role(self, labels=None, rules=None):
        return _cr(
            "d8:custom:project:role-a",
            labels=labels if labels is not None else CUSTOM_ROLE_LABELS,
            rules=rules,
            aggregation=AGGREGATION,
        )

    def test_create_with_rules_is_still_denied(self):
        out = self.run_hook(
            binding_context(
                "d8:custom:project:role-a",
                labels=CUSTOM_ROLE_LABELS,
                rules=AGGREGATED_RULES,
                selector_labels=[{"rbac.deckhouse.io/capability": "namespace-capability.kubernetes.view_logs"}],
            )
        )
        tests.assert_validation_deny(self, out, NO_RULES_MSG)

    def test_labeling_an_aggregated_role_is_allowed(self):
        old = self._role(rules=AGGREGATED_RULES)
        new = self._role(
            labels={**CUSTOM_ROLE_LABELS, "rbac.deckhouse.io/delegatable": "true"},
            rules=AGGREGATED_RULES,
        )
        out = self.run_hook(update_binding_context(old, new))
        tests.assert_validation_allowed(self, out, None)

    def test_changing_the_rules_of_an_aggregated_role_is_denied(self):
        old = self._role(rules=AGGREGATED_RULES)
        new = self._role(rules=AGGREGATED_RULES + [{"apiGroups": [""], "resources": ["secrets"], "verbs": ["*"]}])
        out = self.run_hook(update_binding_context(old, new))
        tests.assert_validation_deny(self, out, NO_RULES_MSG)

    def test_adding_rules_to_a_role_that_had_none_is_denied(self):
        old = self._role()
        new = self._role(rules=AGGREGATED_RULES)
        out = self.run_hook(update_binding_context(old, new))
        tests.assert_validation_deny(self, out, NO_RULES_MSG)

    def test_dropping_the_rules_is_allowed(self):
        old = self._role(rules=AGGREGATED_RULES)
        new = self._role()
        out = self.run_hook(update_binding_context(old, new))
        tests.assert_validation_allowed(self, out, None)


if __name__ == "__main__":
    unittest.main()
