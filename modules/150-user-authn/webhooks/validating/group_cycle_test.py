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
from typing import Optional

from deckhouse import hook, tests
from dotmap import DotMap

from group import main


def _prepare_validation_binding_context(binding_context_json, new_spec: dict, snapshots: dict) -> DotMap:
    ctx_dict = json.loads(binding_context_json)
    ctx = DotMap(ctx_dict)
    ctx.review.request.object.spec = new_spec
    ctx.snapshots = snapshots
    return ctx


def _set_object_name(ctx: DotMap, obj_name: Optional[str]) -> DotMap:
    if obj_name is not None:
        ctx.review.request.name = obj_name
        ctx.review.request.object.metadata.name = obj_name
        if ctx.review.request.oldObject:
            ctx.review.request.oldObject.metadata.name = obj_name
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

def _prepare_update_binding_context(new_spec: dict, snapshots: dict, old_spec: Optional[dict] = None,
                                    obj_name: Optional[str] = None) -> DotMap:
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
    ctx = _prepare_validation_binding_context(binding_context_json, new_spec, snapshots)
    if old_spec is not None:
        ctx.review.request.oldObject.spec = old_spec
    return _set_object_name(ctx, obj_name)

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

def _prepare_create_binding_context(new_spec: dict, snapshots: dict, obj_name: Optional[str] = None) -> DotMap:
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
    ctx = _prepare_validation_binding_context(binding_context_json, new_spec, snapshots)
    return _set_object_name(ctx, obj_name)


def _group_member(name: str) -> dict:
    return {"kind": "Group", "name": name}


def _user_member(name: str) -> dict:
    return {"kind": "User", "name": name}


def _snapshot_group(obj_name: str, group_name: str, members: list[dict]) -> dict:
    return {"filterResult": {"name": obj_name, "groupName": group_name, "members": members}}


# Groups real-a and real-b already contain each other. Their metadata.name differs from spec.name, and nested
# members refer to spec.name.
EXISTING_CYCLE_SNAPSHOT = {
    "groups": [
        _snapshot_group("obj-a", "real-a", [_group_member("real-b")]),
        _snapshot_group("obj-b", "real-b", [_group_member("real-a"), _user_member("test")]),
        _snapshot_group("obj-c", "real-c", [_group_member("real-b")]),
        _snapshot_group("obj-d", "real-d", [_user_member("test")]),
        _snapshot_group("obj-e", "real-e", [_group_member("real-d")]),
    ],
    "users": [{"filterResult": {"userName": "test"}}],
}


def _cycle_denial(path: str) -> str:
    return (f'Invalid group hierarchy: cycle detected! Path: groups.deckhouse.io({path}). Groups must form a tree '
            'without circular references.')


def _existing_cycle_warning(path: str) -> str:
    return (f'Group hierarchy already has a cycle. Path: groups.deckhouse.io({path}). Groups must form a tree '
            'without circular references. To break the cycle, remove one of these groups from the members of the '
            'previous one.')


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
        tests.assert_validation_deny(self, out, _cycle_denial('"new-group" -> "group-2" -> "new-group"'))

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
        tests.assert_validation_deny(self, out, _cycle_denial('"new-group" -> "new-group"'))

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
        tests.assert_validation_deny(self, out, _cycle_denial('"group-1" -> "group-1"'))

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

    def test_update_detects_cycle_when_metadata_name_differs_from_spec_name(self):
        # members[].name for kind Group is spec.name; metadata.name must not be the tree key.
        snapshots = {
            "groups": [
                {
                    "filterResult": {
                        "name": "obj-a",
                        "groupName": "real-a",
                        "members": [{"kind": "Group", "name": "real-b"}],
                    }
                },
                {
                    "filterResult": {
                        "name": "obj-b",
                        "groupName": "real-b",
                        "members": [{"kind": "User", "name": "test"}],
                    }
                },
            ],
            "users": [{"filterResult": {"userName": "test"}}],
        }
        ctx = _prepare_update_binding_context(
            {"name": "real-b", "members": [{"kind": "Group", "name": "real-a"}]},
            snapshots,
            old_spec={"name": "real-b", "members": [{"kind": "User", "name": "test"}]},
            obj_name="obj-b",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, _cycle_denial('"real-b" -> "real-a" -> "real-b"'))

    def test_update_of_unrelated_group_passes_with_warning_about_existing_cycle(self):
        spec = {"name": "real-d", "members": [_user_member("test")]}
        ctx = _prepare_update_binding_context(spec, EXISTING_CYCLE_SNAPSHOT, old_spec=spec, obj_name="obj-d")
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, _existing_cycle_warning('"real-a" -> "real-b" -> "real-a"'))

    def test_create_of_group_passes_with_warning_about_existing_cycle(self):
        ctx = _prepare_create_binding_context(
            {"name": "real-f", "members": [_group_member("real-c")]},
            EXISTING_CYCLE_SNAPSHOT,
            obj_name="obj-f",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, _existing_cycle_warning('"real-b" -> "real-a" -> "real-b"'))

    def test_update_keeping_existing_cycle_passes_with_warning(self):
        # Removing an expired user from a group of the cycle leaves the cycle as it is.
        ctx = _prepare_update_binding_context(
            {"name": "real-b", "members": [_group_member("real-a")]},
            EXISTING_CYCLE_SNAPSHOT,
            old_spec={"name": "real-b", "members": [_group_member("real-a"), _user_member("test")]},
            obj_name="obj-b",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, _existing_cycle_warning('"real-b" -> "real-a" -> "real-b"'))

    def test_update_closing_new_cycle_is_denied_while_another_cycle_exists(self):
        ctx = _prepare_update_binding_context(
            {"name": "real-d", "members": [_user_member("test"), _group_member("real-e")]},
            EXISTING_CYCLE_SNAPSHOT,
            old_spec={"name": "real-d", "members": [_user_member("test")]},
            obj_name="obj-d",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, _cycle_denial('"real-d" -> "real-e" -> "real-d"'))

    def test_update_adding_cycle_through_existing_cycle_is_denied(self):
        ctx = _prepare_update_binding_context(
            {"name": "real-a", "members": [_group_member("real-b"), _group_member("real-c")]},
            EXISTING_CYCLE_SNAPSHOT,
            old_spec={"name": "real-a", "members": [_group_member("real-b")]},
            obj_name="obj-a",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, _cycle_denial('"real-a" -> "real-c" -> "real-b" -> "real-a"'))

    def test_update_renaming_group_to_name_of_nested_member_closing_cycle_is_denied(self):
        snapshots = {
            "groups": [
                _snapshot_group("obj-p", "parent", [_group_member("child-new")]),
                _snapshot_group("obj-q", "child", [_group_member("parent")]),
            ],
            "users": [],
        }
        ctx = _prepare_update_binding_context(
            {"name": "child-new", "members": [_group_member("parent")]},
            snapshots,
            old_spec={"name": "child", "members": [_group_member("parent")]},
            obj_name="obj-q",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, _cycle_denial('"child-new" -> "parent" -> "child-new"'))

    def test_update_breaking_existing_cycle_passes_without_warning(self):
        ctx = _prepare_update_binding_context(
            {"name": "real-b", "members": [_user_member("test")]},
            EXISTING_CYCLE_SNAPSHOT,
            old_spec={"name": "real-b", "members": [_group_member("real-a"), _user_member("test")]},
            obj_name="obj-b",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)
        self.assertNotIn("warnings", out.validations.data[0])

    def test_create_with_name_of_existing_group_is_denied_as_duplicate_not_as_cycle(self):
        # Keyed by spec.name, the new "a" would merge with the stored "a" that "b" contains.
        snapshots = {
            "groups": [
                _snapshot_group("obj-a", "a", []),
                _snapshot_group("obj-b", "b", [_group_member("a")]),
            ],
            "users": [],
        }
        ctx = _prepare_create_binding_context({"name": "a", "members": [_group_member("b")]}, snapshots,
                                              obj_name="obj-a-2")
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, 'groups.deckhouse.io "a" already exists')

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
