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
import json

import identity_collision
import identity_collision_test_factories as factories
from identity_collision import COLLISION_ANNOTATION
from deckhouse import hook, tests
from dotmap import DotMap


class TestIdentityCollisionWithAuthorizationRules(unittest.TestCase):

    def run_hook(self, context_json: str):
        return hook.testrun(identity_collision.main, [DotMap(json.loads(context_json))])

    def deny_message(self, resource: str, field: str, value: str, rule: str) -> str:
        return (f'{resource} "{field}" "{value}" is already granted privileges by {rule}; '
                f'use a different value or set the "{COLLISION_ANNOTATION}: true" annotation '
                f'to confirm the collision')

    def warn_message(self, resource: str, field: str, value: str, rule: str) -> str:
        return (f'{resource} "{field}" "{value}" is already granted privileges by {rule}; '
                f'it inherits the privileges granted by that rule')

    # Group

    def test_group_denied_when_name_is_a_rule_group_subject(self):
        for scenario, group_name, rule in [
            ['ClusterAuthorizationRule subject',
             factories.CLUSTER_RULE_GROUP_SUBJECT, factories.CLUSTER_RULE_DESCRIPTION],
            ['AuthorizationRule subject',
             factories.NAMESPACED_RULE_GROUP_SUBJECT, factories.NAMESPACED_RULE_DESCRIPTION],
        ]:
            with self.subTest(scenario):
                out = self.run_hook(factories.prepare_group_binding_context(group_name))
                tests.assert_validation_deny(self, out, self.deny_message(
                    "groups.deckhouse.io", ".spec.name", group_name, rule))

    def test_group_allowed_when_name_is_not_a_rule_subject(self):
        out = self.run_hook(factories.prepare_group_binding_context("harmless-group"))
        tests.assert_validation_allowed(self, out, None)

    def test_group_allowed_when_no_rules_exist(self):
        out = self.run_hook(factories.prepare_group_binding_context(
            factories.CLUSTER_RULE_GROUP_SUBJECT, with_rules=False))
        tests.assert_validation_allowed(self, out, None)

    def test_group_allowed_with_warning_when_collision_is_acknowledged(self):
        group_name = factories.CLUSTER_RULE_GROUP_SUBJECT
        out = self.run_hook(factories.prepare_group_binding_context(group_name, acknowledged=True))
        tests.assert_validation_allowed(self, out, self.warn_message(
            "groups.deckhouse.io", ".spec.name", group_name, factories.CLUSTER_RULE_DESCRIPTION))

    def test_group_denied_when_renamed_into_a_collision(self):
        group_name = factories.CLUSTER_RULE_GROUP_SUBJECT
        out = self.run_hook(factories.prepare_group_binding_context(
            group_name, operation="UPDATE", old_group_name="harmless-group"))
        tests.assert_validation_deny(self, out, self.deny_message(
            "groups.deckhouse.io", ".spec.name", group_name, factories.CLUSTER_RULE_DESCRIPTION))

    def test_group_allowed_with_warning_when_collision_already_existed(self):
        group_name = factories.CLUSTER_RULE_GROUP_SUBJECT
        out = self.run_hook(factories.prepare_group_binding_context(
            group_name, operation="UPDATE", old_group_name=group_name))
        tests.assert_validation_allowed(self, out, self.warn_message(
            "groups.deckhouse.io", ".spec.name", group_name, factories.CLUSTER_RULE_DESCRIPTION))

    # User

    def test_user_denied_when_email_is_a_rule_user_subject(self):
        for scenario, email, rule in [
            ['ClusterAuthorizationRule subject',
             factories.CLUSTER_RULE_USER_SUBJECT, factories.CLUSTER_RULE_DESCRIPTION],
            ['AuthorizationRule subject',
             factories.NAMESPACED_RULE_USER_SUBJECT, factories.NAMESPACED_RULE_DESCRIPTION],
        ]:
            with self.subTest(scenario):
                out = self.run_hook(factories.prepare_user_binding_context(email))
                tests.assert_validation_deny(self, out, self.deny_message(
                    "users.deckhouse.io", ".spec.email", email, rule))

    def test_user_allowed_when_email_is_not_a_rule_subject(self):
        out = self.run_hook(factories.prepare_user_binding_context("harmless@example.com"))
        tests.assert_validation_allowed(self, out, None)

    def test_user_allowed_with_warning_when_collision_is_acknowledged(self):
        email = factories.CLUSTER_RULE_USER_SUBJECT
        out = self.run_hook(factories.prepare_user_binding_context(email, acknowledged=True))
        tests.assert_validation_allowed(self, out, self.warn_message(
            "users.deckhouse.io", ".spec.email", email, factories.CLUSTER_RULE_DESCRIPTION))

    def test_user_denied_when_email_changed_into_a_collision(self):
        email = factories.CLUSTER_RULE_USER_SUBJECT
        out = self.run_hook(factories.prepare_user_binding_context(
            email, operation="UPDATE", old_email="harmless@example.com"))
        tests.assert_validation_deny(self, out, self.deny_message(
            "users.deckhouse.io", ".spec.email", email, factories.CLUSTER_RULE_DESCRIPTION))

    def test_user_allowed_with_warning_when_collision_already_existed(self):
        email = factories.CLUSTER_RULE_USER_SUBJECT
        out = self.run_hook(factories.prepare_user_binding_context(
            email, operation="UPDATE", old_email=email))
        tests.assert_validation_allowed(self, out, self.warn_message(
            "users.deckhouse.io", ".spec.email", email, factories.CLUSTER_RULE_DESCRIPTION))

    # A rule subject written in another case never matches the token Dex issues, because Dex
    # lowercases the email, so reporting it would be a false positive.
    def test_user_allowed_when_email_differs_only_by_case(self):
        out = self.run_hook(factories.prepare_user_binding_context(
            factories.CLUSTER_RULE_USER_SUBJECT.upper()))
        tests.assert_validation_allowed(self, out, None)

    # Delete

    def test_user_deletion_warns_that_the_rule_survives_it(self):
        email = factories.CLUSTER_RULE_USER_SUBJECT
        out = self.run_hook(factories.prepare_user_binding_context(email, operation="DELETE"))
        tests.assert_validation_allowed(self, out, (
            f'{factories.CLUSTER_RULE_DESCRIPTION} still grants privileges to '
            f'".spec.email" "{email}"'))

    def test_user_deletion_is_silent_when_no_rule_grants_the_email(self):
        out = self.run_hook(factories.prepare_user_binding_context(
            "harmless@example.com", operation="DELETE"))
        tests.assert_validation_allowed(self, out, None)


if __name__ == '__main__':
    unittest.main()
