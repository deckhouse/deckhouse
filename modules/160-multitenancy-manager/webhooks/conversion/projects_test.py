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
import os
import re
import unittest
from pathlib import Path

import yaml
from deckhouse import hook, tests

from pathlib import Path
from deckhouse.tests import load_conversion_webhook

CONVERSIONS = [("v1alpha1", "v1alpha2"), ("v1alpha2", "v1alpha1"), ("v1alpha2", "v1alpha3"), ("v1alpha3", "v1alpha2")]
BINDINGS = [f"{source}_to_{target}" for source, target in CONVERSIONS]

projects = load_conversion_webhook("projects.deckhouse.io", CONVERSIONS, Path(__file__).parent / "projects.py")

ADMIN = {"subject": "User", "name": "alice@example.com"}


def module_dir():
    """The module in the repository, where the golden fixtures are. tests_python_webhooks runs the
    tests from a copy outside of it, and GitLab names the checkout in CI_PROJECT_DIR."""
    candidates = [Path(__file__).resolve().parents[2]]
    if os.environ.get("CI_PROJECT_DIR"):
        candidates.insert(0, Path(os.environ["CI_PROJECT_DIR"], "modules", "160-multitenancy-manager"))
    return next((c for c in candidates if (c / "hooks" / "testdata" / "conversion").is_dir()), None)


def check_golden(test: unittest.TestCase, hook_module, fixture: Path):
    """Runs every case of a golden fixture through the conversion path the file is named after. A
    longer path is the chain shell-operator builds when no single binding covers the pair: the whole
    review goes through each step in turn, and each step converts only the objects at its source
    version."""
    versions = fixture.stem.split("_to_")
    cases = yaml.safe_load(fixture.read_text())
    test.assertTrue(cases, fixture)
    for case in cases:
        with test.subTest(f"{fixture.name}: {case['name']}"):
            objects = case["objects"]
            for source, target in zip(versions, versions[1:]):
                ctx = {"binding": f"{source}_to_{target}", "type": "Conversion",
                       "fromVersion": "deckhouse.io/" + source, "toVersion": "deckhouse.io/" + target,
                       "review": {"request": {"objects": copy.deepcopy(objects)}}}
                out = hook.testrun(hook_module.main, [ctx]).conversions.data[0]
                test.assertIn("convertedObjects", out)
                objects = out["convertedObjects"]
            test.assertEqual(json.loads(json.dumps(objects)), json.loads(json.dumps(case["converted"])))
GROUP = {"subject": "Group", "name": "backend-admins"}
QUANTITY = re.compile(r"[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([KMGTPE]i|[numkMGTPE]|[eE][+-]?[0-9]{1,3})?")


def run(binding, *objects):
    source, target = binding.split("_to_")
    ctx = {"binding": binding, "type": "Conversion",
           "fromVersion": "deckhouse.io/" + source, "toVersion": "deckhouse.io/" + target,
           "review": {"request": {"uid": "1", "desiredAPIVersion": "deckhouse.io/" + target,
                                  "objects": copy.deepcopy(list(objects))}}}
    return hook.testrun(projects.main, [ctx])


def convert(binding, obj):
    out = run(binding, obj).conversions.data[0]
    assert "convertedObjects" in out, out
    return out["convertedObjects"][0]


def project(version, spec=None, status=None, name="test"):
    obj = {"apiVersion": "deckhouse.io/" + version, "kind": "Project", "metadata": {"name": name}}
    if spec is not None:
        obj["spec"] = spec
    if status is not None:
        obj["status"] = status
    return obj


class TestConfig(unittest.TestCase):
    def test_bindings(self):
        bindings = yaml.safe_load(projects.config)["kubernetesCustomResourceConversion"]
        self.assertEqual(sorted(b["name"] for b in bindings), sorted(BINDINGS))
        for binding in bindings:
            source, target = binding["name"].split("_to_")
            self.assertEqual(binding["crdName"], "projects.deckhouse.io")
            self.assertEqual(binding["conversions"], [
                {"fromVersion": "deckhouse.io/" + source, "toVersion": "deckhouse.io/" + target}])
            self.assertTrue(callable(getattr(projects.Conversion, binding["name"])))


class TestV1alpha1(unittest.TestCase):
    def test_up_renames(self):
        got = convert("v1alpha1_to_v1alpha2", project("v1alpha1", {"projectTypeName": "default", "template": {"a": 1}}))
        self.assertEqual(got, project("v1alpha2", {"projectTemplateName": "default", "parameters": {"a": 1}}))

    def test_down_renames(self):
        got = convert("v1alpha2_to_v1alpha1", project("v1alpha2", {"projectTemplateName": "default", "parameters": {}}))
        self.assertEqual(got, project("v1alpha1", {"projectTypeName": "default", "template": {}}))

    def test_an_empty_string_is_carried(self):
        # jq takes an empty string for a value.
        got = convert("v1alpha1_to_v1alpha2", project("v1alpha1", {"projectTypeName": ""}))
        self.assertEqual(got["spec"], {"projectTemplateName": ""})

    def test_null_is_dropped(self):
        got = convert("v1alpha1_to_v1alpha2", project("v1alpha1", {"projectTypeName": None, "template": None,
                                                                   "projectTemplateName": "kept"}))
        self.assertEqual(got["spec"], {"projectTemplateName": "kept"})


class TestLift(unittest.TestCase):
    """The up conversion lifts a parameter into a standard field only if v1alpha3 can hold it and
    the down conversion gives it back as it was, apart from a fraction, which comes back as a string."""

    CASES = (
        # name, parameters, lifted, parameters after the round trip (None: the same)
        ("the shape of the default template", {
            "administrators": [ADMIN, GROUP],
            "resourceQuota": {"requests": {"cpu": 5, "memory": "5Gi", "storage": "1Gi"},
                              "limits": {"cpu": "500m", "memory": "5Gi"}},
            "networkPolicy": "Isolated",
        }, ["administrators", "quota"], None),
        ("flat keys and a dotted resource name next to the nesting", {"resourceQuota": {
            "requests": {"nvidia.com/gpu": 2}, "pods": "10", "count/deployments.apps": "5", "services.loadbalancers": 1,
        }}, ["quota"], None),
        ("a fraction", {"resourceQuota": {"requests": {"cpu": 0.5, "memory": "512Mi"}, "limits": {"cpu": 1}}}, ["quota"],
         {"resourceQuota": {"requests": {"cpu": "0.5", "memory": "512Mi"}, "limits": {"cpu": 1}}}),
        ("a bare requests key", {"resourceQuota": {"requests": "5", "limits": "7"}}, ["quota"], None),
        ("an empty list and an empty quota", {"administrators": [], "resourceQuota": {}}, [], None),
        ("an empty requests next to a flat key", {"resourceQuota": {"requests": {}, "pods": 3}}, [], None),
        ("an empty limits next to requests", {"resourceQuota": {"requests": {"cpu": "1"}, "limits": {}}}, [], None),
        ("names instead of subjects", {"administrators": ["alice@example.com"]}, [], None),
        ("one subject instead of a list", {"administrators": ADMIN}, [], None),
        ("an item with an access level", {"administrators": [ADMIN, {"subject": "Group", "name": "developers", "accessLevel": "Editor"}]}, [], None),
        ("a ServiceAccount subject", {"administrators": [{"subject": "ServiceAccount", "name": "ci"}]}, [], None),
        ("the user and the group of ServiceAccounts", {"administrators": [
            {"subject": "User", "name": "system:serviceaccount:ci:runner"},
            {"subject": "Group", "name": "system:serviceaccounts:ci"},
        ]}, ["administrators"], None),
        ("a name with a line break", {"administrators": [{"subject": "User", "name": "alice\nkind: Group"}]}, [], None),
        ("a name with DEL", {"administrators": [{"subject": "User", "name": "alice\x7f"}]}, [], None),
        ("an empty name", {"administrators": [{"subject": "User", "name": ""}]}, [], None),
        ("a quota given as a string", {"resourceQuota": "10Gi"}, [], None),
        ("a nested quota object", {"resourceQuota": {"requests": {"cpu": "1"}, "services": {"loadbalancers": 2}}}, [], None),
        ("scopes", {"resourceQuota": {"requests": {"cpu": "1"}, "scopes": ["BestEffort"]}}, [], None),
        ("a flat key next to the nesting it collides with", {"resourceQuota": {"requests": {"cpu": "1"}, "requests.cpu": "2"}}, [], None),
        ("a flat key that would come back nested", {"resourceQuota": {"limits.memory": "1Gi"}}, [], None),
        ("a value that is no quantity", {"resourceQuota": {"requests": {"cpu": "lots"}}}, [], None),
        ("a quantity with a trailing newline", {"resourceQuota": {"requests": {"cpu": "1\n"}}}, [], None),
        ("an exponent longer than three digits", {"resourceQuota": {"limits": {"cpu": "1e-1000"}}}, [], None),
        ("the number 0", {"resourceQuota": {"requests": {"cpu": 0, "memory": "5Gi"}, "pods": 10}}, [], None),
        ("the number 0 as a flat key", {"resourceQuota": {"services.loadbalancers": 0}}, [], None),
        ("the string 0", {"resourceQuota": {"requests": {"cpu": "0"}, "limits": {"memory": "0"}}}, ["quota"], None),
        ("a quantity of 64 characters", {"resourceQuota": {"pods": "1" + "0" * 63}}, ["quota"], None),
        ("a quantity longer than 64 characters", {"resourceQuota": {"pods": "1" + "0" * 64}}, [], None),
        ("administrators lifted next to a quota left behind", {
            "administrators": [ADMIN],
            "resourceQuota": {"requests": {"cpu": "1"}, "scopes": ["BestEffort"]},
        }, ["administrators"], None),
    )

    def test_lift_gives_back(self):
        for name, parameters, lifted, back in self.CASES:
            with self.subTest(name):
                original = project("v1alpha2", {"projectTemplateName": "custom", "parameters": copy.deepcopy(parameters)})
                up = convert("v1alpha2_to_v1alpha3", original)
                self.assertEqual(up["apiVersion"], "deckhouse.io/v1alpha3")
                self.assertEqual([f for f in ("administrators", "quota") if f in up["spec"]], lifted)
                self.assert_standard_fields_read(up["spec"])

                down = convert("v1alpha3_to_v1alpha2", up)
                if back is not None:
                    original["spec"]["parameters"] = back
                self.assertEqual(down, original)

    def assert_standard_fields_read(self, spec):
        """The standard fields read the way the controller reads them: quantities in spec.quota, a
        User or Group with a name the v1alpha3 schema accepts in spec.administrators."""
        for value in spec.get("quota", {}).values():
            self.assertTrue(isinstance(value, int) or QUANTITY.fullmatch(value), value)
        for administrator in spec.get("administrators", []):
            self.assertEqual(sorted(administrator), ["kind", "name"])
            self.assertIn(administrator["kind"], ("User", "Group"))
            self.assertRegex(administrator["name"], r"^[^\x00-\x1f\x7f]+\Z")

    def test_standard_fields_win_over_parameters(self):
        spec = {"administrators": [{"kind": "User", "name": "a"}], "quota": {"pods": "1"},
                "parameters": {"administrators": [ADMIN], "resourceQuota": {"pods": "2"}}}
        self.assertEqual(convert("v1alpha2_to_v1alpha3", project("v1alpha2", copy.deepcopy(spec)))["spec"], spec)

    def test_null_parameters_are_dropped(self):
        got = convert("v1alpha2_to_v1alpha3", project("v1alpha2", {"parameters": {"administrators": None, "resourceQuota": None, "x": None}}))
        self.assertEqual(got["spec"], {"parameters": {"x": None}})

    def test_fractions_turn_into_the_text_jq_gives_them(self):
        quota = {"requests": {"cpu": 0.5, "memory": 3e-05, "storage": 1e-07}, "limits": {"cpu": 2.5e-06}, "pods": 1.0}
        got = convert("v1alpha2_to_v1alpha3", project("v1alpha2", {"parameters": {"resourceQuota": quota}}))
        self.assertEqual(got["spec"]["quota"], {"requests.cpu": "0.5", "requests.memory": "0.00003",
                                                "requests.storage": "1E-7", "limits.cpu": "0.0000025", "pods": 1.0})


class TestQuotaRoundTrip(unittest.TestCase):
    """A quota is only partly nesting: "requests.cpu" nests, "pods" does not, "requests" alone is a
    key that merely looks like nesting."""

    CASES = (
        ("requests and limits", {"requests.cpu": "1", "requests.memory": "1Gi", "limits.memory": "2Gi"}, None),
        ("a key with no nesting at all", {"pods": "10", "count/deployments.apps": "5"}, None),
        ("a resource name that itself contains a dot", {"requests.nvidia.com/gpu": "2"}, None),
        ("nesting and flat keys together", {"requests.cpu": "1", "pods": "10", "services.loadbalancers": "2"}, None),
        ("a bare requests key", {"requests": "5", "limits": "7"}, None),
        # v1alpha2 has one key for the two; the nested form wins, whatever the order of the keys.
        ("a bare requests key next to a nested one", {"requests": "5", "requests.cpu": "1"}, {"requests.cpu": "1"}),
        ("a nested key before a bare one", {"requests.cpu": "1", "requests": "5"}, {"requests.cpu": "1"}),
        # The number 0 goes down as the string "0", the only 0 the up conversion lifts.
        ("the number 0", {"requests.cpu": 0, "requests.memory": "4Gi", "services.loadbalancers": 0},
         {"requests.cpu": "0", "requests.memory": "4Gi", "services.loadbalancers": "0"}),
    )

    def test_round_trip(self):
        for name, quota, want in self.CASES:
            with self.subTest(name):
                original = project("v1alpha3", {"projectTemplateName": "default",
                                                "administrators": [{"kind": "User", "name": "alice"}], "quota": quota})
                down = convert("v1alpha3_to_v1alpha2", original)
                self.assertEqual(down["apiVersion"], "deckhouse.io/v1alpha2")
                up = convert("v1alpha2_to_v1alpha3", down)
                self.assertEqual(up["spec"]["quota"], want or quota)
                self.assertEqual(up["spec"]["administrators"], original["spec"]["administrators"])

    def test_down_nesting(self):
        got = convert("v1alpha3_to_v1alpha2", project("v1alpha3", {"quota": {
            "requests.nvidia.com/gpu": 2, "limits.cpu": "1", "requests.": "3", "": "4", "pods": 0}}))
        self.assertEqual(got["spec"], {"parameters": {"resourceQuota": {
            "requests": {"nvidia.com/gpu": 2, "": "3"}, "limits": {"cpu": "1"}, "": "4", "pods": "0"}}})

    def test_empty_standard_fields_go_down(self):
        # An empty list or map is a value to jq, so it replaces the parameter.
        got = convert("v1alpha3_to_v1alpha2", project("v1alpha3", {"administrators": [], "quota": {},
                                                                   "parameters": {"administrators": [ADMIN], "x": 1}}))
        self.assertEqual(got["spec"], {"parameters": {"administrators": [], "x": 1, "resourceQuota": {}}})


class TestStatus(unittest.TestCase):
    def test_namespaces_round_trip(self):
        original = project("v1alpha3", {"projectTemplateName": "default"}, {
            "state": "Deployed",
            "namespaces": [{"name": "backend", "kind": "Main"}, {"name": "backend-cache", "kind": "Additional"}],
        }, name="backend")
        down = convert("v1alpha3_to_v1alpha2", original)
        self.assertEqual(down["status"], {"state": "Deployed", "namespaces": ["backend", "backend-cache"]})
        self.assertEqual(convert("v1alpha2_to_v1alpha3", down)["status"], original["status"])

    def test_items_of_another_shape(self):
        up = convert("v1alpha2_to_v1alpha3", project("v1alpha2", None, {"namespaces": ["test", 5, {"name": "x"}]}))
        self.assertEqual(up["status"]["namespaces"], [{"name": "test", "kind": "Main"}, 5, {"name": "x"}])
        down = convert("v1alpha3_to_v1alpha2", project("v1alpha3", None, {"namespaces": [{"kind": "Main"}, "a", 5, {"name": "b"}]}))
        self.assertEqual(down["status"]["namespaces"], ["a", "b"])

    def test_missing_spec_and_status(self):
        for binding, obj in (
            ("v1alpha1_to_v1alpha2", project("v1alpha1")),
            ("v1alpha2_to_v1alpha3", project("v1alpha2")),
            ("v1alpha3_to_v1alpha2", project("v1alpha3")),
            ("v1alpha2_to_v1alpha3", project("v1alpha2", status={"state": "Error"})),
            ("v1alpha3_to_v1alpha2", project("v1alpha3", status={"namespaces": []})),
        ):
            with self.subTest(binding):
                got = convert(binding, obj)
                self.assertEqual(got["apiVersion"], "deckhouse.io/" + binding.split("_to_")[1])
                self.assertEqual("spec" in got, "spec" in obj, "a missing spec is not conjured")


class TestGolden(unittest.TestCase):
    """The golden fixtures pin each conversion object by object rather than field by field, so
    whatever a change to the hook adds, drops or leaves behind shows up. The controller holds its Go
    copy of the down conversion to the parameters fixture as well."""

    def setUp(self):
        self.module = module_dir()
        if self.module is None:
            self.skipTest("the golden fixtures are in the repository, which is not around")
        self.fixtures = self.module / "hooks" / "testdata" / "conversion" / "projects"

    def test_conversion_paths(self):
        paths = sorted(self.fixtures.glob("*.yaml")) + sorted(self.fixtures.glob("parameters/*.yaml"))
        self.assertEqual(len(paths), 6)
        for path in paths:
            check_golden(self, projects, path)

    def test_every_binding_has_its_fixture(self):
        single_steps = {p.stem for p in self.fixtures.glob("*.yaml") if p.stem.count("_to_") == 1}
        bindings = {b["name"] for b in yaml.safe_load(projects.config)["kubernetesCustomResourceConversion"]}
        self.assertEqual(single_steps, bindings)

    def test_builtin_templates(self):
        """The controller renders converted.yaml with the structured built-in template and checks it
        selects what resources.yaml selected; this keeps converted.yaml what the conversion produces."""
        helm = self.module / "images" / "multitenancy-manager" / "src" / "internal" / "helm" / "testdata" / "helm_template"
        for template in ("default", "secure", "secure-with-dedicated-nodes"):
            with self.subTest(template):
                project_v2 = yaml.safe_load((helm / template / "project.yaml").read_text())
                expected = yaml.safe_load((helm / template / "converted.yaml").read_text())
                self.assertEqual(convert("v1alpha2_to_v1alpha3", project_v2), expected)


class TestRequests(unittest.TestCase):
    def test_mixed_versions(self):
        v1alpha1 = project("v1alpha1", {"projectTypeName": "default"})
        v1alpha3 = project("v1alpha3", {"quota": {"pods": "1"}})
        v1alpha2 = project("v1alpha2", {"parameters": {"resourceQuota": {"pods": "1"}}})
        out = run("v1alpha2_to_v1alpha3", v1alpha1, v1alpha2, v1alpha3)
        tests.assert_conversion(self, out, [v1alpha1, project("v1alpha3", {"parameters": {}, "quota": {"pods": "1"}}), v1alpha3], None)

    def test_empty_request(self):
        for binding in BINDINGS:
            with self.subTest(binding):
                tests.assert_conversion(self, run(binding), [], None)

    def test_wrong_shape_fails_the_whole_request(self):
        # BaseConversionHook reports the exception. The module hook it replaces said "Conversion of
        # projects.deckhouse.io failed" between v1alpha1 and v1alpha2 and "Conversion of
        # deckhouse.io failed" between v1alpha2 and v1alpha3.
        for binding, bad, message in (
            ("v1alpha1_to_v1alpha2", {"spec": "x"}, "cannot index str with 'projectTypeName'"),
            ("v1alpha2_to_v1alpha3", {"spec": {"parameters": "x"}}, "cannot index str with 'administrators'"),
            ("v1alpha2_to_v1alpha3", {"status": "x"}, "cannot index str with 'namespaces'"),
            ("v1alpha3_to_v1alpha2", {"spec": {"quota": {"pods": "1"}, "parameters": "x"}}, "'str' object is not a mapping"),
            ("v1alpha3_to_v1alpha2", {"spec": {"administrators": ["x"]}}, "cannot index str with 'kind'"),
        ):
            with self.subTest(f"{binding} {bad}"):
                obj = dict(project(binding.split("_to_")[0]), **bad)
                out = run(binding, project(binding.split("_to_")[0], {}), obj)
                tests.assert_conversion(self, out, None, "Internal error: " + message)


if __name__ == "__main__":
    unittest.main()
