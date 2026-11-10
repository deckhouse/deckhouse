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

from pathlib import Path
from deckhouse import hook
from deckhouse.tests import load_validation_webhook

dex_client_validation = load_validation_webhook(Path(__file__).parent / "dex_client_validation.py")


def dex_client(spec, deleting=False):
    obj = {
        "apiVersion": "deckhouse.io/v1",
        "kind": "DexClient",
        "metadata": {"name": "app", "namespace": "ns"},
        "spec": spec,
    }
    if deleting:
        obj["metadata"]["deletionTimestamp"] = "2026-01-01T00:00:00Z"
    return obj


def run(obj, providers):
    ctx = {
        "binding": "dexclients-providers.deckhouse.io",
        "type": "Validating",
        "review": {"request": {"operation": "CREATE", "object": obj}},
        "snapshots": {"dexproviders": [{"filterResult": {"name": name}} for name in providers]},
    }
    out = hook.testrun(dex_client_validation.main, [ctx])
    assert len(out.validations.data) == 1, out.validations.data
    return out.validations.data[0]


class TestAllowedProviders(unittest.TestCase):
    def test_known_providers_and_local_pass_without_a_warning(self):
        response = run(dex_client({"allowedProviders": ["corp-ldap", "local"]}), ["corp-ldap", "github"])
        self.assertTrue(response["allowed"])
        self.assertNotIn("warnings", response)

    def test_unknown_provider_is_allowed_with_a_warning(self):
        response = run(dex_client({"allowedProviders": ["corp-ldap", "gitlab", "github"]}), ["corp-ldap"])
        self.assertTrue(response["allowed"])
        self.assertEqual(len(response["warnings"]), 1)
        self.assertIn("'gitlab', 'github'", response["warnings"][0])
        self.assertNotIn("corp-ldap", response["warnings"][0])

    def test_no_allowed_providers_means_no_warning(self):
        for spec in ({}, {"allowedProviders": []}, {"redirectURIs": ["https://a.example.com/cb"]}):
            self.assertNotIn("warnings", run(dex_client(spec), []))

    def test_a_deleted_client_is_not_warned_about(self):
        self.assertNotIn("warnings", run(dex_client({"allowedProviders": ["gone"]}, deleting=True), []))


if __name__ == "__main__":
    unittest.main()
