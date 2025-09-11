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

import unittest
import json
import typing

from group import main
from deckhouse import hook, tests
from dotmap import DotMap

def _prepare_validation_binding_context(binding_context_json, new_spec: dict) -> DotMap:
    ctx_dict = json.loads(binding_context_json)
    ctx = DotMap(ctx_dict)
    ctx.review.request.object.spec = new_spec
    return ctx

def _prepare_update_binding_context(new_spec: dict) -> DotMap:
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
                            "name": "none-exists-2"
                        }
                    ],
                    "name": "candi-admins"
                }
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
    "snapshots": {
        "groups": [
            {
                "filterResult": {
                    "groupName": "candi-admins",
                    "members": [
                        {
                            "kind": "User",
                            "name": "superadmin"
                        }
                    ],
                    "name": "candi-admins"
                }
            },
            {
                "filterResult": {
                    "groupName": "crowd-ro",
                    "members": [
                        {
                            "kind": "User",
                            "name": "test"
                        }
                    ],
                    "name": "crowd-ro"
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
    },
    "type": "Validating"
}
"""
    return _prepare_validation_binding_context(binding_context_json, new_spec)

def _prepare_create_binding_context(new_spec: dict) -> DotMap:
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
                "spec": {
                    "members": [
                        {
                            "kind": "User",
                            "name": "superadmin"
                        }
                    ],
                    "name": "new-group"
                }
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
    "snapshots": {
        "groups": [
            {
                "filterResult": {
                    "groupName": "candi-admins",
                    "members": [
                        {
                            "kind": "User",
                            "name": "superadmin"
                        },
                        {
                            "kind": "Group",
                            "name": "none-exists-2"
                        }
                    ],
                    "name": "candi-admins"
                }
            },
            {
                "filterResult": {
                    "groupName": "crowd-ro",
                    "members": [
                        {
                            "kind": "User",
                            "name": "test"
                        }
                    ],
                    "name": "crowd-ro"
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
    },
    "type": "Validating"
}
"""
    return _prepare_validation_binding_context(binding_context_json, new_spec)

def _prepare_delete_binding_context(delete_spec: dict, has_member : bool) -> DotMap:
    binding_context_json = """
{
    "binding": "groups-unique.deckhouse.io",
    "review": {
        "request": {
            "uid": "d47e6935-8e58-4270-b193-c4a8e2626ba1",
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
            "operation": "DELETE",
            "userInfo": {
                "username": "kubernetes-admin",
                "groups": [
                    "system:masters",
                    "system:authenticated"
                ]
            },
            "object": null,
            "oldObject": {
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
                    "resourceVersion": "1185233604",
                    "uid": "f43bdc3f-61a2-4957-ae5a-241972717118"
                },
                "spec": {
                    "members": [
                        {
                            "kind": "User",
                            "name": "superadmin"
                        }
                    ],
                    "name": "new-group"
                }
            },
            "dryRun": false,
            "options": {
                "kind": "DeleteOptions",
                "apiVersion": "meta.k8s.io/v1",
                "propagationPolicy": "Background"
            }
        }
    },
    "snapshots": {
        "groups": [
            {
                "filterResult": {
                    "groupName": "candi-admins",
                    "members": [
                        {
                            "kind": "User",
                            "name": "superadmin"
                        },
                        {
                            "kind": "Group",
                            "name": "none-exists-2"
                        }
                    ],
                    "name": "candi-admins"
                }
            },
            {
                "filterResult": {
                    "groupName": "crowd-ro",
                    "members": [
                        {
                            "kind": "User",
                            "name": "test"
                        }
                    ],
                    "name": "crowd-ro"
                }
            },
            {
                "filterResult": {
                    "groupName": "new-group",
                    "members": [
                        {
                            "kind": "User",
                            "name": "superadmin"
                        }
                    ],
                    "name": "new"
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
    },
    "type": "Validating"
}
"""
    ctx_dict = json.loads(binding_context_json)
    ctx = DotMap(ctx_dict)
    ctx.review.request.oldObject.spec = delete_spec
    if has_member:
        snp_dict = DotMap({
            "kind": "Group",
            "name": "new",
        })
        ctx.snapshots.groups[0].filterResult.members.append(snp_dict)
        ctx.snapshots.groups[1].filterResult.members.append(snp_dict)
    return ctx

class TestGroupValidationWebhook(unittest.TestCase):
    def test_should_update(self):
        ctx = _prepare_update_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "test"
                },
                {
                    "kind": "Group",
                    "name": "candi-admins-2"
                }
            ],
            "name": "candi-admins"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_update_with_warning_with_new_group_member_not_exists_group(self):
        ctx = _prepare_update_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "superadmin"
                },
                {
                    "kind": "Group",
                    "name": "none-exists-2"
                }
            ],
            "name": "candi-admins"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, 'groups.deckhouse.io "none-exists-2" not exist')

    def test_should_update_with_warning_new_group_member_not_exists_user(self):
        ctx = _prepare_update_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "superadmin"
                },
                {
                    "kind": "User",
                    "name": "not-exists"
                }
            ],
            "name": "candi-admins"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, 'users.deckhouse.io "not-exists" not exist')

    def test_should_update_with_warnings_new_group_member_not_exists_user_and_group(self):
        ctx = _prepare_update_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "superadmin"
                },
                {
                    "kind": "Group",
                    "name": "none-exists-2"
                },
                {
                    "kind": "User",
                    "name": "not-exists"
                }
            ],
            "name": "candi-admins"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, (
            'groups.deckhouse.io "none-exists-2" not exist',
            'users.deckhouse.io "not-exists" not exist'
        ))

    def test_should_create_group(self):
        ctx = _prepare_create_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "superadmin"
                },
            ],
            "name": "new-group"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_create_group_with_warnings_with_not_exists_user_and_group(self):
        ctx = _prepare_create_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "superadmin"
                },
                {
                    "kind": "Group",
                    "name": "none-exists-2"
                },
                {
                    "kind": "User",
                    "name": "not-exists"
                }
            ],
            "name": "new-group"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, (
            'groups.deckhouse.io "none-exists-2" not exist',
            'users.deckhouse.io "not-exists" not exist'
        ))

    def test_create_should_fail_with_already_exist_group(self):
        ctx = _prepare_create_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "superadmin"
                },
            ],
            "name": "candi-admins"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, 'groups.deckhouse.io "candi-admins" already exists')

    def test_create_should_fail_with_system_group(self):
        ctx = _prepare_create_binding_context({
                "members": [
                    {
                        "kind": "User",
                        "name": "superadmin"
                    },
                ],
                "name": "system:new-group"
            })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, 'groups.deckhouse.io "system:new-group" must not start with the "system:" prefix')

    def test_delete(self):
        ctx = _prepare_delete_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "superadmin"
                },
            ],
            "name": "new"
        }, False)
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)


    def test_delete_with_warnings_if_(self):
        ctx = _prepare_delete_binding_context({
            "members": [
                {
                    "kind": "User",
                    "name": "superadmin"
                },
            ],
            "name": "new"
        }, True)
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, (
            'groups.deckhouse.io "candi-admins" contains groups.deckhouse.io "new"',
            'groups.deckhouse.io "crowd-ro" contains groups.deckhouse.io "new"'
        ))

if __name__ == '__main__':
    unittest.main()
