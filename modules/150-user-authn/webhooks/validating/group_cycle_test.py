#!/usr/bin/python3

# Copyright 2024 Flant JSC
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
import unittest

from deckhouse import hook, tests
from dotmap import DotMap

from group import main


def _prepare_validation_binding_context(binding_context_json, new_spec: dict, snapshots: dict) -> DotMap:
    ctx_dict = json.loads(binding_context_json)
    ctx = DotMap(ctx_dict)
    ctx.review.request.object.spec = new_spec
    ctx.snapshots = snapshots
    return ctx

DEFAULT_UPDATE_SNAPSHOT = {
    "groups": [
        {
            "filterResult": {
                "groupName": "group-1",
                "members": [
                    {
                        "kind": "User",
                        "name": "superadmin"
                    }
                ],
                "name": "group-1"
            }
        },
        {
            "filterResult": {
                "groupName": "group-2",
                "members": [
                    {
                        "kind": "User",
                        "name": "test"
                    },
                    {
                        "kind": "Group",
                        "name": "group-1"
                    }
                ],
                "name": "group-2"
            }
        }
    ],
    "users": [
        {
            "filterResult": {
                "userName": "superadmin"
            }
        },
        {
            "filterResult": {
                "userName": "test"
            }
        }
    ]
}

def _prepare_update_binding_context(new_spec: dict, snapshots: dict) -> DotMap:
    binding_context_json = """
{
    "binding": "groups-unique.deckhouse.io",
    "review": {
        "request": {
            "uid": "8af60184-b30b-4b90-a33e-0c190f10e96d",
            "kind": {
                "group": "deckhouse.io",
                "version": "v1alpha1",
                "kind": "Group"
            },
            "resource": {
                "group": "deckhouse.io",
                "version": "v1alpha1",
                "resource": "groups"
            },
            "requestKind": {
                "group": "deckhouse.io",
                "version": "v1alpha1",
                "kind": "Group"
            },
            "requestResource": {
                "group": "deckhouse.io",
                "version": "v1alpha1",
                "resource": "groups"
            },
            "name": "candi-admins",
            "operation": "UPDATE",
            "userInfo": {
                "username": "kubernetes-admin",
                "groups": [
                    "system:masters",
                    "system:authenticated"
                ]
            },
            "object": {
                "apiVersion": "deckhouse.io/v1alpha1",
                "kind": "Group",
                "metadata": {
                    "creationTimestamp": "2023-07-17T13:40:39Z",
                    "generation": 3,
                    "managedFields": [
                        {
                            "apiVersion": "deckhouse.io/v1alpha1",
                            "fieldsType": "FieldsV1",
                            "fieldsV1": {
                                "f:spec": {
                                    ".": {},
                                    "f:name": {}
                                }
                            },
                            "manager": "deckhouse-controller",
                            "operation": "Update",
                            "time": "2023-07-17T13:40:39Z"
                        },
                        {
                            "apiVersion": "deckhouse.io/v1alpha1",
                            "fieldsType": "FieldsV1",
                            "fieldsV1": {
                                "f:spec": {
                                    "f:members": {}
                                }
                            },
                            "manager": "kubectl-edit",
                            "operation": "Update",
                            "time": "2024-11-21T14:44:35Z"
                        }
                    ],
                    "name": "group-1",
                    "resourceVersion": "1184522270",
                    "uid": "7820c68b-6423-49f0-b97f-b7e314e23c0b"
                },
                "spec": {}
            },
            "oldObject": {
                "apiVersion": "deckhouse.io/v1alpha1",
                "kind": "Group",
                "metadata": {
                    "creationTimestamp": "2023-07-17T13:40:39Z",
                    "generation": 2,
                    "managedFields": [
                        {
                            "apiVersion": "deckhouse.io/v1alpha1",
                            "fieldsType": "FieldsV1",
                            "fieldsV1": {
                                "f:spec": {
                                    ".": {},
                                    "f:name": {}
                                }
                            },
                            "manager": "deckhouse-controller",
                            "operation": "Update",
                            "time": "2023-07-17T13:40:39Z"
                        },
                        {
                            "apiVersion": "deckhouse.io/v1alpha1",
                            "fieldsType": "FieldsV1",
                            "fieldsV1": {
                                "f:spec": {
                                    "f:members": {}
                                }
                            },
                            "manager": "kubectl-edit",
                            "operation": "Update",
                            "time": "2024-11-20T14:00:21Z"
                        }
                    ],
                    "name": "candi-admins",
                    "resourceVersion": "1184522270",
                    "uid": "7820c68b-6423-49f0-b97f-b7e314e23c0b"
                },
                "spec": {
                    "members": [
                        {
                            "kind": "User",
                            "name": "superadmin"
                        },
                        {
                            "kind": "Group",
                            "name": "none-exists"
                        }
                    ],
                    "name": "candi-admins"
                }
            },
            "dryRun": false,
            "options": {
                "kind": "UpdateOptions",
                "apiVersion": "meta.k8s.io/v1",
                "fieldManager": "kubectl-edit",
                "fieldValidation": "Strict"
            }
        }
    },
    "snapshots": {},
    "type": "Validating"
}
"""
    return _prepare_validation_binding_context(binding_context_json, new_spec, snapshots)

DEFAULT_CREATE_SNAPSHOT = {
    "groups": [
            {
                "filterResult": {
                    "groupName": "admins",
                    "members": [
                        {
                            "kind": "User",
                            "name": "admin"
                        },
                        {
                            "kind": "User",
                            "name": "weak-admin"
                        }
                    ],
                    "name": "admins"
                }
            },
            {
                "filterResult": {
                    "groupName": "group-1",
                    "members": [
                        {
                            "kind": "User",
                            "name": "superadmin"
                        },
                        {
                            "kind": "Group",
                            "name": "group-2"
                        },
                        {
                            "kind": "Group",
                            "name": "new-group"
                        }
                    ],
                    "name": "group-1"
                }
            },
            {
                "filterResult": {
                    "groupName": "group-2",
                    "members": [
                        {
                            "kind": "User",
                            "name": "test"
                        },
                        {
                            "kind": "Group",
                            "name": "new-group"
                        }
                    ],
                    "name": "group-2"
                }
            },
            {
                "filterResult": {
                    "groupName": "group-3",
                    "members": [
                        {
                            "kind": "User",
                            "name": "test"
                        },
                        {
                            "kind": "Group",
                            "name": "new-group"
                        }
                    ],
                    "name": "group-3"
                }
            }
        ],
        "users": [
            {
                "filterResult": {
                    "userName": "superadmin"
                }
            },
            {
                "filterResult": {
                    "userName": "test"
                }
            }
        ]
    }

def _prepare_create_binding_context(new_spec: dict, snapshots: dict) -> DotMap:
    binding_context_json = """
{
    "binding": "groups-unique.deckhouse.io",
    "review": {
        "request": {
            "uid": "adedd292-0be9-476b-b2fa-8286053a1b1b",
            "kind": {
                "group": "deckhouse.io",
                "version": "v1alpha1",
                "kind": "Group"
            },
            "resource": {
                "group": "deckhouse.io",
                "version": "v1alpha1",
                "resource": "groups"
            },
            "requestKind": {
                "group": "deckhouse.io",
                "version": "v1alpha1",
                "kind": "Group"
            },
            "requestResource": {
                "group": "deckhouse.io",
                "version": "v1alpha1",
                "resource": "groups"
            },
            "name": "new",
            "operation": "CREATE",
            "userInfo": {
                "username": "kubernetes-admin",
                "groups": [
                    "system:masters",
                    "system:authenticated"
                ]
            },
            "object": {
                "apiVersion": "deckhouse.io/v1alpha1",
                "kind": "Group",
                "metadata": {
                    "creationTimestamp": "2024-11-22T08:00:33Z",
                    "generation": 1,
                    "managedFields": [
                        {
                            "apiVersion": "deckhouse.io/v1alpha1",
                            "fieldsType": "FieldsV1",
                            "fieldsV1": {
                                "f:spec": {
                                    ".": {},
                                    "f:members": {},
                                    "f:name": {}
                                }
                            },
                            "manager": "kubectl-create",
                            "operation": "Update",
                            "time": "2024-11-22T08:00:33Z"
                        }
                    ],
                    "name": "new",
                    "uid": "f43bdc3f-61a2-4957-ae5a-241972717118"
                },
                "spec": {}
            },
            "oldObject": null,
            "dryRun": false,
            "options": {
                "kind": "CreateOptions",
                "apiVersion": "meta.k8s.io/v1",
                "fieldManager": "kubectl-create",
                "fieldValidation": "Strict"
            }
        }
    },
    "snapshots": {},
    "type": "Validating"
}
"""
    return _prepare_validation_binding_context(binding_context_json, new_spec, snapshots)


class TestGroupCycleValidationWebhook(unittest.TestCase):
    def test_create_group_should_fail_with_cycle_detected(self):
        ctx = _prepare_create_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "test"
                },
                {
                    "kind": "Group",
                    "name": "group-2"
                }
            ],
            "name": "new-group"
        }, DEFAULT_CREATE_SNAPSHOT)
        out = hook.testrun(main, [ctx])
        err_msg = (f'Invalid group hierarchy: cycle detected! Path: groups.deckhouse.io("group-1" -> "group-2" -> '
                   '"new-group" -> "group-2"). Groups must form a tree without circular references.')
        tests.assert_validation_deny(self, out, err_msg)

    def test_create_group_should_fail_with_cycle_detected_2(self):
        ctx = _prepare_create_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "test"
                },
                {
                    "kind": "Group",
                    "name": "new-group"
                }
            ],
            "name": "new-group"
        }, DEFAULT_CREATE_SNAPSHOT)
        out = hook.testrun(main, [ctx])
        err_msg = (f'Invalid group hierarchy: cycle detected! Path: groups.deckhouse.io("group-1" -> "group-2" -> '
                   '"new-group" -> "new-group"). Groups must form a tree without circular references.')
        tests.assert_validation_deny(self, out, err_msg)

    def test_create_group_should_fail_with_cycle_detected_3(self):
        ctx = _prepare_create_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "test"
                },
                {
                    "kind": "Group",
                    "name": "new-group"
                }
            ],
            "name": "new-group"
        },
        {
            "groups": [
                {
                    "filterResult": {
                        "groupName": "admins",
                        "members": [
                            {
                                "kind": "User",
                                "name": "admin"
                            },
                            {
                                "kind": "User",
                                "name": "weak-admin"
                            }
                        ],
                        "name": "admins"
                    }
                }
            ]
        })
        out = hook.testrun(main, [ctx])
        err_msg = (f'Invalid group hierarchy: cycle detected! Path: groups.deckhouse.io("new-group" -> "new-group"). '
                   'Groups must form a tree without circular references.')
        tests.assert_validation_deny(self, out, err_msg)

    def test_update_group_should_fail_with_cycle_detected(self):
        ctx = _prepare_update_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "superadmin"
                },
                {
                    "kind": "Group",
                    "name": "group-2"
                },
                {
                    "kind": "User",
                    "name": "not-exists"
                }
            ],
            "name": "group-1"
        }, DEFAULT_UPDATE_SNAPSHOT)
        out = hook.testrun(main, [ctx])
        err_msg = (f'Invalid group hierarchy: cycle detected! Path: groups.deckhouse.io("group-1" -> "group-2" -> '
                   '"group-1"). Groups must form a tree without circular references.')
        tests.assert_validation_deny(self, out, err_msg)

    def test_update_group_should_fail_with_cycle_detected_2(self):
        ctx = _prepare_update_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "superadmin"
                },
                {
                    "kind": "Group",
                    "name": "group-1"
                },
                {
                    "kind": "User",
                    "name": "not-exists"
                }
            ],
            "name": "group-1"
        }, DEFAULT_UPDATE_SNAPSHOT)
        out = hook.testrun(main, [ctx])
        err_msg = (f'Invalid group hierarchy: cycle detected! Path: groups.deckhouse.io("group-2" -> "group-1" -> '
                   '"group-1"). Groups must form a tree without circular references.')
        tests.assert_validation_deny(self, out, err_msg)

    def test_update_group_should_fail_with_cycle_detected_3(self):
        ctx = _prepare_update_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "test"
                },
                {
                    "kind": "Group",
                    "name": "new-group"
                }
            ],
            "name": "new-group"
        },
        {
            "groups": [
                {
                    "filterResult": {
                        "groupName": "admins",
                        "members": [
                            {
                                "kind": "User",
                                "name": "admin"
                            },
                            {
                                "kind": "User",
                                "name": "weak-admin"
                            }
                        ],
                        "name": "admins"
                    }
                }
            ]
        })
        out = hook.testrun(main, [ctx])
        err_msg = (f'Invalid group hierarchy: cycle detected! Path: groups.deckhouse.io("new-group" -> "new-group"). '
                'Groups must form a tree without circular references.')
        tests.assert_validation_deny(self, out, err_msg)

    def test_should_create_group(self):
        ctx = _prepare_create_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "test"
                },
                {
                    "kind": "Group",
                    "name": "some-none-exists-group"
                }
            ],
            "name": "new-group"
        }, DEFAULT_CREATE_SNAPSHOT)
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)


if __name__ == '__main__':
    unittest.main()
