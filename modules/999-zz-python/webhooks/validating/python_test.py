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
from deckhouse_sdk import hook

# THIS FILE MUST NOT BE EXECUTABLE! Deckhouse runs all files with execute bit. Since tests are not
# meant to be run, make sure to `chmod -x` them.


def test_parse_version():
    def f(v):
        return python.parse_version(v)

    assert f("3.11") == {"major": 3, "minor": 11}
    assert f({"major": 3, "minor": 11}) == {"major": 3, "minor": 11}
    assert f({"major": "3", "minor": "11"}) == {"major": 3, "minor": 11}


def test_valid_v1alpha1():
    out = hook.testrun(
        func=python.main,
        binding_context=bctx_create_v1alpha1_valid,
    )
    print(out.validations.data)
    assert out.validations.data == [{"allowed": True}]


def test_invalid_name_schema_v1beta1():
    out = hook.testrun(
        func=python.main,
        binding_context=bctx_create_v1beta1_name_malformed,
    )
    assert out.validations.data == [
        {
            "allowed": False,
            "message": "Name must comply with schema python-$major-$minor, got python-3-11-what",
        }
    ]


bctx_create_v1alpha1_valid = [
    {
        # Name as defined in binding configuration.
        "binding": "python-crd-name.deckhouse.io",
        # Validating to distinguish from other events.
        "type": "Validating",
        # AdmissionReview object.
        "review": {
            "apiVersion": "admission.k8s.io/v1",
            "kind": "AdmissionReview",
            "request": {
                "dryRun": False,
                "kind": {
                    "group": "deckhouse.io",
                    "kind": "Python",
                    "version": "v1alpha1",
                },
                "name": "python-3-11",
                "object": {
                    "apiVersion": "deckhouse.io/v1alpha1",
                    "kind": "Python",
                    "metadata": {
                        "creationTimestamp": "2023-01-30T13:47:23Z",
                        "generation": 1,
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
                                    "f:spec": {
                                        ".": {},
                                        "f:modules": {},
                                        "f:version": {
                                            ".": {},
                                            "f:major": {},
                                            "f:minor": {},
                                        },
                                    },
                                },
                                "manager": "kubectl-client-side-apply",
                                "operation": "Update",
                                "time": "2023-01-30T13:47:23Z",
                            }
                        ],
                        "name": "python-3-11",
                        "uid": "51688527-dc6f-4eb2-81c8-337e3670c44b",
                    },
                    "spec": {
                        "modules": ["dotmap", "yaml"],
                        "version": "3.11",
                    },
                },
                "oldObject": None,
                "operation": "CREATE",
                "options": {
                    "apiVersion": "meta.k8s.io/v1",
                    "fieldManager": "kubectl-client-side-apply",
                    "kind": "CreateOptions",
                },
                "requestKind": {
                    "group": "deckhouse.io",
                    "kind": "Python",
                    "version": "v1alpha1",
                },
                "requestResource": {
                    "group": "deckhouse.io",
                    "resource": "pythons",
                    "version": "v1alpha1",
                },
                "resource": {
                    "group": "deckhouse.io",
                    "resource": "pythons",
                    "version": "v1alpha1",
                },
                "uid": "7ee5aadd-6cad-49c9-a085-ae8ae8291257",
                "userInfo": {
                    "groups": ["system:masters", "system:authenticated"],
                    "username": "kubernetes-admin",
                },
            },
        },
    }
]
bctx_create_v1beta1_name_malformed = [
    {
        # Name as defined in binding configuration.
        "binding": "python-crd-name.deckhouse.io",
        # Validating to distinguish from other events.
        "type": "Validating",
        # AdmissionReview object.
        "review": {
            "apiVersion": "admission.k8s.io/v1",
            "kind": "AdmissionReview",
            "request": {
                "dryRun": False,
                "kind": {
                    "group": "deckhouse.io",
                    "kind": "Python",
                    "version": "v1beta1",
                },
                "name": "python-3-11-what",
                "object": {
                    "apiVersion": "deckhouse.io/v1beta1",
                    "kind": "Python",
                    "metadata": {
                        "creationTimestamp": "2023-01-30T13:47:23Z",
                        "generation": 1,
                        "managedFields": [
                            {
                                "apiVersion": "deckhouse.io/v1beta1",
                                "fieldsType": "FieldsV1",
                                "fieldsV1": {
                                    "f:metadata": {
                                        "f:annotations": {
                                            ".": {},
                                            "f:kubectl.kubernetes.io/last-applied-configuration": {},
                                        }
                                    },
                                    "f:spec": {
                                        ".": {},
                                        "f:modules": {},
                                        "f:version": {
                                            ".": {},
                                            "f:major": {},
                                            "f:minor": {},
                                        },
                                    },
                                },
                                "manager": "kubectl-client-side-apply",
                                "operation": "Update",
                                "time": "2023-01-30T13:47:23Z",
                            }
                        ],
                        "name": "python-3-11-what",
                        "uid": "51688527-dc6f-4eb2-81c8-337e3670c44b",
                    },
                    "spec": {
                        "modules": ["dotmap", "yaml"],
                        "version": {"major": 3, "minor": 11},
                    },
                },
                "oldObject": None,
                "operation": "CREATE",
                "options": {
                    "apiVersion": "meta.k8s.io/v1",
                    "fieldManager": "kubectl-client-side-apply",
                    "kind": "CreateOptions",
                },
                "requestKind": {
                    "group": "deckhouse.io",
                    "kind": "Python",
                    "version": "v1beta1",
                },
                "requestResource": {
                    "group": "deckhouse.io",
                    "resource": "pythons",
                    "version": "v1beta1",
                },
                "resource": {
                    "group": "deckhouse.io",
                    "resource": "pythons",
                    "version": "v1beta1",
                },
                "uid": "7ee5aadd-6cad-49c9-a085-ae8ae8291257",
                "userInfo": {
                    "groups": ["system:masters", "system:authenticated"],
                    "username": "kubernetes-admin",
                },
            },
        },
    }
]
