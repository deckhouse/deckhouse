#!/usr/bin/env python3
#
# Copyright 2023 Flant JSC
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

import python
from deckhouse import hook
from dotmap import DotMap

# THIS FILE MUST NOT BE EXECUTABLE! Deckhouse runs all files with execute bit. Since tests are not
# meant to be run, make sure to `chmod -x` them.


def test_conversion_from_v1alpha1_to_v1beta1():
    out = hook.testrun(
        func=python.main,
        binding_context=__conv_request(obj_v1a1, to_version="deckhouse.io/v1beta1"),
    )
    result = out.conversions.data

    # check output data structure, which is the library responsibility
    assert isinstance(result, list)
    assert len(result) == 1
    assert "convertedObjects" in result[0]

    # check the number of converted objects, which is the hook responsibility
    converted_objects = result[0]["convertedObjects"]
    assert len(converted_objects) == 1

    # check unchanged parts
    converted_v1b1 = converted_objects[0]
    assert converted_v1b1["kind"] == "Python"
    assert converted_v1b1["metadata"]["name"] == "python-3-10"
    assert converted_v1b1["spec"]["modules"] == ["dotmap", "pyyaml"]

    # check converted parts
    assert converted_v1b1["apiVersion"] == "deckhouse.io/v1beta1"
    assert converted_v1b1["spec"]["version"] == {"major": 3, "minor": 10}

    # check as a whole
    assert converted_v1b1 == obj_v1b1


def test_conversion_from_v1beta1_to_v1():
    out = hook.testrun(
        func=python.main,
        binding_context=__conv_request(obj_v1b1, to_version="deckhouse.io/v1"),
    )
    result = out.conversions.data

    # check output data structure, which is the library responsibility
    assert isinstance(result, list)
    assert len(result) == 1
    assert "convertedObjects" in result[0]

    # check the number of converted objects, which is the hook responsibility
    converted_objects = result[0]["convertedObjects"]
    assert len(converted_objects) == 1

    # check unchanged parts
    converted_v1 = converted_objects[0]
    assert converted_v1["kind"] == "Python"
    assert converted_v1["metadata"]["name"] == "python-3-10"
    assert converted_v1["spec"]["version"] == {"major": 3, "minor": 10}

    # check converted parts
    assert converted_v1["apiVersion"] == "deckhouse.io/v1"
    assert converted_v1["spec"]["modules"] == [
        {"name": "dotmap"},
        {"name": "pyyaml"},
    ]

    # check as a whole
    assert converted_v1 == obj_v1


def test_backward_conversion_from_v1_to_v1beta1():
    out = hook.testrun(
        func=python.main,
        binding_context=__conv_request(obj_v1, to_version="deckhouse.io/v1beta1"),
    )
    result = out.conversions.data

    # check output data structure, which is the library responsibility
    assert isinstance(result, list)
    assert len(result) == 1
    assert "convertedObjects" in result[0]

    # check the number of converted objects, which is the hook responsibility
    converted_objects = result[0]["convertedObjects"]
    assert len(converted_objects) == 1

    # check as a whole
    converted_v1b1 = converted_objects[0]
    assert converted_v1b1 == obj_v1b1


def test_backward_conversion_from_v1beta1_to_v1alpha():
    out = hook.testrun(
        func=python.main,
        binding_context=__conv_request(obj_v1b1, to_version="deckhouse.io/v1alpha1"),
    )
    result = out.conversions.data

    # check output data structure, which is the library responsibility
    assert isinstance(result, list)
    assert len(result) == 1
    assert "convertedObjects" in result[0]

    # check the number of converted objects, which is the hook responsibility
    converted_objects = result[0]["convertedObjects"]
    assert len(converted_objects) == 1

    # check as a whole
    converted_v1a1 = converted_objects[0]
    assert converted_v1a1 == obj_v1a1


def __conv_request(obj: dict, to_version: str):
    return [
        {
            "binding": "python_conversions",
            "type": "Conversion",
            "fromVersion": obj["apiVersion"],
            "toVersion": to_version,
            "review": {
                "apiVersion": "apiextensions.k8s.io/v1",
                "kind": "ConversionReview",
                "request": {
                    "desiredAPIVersion": to_version,
                    "objects": [obj],
                    "uid": "78eed1d5-44b1-4836-8ed1-c22cae938c30",
                },
            },
        }
    ]


obj_v1a1 = {
    "apiVersion": "deckhouse.io/v1alpha1",
    "kind": "Python",
    "metadata": {
        "creationTimestamp": "2023-01-24T15:05:50Z",
        "generation": 2,
        "managedFields": [
            {
                "apiVersion": "deckhouse.io/v1alpha1",
                "fieldsType": "FieldsV1",
                "fieldsV1": {
                    "f:metadata": {
                        "f:annotations": {
                            ".": {},
                            "f:kubectl.kubernetes.io/last-applied-configuration": {},
                        }
                    },
                    "f:spec": {".": {}, "f:version": {}},
                },
                "manager": "kubectl-client-side-apply",
                "operation": "Update",
                "time": "2023-01-24T15:05:50Z",
            },
            {
                "apiVersion": "deckhouse.io/v1alpha1",
                "fieldsType": "FieldsV1",
                "fieldsV1": {"f:spec": {"f:modules": {}}},
                "manager": "kubectl-edit",
                "operation": "Update",
                "time": "2023-01-26T14:26:00Z",
            },
        ],
        "name": "python-3-10",
        "uid": "5d9963f8-52fd-4137-970d-2ccfb50efc61",
    },
    "spec": {
        "modules": ["dotmap", "pyyaml"],
        "version": "3.10",
    },
}


obj_v1b1 = {
    "apiVersion": "deckhouse.io/v1beta1",
    "kind": "Python",
    "metadata": {
        "creationTimestamp": "2023-01-24T15:05:50Z",
        "generation": 2,
        "managedFields": [
            {
                "apiVersion": "deckhouse.io/v1alpha1",
                "fieldsType": "FieldsV1",
                "fieldsV1": {
                    "f:metadata": {
                        "f:annotations": {
                            ".": {},
                            "f:kubectl.kubernetes.io/last-applied-configuration": {},
                        }
                    },
                    "f:spec": {".": {}, "f:version": {}},
                },
                "manager": "kubectl-client-side-apply",
                "operation": "Update",
                "time": "2023-01-24T15:05:50Z",
            },
            {
                "apiVersion": "deckhouse.io/v1alpha1",
                "fieldsType": "FieldsV1",
                "fieldsV1": {"f:spec": {"f:modules": {}}},
                "manager": "kubectl-edit",
                "operation": "Update",
                "time": "2023-01-26T14:26:00Z",
            },
        ],
        "name": "python-3-10",
        "uid": "5d9963f8-52fd-4137-970d-2ccfb50efc61",
    },
    "spec": {
        "modules": ["dotmap", "pyyaml"],
        "version": {
            "major": 3,
            "minor": 10,
        },
    },
}


obj_v1 = {
    "apiVersion": "deckhouse.io/v1",
    "kind": "Python",
    "metadata": {
        "creationTimestamp": "2023-01-24T15:05:50Z",
        "generation": 2,
        "managedFields": [
            {
                "apiVersion": "deckhouse.io/v1alpha1",
                "fieldsType": "FieldsV1",
                "fieldsV1": {
                    "f:metadata": {
                        "f:annotations": {
                            ".": {},
                            "f:kubectl.kubernetes.io/last-applied-configuration": {},
                        }
                    },
                    "f:spec": {".": {}, "f:version": {}},
                },
                "manager": "kubectl-client-side-apply",
                "operation": "Update",
                "time": "2023-01-24T15:05:50Z",
            },
            {
                "apiVersion": "deckhouse.io/v1alpha1",
                "fieldsType": "FieldsV1",
                "fieldsV1": {"f:spec": {"f:modules": {}}},
                "manager": "kubectl-edit",
                "operation": "Update",
                "time": "2023-01-26T14:26:00Z",
            },
        ],
        "name": "python-3-10",
        "uid": "5d9963f8-52fd-4137-970d-2ccfb50efc61",
    },
    "spec": {
        "modules": [
            {"name": "dotmap"},
            {"name": "pyyaml"},
        ],
        "version": {
            "major": 3,
            "minor": 10,
        },
    },
}
