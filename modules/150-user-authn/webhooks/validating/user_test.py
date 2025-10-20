#!/usr/bin/python3

# Copyright 2025 Flant JSC
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

from user import main
from deckhouse import hook, tests
from dotmap import DotMap


def _prepare_validation_binding_context(binding_context_json, new_spec: dict) -> DotMap:
    ctx_dict = json.loads(binding_context_json)
    ctx = DotMap(ctx_dict)
    ctx.review.request.object.spec = new_spec
    return ctx


def _prepare_update_binding_context(new_spec: dict, old_spec: dict = None) -> DotMap:
    binding_context_json = """
{
    "binding": "users-unique.deckhouse.io",
    "review": {
        "request": {
            "uid": "8af60184-b30b-4b90-a33e-0c190f10e96d",
            "kind": {
                "group": "deckhouse.io",
                "version": "v1",
                "kind": "User"
            },
            "resource": {
                "group": "deckhouse.io",
                "version": "v1",
                "resource": "users"
            },
            "requestKind": {
                "group": "deckhouse.io",
                "version": "v1",
                "kind": "User"
            },
            "requestResource": {
                "group": "deckhouse.io",
                "version": "v1",
                "resource": "users"
            },
            "name": "testuser",
            "operation": "UPDATE",
            "userInfo": {
                "username": "kubernetes-admin",
                "groups": [
                    "system:masters",
                    "system:authenticated"
                ]
            },
            "object": {
                "apiVersion": "deckhouse.io/v1",
                "kind": "User",
                "metadata": {
                    "creationTimestamp": "2023-07-17T13:40:39Z",
                    "generation": 3,
                    "managedFields": [
                        {
                            "apiVersion": "deckhouse.io/v1",
                            "fieldsType": "FieldsV1",
                            "fieldsV1": {
                                "f:spec": {
                                    ".": {},
                                    "f:email": {}
                                }
                            },
                            "manager": "deckhouse-controller",
                            "operation": "Update",
                            "time": "2023-07-17T13:40:39Z"
                        }
                    ],
                    "name": "testuser",
                    "resourceVersion": "1184522270",
                    "uid": "7820c68b-6423-49f0-b97f-b7e314e23c0b"
                },
                "spec": {}
            },
            "oldObject": {
                "apiVersion": "deckhouse.io/v1",
                "kind": "User",
                "metadata": {
                    "creationTimestamp": "2023-07-17T13:40:39Z",
                    "generation": 2,
                    "managedFields": [
                        {
                            "apiVersion": "deckhouse.io/v1",
                            "fieldsType": "FieldsV1",
                            "fieldsV1": {
                                "f:spec": {
                                    ".": {},
                                    "f:email": {}
                                }
                            },
                            "manager": "deckhouse-controller",
                            "operation": "Update",
                            "time": "2023-07-17T13:40:39Z"
                        }
                    ],
                    "name": "testuser",
                    "resourceVersion": "1184522270",
                    "uid": "7820c68b-6423-49f0-b97f-b7e314e23c0b"
                },
                "spec": {}
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
        "users": [
            {
                "filterResult": {
                    "name": "existinguser",
                    "userID": "12345",
                    "email": "existing@example.com",
                    "groups": []
                }
            },
            {
                "filterResult": {
                    "name": "uppercaseuser",
                    "userID": "67890",
                    "email": "UPPERCASE@EXAMPLE.COM",
                    "groups": []
                }
            }
        ],
        "groups": []
    },
    "type": "Validating"
}
"""
    ctx = _prepare_validation_binding_context(binding_context_json, new_spec)
    if old_spec:
        ctx.review.request.oldObject.spec = old_spec
    return ctx


def _prepare_create_binding_context(new_spec: dict) -> DotMap:
    binding_context_json = """
{
    "binding": "users-unique.deckhouse.io",
    "review": {
        "request": {
            "uid": "adedd292-0be9-476b-b2fa-8286053a1b1b",
            "kind": {
                "group": "deckhouse.io",
                "version": "v1",
                "kind": "User"
            },
            "resource": {
                "group": "deckhouse.io",
                "version": "v1",
                "resource": "users"
            },
            "requestKind": {
                "group": "deckhouse.io",
                "version": "v1",
                "kind": "User"
            },
            "requestResource": {
                "group": "deckhouse.io",
                "version": "v1",
                "resource": "users"
            },
            "name": "newuser",
            "operation": "CREATE",
            "userInfo": {
                "username": "kubernetes-admin",
                "groups": [
                    "system:masters",
                    "system:authenticated"
                ]
            },
            "object": {
                "apiVersion": "deckhouse.io/v1",
                "kind": "User",
                "metadata": {
                    "creationTimestamp": "2024-11-22T08:00:33Z",
                    "generation": 1,
                    "managedFields": [
                        {
                            "apiVersion": "deckhouse.io/v1",
                            "fieldsType": "FieldsV1",
                            "fieldsV1": {
                                "f:spec": {
                                    ".": {},
                                    "f:email": {}
                                }
                            },
                            "manager": "kubectl-create",
                            "operation": "Update",
                            "time": "2024-11-22T08:00:33Z"
                        }
                    ],
                    "name": "newuser",
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
    "snapshots": {
        "users": [
            {
                "filterResult": {
                    "name": "existinguser",
                    "userID": "12345",
                    "email": "existing@example.com",
                    "groups": []
                }
            },
            {
                "filterResult": {
                    "name": "uppercaseuser",
                    "userID": "67890",
                    "email": "UPPERCASE@EXAMPLE.COM",
                    "groups": []
                }
            }
        ],
        "groups": []
    },
    "type": "Validating"
}
"""
    return _prepare_validation_binding_context(binding_context_json, new_spec)


def _prepare_delete_binding_context(delete_spec: dict) -> DotMap:
    binding_context_json = """
{
    "binding": "users-unique.deckhouse.io",
    "review": {
        "request": {
            "uid": "d47e6935-8e58-4270-b193-c4a8e2626ba1",
            "kind": {
                "group": "deckhouse.io",
                "version": "v1",
                "kind": "User"
            },
            "resource": {
                "group": "deckhouse.io",
                "version": "v1",
                "resource": "users"
            },
            "requestKind": {
                "group": "deckhouse.io",
                "version": "v1",
                "kind": "User"
            },
            "requestResource": {
                "group": "deckhouse.io",
                "version": "v1",
                "resource": "users"
            },
            "name": "testuser",
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
                "apiVersion": "deckhouse.io/v1",
                "kind": "User",
                "metadata": {
                    "creationTimestamp": "2024-11-22T08:00:33Z",
                    "generation": 1,
                    "managedFields": [
                        {
                            "apiVersion": "deckhouse.io/v1",
                            "fieldsType": "FieldsV1",
                            "fieldsV1": {
                                "f:spec": {
                                    ".": {},
                                    "f:email": {}
                                }
                            },
                            "manager": "kubectl-create",
                            "operation": "Update",
                            "time": "2024-11-22T08:00:33Z"
                        }
                    ],
                    "name": "testuser",
                    "resourceVersion": "1185233604",
                    "uid": "f43bdc3f-61a2-4957-ae5a-241972717118"
                },
                "spec": {}
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
        "users": [
            {
                "filterResult": {
                    "name": "existinguser",
                    "userID": "12345",
                    "email": "existing@example.com",
                    "groups": []
                }
            }
        ],
        "groups": [
            {
                "filterResult": {
                    "name": "testgroup",
                    "members": [
                        {
                            "kind": "User",
                            "name": "testuser"
                        }
                    ]
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
    return ctx


class TestUserValidationWebhook(unittest.TestCase):
    # Email validation tests - CREATE operations
    def test_create_lowercase_email_should_allow(self):
        ctx = _prepare_create_binding_context({
            "email": "test@example.com"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_create_uppercase_email_should_deny(self):
        ctx = _prepare_create_binding_context({
            "email": "TEST@EXAMPLE.COM"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, 'users.deckhouse.io "newuser", ".spec.email" must be lowercase. Use "test@example.com" instead')

    def test_create_mixed_case_email_should_deny(self):
        ctx = _prepare_create_binding_context({
            "email": "Test@Example.COM"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, 'users.deckhouse.io "newuser", ".spec.email" must be lowercase. Use "test@example.com" instead')

    def test_create_duplicate_email_case_insensitive_should_deny(self):
        ctx = _prepare_create_binding_context({
            "email": "EXISTING@EXAMPLE.COM"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, 'users.deckhouse.io "newuser", user "existinguser" is already using email "existing@example.com" (case-insensitive match)')

    # Email validation tests - UPDATE operations
    def test_update_email_not_changed_uppercase_should_allow_with_warning(self):
        ctx = _prepare_update_binding_context(
            {"email": "UPPERCASE@EXAMPLE.COM"},
            {"email": "UPPERCASE@EXAMPLE.COM"}
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, '".spec.email" contains uppercase; Dex lowercases emails. Consider migrating to lowercase.')

    def test_update_email_changed_lowercase_to_uppercase_should_deny(self):
        ctx = _prepare_update_binding_context(
            {"email": "NEWUPPERCASE@EXAMPLE.COM"},
            {"email": "lowercase@example.com"}
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, 'users.deckhouse.io "testuser", changing ".spec.email" to contain uppercase is forbidden; use lowercase')

    def test_update_email_changed_uppercase_to_lowercase_should_allow(self):
        ctx = _prepare_update_binding_context(
            {"email": "newlowercase@example.com"},
            {"email": "UPPERCASE@EXAMPLE.COM"}
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_update_email_changed_to_duplicate_case_insensitive_should_deny(self):
        ctx = _prepare_update_binding_context(
            {"email": "EXISTING@EXAMPLE.COM"},
            {"email": "different@example.com"}
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, 'users.deckhouse.io "testuser", user "existinguser" is already using email "existing@example.com" (case-insensitive match)')

    def test_update_email_changed_lowercase_to_lowercase_should_allow(self):
        ctx = _prepare_update_binding_context(
            {"email": "newlowercase@example.com"},
            {"email": "oldlowercase@example.com"}
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    # Regression tests - existing functionality
    def test_create_with_groups_should_deny(self):
        ctx = _prepare_create_binding_context({
            "email": "test@example.com",
            "groups": ["admin"]
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, '".spec.groups" is deprecated, use the "Group" object.')

    def test_update_with_groups_modification_should_deny(self):
        ctx = _prepare_update_binding_context(
            {"email": "test@example.com", "groups": ["admin"]},
            {"email": "test@example.com", "groups": ["admin", "user"]}
        )
        # Ensure snapshot contains current user with initial groups to trigger denial on removal
        ctx.snapshots.users.append(DotMap({
            "filterResult": {
                "name": "testuser",
                "userID": "abc",
                "email": "test@example.com",
                "groups": ["admin", "user"]
            }
        }))
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, '".spec.groups" is deprecated, modification is forbidden, only removal of all elements is allowed')

    def test_create_with_system_prefix_should_deny(self):
        ctx = _prepare_create_binding_context({
            "email": "system:test@example.com"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, 'users.deckhouse.io "newuser", ".spec.email" must not start with the "system:" prefix')

    def test_create_with_userid_should_allow_with_warning(self):
        ctx = _prepare_create_binding_context({
            "email": "test@example.com",
            "userID": "12345"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, '".spec.userID" is deprecated and shouldn\'t be set manually (if set, its value is ignored)')

    def test_delete_should_allow(self):
        ctx = _prepare_delete_binding_context({
            "email": "test@example.com"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_delete_with_warnings_if_user_in_group(self):
        ctx = _prepare_delete_binding_context({
            "email": "test@example.com"
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, 'groups.deckhouse.io "testgroup" contains users.deckhouse.io "testuser"')


if __name__ == '__main__':
    unittest.main()
