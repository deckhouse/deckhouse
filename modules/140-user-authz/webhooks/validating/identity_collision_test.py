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

import json
import shutil
import subprocess
import unittest

import identity_collision
import identity_collision_test_factories as factories
import yaml
from identity_collision import COLLISION_ANNOTATION
from deckhouse import hook, tests
from dotmap import DotMap


class TestIdentityCollisionWithAuthorizationRules(unittest.TestCase):

    def run_hook(self, context_json: str):
        return hook.testrun(identity_collision.main, [DotMap(json.loads(context_json))])

    def assert_allowed_without_warnings(self, out):
        """
        tests.assert_validation_allowed(..., None) returns early without looking at the warnings,
        so a test that must observe silence has to check it itself.
        """
        self.assertEqual(len(out.validations.data), 1)
        self.assertTrue(out.validations.data[0]["allowed"])
        self.assertNotIn("warnings", out.validations.data[0])

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
        self.assert_allowed_without_warnings(out)

    def test_group_allowed_when_no_rules_exist(self):
        out = self.run_hook(factories.prepare_group_binding_context(
            factories.CLUSTER_RULE_GROUP_SUBJECT, with_rules=False))
        self.assert_allowed_without_warnings(out)

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

    def test_group_denied_when_the_old_object_cannot_be_read(self):
        """An UPDATE with an unreadable oldObject counts as introducing the name: fail closed."""
        group_name = factories.CLUSTER_RULE_GROUP_SUBJECT
        out = self.run_hook(factories.prepare_group_binding_context(
            group_name, operation="UPDATE", with_old_object=False))
        tests.assert_validation_deny(self, out, self.deny_message(
            "groups.deckhouse.io", ".spec.name", group_name, factories.CLUSTER_RULE_DESCRIPTION))

    # Group.spec.name is not normalised anywhere between the Group object and the "groups" claim
    # (modules/150-user-authn/hooks/get_dex_user_crds.go, makeUserGroupsMap builds the claim from
    # group.Spec.Name verbatim), so a case-only difference is a genuinely different group and
    # reporting it either way round would be a false positive.
    def test_group_allowed_when_name_differs_from_the_subject_only_by_case(self):
        out = self.run_hook(factories.prepare_group_binding_context(
            factories.CLUSTER_RULE_GROUP_SUBJECT.upper()))
        self.assert_allowed_without_warnings(out)

    def test_group_allowed_when_the_rule_subject_differs_only_by_case(self):
        out = self.run_hook(factories.prepare_group_binding_context(
            factories.MIXED_CASE_RULE_GROUP_SUBJECT.lower(),
            cluster_rule_group_subjects=[factories.MIXED_CASE_RULE_GROUP_SUBJECT]))
        self.assert_allowed_without_warnings(out)

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
        self.assert_allowed_without_warnings(out)

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

    def test_user_denied_when_the_old_object_cannot_be_read(self):
        """An UPDATE with an unreadable oldObject counts as introducing the email: fail closed."""
        email = factories.CLUSTER_RULE_USER_SUBJECT
        out = self.run_hook(factories.prepare_user_binding_context(
            email, operation="UPDATE", with_old_object=False))
        tests.assert_validation_deny(self, out, self.deny_message(
            "users.deckhouse.io", ".spec.email", email, factories.CLUSTER_RULE_DESCRIPTION))

    # Email case. Deckhouse lowercases spec.email before it reaches the Password object
    # (modules/150-user-authn/hooks/get_dex_user_crds.go:276), and the username claim the API
    # server consumes is that email (modules/040-control-plane-manager/templates/
    # _authentication_configuration.tpl:18-20). The two directions are therefore not symmetric.

    def test_user_denied_when_email_reaches_the_subject_only_after_lowercasing(self):
        """
        The incoming email is mixed case and the rule subject is lowercase.

        The issued token carries the lowercased email, so the rule grants it. Comparing verbatim
        would let an uppercase spelling walk past the check.
        """
        email = factories.CLUSTER_RULE_USER_SUBJECT.upper()
        out = self.run_hook(factories.prepare_user_binding_context(email))
        tests.assert_validation_deny(self, out, self.deny_message(
            "users.deckhouse.io", ".spec.email", email, factories.CLUSTER_RULE_DESCRIPTION))

    def test_user_allowed_with_warning_when_a_colliding_email_only_changes_case(self):
        """
        Both spellings lowercase to the same rule subject, so the collision is not being
        introduced by this UPDATE and the object must stay modifiable.
        """
        email = factories.CLUSTER_RULE_USER_SUBJECT.upper()
        out = self.run_hook(factories.prepare_user_binding_context(
            email, operation="UPDATE", old_email=factories.CLUSTER_RULE_USER_SUBJECT.capitalize()))
        tests.assert_validation_allowed(self, out, self.warn_message(
            "users.deckhouse.io", ".spec.email", email, factories.CLUSTER_RULE_DESCRIPTION))

    def test_user_allowed_when_the_rule_subject_itself_is_mixed_case(self):
        """
        The rule subject is mixed case and the incoming email is already lowercase.

        Subjects reach the RoleBinding verbatim and RBAC matches them exactly, so such a subject
        can never match an issued token and reporting it would be a false positive.
        """
        out = self.run_hook(factories.prepare_user_binding_context(
            factories.MIXED_CASE_RULE_USER_SUBJECT.lower(),
            cluster_rule_user_subjects=[factories.MIXED_CASE_RULE_USER_SUBJECT]))
        self.assert_allowed_without_warnings(out)

    # Delete

    def test_user_deletion_warns_that_the_rule_survives_it(self):
        email = factories.CLUSTER_RULE_USER_SUBJECT
        out = self.run_hook(factories.prepare_user_binding_context(email, operation="DELETE"))
        tests.assert_validation_allowed(self, out, (
            f'{factories.CLUSTER_RULE_DESCRIPTION} still grants privileges to '
            f'".spec.email" "{email}"'))

    def test_user_deletion_warns_for_a_mixed_case_email(self):
        email = factories.CLUSTER_RULE_USER_SUBJECT.upper()
        out = self.run_hook(factories.prepare_user_binding_context(email, operation="DELETE"))
        tests.assert_validation_allowed(self, out, (
            f'{factories.CLUSTER_RULE_DESCRIPTION} still grants privileges to '
            f'".spec.email" "{email}"'))

    def test_user_deletion_is_silent_when_no_rule_grants_the_email(self):
        out = self.run_hook(factories.prepare_user_binding_context(
            "harmless@example.com", operation="DELETE"))
        self.assert_allowed_without_warnings(out)

    def test_group_deletion_warns_that_the_rule_survives_it(self):
        group_name = factories.CLUSTER_RULE_GROUP_SUBJECT
        out = self.run_hook(factories.prepare_group_binding_context(
            group_name, operation="DELETE"))
        tests.assert_validation_allowed(self, out, (
            f'{factories.CLUSTER_RULE_DESCRIPTION} still grants privileges to '
            f'".spec.name" "{group_name}"'))

    def test_group_deletion_is_silent_when_no_rule_grants_the_name(self):
        out = self.run_hook(factories.prepare_group_binding_context(
            "harmless-group", operation="DELETE"))
        self.assert_allowed_without_warnings(out)


@unittest.skipUnless(shutil.which("jq"), "jq is required to execute the hook's jqFilter programs")
class TestSnapshotJQFilters(unittest.TestCase):
    """
    Run the jqFilter programs from CONFIG the way shell-operator would.

    The binding contexts above carry a hand-built filterResult, so without this the filters
    themselves are never executed and a change to either of them would go unnoticed.
    """

    @classmethod
    def setUpClass(cls):
        config = yaml.safe_load(identity_collision.CONFIG)
        cls.filters = {b["name"]: b["jqFilter"] for b in config["kubernetes"]}

    def run_filter(self, snapshot_name: str, obj: dict) -> dict:
        result = subprocess.run(
            ["jq", "-c", self.filters[snapshot_name]],
            input=json.dumps(obj), capture_output=True, text=True, check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def test_config_declares_both_snapshots(self):
        self.assertEqual(
            sorted(self.filters),
            sorted([identity_collision.CLUSTER_RULES_SNAPSHOT_NAME,
                    identity_collision.NAMESPACED_RULES_SNAPSHOT_NAME]))

    def test_cluster_rule_filter_keeps_only_group_and_user_subjects(self):
        out = self.run_filter(
            identity_collision.CLUSTER_RULES_SNAPSHOT_NAME,
            factories.prepare_cluster_authorization_rule(factories.MIXED_KIND_SUBJECTS))
        self.assertEqual(out, {
            "name": "admin-rule",
            "groupSubjects": [factories.CLUSTER_RULE_GROUP_SUBJECT],
            "userSubjects": [factories.CLUSTER_RULE_USER_SUBJECT],
        })

    def test_namespaced_rule_filter_keeps_only_group_and_user_subjects(self):
        out = self.run_filter(
            identity_collision.NAMESPACED_RULES_SNAPSHOT_NAME,
            factories.prepare_authorization_rule(factories.MIXED_KIND_SUBJECTS))
        self.assertEqual(out, {
            "name": "team-rule",
            "namespace": "team-a",
            "groupSubjects": [factories.CLUSTER_RULE_GROUP_SUBJECT],
            "userSubjects": [factories.CLUSTER_RULE_USER_SUBJECT],
        })

    def test_filters_tolerate_a_rule_without_subjects(self):
        """The `[]?` guard: `.spec.subjects[]` on a missing key would abort the whole filter."""
        for snapshot_name, rule in [
            (identity_collision.CLUSTER_RULES_SNAPSHOT_NAME,
             factories.prepare_cluster_authorization_rule(None)),
            (identity_collision.NAMESPACED_RULES_SNAPSHOT_NAME,
             factories.prepare_authorization_rule(None)),
        ]:
            with self.subTest(snapshot_name):
                out = self.run_filter(snapshot_name, rule)
                self.assertEqual(out["groupSubjects"], [])
                self.assertEqual(out["userSubjects"], [])

    def test_filter_output_feeds_the_lookup(self):
        """The filter output is what granting_rule_for indexes, so wire the two together."""
        filter_result = self.run_filter(
            identity_collision.CLUSTER_RULES_SNAPSHOT_NAME,
            factories.prepare_cluster_authorization_rule(factories.MIXED_KIND_SUBJECTS))
        ctx = DotMap({"snapshots": {
            identity_collision.CLUSTER_RULES_SNAPSHOT_NAME: [{"filterResult": filter_result}],
            identity_collision.NAMESPACED_RULES_SNAPSHOT_NAME: [],
        }})

        self.assertEqual(
            identity_collision.granting_rule_for(
                ctx, identity_collision.IDENTITY_KINDS["user"],
                factories.CLUSTER_RULE_USER_SUBJECT.upper()),
            factories.CLUSTER_RULE_DESCRIPTION)
        self.assertIsNone(
            identity_collision.granting_rule_for(
                ctx, identity_collision.IDENTITY_KINDS["user"], "builder"))


if __name__ == '__main__':
    unittest.main()
