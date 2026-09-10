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

    The two directions of being wrong are not symmetric, and the tests are arranged around that.
    Letting a bad pattern through costs a quarantine, which is the behaviour without this check at
    all. Refusing a good one blocks an administrator from editing a rule that works. So the first
    group below - patterns RE2 accepts and Python's own re does not - is the one that matters most.
    """

    def car(self, *patterns):
        return DotMap({
            "metadata": {"name": "team-a"},
            "spec": {"limitNamespaces": list(patterns)},
        })

    def errors_for(self, *patterns):
        errors, _ = multitenancy.validate_car_limit_namespaces_patterns(self.car(*patterns))
        return errors

    def test_patterns_valid_in_re2_but_not_in_python_are_accepted(self):
        """These all raise re.error in Python and compile fine in Go.

        Deciding validity by compiling with Python's re would refuse every one of them, and each
        refusal is an administrator unable to save a rule the cluster is already running.
        """
        for pattern in [r"\z", r"\pL", r"\Q(\E", r"\Qa|b\E", r"\x{41}", "(?<name>a)", "(?U)a"]:
            with self.subTest(pattern=pattern):
                self.assertEqual([], self.errors_for(pattern), "must not be rejected: RE2 compiles it")

    def test_escaped_quantifiers_and_octal_escapes_are_accepted(self):
        """The same class of mistake as compiling with Python's re, from two other directions.

        In `a\\?+` the "?" is an escaped literal and the "+" repeats it: RE2 compiles it, and a check
        that reads the character before the "+" out of the raw string sees a possessive quantifier
        that is not there. And `\\123` is an octal escape, not a backreference - RE2 reads a
        backslash and two or three octal digits as one character.

        Every pattern here was compiled with Go's regexp to confirm it is accepted.
        """
        for pattern in [r"a\?+", r"a\*+", r"a\++", r"a\}+", r"\123", r"\12", r"\777",
                        r"\0", r"\000", r"[+]+", r"\Q+\E+"]:
            with self.subTest(pattern=pattern):
                self.assertEqual([], self.errors_for(pattern), "must not be rejected: RE2 compiles it")

    def test_quantifiers_and_escapes_re2_refuses_are_still_rejected(self):
        """The other half: what Go's regexp does refuse must keep being refused."""
        for pattern, description in [
            (r"a*+", "possessive quantifier"),
            (r"a?+", "possessive quantifier"),
            (r"a{2}+", "possessive quantifier"),
            (r"(a)++", "possessive quantifier"),
            (r"\1", "backreference"),
            (r"\9", "backreference"),
            (r"\18", "backreference"),
            (r"\800", "backreference"),
        ]:
            with self.subTest(pattern=pattern):
                errors = self.errors_for(pattern)
                self.assertEqual(1, len(errors), errors)
                self.assertIn(description, errors[0])

    def test_ordinary_patterns_are_accepted(self):
        for pattern in ["team-a", "team-.*", "team-[0-9]+", "team-.*|kube-system", "(a|b)-ns",
                        ".*", "(?i)team", "(?is)team", "(?i:team)", "a{2,10}", "a{1000}"]:
            with self.subTest(pattern=pattern):
                self.assertEqual([], self.errors_for(pattern))

    def test_structurally_broken_patterns_are_rejected(self):
        for pattern, reason in [
            ("team-(", "unclosed '('"),
            ("team-[", "unclosed '['"),
            ("team-a)", "unbalanced ')'"),
        ]:
            with self.subTest(pattern=pattern):
                errors = self.errors_for(pattern)
                self.assertEqual(1, len(errors), errors)
                self.assertIn(reason, errors[0])
                self.assertIn("team-a", errors[0])  # the rule is named

    def test_constructs_re2_does_not_have_are_rejected(self):
        """Python accepts every one of these, so compiling would not catch them."""
        for pattern, description in [
            ("team-(?=a)", "lookahead"),
            ("team-(?!a)", "negative lookahead"),
            ("(?<=team-)a", "lookbehind"),
            ("(?<!team-)a", "negative lookbehind"),
            ("(?>team)", "atomic group"),
            ("(?#note)a", "inline comment"),
            (r"(a)\1", "backreference"),
            ("(?(1)a|b)", "conditional"),
            ("a++", "possessive quantifier"),
            ("(?x)team", "inline flag"),
            ("(?a)team", "inline flag"),
            ("a{1001}", "above RE2's limit"),
        ]:
            with self.subTest(pattern=pattern):
                errors = self.errors_for(pattern)
                self.assertEqual(1, len(errors), errors)
                self.assertIn(description, errors[0])

    def test_a_metacharacter_inside_a_class_or_a_quote_is_a_literal(self):
        """A scan that does not understand [...] and \Q...\E reports imbalance that is not there."""
        for pattern in [r"[(]", r"[)]", r"[[]", r"\Q(\E", r"\Q)\E", r"\Q[\E", r"a\(b", r"a\)b"]:
            with self.subTest(pattern=pattern):
                self.assertEqual([], self.errors_for(pattern))

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
