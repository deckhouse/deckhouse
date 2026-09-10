#!/usr/bin/python3

# Copyright 2025 Flant JSC
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
import json
import multitenancy
import multitenancy_test_factories as factories
from deckhouse import hook, tests
from dotmap import DotMap


class TestLimitNamespacesPatternValidation(unittest.TestCase):
    """A limitNamespaces pattern the authorization webhook cannot compile is rejected at admission.

    Without this, such a rule is accepted and then quarantined at runtime: its subjects silently get
    LESS access than the author wrote, and the only trace is a metric nobody is looking at while
    they are typing kubectl apply.
    """

    def car(self, *patterns):
        return DotMap({
            "metadata": {"name": "team-a"},
            "spec": {"limitNamespaces": list(patterns)},
        })

    def errors_for(self, *patterns):
        errors, _ = multitenancy.validate_car_limit_namespaces_patterns(self.car(*patterns))
        return errors

    def test_valid_patterns_pass(self):
        for pattern in ["team-a", "team-.*", "team-[0-9]+", "team-.*|kube-system", "(a|b)-ns", ".*"]:
            with self.subTest(pattern=pattern):
                self.assertEqual([], self.errors_for(pattern))

    def test_uncompilable_pattern_is_rejected(self):
        for pattern in ["team-(", "team-[", "*-ns", "team-a{2,1}"]:
            with self.subTest(pattern=pattern):
                errors = self.errors_for(pattern)
                self.assertEqual(1, len(errors), errors)
                self.assertIn("is not a valid regular expression", errors[0])
                self.assertIn("team-a", errors[0])  # the rule is named

    def test_constructs_go_does_not_support_are_rejected(self):
        """Python's re accepts these; Go's RE2 does not.

        Compiling the pattern here is therefore not enough on its own - a lookahead would sail
        through admission and be quarantined by the webhook, which is the outcome being prevented.
        """
        for pattern, description in [
            ("team-(?=a)", "lookahead"),
            ("team-(?!a)", "negative lookahead"),
            ("(?<=team-)a", "lookbehind"),
            ("(?<!team-)a", "negative lookbehind"),
            ("(a)\\1", "backreference"),
        ]:
            with self.subTest(pattern=pattern):
                errors = self.errors_for(pattern)
                self.assertEqual(1, len(errors), errors)
                self.assertIn(description, errors[0])
                self.assertIn("RE2", errors[0])

    def test_an_escaped_backslash_before_a_digit_is_not_a_backreference(self):
        self.assertEqual([], self.errors_for("team\\\\1"))

    def test_every_bad_pattern_is_named(self):
        errors = self.errors_for("team-a", "team-(", "ops-[")
        self.assertEqual(2, len(errors), errors)

    def test_a_rule_without_limit_namespaces_is_not_examined(self):
        obj = DotMap({"metadata": {"name": "team-a"}, "spec": {"accessLevel": "User"}})
        self.assertEqual(([], []), multitenancy.validate_car_limit_namespaces_patterns(obj))


class TestMultiTenancyValidationForCarsAndModuleConfig(unittest.TestCase):

    def run_hook(self, context_json: str):
        ctx_dict = json.loads(context_json)
        return hook.testrun(multitenancy.main, [DotMap(ctx_dict)])

    def test_car_denied_when_multitenancy_disabled_and_multitenancy_related_fields_used(self):
        for scenario, ctx_json in [
            ['enableMultiTenancy is None',
             factories.prepare_car_binding_context(
                 car_restricted_multitenancy_fields=True, module_enable_multitenancy_field=None)
            ],
            ['enableMultiTenancy is False',
             factories.prepare_car_binding_context(
                 car_restricted_multitenancy_fields=True, module_enable_multitenancy_field=False)
            ],
        ]:
            with self.subTest(title=scenario):
                tests.assert_validation_deny(self, self.run_hook(ctx_json), '; '.join([
                    "You must enable userAuthz.enableMultiTenancy to use the allowAccessToSystemNamespaces flag in ClusterAuthorizationRule 'user1'",
                    "You must enable userAuthz.enableMultiTenancy to use the namespaceSelector option in ClusterAuthorizationRule 'user1'",
                    "You must enable userAuthz.enableMultiTenancy to use the limitNamespaces option in ClusterAuthorizationRule 'user1'",
                ]))

    def test_car_allowed_when_multitenancy_disabled_and_no_multitenancy_related_fields(self):
        for scenario, ctx_json in [
            ['enableMultiTenancy is None',
             factories.prepare_car_binding_context(
                car_restricted_multitenancy_fields=False, module_enable_multitenancy_field=None)
            ],
            ['enableMultiTenancy is False',
             factories.prepare_car_binding_context(
                car_restricted_multitenancy_fields=False, module_enable_multitenancy_field=False)
            ],
        ]:
            with self.subTest(scenario):
                tests.assert_validation_allowed(self, self.run_hook(ctx_json), None)

    def test_car_allowed_when_multitenancy_enabled_and_multitenancy_related_fields_used(self):
        for scenario, ctx_json in [
            ['enableMultiTenancy is True',
             factories.prepare_car_binding_context(
                car_restricted_multitenancy_fields=True, module_enable_multitenancy_field=True)
            ],
        ]:
            with self.subTest(scenario):
                tests.assert_validation_allowed(self, self.run_hook(ctx_json), None)

    def test_car_allowed_when_multitenancy_enabled_and_no_multitenancy_related_fields(self):
        for scenario, ctx_json in [
            ['enableMultiTenancy is True',
             factories.prepare_car_binding_context(
                car_restricted_multitenancy_fields=False, module_enable_multitenancy_field=True)
            ],
        ]:
            with self.subTest(scenario):
                tests.assert_validation_allowed(self, self.run_hook(ctx_json), None)

    def test_module_config_denied_when_multitenancy_disabled_and_some_cars_have_multitenancy_related_fields(self):
        for scenario, ctx_json in [
            ['enableMultiTenancy is None, 3 mixed cars with multitenancy-related and not multitenancy-related fields',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=None, cars=factories.build_three_mixed_multitenancy_related_and_not_related_cars())
            ],
            ['enableMultiTenancy is False, 3 mixed cars with multitenancy-related and not multitenancy-related fields',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=False, cars=factories.build_three_mixed_multitenancy_related_and_not_related_cars())
            ]
        ]:
             with self.subTest(scenario):
                tests.assert_validation_deny(self, self.run_hook(ctx_json), "; ".join([
                    "You must enable userAuthz.enableMultiTenancy to use the allowAccessToSystemNamespaces flag in ClusterAuthorizationRule 'user1'",
                    "You must enable userAuthz.enableMultiTenancy to use the namespaceSelector option in ClusterAuthorizationRule 'user1'",
                    "You must enable userAuthz.enableMultiTenancy to use the limitNamespaces option in ClusterAuthorizationRule 'user1'",
                    "You must enable userAuthz.enableMultiTenancy to use the allowAccessToSystemNamespaces flag in ClusterAuthorizationRule 'user3'",
                    "You must enable userAuthz.enableMultiTenancy to use the namespaceSelector option in ClusterAuthorizationRule 'user3'",
                    "You must enable userAuthz.enableMultiTenancy to use the limitNamespaces option in ClusterAuthorizationRule 'user3'",
                ]))

    def test_module_config_allowed_when_multitenancy_disabled_and_no_cars_have_multitenancy_related_fields(self):
        for scenario, ctx_json in [
            ['enableMultiTenancy is None, 3 cars without multitenancy-related fields',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=None, cars=factories.build_three_not_multitenancy_related_cars())
            ],
            ['enableMultiTenancy is False, 3 cars without multitenancy-related fields',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=False, cars=factories.build_three_not_multitenancy_related_cars())
            ],
            ['enableMultiTenancy is None, no cars',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=None, cars=[])
            ],
            ['enableMultiTenancy is False, no cars',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=False, cars=[])
            ],
        ]:
            with self.subTest(scenario):
                tests.assert_validation_allowed(self, self.run_hook(ctx_json), None)

    def test_module_config_allowed_when_multitenancy_enabled_and_some_cars_have_multitenancy_related_fields(self):
        for scenario, ctx_json in [
            ['enableMultiTenancy is True, 3 mixed cars with multitenancy-related and not multitenancy-related fields',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=True, cars=factories.build_three_mixed_multitenancy_related_and_not_related_cars())
            ],
        ]:
            with self.subTest(scenario):
                tests.assert_validation_allowed(self, self.run_hook(ctx_json), None)

    def test_module_config_allowed_when_multitenancy_enabled_and_no_cars_have_multitenancy_related_fields(self):
        for scenario, ctx_json in [
            ['enableMultiTenancy is True, 3 cars with multitenancy-related fields',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=True, cars=factories.build_three_not_multitenancy_related_cars())
            ],
            ['enableMultiTenancy is True, no cars',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=True, cars=[])
            ],
        ]:
            with self.subTest(scenario):
                tests.assert_validation_allowed(self, self.run_hook(ctx_json), None)

    def test_module_config_allowed_when_field_absent_from_request_but_effectively_enabled(self):
        """
        enableMultiTenancy is absent from the submitted spec.settings, and was already
        absent before this request too (e.g. an unrelated ModuleConfig edit, or a
        CSE-edition cluster where it was never set explicitly) — the effective value isn't
        changing, so the state ConfigMap's mirrored value (explicit, or via the edition's
        config-schema default) is trusted and existing CARs must not be re-validated.
        """
        for scenario, ctx_json in [
            ['enableMultiTenancy absent from request, effectively enabled (schema default)',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=None,
                cars=factories.build_three_mixed_multitenancy_related_and_not_related_cars(),
                current_multitenancy_state=True)
            ],
        ]:
            with self.subTest(scenario):
                tests.assert_validation_allowed(self, self.run_hook(ctx_json), None)

    def test_module_config_denied_when_field_absent_from_request_and_effectively_disabled(self):
        for scenario, ctx_json in [
            ['enableMultiTenancy absent from request, effectively disabled',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=None,
                cars=factories.build_three_mixed_multitenancy_related_and_not_related_cars(),
                current_multitenancy_state=False)
            ],
            ['enableMultiTenancy absent from request, no state ConfigMap yet',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=None,
                cars=factories.build_three_mixed_multitenancy_related_and_not_related_cars(),
                current_multitenancy_state=None)
            ],
        ]:
            with self.subTest(scenario):
                tests.assert_validation_deny(self, self.run_hook(ctx_json), "; ".join([
                    "You must enable userAuthz.enableMultiTenancy to use the allowAccessToSystemNamespaces flag in ClusterAuthorizationRule 'user1'",
                    "You must enable userAuthz.enableMultiTenancy to use the namespaceSelector option in ClusterAuthorizationRule 'user1'",
                    "You must enable userAuthz.enableMultiTenancy to use the limitNamespaces option in ClusterAuthorizationRule 'user1'",
                    "You must enable userAuthz.enableMultiTenancy to use the allowAccessToSystemNamespaces flag in ClusterAuthorizationRule 'user3'",
                    "You must enable userAuthz.enableMultiTenancy to use the namespaceSelector option in ClusterAuthorizationRule 'user3'",
                    "You must enable userAuthz.enableMultiTenancy to use the limitNamespaces option in ClusterAuthorizationRule 'user3'",
                ]))

    def test_module_config_denied_when_explicit_field_is_removed_even_if_previously_enabled(self):
        """
        enableMultiTenancy WAS explicitly present in spec.settings before this request
        (e.g. "true"), and this request removes it. Unlike a request that never touched the
        field, removing an existing explicit value is itself a potential disable (e.g. on
        editions where the schema default is false) — the state ConfigMap still reflects
        the pre-request value, so it must not be trusted here; existing CARs have to be
        re-validated regardless of what the ConfigMap currently says.
        """
        for scenario, ctx_json in [
            ['enableMultiTenancy was true, request removes the field, state ConfigMap still says true',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=None,
                previous_enable_multitenancy_field=True,
                cars=factories.build_three_mixed_multitenancy_related_and_not_related_cars(),
                current_multitenancy_state=True)
            ],
        ]:
            with self.subTest(scenario):
                tests.assert_validation_deny(self, self.run_hook(ctx_json), "; ".join([
                    "You must enable userAuthz.enableMultiTenancy to use the allowAccessToSystemNamespaces flag in ClusterAuthorizationRule 'user1'",
                    "You must enable userAuthz.enableMultiTenancy to use the namespaceSelector option in ClusterAuthorizationRule 'user1'",
                    "You must enable userAuthz.enableMultiTenancy to use the limitNamespaces option in ClusterAuthorizationRule 'user1'",
                    "You must enable userAuthz.enableMultiTenancy to use the allowAccessToSystemNamespaces flag in ClusterAuthorizationRule 'user3'",
                    "You must enable userAuthz.enableMultiTenancy to use the namespaceSelector option in ClusterAuthorizationRule 'user3'",
                    "You must enable userAuthz.enableMultiTenancy to use the limitNamespaces option in ClusterAuthorizationRule 'user3'",
                ]))

    def test_module_config_allowed_when_field_absent_before_and_after_regardless_of_unrelated_edit(self):
        """
        Sanity check that an unrelated edit (field absent from both oldObject and object)
        still takes the ConfigMap-fallback shortcut — this is the case the fix must not
        regress while closing the "field removed" gap above.
        """
        for scenario, ctx_json in [
            ['enableMultiTenancy absent before and after, effectively enabled',
             factories.prepare_module_config_binding_context(
                module_enable_multitenancy_field=None,
                previous_enable_multitenancy_field=None,
                cars=factories.build_three_mixed_multitenancy_related_and_not_related_cars(),
                current_multitenancy_state=True)
            ],
        ]:
            with self.subTest(scenario):
                tests.assert_validation_allowed(self, self.run_hook(ctx_json), None)


if __name__ == '__main__':
    unittest.main()
