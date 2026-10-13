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

system_roles = load_validation_webhook(Path(__file__).parent / "system_roles.py")

USER = "kubernetes-admin"
DENIED = 'ClusterRole "{}" has forbidden label "heritage:deckhouse"'


def _role(name="d8:manage:all:admin", labels=None):
    metadata = {"name": name}
    if labels is not None:
        metadata["labels"] = labels
    return {"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "ClusterRole", "metadata": metadata}


def _run(operation, obj, old=None, user=USER):
    request = {"operation": operation, "userInfo": {"username": user}, "object": obj}
    if old is not None:
        request["oldObject"] = old
    ctx = {"binding": "system-roles.deckhouse.io", "type": "Validating", "review": {"request": request}}
    return hook.testrun(system_roles.main, [ctx])


class TestCreate(unittest.TestCase):
    def test_marked_role_is_denied(self):
        out = _run("CREATE", _role(labels={"heritage": "deckhouse"}))
        tests.assert_validation_deny(self, out, DENIED.format("d8:manage:all:admin"))

    def test_deckhouse_may_create(self):
        out = _run("CREATE", _role(labels={"heritage": "deckhouse"}), user=system_roles.DECKHOUSE_SA)
        tests.assert_validation_allowed(self, out, None)

    def test_user_name_with_trailing_line_break_counts_as_deckhouse(self):
        out = _run("CREATE", _role(), user=system_roles.DECKHOUSE_SA + "\n")
        tests.assert_validation_allowed(self, out, None)

    def test_missing_name(self):
        out = _run("CREATE", {"metadata": {}})
        tests.assert_validation_deny(self, out, DENIED.format("null"))


class TestUpdate(unittest.TestCase):
    def test_role_already_marked(self):
        out = _run("UPDATE", _role(labels={"heritage": "deckhouse"}), _role(labels={"heritage": "deckhouse"}))
        tests.assert_validation_allowed(self, out, None)

    def test_label_removed(self):
        out = _run("UPDATE", _role(labels={}), _role(labels={"heritage": "deckhouse"}))
        tests.assert_validation_allowed(self, out, None)

    def test_label_added(self):
        for old in (_role(), _role(labels={}), _role(labels={"heritage": "other"}), _role(labels={"heritage": ""})):
            out = _run("UPDATE", _role(name="mine", labels={"heritage": "deckhouse"}), old)
            tests.assert_validation_deny(self, out, DENIED.format("mine"))

    def test_without_old_object(self):
        out = _run("UPDATE", _role(name="mine", labels={"heritage": "deckhouse"}))
        tests.assert_validation_deny(self, out, DENIED.format("mine"))

    def test_deckhouse_may_mark(self):
        out = _run("UPDATE", _role(labels={"heritage": "deckhouse"}), _role(), user=system_roles.DECKHOUSE_SA)
        tests.assert_validation_allowed(self, out, None)


class TestOtherOperations(unittest.TestCase):
    def test_allowed(self):
        for operation in ("DELETE", "CONNECT", "", None):
            out = _run(operation, _role(labels={"heritage": "deckhouse"}))
            tests.assert_validation_allowed(self, out, None)


if __name__ == "__main__":
    unittest.main()
