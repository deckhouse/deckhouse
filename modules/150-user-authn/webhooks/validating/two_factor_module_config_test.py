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

two_factor_module_config = load_validation_webhook(Path(__file__).parent / "two_factor_module_config.py")

# A copy of the jqFilter of the dexproviders context binding in
# templates/two-factor-module-config-validation-webhook.yaml; the template tests keep the two identical.
JQ_FILTER = '''
{
  "name": .metadata.name,
  "type": .spec.type,
  "enabled": (.spec.enabled != false),
  "basicAuth": ((if .spec.type == "LDAP" then .spec.ldap.enableBasicAuth elif .spec.type == "Crowd" then .spec.crowd.enableBasicAuth else false end) == true)
}
'''


def provider(name, provider_type, basic_auth=None, enabled=None):
    spec = {"type": provider_type, "displayName": name}
    if enabled is not None:
        spec["enabled"] = enabled
    block = {"enableBasicAuth": basic_auth} if basic_auth is not None else {}
    spec[{"LDAP": "ldap", "Crowd": "crowd", "OIDC": "oidc"}[provider_type]] = block
    return {"apiVersion": "deckhouse.io/v1", "kind": "DexProvider", "metadata": {"name": name}, "spec": spec}


def snapshot(obj):
    """Runs the jqFilter of the hook over a DexProvider, as shell-operator does for the snapshot."""
    out = subprocess.run(["jq", JQ_FILTER], input=json.dumps(obj), capture_output=True, text=True, check=True)
    return {"filterResult": json.loads(out.stdout)}


def module_config(settings, name="user-authn", deleting=False, version=4):
    obj = {
        "apiVersion": "deckhouse.io/v1alpha1",
        "kind": "ModuleConfig",
        "metadata": {"name": name},
        "spec": {"version": version, "settings": settings},
    }
    if version is None:
        del obj["spec"]["version"]
    if deleting:
        obj["metadata"]["deletionTimestamp"] = "2026-01-01T00:00:00Z"
    return obj


def run(obj, *providers):
    ctx = {
        "binding": "twofactor-basic-auth-settings.deckhouse.io",
        "type": "Validating",
        "review": {"request": {"operation": "UPDATE", "object": obj}},
        "snapshots": {"dexproviders": [snapshot(p) for p in providers]},
    }
    out = hook.testrun(two_factor_module_config.main, [ctx])
    assert len(out.validations.data) == 1, out.validations.data
    return out.validations.data[0]


TWO_FACTOR_FOR_LDAP = {"twoFactor": {"enabled": True, "providerTypes": ["Local", "LDAP"]}}


class TestTwoFactorWithBasicAuth(unittest.TestCase):
    def test_refuses_a_covered_type_with_basic_auth(self):
        response = run(module_config(TWO_FACTOR_FOR_LDAP), provider("corp", "LDAP", basic_auth=True))
        self.assertFalse(response["allowed"])
        self.assertIn("'corp'", response["message"])
        self.assertIn("enableBasicAuth", response["message"])

    def test_names_the_types_the_settings_cover(self):
        response = run(module_config(TWO_FACTOR_FOR_LDAP), provider("corp", "LDAP", basic_auth=True))
        self.assertIn("the LDAP providers", response["message"])
        self.assertNotIn("Crowd", response["message"])
        settings = {"twoFactor": {"enabled": True, "providerTypes": ["Crowd"]}}
        response = run(module_config(settings), provider("corp", "Crowd", basic_auth=True))
        self.assertIn("the Crowd providers", response["message"])
        self.assertNotIn("LDAP", response["message"])
        settings = {"twoFactor": {"enabled": True, "providerTypes": ["Local", "Crowd", "LDAP"]}}
        response = run(module_config(settings), provider("corp", "LDAP", basic_auth=True))
        self.assertIn("the LDAP and Crowd providers", response["message"])

    # Deckhouse converts a ModuleConfig of version 3: staticUsers2FA becomes twoFactor. Written into it, providerTypes
    # cover the providers of those types as well, and a ModuleConfig without a version is converted too.
    def test_refuses_the_settings_of_version_3_that_cover_the_type(self):
        for version in (3, 2, None):
            settings = {"staticUsers2FA": {"enabled": True, "providerTypes": ["LDAP"]}}
            response = run(module_config(settings, version=version), provider("corp", "LDAP", basic_auth=True))
            self.assertFalse(response["allowed"], version)
            self.assertIn("the LDAP providers", response["message"])

    def test_version_3_takes_static_users_2fa_over_two_factor(self):
        settings = {"twoFactor": {"enabled": False}, "staticUsers2FA": {"enabled": True, "providerTypes": ["LDAP"]}}
        self.assertFalse(run(module_config(settings, version=3), provider("corp", "LDAP", basic_auth=True))["allowed"])

    def test_version_4_reads_two_factor(self):
        settings = {"twoFactor": {"enabled": False}, "staticUsers2FA": {"enabled": True, "providerTypes": ["LDAP"]}}
        self.assertTrue(run(module_config(settings, version=4), provider("corp", "LDAP", basic_auth=True))["allowed"])

    def test_refuses_crowd_too_and_names_every_provider(self):
        settings = {"twoFactor": {"enabled": True, "providerTypes": ["LDAP", "Crowd"]}}
        response = run(module_config(settings), provider("b-crowd", "Crowd", basic_auth=True),
                       provider("a-ldap", "LDAP", basic_auth=True))
        self.assertFalse(response["allowed"])
        self.assertIn("'a-ldap', 'b-crowd'", response["message"])

    def test_ldap_false_ignores_a_leftover_crowd_true(self):
        obj = provider("corp", "LDAP", basic_auth=False)
        obj["spec"]["crowd"] = {"enableBasicAuth": True}
        self.assertFalse(snapshot(obj)["filterResult"]["basicAuth"])
        self.assertTrue(run(module_config(TWO_FACTOR_FOR_LDAP), obj)["allowed"])

    def test_allows_a_provider_without_basic_auth(self):
        for basic_auth in (None, False):
            self.assertTrue(run(module_config(TWO_FACTOR_FOR_LDAP), provider("corp", "LDAP", basic_auth=basic_auth))["allowed"])

    def test_allows_a_type_the_second_factor_does_not_cover(self):
        settings = {"twoFactor": {"enabled": True, "providerTypes": ["LDAP"]}}
        self.assertTrue(run(module_config(settings), provider("crowd", "Crowd", basic_auth=True))["allowed"])

    def test_allows_basic_auth_of_oidc_whatever_the_settings(self):
        # An OIDC provider has no second factor of its own to keep.
        self.assertTrue(run(module_config(TWO_FACTOR_FOR_LDAP), provider("idp", "OIDC", basic_auth=True))["allowed"])

    def test_allows_a_disabled_provider(self):
        self.assertTrue(run(module_config(TWO_FACTOR_FOR_LDAP), provider("corp", "LDAP", basic_auth=True, enabled=False))["allowed"])

    def test_allows_settings_without_the_second_factor(self):
        for settings in ({}, {"twoFactor": {"enabled": False, "providerTypes": ["LDAP"]}}, {"twoFactor": {"enabled": True}},
                         {"twoFactor": {"enabled": True, "providerTypes": ["Local"]}}):
            self.assertTrue(run(module_config(settings), provider("corp", "LDAP", basic_auth=True))["allowed"], settings)

    def test_leaves_the_other_module_configs_alone(self):
        self.assertTrue(run(module_config(TWO_FACTOR_FOR_LDAP, name="prometheus"), provider("corp", "LDAP", basic_auth=True))["allowed"])

    def test_leaves_a_deleted_module_config_alone(self):
        self.assertTrue(run(module_config(TWO_FACTOR_FOR_LDAP, deleting=True), provider("corp", "LDAP", basic_auth=True))["allowed"])

    def test_legacy_settings_cover_the_local_users_only(self):
        settings = {"staticUsers2FA": {"enabled": True}}
        self.assertTrue(run(module_config(settings), provider("corp", "LDAP", basic_auth=True))["allowed"])


if __name__ == "__main__":
    unittest.main()
