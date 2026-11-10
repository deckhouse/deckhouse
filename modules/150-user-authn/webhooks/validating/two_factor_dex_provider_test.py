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

two_factor_dex_provider = load_validation_webhook(Path(__file__).parent / "two_factor_dex_provider.py")

# A copy of the jqFilter of the modulesettings context binding in
# templates/two-factor-dex-provider-validation-webhook.yaml; the template tests keep the two identical.
JQ_FILTER = '''
(.spec.settings // {}) as $s
| (if (.spec.version // 0) < 4 and ($s | has("staticUsers2FA")) then $s.staticUsers2FA else $s.twoFactor end) as $tf
| {
    "enabled": ($tf.enabled // false),
    "providerTypes": ($tf.providerTypes // ["Local"])
  }
'''


def settings_snapshot(settings, version=4):
    """Runs the jqFilter of the hook over the user-authn ModuleConfig, as shell-operator does for the snapshot."""
    obj = {"apiVersion": "deckhouse.io/v1alpha1", "kind": "ModuleConfig", "metadata": {"name": "user-authn"},
           "spec": {"version": version, "settings": settings}}
    out = subprocess.run(["jq", JQ_FILTER], input=json.dumps(obj), capture_output=True, text=True, check=True)
    return {"filterResult": json.loads(out.stdout)}


def provider(provider_type, basic_auth=None, enabled=None, deleting=False):
    spec = {"type": provider_type, "displayName": "corp"}
    if enabled is not None:
        spec["enabled"] = enabled
    block = {"enableBasicAuth": basic_auth} if basic_auth is not None else {}
    spec[{"LDAP": "ldap", "Crowd": "crowd", "OIDC": "oidc"}[provider_type]] = block
    obj = {"apiVersion": "deckhouse.io/v1", "kind": "DexProvider", "metadata": {"name": "corp"}, "spec": spec}
    if deleting:
        obj["metadata"]["deletionTimestamp"] = "2026-01-01T00:00:00Z"
    return obj


def run(obj, *settings):
    ctx = {
        "binding": "twofactor-basic-auth-providers.deckhouse.io",
        "type": "Validating",
        "review": {"request": {"operation": "CREATE", "object": obj}},
        "snapshots": {"modulesettings": [settings_snapshot(s[1], s[0]) if isinstance(s, tuple) else settings_snapshot(s)
                                         for s in settings]},
    }
    out = hook.testrun(two_factor_dex_provider.main, [ctx])
    assert len(out.validations.data) == 1, out.validations.data
    return out.validations.data[0]


LDAP_AND_LOCAL = {"twoFactor": {"enabled": True, "providerTypes": ["Local", "LDAP"]}}


class TestBasicAuthWithTwoFactor(unittest.TestCase):
    def test_refuses_basic_auth_of_a_covered_type(self):
        response = run(provider("LDAP", basic_auth=True), LDAP_AND_LOCAL)
        self.assertFalse(response["allowed"])
        self.assertIn("enableBasicAuth", response["message"])
        self.assertIn("LDAP", response["message"])

    def test_refuses_crowd_when_it_is_covered(self):
        settings = {"twoFactor": {"enabled": True, "providerTypes": ["Crowd"]}}
        self.assertFalse(run(provider("Crowd", basic_auth=True), settings)["allowed"])

    def test_allows_a_provider_without_basic_auth(self):
        for basic_auth in (None, False):
            self.assertTrue(run(provider("LDAP", basic_auth=basic_auth), LDAP_AND_LOCAL)["allowed"])

    def test_allows_a_type_the_second_factor_does_not_cover(self):
        self.assertTrue(run(provider("Crowd", basic_auth=True), LDAP_AND_LOCAL)["allowed"])

    def test_allows_oidc_with_basic_auth(self):
        self.assertTrue(run(provider("OIDC", basic_auth=True), LDAP_AND_LOCAL)["allowed"])

    def test_allows_a_disabled_provider(self):
        self.assertTrue(run(provider("LDAP", basic_auth=True, enabled=False), LDAP_AND_LOCAL)["allowed"])

    def test_allows_basic_auth_without_the_second_factor(self):
        for settings in ({}, {"twoFactor": {"enabled": False, "providerTypes": ["LDAP"]}}, {"twoFactor": {"enabled": True}},
                         {"staticUsers2FA": {"enabled": True}}):
            self.assertTrue(run(provider("LDAP", basic_auth=True), settings)["allowed"], settings)

    # Deckhouse converts a ModuleConfig of version 3: staticUsers2FA becomes twoFactor. Written into it, providerTypes
    # cover the providers of those types as well.
    def test_refuses_basic_auth_when_the_settings_of_version_3_cover_the_type(self):
        v3 = (3, {"staticUsers2FA": {"enabled": True, "providerTypes": ["LDAP"]}})
        self.assertFalse(run(provider("LDAP", basic_auth=True), v3)["allowed"])

    def test_version_3_takes_static_users_2fa_over_two_factor(self):
        # The conversion replaces twoFactor with staticUsers2FA.
        v3 = (3, {"twoFactor": {"enabled": False}, "staticUsers2FA": {"enabled": True, "providerTypes": ["LDAP"]}})
        self.assertFalse(run(provider("LDAP", basic_auth=True), v3)["allowed"])

    def test_version_3_for_the_local_users_only_allows_basic_auth(self):
        v3 = (3, {"staticUsers2FA": {"enabled": True}})
        self.assertTrue(run(provider("LDAP", basic_auth=True), v3)["allowed"])

    def test_allows_basic_auth_without_the_module_config(self):
        self.assertTrue(run(provider("LDAP", basic_auth=True))["allowed"])

    def test_leaves_a_deleted_provider_alone(self):
        self.assertTrue(run(provider("LDAP", basic_auth=True, deleting=True), LDAP_AND_LOCAL)["allowed"])


if __name__ == "__main__":
    unittest.main()
