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

import copy
import unittest

import yaml
from deckhouse import hook, tests

from pathlib import Path
from deckhouse.tests import load_conversion_webhook

V1ALPHA1, V1 = "deckhouse.io/v1alpha1", "deckhouse.io/v1"
BINDINGS = {"v1alpha1_to_v1": (V1ALPHA1, V1), "v1_to_v1alpha1": (V1, V1ALPHA1)}

dex_provider_conversion = load_conversion_webhook(
    "dexproviders.deckhouse.io", [("v1alpha1", "v1"), ("v1", "v1alpha1")], Path(__file__).parent / "dex_provider_conversion.py")


def provider(api_version, team_name_field=None, name="github"):
    obj = {"apiVersion": api_version, "kind": "DexProvider", "metadata": {"name": name},
           "spec": {"type": "Github", "displayName": "GitHub", "github": {"clientID": "id", "clientSecret": "secret"}}}
    if team_name_field is not None:
        obj["spec"]["github"]["teamNameField"] = team_name_field
    return obj


def convert(binding, *objects):
    from_version, to_version = BINDINGS[binding]
    ctx = {"binding": binding, "type": "Conversion", "fromVersion": from_version, "toVersion": to_version,
           "review": {"request": {"uid": "1", "desiredAPIVersion": to_version, "objects": copy.deepcopy(list(objects))}}}
    return hook.testrun(dex_provider_conversion.main, [ctx])


class TestConfig(unittest.TestCase):
    def test_bindings(self):
        config = yaml.safe_load(dex_provider_conversion.config)
        self.assertEqual(
            {b["name"]: (b["conversions"][0]["fromVersion"], b["conversions"][0]["toVersion"])
             for b in config["kubernetesCustomResourceConversion"]},
            BINDINGS,
        )
        for binding in config["kubernetesCustomResourceConversion"]:
            self.assertEqual(binding["crdName"], "dexproviders.deckhouse.io")
            self.assertTrue(callable(getattr(dex_provider_conversion.Conversion, binding["name"])))


class TestConversion(unittest.TestCase):
    def test_up_capitalizes_team_name_field(self):
        for value, expected in (("name", "Name"), ("slug", "Slug"), ("both", "Both"), ("Name", "Name"), ("", "")):
            with self.subTest(value):
                out = convert("v1alpha1_to_v1", provider(V1ALPHA1, value))
                tests.assert_conversion(self, out, provider(V1, expected), None)

    def test_down_lowercases_team_name_field(self):
        for value, expected in (("Name", "name"), ("Slug", "slug"), ("Both", "both"), ("name", "name"), ("", "")):
            with self.subTest(value):
                out = convert("v1_to_v1alpha1", provider(V1, value))
                tests.assert_conversion(self, out, provider(V1ALPHA1, expected), None)

    def test_without_team_name_field(self):
        no_github = {"apiVersion": V1ALPHA1, "kind": "DexProvider", "metadata": {"name": "oidc"}, "spec": {"type": "OIDC"}}
        out = convert("v1alpha1_to_v1", provider(V1ALPHA1), no_github, {"apiVersion": V1ALPHA1, "metadata": {"name": "x"}})
        tests.assert_conversion(
            self, out,
            [provider(V1), dict(no_github, apiVersion=V1), {"apiVersion": V1, "metadata": {"name": "x"}}],
            None,
        )

    def test_round_trip(self):
        original = provider(V1ALPHA1, "slug")
        up = convert("v1alpha1_to_v1", original).conversions.data[0]["convertedObjects"][0]
        down = convert("v1_to_v1alpha1", up).conversions.data[0]["convertedObjects"][0]
        self.assertEqual(down, original)

    def test_objects_of_other_versions_pass_through(self):
        out = convert("v1alpha1_to_v1", provider(V1, "Slug", "a"), provider(V1ALPHA1, "slug", "b"))
        tests.assert_conversion(self, out, [provider(V1, "Slug", "a"), provider(V1, "Slug", "b")], None)

    def test_empty_request(self):
        tests.assert_conversion(self, convert("v1_to_v1alpha1"), [], None)

    def test_wrong_type_fails_the_whole_request(self):
        bad = provider(V1ALPHA1)
        bad["spec"]["github"]["teamNameField"] = 5
        out = convert("v1alpha1_to_v1", provider(V1ALPHA1, "name"), bad)
        # BaseConversionHook reports the exception; the module hook it replaces said
        # "Conversion of dexproviders.deckhouse.io failed".
        tests.assert_conversion(self, out, None, "Internal error: 'int' object is not subscriptable")


if __name__ == "__main__":
    unittest.main()
