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
from projects_test import check_golden, module_dir

projecttemplates = load_conversion_webhook(
    "projecttemplates.deckhouse.io", [("v1alpha1", "v1alpha2"), ("v1alpha2", "v1alpha1")], Path(__file__).parent / "projecttemplates.py")

V1ALPHA1, V1ALPHA2 = "deckhouse.io/v1alpha1", "deckhouse.io/v1alpha2"


def run(binding, *objects):
    source, target = binding.split("_to_")
    ctx = {"binding": binding, "type": "Conversion",
           "fromVersion": "deckhouse.io/" + source, "toVersion": "deckhouse.io/" + target,
           "review": {"request": {"uid": "1", "objects": copy.deepcopy(list(objects))}}}
    return hook.testrun(projecttemplates.main, [ctx])


def convert(binding, obj):
    out = run(binding, obj).conversions.data[0]
    assert "convertedObjects" in out, out
    return out["convertedObjects"][0]


def template(api_version, spec=None):
    obj = {"apiVersion": api_version, "kind": "ProjectTemplate", "metadata": {"name": "test"}}
    if spec is not None:
        obj["spec"] = spec
    return obj


class TestConfig(unittest.TestCase):
    def test_bindings(self):
        bindings = yaml.safe_load(projecttemplates.config)["kubernetesCustomResourceConversion"]
        self.assertEqual([b["name"] for b in bindings], ["v1alpha1_to_v1alpha2", "v1alpha2_to_v1alpha1"])
        for binding in bindings:
            self.assertEqual(binding["crdName"], "projecttemplates.deckhouse.io")
            self.assertTrue(callable(getattr(projecttemplates.Conversion, binding["name"])))


class TestGolden(unittest.TestCase):
    """The golden fixtures pin both conversions object by object: the version bump on the way up, the
    backfill on the way down, and everything the hook leaves alone."""

    def test_conversion_paths(self):
        module = module_dir()
        if module is None:
            self.skipTest("the golden fixtures are in the repository, which is not around")
        fixtures = sorted((module / "hooks" / "testdata" / "conversion" / "projecttemplates").glob("*.yaml"))
        self.assertEqual([p.stem for p in fixtures], ["v1alpha1_to_v1alpha2", "v1alpha2_to_v1alpha1"])
        for path in fixtures:
            check_golden(self, projecttemplates, path)


class TestConversion(unittest.TestCase):
    def test_missing_spec(self):
        """The apiserver keeps asking for the conversion of whatever v1alpha1 objects it holds, and a
        template with no spec at all is a shape it can hold."""
        up = convert("v1alpha1_to_v1alpha2", template(V1ALPHA1))
        self.assertEqual(up, template(V1ALPHA2))

        down = convert("v1alpha2_to_v1alpha1", template(V1ALPHA2))
        self.assertEqual(down["spec"], {"resourcesTemplate": "", "parametersSchema": {"openAPIV3Schema": {}}})

    def test_version_bump(self):
        """The fields both versions describe survive the round trip, the Helm resourcesTemplate among them."""
        original = template(V1ALPHA2, {
            "description": "a template",
            "resourcesTemplate": "---\napiVersion: v1\nkind: Namespace\n",
            "parametersSchema": {"openAPIV3Schema": {"type": "object"}},
        })
        down = convert("v1alpha2_to_v1alpha1", original)
        self.assertEqual(down, dict(original, apiVersion=V1ALPHA1))
        self.assertEqual(convert("v1alpha1_to_v1alpha2", down), original)

    def test_backfills_what_v1alpha1_requires(self):
        for spec, expected in (
            ({"description": "structured only"},
             {"description": "structured only", "resourcesTemplate": "", "parametersSchema": {"openAPIV3Schema": {}}}),
            ({"resourcesTemplate": None, "parametersSchema": None},
             {"resourcesTemplate": "", "parametersSchema": {"openAPIV3Schema": {}}}),
            ({"parametersSchema": {}}, {"resourcesTemplate": "", "parametersSchema": {"openAPIV3Schema": {}}}),
            ({"parametersSchema": {"openAPIV3Schema": None, "x": 1}},
             {"resourcesTemplate": "", "parametersSchema": {"openAPIV3Schema": {}, "x": 1}}),
            (None, {"resourcesTemplate": "", "parametersSchema": {"openAPIV3Schema": {}}}),
        ):
            with self.subTest(spec):
                obj = template(V1ALPHA2)
                obj["spec"] = spec
                self.assertEqual(convert("v1alpha2_to_v1alpha1", obj)["spec"], expected)

    def test_structured_fields_are_left_to_pruning(self):
        spec = {"resourcesTemplate": "", "parametersSchema": {"openAPIV3Schema": {}}, "administrators": [{"kind": "User", "name": "a"}]}
        self.assertEqual(convert("v1alpha2_to_v1alpha1", template(V1ALPHA2, copy.deepcopy(spec)))["spec"], spec)

    def test_mixed_versions(self):
        out = run("v1alpha1_to_v1alpha2", template(V1ALPHA2, {"description": "a"}), template(V1ALPHA1, {"description": "b"}))
        tests.assert_conversion(self, out, [template(V1ALPHA2, {"description": "a"}), template(V1ALPHA2, {"description": "b"})], None)

    def test_empty_request(self):
        tests.assert_conversion(self, run("v1alpha2_to_v1alpha1"), [], None)

    def test_wrong_shape_fails_the_whole_request(self):
        out = run("v1alpha2_to_v1alpha1", template(V1ALPHA2), template(V1ALPHA2, {"parametersSchema": "x"}))
        # BaseConversionHook reports the exception; the module hook it replaces said
        # "Conversion of projecttemplates.deckhouse.io failed".
        tests.assert_conversion(self, out, None, "Internal error: 'str' object has no attribute 'get'")


if __name__ == "__main__":
    unittest.main()
