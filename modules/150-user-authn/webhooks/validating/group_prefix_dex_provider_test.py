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

group_prefix_dex_provider = load_validation_webhook(Path(__file__).parent / "group_prefix_dex_provider.py")

# Copies of the jqFilters of the dexproviders and tokenexchangeproviders context bindings in
# templates/group-prefix-dex-provider-validation-webhook.yaml; the template tests keep them identical.
JQ_FILTER = '''
{
  "name": .metadata.name,
  "enabled": (.spec.enabled != false),
  "groupPrefix": (.spec.groupPrefix // "")
}
'''
TEP_JQ_FILTER = '''
{
  "name": .metadata.name,
  "groupPrefixes": [(.spec.identity.groups // [])[] | .prefix | select(. != null and . != "")]
}
'''


def snapshot(jq_filter, obj):
    """Runs a jqFilter of the hook over an object, as shell-operator does for the snapshot."""
    out = subprocess.run(["jq", jq_filter], input=json.dumps(obj), capture_output=True, text=True, check=True)
    return {"filterResult": json.loads(out.stdout)}


def provider(name, prefix=None, enabled=None, deleting=False):
    spec = {"type": "LDAP", "displayName": name, "ldap": {}}
    if prefix is not None:
        spec["groupPrefix"] = prefix
    if enabled is not None:
        spec["enabled"] = enabled
    obj = {"apiVersion": "deckhouse.io/v1", "kind": "DexProvider", "metadata": {"name": name}, "spec": spec}
    if deleting:
        obj["metadata"]["deletionTimestamp"] = "2026-01-01T00:00:00Z"
    return obj


def exchange(name, *prefixes):
    groups = [{"claim": "groups", "prefix": prefix} for prefix in prefixes]
    return {"apiVersion": "deckhouse.io/v1alpha1", "kind": "TokenExchangeProvider", "metadata": {"name": name},
            "spec": {"identity": {"groups": groups}}}


def run(obj, providers=(), exchanges=()):
    ctx = {
        "binding": "groupprefix-dex-providers.deckhouse.io",
        "type": "Validating",
        "review": {"request": {"operation": "CREATE", "object": obj}},
        "snapshots": {
            "dexproviders": [snapshot(JQ_FILTER, p) for p in providers],
            "tokenexchangeproviders": [snapshot(TEP_JQ_FILTER, e) for e in exchanges],
        },
    }
    out = hook.testrun(group_prefix_dex_provider.main, [ctx])
    assert len(out.validations.data) == 1, out.validations.data
    return out.validations.data[0]


class TestGroupPrefixOfDexProvider(unittest.TestCase):
    def test_refuses_the_prefix_of_another_provider(self):
        response = run(provider("b", "corp:"), providers=[provider("a", "corp:")])
        self.assertFalse(response["allowed"])
        self.assertIn("'corp:'", response["message"])
        self.assertIn("DexProvider a", response["message"])

    def test_refuses_a_prefix_that_starts_with_another(self):
        self.assertFalse(run(provider("b", "corp:ldap:"), providers=[provider("a", "corp:")])["allowed"])

    def test_refuses_a_prefix_that_another_starts_with(self):
        self.assertFalse(run(provider("b", "corp:"), providers=[provider("a", "corp:ldap:")])["allowed"])

    def test_allows_prefixes_that_differ(self):
        # The colon at the end keeps corp: and corp2: apart.
        self.assertTrue(run(provider("b", "corp2:"), providers=[provider("a", "corp:")])["allowed"])
        self.assertTrue(run(provider("b", "ldap:"), providers=[provider("a", "oidc:")])["allowed"])

    def test_refuses_the_prefix_of_a_token_exchange_provider(self):
        response = run(provider("b", "ci:"), exchanges=[exchange("ci-tokens", "other:", "ci:")])
        self.assertFalse(response["allowed"])
        self.assertIn("TokenExchangeProvider ci-tokens", response["message"])

    def test_refuses_a_prefix_nested_with_one_of_a_token_exchange_provider(self):
        self.assertFalse(run(provider("b", "ci:ldap:"), exchanges=[exchange("ci-tokens", "ci:")])["allowed"])
        self.assertFalse(run(provider("b", "ci:"), exchanges=[exchange("ci-tokens", "ci:ldap:")])["allowed"])

    def test_allows_a_token_exchange_provider_with_other_prefixes_or_none(self):
        self.assertTrue(run(provider("b", "ci:"), exchanges=[exchange("a", "gh:"), exchange("none")])["allowed"])

    def test_an_update_does_not_meet_the_provider_itself(self):
        self.assertTrue(run(provider("a", "corp:"), providers=[provider("a", "corp:")])["allowed"])

    def test_ignores_a_disabled_provider_among_the_others(self):
        self.assertTrue(run(provider("b", "corp:"), providers=[provider("a", "corp:", enabled=False)])["allowed"])

    def test_ignores_a_provider_without_a_prefix_among_the_others(self):
        self.assertTrue(run(provider("b", "corp:"), providers=[provider("a")])["allowed"])

    def test_allows_a_disabled_provider(self):
        self.assertTrue(run(provider("b", "corp:", enabled=False), providers=[provider("a", "corp:")])["allowed"])

    def test_allows_a_provider_without_a_prefix(self):
        self.assertTrue(run(provider("b"), providers=[provider("a", "corp:")], exchanges=[exchange("e", "ci:")])["allowed"])

    def test_leaves_a_deleted_provider_alone(self):
        self.assertTrue(run(provider("b", "corp:", deleting=True), providers=[provider("a", "corp:")])["allowed"])

    def test_without_other_objects(self):
        self.assertTrue(run(provider("a", "corp:"))["allowed"])


if __name__ == "__main__":
    unittest.main()
