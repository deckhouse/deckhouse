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
from dotmap import DotMap

import validation_webhook_test_helper

security_policy = validation_webhook_test_helper.load("security_policy.py")


def _policy(name: str, references: list) -> dict:
    spec = {"policies": {}}
    if references:
        spec["policies"]["verifyImageSignatures"] = [
            {"reference": reference, "publicKeys": ["test-public-key"]} for reference in references
        ]
    return {"apiVersion": "deckhouse.io/v1alpha1", "kind": "SecurityPolicy", "metadata": {"name": name}, "spec": spec}


def _snapshot_item(name: str, references: list) -> dict:
    """A policies snapshot item as the binding's jqFilter builds it."""
    return {"filterResult": {"name": name, "references": references}}


def _run(request_object: dict, existing: list, operation: str = "CREATE"):
    ctx = {
        "binding": "securitypolicies.deckhouse.io",
        "review": {
            "request": {
                "uid": "5d1f4b0e-0000-0000-0000-000000000000",
                "kind": {"group": "deckhouse.io", "version": "v1alpha1", "kind": "SecurityPolicy"},
                "name": request_object["metadata"]["name"],
                "operation": operation,
                "object": request_object,
            }
        },
        "snapshots": {"policies": existing},
    }
    return hook.testrun(security_policy.main, [DotMap(ctx)])


class TestVerifyImageSignatures(unittest.TestCase):
    def test_policy_without_image_signatures_is_allowed(self):
        out = _run(_policy("plain", []), [_snapshot_item("other", ["registry.example.com/team/*"])])
        tests.assert_validation_allowed(self, out, None)

    def test_intersecting_reference_is_denied(self):
        out = _run(_policy("new", ["registry.example.com/*"]),
                   [_snapshot_item("existing", ["registry.example.com/team/*"])])
        tests.assert_validation_deny(
            self, out, 'ImageReference "registry.example.com/*" has intersection in the SecurityPolicy "existing"')

    def test_equal_reference_is_allowed(self):
        out = _run(_policy("new", ["registry.example.com/team/*"]),
                   [_snapshot_item("existing", ["registry.example.com/team/*"])])
        tests.assert_validation_allowed(self, out, None)

    def test_disjoint_reference_is_allowed(self):
        out = _run(_policy("new", ["registry.example.com/one/*"]),
                   [_snapshot_item("existing", ["registry.example.com/two/*"])])
        tests.assert_validation_allowed(self, out, None)

    def test_wildcard_reference_is_treated_as_default(self):
        out = _run(_policy("new", ["*"]), [_snapshot_item("existing", ["registry.example.com/team/*"])])
        tests.assert_validation_allowed(self, out, None)

    def test_update_does_not_intersect_with_itself(self):
        out = _run(_policy("existing", ["registry.example.com/*"]),
                   [_snapshot_item("existing", ["registry.example.com/team/*"])], operation="UPDATE")
        tests.assert_validation_allowed(self, out, None)

    def test_policy_without_image_signatures_in_snapshot_is_skipped(self):
        out = _run(_policy("new", ["registry.example.com/*"]), [_snapshot_item("existing", [])])
        tests.assert_validation_allowed(self, out, None)


if __name__ == "__main__":
    unittest.main()
