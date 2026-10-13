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

from deckhouse import hook, tests

from pathlib import Path
from deckhouse.tests import load_validation_webhook

cluster_authorization_rule = load_validation_webhook(Path(__file__).parent / "cluster_authorization_rule.py")


def _run(spec, operation="CREATE"):
    obj = {
        "apiVersion": "deckhouse.io/v1",
        "kind": "ClusterAuthorizationRule",
        "metadata": {"name": "test"},
    }
    if spec is not None:
        obj["spec"] = spec
    ctx = {
        "binding": "cluster-authorization-rules.deckhouse.io",
        "type": "Validating",
        "review": {"request": {"operation": operation, "object": obj}},
    }
    return hook.testrun(cluster_authorization_rule.main, [ctx])


def _subjects(*subjects):
    return {"subjects": list(subjects), "accessLevel": "User"}


class TestAllowed(unittest.TestCase):
    def test_user_and_group(self):
        out = _run(_subjects({"kind": "User", "name": "admin@example.com"}, {"kind": "Group", "name": "admins"}))
        tests.assert_validation_allowed(self, out, None)

    def test_service_account_with_namespace(self):
        for name in ("sa", "a.b-c", "0", "sa-1.sa-2", "trailing\n"):
            out = _run(_subjects({"kind": "ServiceAccount", "name": name, "namespace": "default"}), "UPDATE")
            tests.assert_validation_allowed(self, out, None)

    def test_service_account_named_false_without_namespace(self):
        # The old hook read the name "false" as "no failure".
        out = _run(_subjects({"kind": "ServiceAccount", "name": "false"}))
        tests.assert_validation_allowed(self, out, None)


class TestDenied(unittest.TestCase):
    def test_empty_subjects(self):
        for spec in ({"subjects": []}, {"subjects": None}, {}, None):
            out = _run(spec)
            tests.assert_validation_deny(self, out, "error: Field <.spec.subjects> must not be empty")

    def test_invalid_service_account_name(self):
        out = _run(_subjects({"kind": "ServiceAccount", "name": "Bad_Name", "namespace": "default"}))
        tests.assert_validation_deny(self, out, "error: ServiceAccount name is invalid - Bad_Name ;")

    def test_invalid_name_loses_trailing_line_breaks(self):
        out = _run(_subjects({"kind": "ServiceAccount", "name": "a\n\n", "namespace": "default"}))
        tests.assert_validation_deny(self, out, "error: ServiceAccount name is invalid - a ;")

    def test_service_account_without_namespace(self):
        out = _run(_subjects({"kind": "ServiceAccount", "name": "sa"}))
        tests.assert_validation_deny(self, out, "error: Namespace for the ServiceAccount sa is not provided ;")

    def test_invalid_service_account_without_namespace(self):
        out = _run(_subjects({"kind": "ServiceAccount", "name": "-sa"}))
        tests.assert_validation_deny(
            self,
            out,
            "error: ServiceAccount name is invalid - -sa ; Namespace for the ServiceAccount -sa is not provided ;",
        )

    def test_user_and_group_with_namespace(self):
        out = _run(_subjects(
            {"kind": "User", "name": "alice", "namespace": "default"},
            {"kind": "Group", "name": "devs", "namespace": "default"},
        ))
        tests.assert_validation_deny(
            self,
            out,
            "error: Namespace for the User alice must not be provided ; Namespace for the Group devs must not be provided ;",
        )

    def test_messages_accumulate_in_order(self):
        out = _run(_subjects(
            {"kind": "User", "name": "alice"},
            {"kind": "ServiceAccount", "name": "UPPER"},
            {"kind": "Group", "name": "devs", "namespace": "ns"},
        ))
        tests.assert_validation_deny(
            self,
            out,
            "error: ServiceAccount name is invalid - UPPER ; Namespace for the ServiceAccount UPPER is not provided ;"
            " Namespace for the Group devs must not be provided ;",
        )


class TestFileNamePatterns(unittest.TestCase):
    """The old hook failed on a subject whose JSON Bash took for a file name pattern."""

    def test_pattern_fails_the_hook(self):
        # SystemExit, which the template's main lets through: the hook exits with 1 as before.
        for name in ("dev*", "dev?", "team[1]", "a\\*"):
            with self.assertRaises(SystemExit, msg=name):
                _run(_subjects({"kind": "Group", "name": "fine"}, {"kind": "Group", "name": name}))

    def test_bracket_without_pattern(self):
        for name in ("a[", "a]", "][", "[/]", "a\\b", 'a"b'):
            out = _run(_subjects({"kind": "Group", "name": name}))
            tests.assert_validation_allowed(self, out, None)

    def test_looks_like_glob(self):
        cases = {
            '{"n":"[]"}': True,
            '{"n":"[\\\\]"}': True,
            '{"n":"\\\\*"}': True,
            '{"n":"a[","m":"b]"}': True,
            '{"n":"a[/","m":"b]"}': False,
            '{"n":"\\\\\\\\"}': False,
            '{"n":"a\\"b"}': False,
        }
        for word, expected in cases.items():
            self.assertEqual(cluster_authorization_rule.looks_like_glob(word), expected, word)


if __name__ == "__main__":
    unittest.main()
