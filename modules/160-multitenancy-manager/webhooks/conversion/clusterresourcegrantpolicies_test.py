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

crgp = load_conversion_webhook(
    "clusterresourcegrantpolicies.multitenancy.deckhouse.io",
    [("v1alpha1", "v1alpha2"), ("v1alpha2", "v1alpha1")],
    Path(__file__).parent / "clusterresourcegrantpolicies.py",
)

V1ALPHA1 = "multitenancy.deckhouse.io/v1alpha1"
V1ALPHA2 = "multitenancy.deckhouse.io/v1alpha2"


def run(binding, *objects):
    source, target = binding.split("_to_")
    ctx = {"binding": binding, "type": "Conversion",
           "fromVersion": "multitenancy.deckhouse.io/" + source, "toVersion": "multitenancy.deckhouse.io/" + target,
           "review": {"request": {"uid": "1", "objects": copy.deepcopy(list(objects))}}}
    return hook.testrun(crgp.main, [ctx])


def convert(binding, obj):
    out = run(binding, obj).conversions.data[0]
    assert "convertedObjects" in out, out
    return out["convertedObjects"][0]


def policy(api_version, spec, annotations=None):
    obj = {"apiVersion": api_version, "kind": "ClusterResourceGrantPolicy", "metadata": {"name": "p"}}
    if annotations is not None:
        obj["metadata"]["annotations"] = annotations
    if spec is not None:
        obj["spec"] = spec
    return obj


def selector(key, value):
    return {"matchLabels": {key: value}}


def kept(value):
    return {crgp.Conversion.ANNOTATION: value}


class TestConfig(unittest.TestCase):
    def test_bindings(self):
        bindings = yaml.safe_load(crgp.config)["kubernetesCustomResourceConversion"]
        self.assertEqual([b["name"] for b in bindings], ["v1alpha1_to_v1alpha2", "v1alpha2_to_v1alpha1"])
        for binding in bindings:
            self.assertEqual(binding["crdName"], "clusterresourcegrantpolicies.multitenancy.deckhouse.io")
            self.assertTrue(callable(getattr(crgp.Conversion, binding["name"])))


class TestUp(unittest.TestCase):
    def test_keeps_the_released_meaning(self):
        """In v1alpha1 projectSelector was matched against namespace labels only, which is exactly
        what v1alpha2 calls namespaceSelector."""
        expressions = {"matchExpressions": [{"key": "projects.deckhouse.io/project-namespace", "operator": "DoesNotExist"}]}
        for v1, expected in (
            ({"projectSelector": selector("projects.deckhouse.io/project", "a"), "resources": []},
             {"namespaceSelector": selector("projects.deckhouse.io/project", "a"), "resources": []}),
            ({"projectSelector": {}, "resources": []}, {"namespaceSelector": {}, "resources": []}),
            ({"resources": []}, {"resources": []}),
            ({"projectSelector": expressions, "resources": []}, {"namespaceSelector": expressions, "resources": []}),
            ({"projectSelector": None, "namespaceSelector": selector("stale", "x")}, {}),
        ):
            with self.subTest(v1):
                got = convert("v1alpha1_to_v1alpha2", policy(V1ALPHA1, v1))
                self.assertEqual(got, policy(V1ALPHA2, expected))

    def test_accepts_every_selector_shape(self):
        for selector_value in (
            {},
            {"matchLabels": {}},
            {"matchExpressions": [{"key": "team", "operator": "In", "values": ["a", "b"]},
                                  {"key": "tier", "operator": "Exists"}]},
            {"matchLabels": {"team": "a"}, "matchExpressions": [{"key": "tier", "operator": "DoesNotExist", "values": []}]},
            {"matchLabels": None, "matchExpressions": [{"key": "tier", "operator": "Exists", "values": None}]},
        ):
            with self.subTest(selector_value):
                obj = policy(V1ALPHA1, {"resources": []}, kept(json.dumps(selector_value)))
                got = convert("v1alpha1_to_v1alpha2", obj)
                self.assertEqual(got["spec"]["projectSelector"], selector_value)
                self.assertNotIn("annotations", got["metadata"])

    def test_refuses_an_annotation_that_is_not_a_selector(self):
        """A projectSelector of the wrong shape would make every typed list of the policies fail to
        decode, so the conversion refuses the write instead."""
        for value in (
            '{"matchLabels":{"tier":1}}', "42", '"prod"', "[]", '{"foo":"bar"}', "null", "NaN",
            '{"matchLabels":{"tier":"gold"},"extra":true}',
            '{"matchExpressions":[{"key":"a","operator":"In","values":"x"}]}',
            '{"matchExpressions":[{"key":"a","operator":"Gt","values":["1"]}]}',
            '{"matchExpressions":[{"key":1,"operator":"Exists"}]}',
            '{"matchExpressions":{"key":"a","operator":"Exists"}}',
            '{"matchExpressions":[{"key":"a","operator":"Exists","x":1}]}',
            '{"matchLabels":false}', '{"matchExpressions":false}',
            '{"matchExpressions":[{"key":"a","operator":"Exists","values":false}]}',
            '{"matchLabels":{"tier":false}}', '{} {}', "", "not json",
        ):
            with self.subTest(value):
                out = run("v1alpha1_to_v1alpha2", policy(V1ALPHA1, {}), policy(V1ALPHA1, {"resources": []}, kept(value)))
                tests.assert_conversion(self, out, None, crgp.Conversion.UP_FAILED)

    def test_annotation_without_spec_is_dropped(self):
        got = convert("v1alpha1_to_v1alpha2", policy(V1ALPHA1, None, kept("not json")))
        self.assertEqual(got, policy(V1ALPHA2, None))

    def test_other_annotations_stay(self):
        got = convert("v1alpha1_to_v1alpha2", policy(V1ALPHA1, {}, {**kept("{}"), "owner": "platform"}))
        self.assertEqual(got["metadata"]["annotations"], {"owner": "platform"})


class TestDown(unittest.TestCase):
    def test_shows_the_namespace_selector(self):
        """A v1alpha1 client reads what v1alpha2 calls namespaceSelector in the field it knows, and
        the project selector it cannot express in an annotation."""
        down = convert("v1alpha2_to_v1alpha1", policy(V1ALPHA2, {
            "projectSelector": selector("team", "a"), "namespaceSelector": selector("env", "prod"), "resources": []}))
        self.assertEqual(down["spec"], {"resources": [], "projectSelector": selector("env", "prod")})
        self.assertEqual(down["metadata"]["annotations"], kept('{"matchLabels":{"team":"a"}}'))

    def test_annotation_is_compact_json_as_jq_writes_it(self):
        down = convert("v1alpha2_to_v1alpha1", policy(V1ALPHA2, {"projectSelector": {"matchLabels": {"é": "a\x7f\n/"}}}))
        self.assertEqual(down["metadata"]["annotations"][crgp.Conversion.ANNOTATION], '{"matchLabels":{"é":"a\\u007f\\n/"}}')

    def test_drops_a_stale_annotation(self):
        """The annotation a v1alpha1 client sees is always derived from spec.projectSelector."""
        for spec in ({"resources": []}, {"namespaceSelector": selector("env", "prod"), "resources": []},
                     {"projectSelector": selector("team", "a"), "resources": []}):
            with self.subTest(spec):
                stale = policy(V1ALPHA2, copy.deepcopy(spec), kept("{}"))
                up = convert("v1alpha1_to_v1alpha2", convert("v1alpha2_to_v1alpha1", stale))
                self.assertEqual(up["spec"], spec)
                self.assertNotIn("annotations", up["metadata"])

    def test_wrong_shape_fails_the_whole_request(self):
        out = run("v1alpha2_to_v1alpha1", policy(V1ALPHA2, {}), policy(V1ALPHA2, "x"))
        # BaseConversionHook reports the exception; the module hook it replaces said
        # "Conversion of clusterresourcegrantpolicies.multitenancy.deckhouse.io failed".
        tests.assert_conversion(self, out, None, "Internal error: 'str' object has no attribute 'pop'")


class TestRoundTrips(unittest.TestCase):
    def test_v1alpha2_served_as_v1alpha1_comes_back_unchanged(self):
        for spec in (
            {"projectSelector": selector("team", "a"), "namespaceSelector": selector("projects.deckhouse.io/project-namespace", "cache"), "resources": []},
            {"projectSelector": selector("team", "a"), "resources": []},
            {"namespaceSelector": selector("env", "prod"), "resources": []},
            {"projectSelector": {}, "resources": []},
            {"resources": []},
        ):
            with self.subTest(spec):
                original = policy(V1ALPHA2, spec)
                down = convert("v1alpha2_to_v1alpha1", original)
                self.assertNotIn("namespaceSelector", down["spec"])
                self.assertEqual(convert("v1alpha1_to_v1alpha2", down), original)

    def test_keeps_other_annotations(self):
        original = policy(V1ALPHA2, {"projectSelector": selector("team", "a"), "resources": []}, {"owner": "platform"})
        up = convert("v1alpha1_to_v1alpha2", convert("v1alpha2_to_v1alpha1", original))
        self.assertEqual(up, original)

    def test_missing_spec(self):
        for binding, version in (("v1alpha1_to_v1alpha2", V1ALPHA1), ("v1alpha2_to_v1alpha1", V1ALPHA2)):
            with self.subTest(binding):
                self.assertNotIn("spec", convert(binding, policy(version, None)))

    def test_mixed_versions(self):
        v2 = policy(V1ALPHA2, {"namespaceSelector": {}})
        out = run("v1alpha1_to_v1alpha2", v2, policy(V1ALPHA1, {"projectSelector": {}}))
        tests.assert_conversion(self, out, [v2, v2], None)


if __name__ == "__main__":
    unittest.main()
