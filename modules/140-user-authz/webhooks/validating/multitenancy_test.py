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
                    "You must enable userAuthz.enableMultiTenancy to use the allowAccessToSystemNamespaces flag in ClusterAuthorizationRule 'user1' (EE Only)",
                    "You must enable userAuthz.enableMultiTenancy to use the namespaceSelector option in ClusterAuthorizationRule 'user1' (EE Only)",
                    "You must enable userAuthz.enableMultiTenancy to use the limitNamespaces option in ClusterAuthorizationRule 'user1' (EE Only)",
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
                    "You must enable userAuthz.enableMultiTenancy to use the allowAccessToSystemNamespaces flag in ClusterAuthorizationRule 'user1' (EE Only)",
                    "You must enable userAuthz.enableMultiTenancy to use the namespaceSelector option in ClusterAuthorizationRule 'user1' (EE Only)",
                    "You must enable userAuthz.enableMultiTenancy to use the limitNamespaces option in ClusterAuthorizationRule 'user1' (EE Only)",
                    "You must enable userAuthz.enableMultiTenancy to use the allowAccessToSystemNamespaces flag in ClusterAuthorizationRule 'user3' (EE Only)",
                    "You must enable userAuthz.enableMultiTenancy to use the namespaceSelector option in ClusterAuthorizationRule 'user3' (EE Only)",
                    "You must enable userAuthz.enableMultiTenancy to use the limitNamespaces option in ClusterAuthorizationRule 'user3' (EE Only)",
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


if __name__ == '__main__':
    unittest.main()
