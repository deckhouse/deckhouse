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
import unittest

from deckhouse import hook, tests
from dotmap import DotMap

import reserved_policy_names as reserved
from reserved_policy_names import main


def _binding_context(name: str) -> DotMap:
    """The request the webhook is called with, cut down to the fields it reads."""
    return DotMap(json.loads("""
{
    "binding": "reserved-policy-names.deckhouse.io",
    "review": {
        "request": {
            "uid": "3f2a2c3a-4f39-4a53-9b4c-4a1f2b6f0f01",
            "kind": {"group": "deckhouse.io", "version": "v1alpha1", "kind": "SecurityPolicy"},
            "resource": {"group": "deckhouse.io", "version": "v1alpha1", "resource": "securitypolicies"},
            "name": "%s",
            "operation": "CREATE",
            "userInfo": {"username": "kubernetes-admin", "groups": ["system:masters"]},
            "object": {
                "apiVersion": "deckhouse.io/v1alpha1",
                "kind": "SecurityPolicy",
                "metadata": {"name": "%s"},
                "spec": {"match": {"namespaceSelector": {"labelSelector": {}}}, "policies": {}}
            },
            "dryRun": false
        }
    },
    "type": "Validating"
}
""" % (name, name)))


class TestCheckReservedName(unittest.TestCase):
    def test_ordinary_names_are_allowed(self):
        for name in ["foo", "d8-foo", "d8-system-warn-foo", "d8-system-excluded-foo", "system-default-foo", "foo-system", "d8-system"]:
            with self.subTest(name=name):
                self.assertIsNone(reserved.check_reserved_name(name))

    def test_reserved_prefixes_are_denied(self):
        for name in [
            "d8-system-default-foo",
            "d8-pod-security-baseline-deny-default",
        ]:
            with self.subTest(name=name):
                error = reserved.check_reserved_name(name)
                self.assertIsNotNone(error)
                self.assertIn(name, error)

    def test_the_documented_limit_is_the_one_in_force(self):
        # The security policy pages quote this number. Deriving it from the prefixes, as the
        # webhook does, hides a change of the list from the documentation — which is how the
        # pages came to promise 234 while the code allowed 235.
        self.assertEqual(reserved.MAX_NAME_LENGTH, 235)

    def test_a_name_that_leaves_room_for_the_prefix_is_allowed(self):
        self.assertIsNone(reserved.check_name_length("a" * reserved.MAX_NAME_LENGTH))

    def test_a_name_that_overflows_the_derived_constraint_is_denied(self):
        name = "a" * (reserved.MAX_NAME_LENGTH + 1)
        error = reserved.check_name_length(name)
        self.assertIsNotNone(error)
        self.assertIn(str(reserved.MAX_NAME_LENGTH), error)

    def test_every_derived_name_fits_the_object_name_limit(self):
        longest = "a" * reserved.MAX_NAME_LENGTH
        for prefix in reserved.RESERVED_NAME_PREFIXES:
            with self.subTest(prefix=prefix):
                self.assertLessEqual(len(prefix + longest), 253)

    def test_no_prefix_contains_another(self):
        # The prefixes exist so that the constraints derived from any two policies stay
        # distinct, which holds only while no prefix is a prefix of another.
        for one in reserved.RESERVED_NAME_PREFIXES:
            for other in reserved.RESERVED_NAME_PREFIXES:
                if one is not other:
                    self.assertFalse(one.startswith(other), f"{one} starts with {other}")



class TestWebhook(unittest.TestCase):
    """Drives the hook itself, so that the path from the request to the verdict is covered too."""

    def test_an_ordinary_name_is_allowed(self):
        out = hook.testrun(main, [_binding_context("my-policy")])
        tests.assert_validation_allowed(self, out, None)

    def test_a_reserved_prefix_is_denied(self):
        out = hook.testrun(main, [_binding_context("d8-system-default-my-policy")])
        tests.assert_validation_deny(
            self,
            out,
            'Name "d8-system-default-my-policy" starts with the reserved prefix '
            '"d8-system-default-". The module uses this prefix for the constraints it derives '
            "from a policy that applies to system namespaces. Choose another name.",
        )

    def test_a_name_that_leaves_no_room_for_the_prefix_is_denied(self):
        name = "a" * (reserved.MAX_NAME_LENGTH + 1)
        out = hook.testrun(main, [_binding_context(name)])
        tests.assert_validation_deny(
            self,
            out,
            f'Name "{name}" is {len(name)} characters long, which leaves no room for the prefix '
            "the module adds to the constraints it derives from a policy. "
            f"Use a name of at most {reserved.MAX_NAME_LENGTH} characters.",
        )


if __name__ == "__main__":
    unittest.main()
