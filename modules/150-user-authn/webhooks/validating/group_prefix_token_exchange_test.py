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
import subprocess
import unittest

from pathlib import Path
from deckhouse import hook
from deckhouse.tests import load_validation_webhook

group_prefix_token_exchange = load_validation_webhook(Path(__file__).parent / "group_prefix_token_exchange.py")

# A copy of the jqFilter of the dexproviders context binding in
# templates/group-prefix-token-exchange-validation-webhook.yaml; the template tests keep the two identical.
JQ_FILTER = '''
{
  "name": .metadata.name,
  "enabled": (.spec.enabled != false),
  "groupPrefix": (.spec.groupPrefix // "")
}
'''


def snapshot(obj):
    """Runs the jqFilter of the hook over a DexProvider, as shell-operator does for the snapshot."""
    out = subprocess.run(["jq", JQ_FILTER], input=json.dumps(obj), capture_output=True, text=True, check=True)
    return {"filterResult": json.loads(out.stdout)}


def provider(name, prefix=None, enabled=None):
    spec = {"type": "LDAP", "displayName": name, "ldap": {}}
    if prefix is not None:
        spec["groupPrefix"] = prefix
    if enabled is not None:
        spec["enabled"] = enabled
    return {"apiVersion": "deckhouse.io/v1", "kind": "DexProvider", "metadata": {"name": name}, "spec": spec}


def exchange(name, *prefixes, deleting=False):
    groups = [{"claim": "groups", "prefix": prefix} for prefix in prefixes]
    obj = {"apiVersion": "deckhouse.io/v1alpha1", "kind": "TokenExchangeProvider", "metadata": {"name": name},
           "spec": {"identity": {"groups": groups}}}
    if deleting:
        obj["metadata"]["deletionTimestamp"] = "2026-01-01T00:00:00Z"
    return obj


def run(obj, *providers):
    ctx = {
        "binding": "groupprefix-token-exchange-providers.deckhouse.io",
        "type": "Validating",
        "review": {"request": {"operation": "CREATE", "object": obj}},
        "snapshots": {"dexproviders": [snapshot(p) for p in providers]},
    }
    out = hook.testrun(group_prefix_token_exchange.main, [ctx])
    assert len(out.validations.data) == 1, out.validations.data
    return out.validations.data[0]


class TestGroupPrefixOfTokenExchangeProvider(unittest.TestCase):
    def test_refuses_the_prefix_of_a_dex_provider(self):
        response = run(exchange("ci", "gh:", "corp:"), provider("ldap", "corp:"))
        self.assertFalse(response["allowed"])
        self.assertIn("'corp:'", response["message"])
        self.assertIn("DexProvider ldap", response["message"])

    def test_refuses_nested_prefixes(self):
        self.assertFalse(run(exchange("ci", "corp:ci:"), provider("ldap", "corp:"))["allowed"])
        self.assertFalse(run(exchange("ci", "corp:"), provider("ldap", "corp:ldap:"))["allowed"])

    def test_allows_prefixes_that_differ(self):
        self.assertTrue(run(exchange("ci", "ci:"), provider("ldap", "corp:"), provider("oidc", "idp:"))["allowed"])

    def test_ignores_a_disabled_provider_and_one_without_a_prefix(self):
        self.assertTrue(run(exchange("ci", "corp:"), provider("a", "corp:", enabled=False), provider("b"))["allowed"])

    def test_allows_a_provider_without_group_prefixes(self):
        self.assertTrue(run(exchange("ci"), provider("ldap", "corp:"))["allowed"])

    def test_leaves_a_deleted_provider_alone(self):
        self.assertTrue(run(exchange("ci", "corp:", deleting=True), provider("ldap", "corp:"))["allowed"])

    def test_without_dex_providers(self):
        self.assertTrue(run(exchange("ci", "corp:"))["allowed"])


if __name__ == "__main__":
    unittest.main()
