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

from deckhouse import hook, tests

from pathlib import Path
from deckhouse.tests import load_conversion_webhook

# The conversion steps in the order of the ConversionWebhook in templates/conversion-webhook.yaml.
webhook = load_conversion_webhook(
    "yandexinstanceclasses.deckhouse.io",
    [("v1alpha1", "v1"), ("v1", "v1alpha1")],
    Path(__file__).parent / "yandex_instance_class.py",
)
main = webhook.main

ALPHA1 = "deckhouse.io/v1alpha1"
V1 = "deckhouse.io/v1"
FAILED_MESSAGE = "Conversion of yandexinstanceclasses.deckhouse.io failed"


def instance_class(api_version: str, name: str = "worker", **spec) -> dict:
    return {
        "apiVersion": api_version,
        "kind": "YandexInstanceClass",
        "metadata": {"name": name},
        "spec": {"cores": 4, "memory": 8192, **spec},
    }


def context(binding: str, objects, from_version: str = ALPHA1, to_version: str = V1) -> dict:
    return {
        "binding": binding,
        "type": "Conversion",
        "fromVersion": from_version,
        "toVersion": to_version,
        "review": {"request": {"uid": "42", "desiredAPIVersion": to_version, "objects": objects}},
    }


def run(binding: str, objects, **kwargs) -> hook.Output:
    return hook.testrun(main, [context(binding, objects, **kwargs)])


def to_v1(objects) -> hook.Output:
    return run("v1alpha1_to_v1", objects)


def to_alpha1(objects) -> hook.Output:
    return run("v1_to_v1alpha1", objects, from_version=V1, to_version=ALPHA1)


def assert_internal_error(t: unittest.TestCase, out: hook.Output):
    """BaseConversionHook reports an exception as "Internal error: <exception>"."""
    t.assertEqual(len(out.conversions.data), 1)
    t.assertEqual(list(out.conversions.data[0]), ["failedMessage"])
    t.assertTrue(out.conversions.data[0]["failedMessage"].startswith("Internal error"), out.conversions.data[0])


class TestSteps(unittest.TestCase):
    def test_methods_are_named_as_the_bindings(self):
        # webhook-operator names the binding of a conversion step <from>_to_<to>, and
        # BaseConversionHook calls the method with that name.
        self.assertTrue(callable(webhook.Conversion.v1alpha1_to_v1))
        self.assertTrue(callable(webhook.Conversion.v1_to_v1alpha1))


class TestAlpha1ToV1(unittest.TestCase):
    def test_network_types_are_renamed(self):
        out = to_v1([
            instance_class(ALPHA1, "a", networkType="STANDARD"),
            instance_class(ALPHA1, "b", networkType="SOFTWARE_ACCELERATED"),
        ])
        tests.assert_conversion(self, out, [
            instance_class(V1, "a", networkType="Standard"),
            instance_class(V1, "b", networkType="SoftwareAccelerated"),
        ], None)

    def test_unknown_and_already_converted_values_are_kept(self):
        out = to_v1([
            instance_class(ALPHA1, "a", networkType="Custom"),
            instance_class(ALPHA1, "b", networkType="Standard"),
        ])
        tests.assert_conversion(self, out, [
            instance_class(V1, "a", networkType="Custom"),
            instance_class(V1, "b", networkType="Standard"),
        ], None)

    def test_without_network_type_only_api_version_changes(self):
        obj = instance_class(ALPHA1)
        no_spec = {"apiVersion": ALPHA1, "metadata": {"name": "no-spec"}}
        out = to_v1([obj, no_spec, {**no_spec, "spec": None}])
        tests.assert_conversion(self, out, [
            instance_class(V1),
            {**no_spec, "apiVersion": V1},
            {**no_spec, "apiVersion": V1, "spec": None},
        ], None)

    def test_non_string_and_empty_network_type_values_are_kept(self):
        values = [None, False, 0, "", [], {}]
        out = to_v1([instance_class(ALPHA1, str(i), networkType=v) for i, v in enumerate(values)])
        tests.assert_conversion(self, out, [instance_class(V1, str(i), networkType=v) for i, v in enumerate(values)], None)

    def test_input_object_is_not_mutated(self):
        obj = instance_class(ALPHA1, networkType="STANDARD")
        to_v1([obj])
        self.assertEqual(obj, instance_class(ALPHA1, networkType="STANDARD"))


class TestV1ToAlpha1(unittest.TestCase):
    def test_network_types_are_renamed(self):
        out = to_alpha1([
            instance_class(V1, "a", networkType="Standard"),
            instance_class(V1, "b", networkType="SoftwareAccelerated"),
            instance_class(V1, "c", networkType="STANDARD"),
            instance_class(V1, "d"),
        ])
        tests.assert_conversion(self, out, [
            instance_class(ALPHA1, "a", networkType="STANDARD"),
            instance_class(ALPHA1, "b", networkType="SOFTWARE_ACCELERATED"),
            instance_class(ALPHA1, "c", networkType="STANDARD"),
            instance_class(ALPHA1, "d"),
        ], None)

    def test_round_trip(self):
        originals = [
            instance_class(ALPHA1, "a", networkType="STANDARD"),
            instance_class(ALPHA1, "b", networkType="SOFTWARE_ACCELERATED"),
            instance_class(ALPHA1, "c"),
        ]
        v1 = to_v1(originals).conversions.data[0]["convertedObjects"]
        tests.assert_conversion(self, to_alpha1(v1), originals, None)


class TestObjectsList(unittest.TestCase):
    def test_objects_in_other_versions_pass_through(self):
        foreign = [
            instance_class(V1, "v1", networkType="STANDARD"),
            instance_class("deckhouse.io/v2", "v2", networkType="STANDARD"),
            {"apiVersion": V1, "metadata": {"name": "foreign-broken-spec"}, "spec": "not an object"},
        ]
        out = to_v1([instance_class(ALPHA1, "alpha1", networkType="STANDARD"), *foreign])
        tests.assert_conversion(self, out, [instance_class(V1, "alpha1", networkType="Standard"), *foreign], None)

    def test_empty_objects(self):
        tests.assert_conversion(self, to_v1([]), [], None)
        tests.assert_conversion(self, to_v1(""), [], None)

    def test_missing_objects_is_an_internal_error(self):
        for objects in (None, False):
            with self.subTest(objects=objects):
                assert_internal_error(self, to_v1(objects))
        assert_internal_error(self, hook.testrun(main, [{**context("v1alpha1_to_v1", []), "review": {"request": {}}}]))

    def test_objects_map_is_an_internal_error(self):
        assert_internal_error(self, to_v1({"x": instance_class(ALPHA1, networkType="STANDARD")}))

    def test_object_without_api_version_is_an_internal_error(self):
        assert_internal_error(self, to_v1([{"metadata": {"name": "no-api-version"}, "spec": {}}]))


class TestErrors(unittest.TestCase):
    def test_failed_message(self):
        for run_conversion, source in ((to_v1, ALPHA1), (to_alpha1, V1)):
            cases = {
                "spec is a string": [instance_class(source) | {"spec": "x"}],
                "spec is false": [instance_class(source) | {"spec": False}],
                "spec is a list": [instance_class(source) | {"spec": []}],
            }
            for name, objects in cases.items():
                with self.subTest(f"{source}: {name}"):
                    tests.assert_conversion(self, run_conversion(objects), None, FAILED_MESSAGE)

    def test_failed_message_per_broken_object(self):
        # BaseConversionHook joins the messages of the objects it could not convert with ";".
        broken = instance_class(ALPHA1) | {"spec": "x"}
        out = to_v1([broken, instance_class(ALPHA1, "ok"), broken])
        tests.assert_conversion(self, out, None, f"{FAILED_MESSAGE};{FAILED_MESSAGE}")

    def test_internal_error_on_malformed_objects(self):
        for run_conversion in (to_v1, to_alpha1):
            cases = {
                "objects is a string": "objects",
                "objects is a number": 1,
                "objects is zero": 0,
                "objects is true": True,
                "object is a string": ["obj"],
                "object is a list": [[]],
                "object is null": [None],
            }
            for name, objects in cases.items():
                with self.subTest(f"{run_conversion.__name__}: {name}"):
                    assert_internal_error(self, run_conversion(objects))

    def test_internal_error_on_broken_review(self):
        for review in ("x", {"request": 1}):
            with self.subTest(review=review):
                assert_internal_error(self, hook.testrun(main, [{**context("v1alpha1_to_v1", []), "review": review}]))

    def test_unknown_binding_is_an_error(self):
        out = hook.testrun(main, [context("alpha1_to_v1", [instance_class(ALPHA1)])])
        tests.assert_conversion(self, out, None, "Internal error. Handler for binding alpha1_to_v1 not found")


if __name__ == "__main__":
    unittest.main()
