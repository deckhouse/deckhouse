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
import json
import unittest

import yaml
from deckhouse import hook, tests

from pathlib import Path
from deckhouse.tests import load_conversion_webhook

V1ALPHA1, V1, V2ALPHA1 = "deckhouse.io/v1alpha1", "deckhouse.io/v1", "deckhouse.io/v2alpha1"
BINDINGS = {
    "v1_to_v2alpha1": (V1, V2ALPHA1),
    "v2alpha1_to_v1": (V2ALPHA1, V1),
    "v1_to_v1alpha1": (V1, V1ALPHA1),
    "v1alpha1_to_v1": (V1ALPHA1, V1),
}

dex_authenticator_conversion = load_conversion_webhook(
    "dexauthenticators.deckhouse.io",
    [tuple(binding.split("_to_")) for binding in BINDINGS],
    Path(__file__).parent / "dex_authenticator_conversion.py",
)


def authenticator(api_version, spec):
    obj = {"apiVersion": api_version, "kind": "DexAuthenticator", "metadata": {"name": "test", "namespace": "test"}}
    if spec is not None:
        obj["spec"] = spec
    return obj


def run(binding, *objects):
    from_version, to_version = BINDINGS[binding]
    ctx = {"binding": binding, "type": "Conversion", "fromVersion": from_version, "toVersion": to_version,
           "review": {"request": {"uid": "1", "desiredAPIVersion": to_version, "objects": copy.deepcopy(list(objects))}}}
    return hook.testrun(dex_authenticator_conversion.main, [ctx])


def convert(binding, obj):
    out = run(binding, obj).conversions.data[0]
    assert "convertedObjects" in out, out
    return out["convertedObjects"][0]


class TestConfig(unittest.TestCase):
    def test_bindings(self):
        config = yaml.safe_load(dex_authenticator_conversion.config)
        self.assertEqual(
            {b["name"]: (b["conversions"][0]["fromVersion"], b["conversions"][0]["toVersion"])
             for b in config["kubernetesCustomResourceConversion"]},
            BINDINGS,
        )
        for binding in config["kubernetesCustomResourceConversion"]:
            self.assertEqual(binding["crdName"], "dexauthenticators.deckhouse.io")
            self.assertTrue(callable(getattr(dex_authenticator_conversion.Conversion, binding["name"])))


class TestV2alpha1ToV1(unittest.TestCase):
    def test_first_application_moves_into_spec(self):
        obj = authenticator(V2ALPHA1, {
            "sendAuthorizationHeader": True,
            "applications": [
                {"domain": "app.example.com", "ingressClassName": "nginx", "ingressSecretName": "tls",
                 "signOutURL": "/logout", "whitelistSourceRanges": ["192.0.2.0/24"],
                 "gatewayAPI": {"httpRouteListenerSetName": "app", "httpRouteListenerSetSectionName": "https"}},
                {"domain": "second.example.com", "ingressClassName": "nginx"},
            ],
        })
        self.assertEqual(convert("v2alpha1_to_v1", obj), authenticator(V1, {
            "sendAuthorizationHeader": True,
            "applicationDomain": "app.example.com",
            "applicationIngressClassName": "nginx",
            "applicationIngressCertificateSecretName": "tls",
            "gatewayAPI": {"applicationHTTPRouteListenerSetName": "app", "applicationHTTPRouteListenerSetSectionName": "https"},
            "signOutURL": "/logout",
            "whitelistSourceRanges": ["192.0.2.0/24"],
            "additionalApplications": [{"domain": "second.example.com", "ingressClassName": "nginx"}],
        }))

    def test_optional_fields_stay_as_nulls(self):
        # The jq program this hook replaced kept these keys with a null value, and so does the hook.
        obj = authenticator(V2ALPHA1, {"applications": [{"domain": "app.example.com", "ingressClassName": "nginx"}]})
        self.assertEqual(convert("v2alpha1_to_v1", obj)["spec"], {
            "applicationDomain": "app.example.com",
            "applicationIngressClassName": "nginx",
            "applicationIngressCertificateSecretName": None,
            "signOutURL": None,
            "whitelistSourceRanges": None,
        })

    def test_empty_gateway_api_stays_empty(self):
        obj = authenticator(V2ALPHA1, {"applications": [{"domain": "a.example.com", "gatewayAPI": {}}]})
        spec = convert("v2alpha1_to_v1", obj)["spec"]
        self.assertEqual(spec["gatewayAPI"], {})
        self.assertNotIn("applicationIngressClassName", spec)

    def test_without_applications(self):
        for spec in ({"applications": []}, {}, None):
            with self.subTest(spec):
                self.assertEqual(convert("v2alpha1_to_v1", authenticator(V2ALPHA1, spec))["spec"], {
                    "applicationDomain": None,
                    "applicationIngressCertificateSecretName": None,
                    "signOutURL": None,
                    "whitelistSourceRanges": None,
                })


class TestV1ToV2alpha1(unittest.TestCase):
    def test_spec_moves_into_the_first_application(self):
        obj = authenticator(V1, {
            "applicationDomain": "app.example.com",
            "applicationIngressClassName": "nginx",
            "applicationIngressCertificateSecretName": "tls",
            "gatewayAPI": {"applicationHTTPRouteListenerSetName": "app", "applicationHTTPRouteListenerSetNamespace": "gw"},
            "signOutURL": "/logout",
            "whitelistSourceRanges": ["192.0.2.0/24"],
            "additionalApplications": [{"domain": "second.example.com", "ingressClassName": "nginx"}],
            "keepUsersLoggedInFor": "1h",
        })
        self.assertEqual(convert("v1_to_v2alpha1", obj), authenticator(V2ALPHA1, {
            "keepUsersLoggedInFor": "1h",
            "sendAuthorizationHeader": None,
            "applications": [
                {"domain": "app.example.com", "ingressClassName": "nginx", "ingressSecretName": "tls",
                 "signOutURL": "/logout", "whitelistSourceRanges": ["192.0.2.0/24"],
                 "gatewayAPI": {"httpRouteListenerSetName": "app", "httpRouteListenerSetNamespace": "gw"}},
                {"domain": "second.example.com", "ingressClassName": "nginx"},
            ],
        }))

    def test_empty_strings_and_lists_are_carried(self):
        obj = authenticator(V1, {"applicationDomain": "a.example.com", "applicationIngressClassName": "nginx",
                                 "applicationIngressCertificateSecretName": "", "signOutURL": "",
                                 "whitelistSourceRanges": [], "sendAuthorizationHeader": False})
        self.assertEqual(convert("v1_to_v2alpha1", obj)["spec"], {
            "sendAuthorizationHeader": False,
            "applications": [{"domain": "a.example.com", "ingressClassName": "nginx", "ingressSecretName": "",
                              "signOutURL": "", "whitelistSourceRanges": []}],
        })

    def test_legacy_empty_gateway(self):
        """Older converters emitted a gatewayAPI object full of nulls even for Ingress-only applications."""
        for gateway in (None, {}, {"applicationHTTPRouteListenerSetName": None,
                                   "applicationHTTPRouteListenerSetNamespace": None,
                                   "applicationHTTPRouteListenerSetSectionName": None}):
            with self.subTest(gateway):
                obj = authenticator(V1, {"applicationDomain": "app.example.com", "applicationIngressClassName": "nginx",
                                         "gatewayAPI": gateway})
                apps = convert("v1_to_v2alpha1", obj)["spec"]["applications"]
                self.assertEqual(apps, [{"domain": "app.example.com", "ingressClassName": "nginx"}])

    def test_without_spec(self):
        self.assertEqual(convert("v1_to_v2alpha1", authenticator(V1, None))["spec"],
                         {"sendAuthorizationHeader": None, "applications": [{"domain": None}]})


class TestRoundTrips(unittest.TestCase):
    def test_conversion_preserves_routing(self):
        for fixture in (
            '[{"domain":"app.example.com","ingressClassName":"nginx"}]',
            '[{"domain":"app.example.com","gatewayAPI":{"httpRouteListenerSetName":"app"}}]',
            '[{"domain":"app.example.com","gatewayAPI":{"httpRouteListenerSetName":"app","httpRouteListenerSetNamespace":"gateway","httpRouteListenerSetSectionName":"https"},"signOutURL":"/logout"},'
            '{"domain":"second.example.com","gatewayAPI":{"httpRouteListenerSetName":"second","httpRouteListenerSetNamespace":"gateway","httpRouteListenerSetSectionName":"https"},"signOutURL":"/bye","whitelistSourceRanges":["192.0.2.0/24"]}]',
            '[{"domain":"app.example.com","ingressClassName":"nginx","gatewayAPI":{"httpRouteListenerSetName":"app"}},'
            '{"domain":"second.example.com","gatewayAPI":{"httpRouteListenerSetName":"second"}},'
            '{"domain":"third.example.com","ingressClassName":"nginx"}]',
        ):
            with self.subTest(fixture):
                apps = json.loads(fixture)
                stored = convert("v2alpha1_to_v1", authenticator(V2ALPHA1, {"applications": apps}))
                self.assertEqual("ingressClassName" in apps[0], "applicationIngressClassName" in stored["spec"])
                self.assertEqual("gatewayAPI" in apps[0], "gatewayAPI" in stored["spec"])
                self.assertNotIn(None, stored["spec"].get("gatewayAPI", {}).values())

                legacy = convert("v1_to_v1alpha1", stored)
                self.assertEqual(legacy["apiVersion"], V1ALPHA1)
                stored = convert("v1alpha1_to_v1", legacy)
                restored = convert("v1_to_v2alpha1", stored)
                self.assertEqual(restored["spec"]["applications"], apps)

    def test_v1_and_v1alpha1_differ_in_api_version_only(self):
        obj = authenticator(V1ALPHA1, {"applicationDomain": "a.example.com", "applicationIngressClassName": "nginx"})
        self.assertEqual(convert("v1alpha1_to_v1", obj), dict(obj, apiVersion=V1))
        self.assertEqual(convert("v1_to_v1alpha1", dict(obj, apiVersion=V1)), obj)


class TestRequests(unittest.TestCase):
    def test_mixed_versions(self):
        """Objects not in the version a binding converts from pass through unchanged."""
        v1 = authenticator(V1, {"applicationDomain": "a.example.com", "applicationIngressClassName": "nginx"})
        v1alpha1 = dict(v1, apiVersion=V1ALPHA1)
        v2alpha1 = authenticator(V2ALPHA1, {"applications": [{"domain": "a.example.com", "ingressClassName": "nginx"}]})
        out = run("v1_to_v1alpha1", v2alpha1, v1, v1alpha1)
        tests.assert_conversion(self, out, [v2alpha1, v1alpha1, v1alpha1], None)

    def test_empty_request(self):
        for binding in BINDINGS:
            with self.subTest(binding):
                tests.assert_conversion(self, run(binding), [], None)

    def test_wrong_shape_fails_the_whole_request(self):
        good = authenticator(V1, {"applicationDomain": "a.example.com", "applicationIngressClassName": "nginx"})
        # BaseConversionHook reports the exception; the module hook it replaces said
        # "Conversion of dexproviders.deckhouse.io failed".
        for binding, bad, message in (
            ("v1_to_v2alpha1", authenticator(V1, {"additionalApplications": {"a": 1}}),
             "spec.additionalApplications is not a list"),
            ("v1_to_v2alpha1", authenticator(V1, {"gatewayAPI": "x"}),
             "cannot index str with 'applicationHTTPRouteListenerSetName'"),
            ("v2alpha1_to_v1", authenticator(V2ALPHA1, {"applications": "x"}), "cannot index str with 0"),
            ("v2alpha1_to_v1", authenticator(V2ALPHA1, {"applications": [{"domain": "a", "gatewayAPI": "x"}]}),
             "cannot index str with 'httpRouteListenerSetName'"),
        ):
            with self.subTest(bad["spec"]):
                out = run(binding, dict(good, apiVersion=BINDINGS[binding][0]), bad)
                tests.assert_conversion(self, out, None, "Internal error: " + message)


if __name__ == "__main__":
    unittest.main()
