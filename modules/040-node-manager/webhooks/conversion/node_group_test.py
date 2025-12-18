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

import typing
import unittest

from dotmap import DotMap
from node_group import NodeGroupConversion, main

from deckhouse import hook, tests


def test_dispatcher_for_unit_tests(snapshots: dict | None) -> NodeGroupConversion:
    output = hook.Output(
        hook.MetricsCollector(),
        hook.KubeOperationCollector(),
        hook.ValuesPatchesCollector({}),
        hook.ConversionsCollector(),
        hook.ValidationsCollector(),
    )

    bctx = {}
    if snapshots is not None:
        bctx = {"snapshots": snapshots}

    return NodeGroupConversion(hook.Context(bctx, {}, {}, output))



class TestUnitAlpha1ToAlpha2Method(unittest.TestCase):
    def test_should_remove_static_and_spec_kubernetes_version(self):
        obj = {
            "apiVersion": "deckhouse.io/v1alpha1",
            "kind": "NodeGroup",
            "metadata": {
                "name": "worker-static",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "kubernetesVersion": "1.11"
            },
            "static": {
                "internalNetworkCIDR": "127.0.0.1/8"
            }
        }

        err, res_obj = test_dispatcher_for_unit_tests(None).alpha1_to_alpha2(obj)

        self.assertIsNone(err)
        self.assertEqual(res_obj, {
            "apiVersion": "deckhouse.io/v1alpha2",
            "kind": "NodeGroup",
            "metadata": {
                "name": "worker-static",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
            },
        })


    def test_should_move_docker_to_cri_docker_cri_not_set(self):
        obj = {
            "apiVersion": "deckhouse.io/v1alpha1",
            "kind": "NodeGroup",
            "metadata": {
                "name": "worker-static",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "docker": {
                    "manage": False,
                    "maxConcurrentDownloads": 4
                }
            },

        }

        err, res_obj = test_dispatcher_for_unit_tests(None).alpha1_to_alpha2(obj)

        self.assertIsNone(err)
        self.assertEqual(res_obj, {
            "apiVersion": "deckhouse.io/v1alpha2",
            "kind": "NodeGroup",
            "metadata": {
                "name": "worker-static",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                }
            },
        })


class TestUnitAlpha2ToAlpha1Method(unittest.TestCase):
    def test_should_move_spec_cri_docker_to_spec_docker(self):
        obj = {
            "apiVersion": "deckhouse.io/v1alpha2",
            "kind": "NodeGroup",
            "metadata": {
                "name": "worker-static",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                }
            },
        }

        err, res_obj = test_dispatcher_for_unit_tests(None).alpha2_to_alpha1(obj)

        self.assertIsNone(err)
        self.assertEqual(res_obj, {
            "apiVersion": "deckhouse.io/v1alpha1",
            "kind": "NodeGroup",
            "metadata": {
                "name": "worker-static",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {},
                "docker": {
                    "manage": False,
                    "maxConcurrentDownloads": 4
                }
            },

        })


class TestUnitAlpha2ToV1Method(unittest.TestCase):
    __snapshots = {
        "cluster_config": [
            {
                "filterResult": "YXBpVmVyc2lvbjogZGVja2hvdXNlLmlvL3YxYWxwaGExCmtpbmQ6IE9wZW5TdGFja0NsdXN0ZXJDb25maWd1cmF0aW9uCmxheW91dDogU3RhbmRhcmQKbWFzdGVyTm9kZUdyb3VwOgogIGluc3RhbmNlQ2xhc3M6CiAgICBhZGRpdGlvbmFsU2VjdXJpdHlHcm91cHM6CiAgICAtIHNhbmRib3gKICAgIC0gc2FuZGJveC1mcm9udGVuZAogICAgLSB0c3Qtc2VjLWdyb3VwCiAgICBldGNkRGlza1NpemVHYjogMTAKICAgIGZsYXZvck5hbWU6IG0xLmxhcmdlLTUwZwogICAgaW1hZ2VOYW1lOiB1YnVudHUtMTgtMDQtY2xvdWQtYW1kNjQKICByZXBsaWNhczogMQogIHZvbHVtZVR5cGVNYXA6CiAgICBub3ZhOiBjZXBoLXNzZApub2RlR3JvdXBzOgotIGluc3RhbmNlQ2xhc3M6CiAgICBhZGRpdGlvbmFsU2VjdXJpdHlHcm91cHM6CiAgICAtIHNhbmRib3gKICAgIC0gc2FuZGJveC1mcm9udGVuZAogICAgY29uZmlnRHJpdmU6IGZhbHNlCiAgICBmbGF2b3JOYW1lOiBtMS54c21hbGwKICAgIGltYWdlTmFtZTogdWJ1bnR1LTE4LTA0LWNsb3VkLWFtZDY0CiAgICBtYWluTmV0d29yazogc2FuZGJveAogICAgcm9vdERpc2tTaXplOiAxNQogIG5hbWU6IGZyb250LW5tCiAgbm9kZVRlbXBsYXRlOgogICAgbGFiZWxzOgogICAgICBhYWE6IGFhYWEKICAgICAgY2NjOiBjY2NjCiAgcmVwbGljYXM6IDIKICB2b2x1bWVUeXBlTWFwOgogICAgbm92YTogY2VwaC1zc2QKcHJvdmlkZXI6CiAgYXV0aFVSTDogaHR0cHM6Ly9jbG91ZC5leGFtcGxlLmNvbS92My8KICBkb21haW5OYW1lOiBEZWZhdWx0CiAgcGFzc3dvcmQ6IHBhc3N3b3JkCiAgcmVnaW9uOiByZWcKICB0ZW5hbnROYW1lOiB1c2VyCiAgdXNlcm5hbWU6IHVzZXIKc3NoUHVibGljS2V5OiBzc2gtcnNhIEFBQQpzdGFuZGFyZDoKICBleHRlcm5hbE5ldHdvcmtOYW1lOiBwdWJsaWMKICBpbnRlcm5hbE5ldHdvcmtDSURSOiAxOTIuMTY4LjE5OC4wLzI0CiAgaW50ZXJuYWxOZXR3b3JrRE5TU2VydmVyczoKICAtIDguOC44LjgKICAtIDEuMS4xLjEKICAtIDguOC40LjQKICBpbnRlcm5hbE5ldHdvcmtTZWN1cml0eTogdHJ1ZQp0YWdzOgogIGE6IGIK"
            }
        ]
    }

    def test_change_node_type_from_cloud_to_cloud_ephemeral(self):
        obj = {
            "apiVersion": "deckhouse.io/v1alpha1",
            "kind": "NodeGroup",
            "metadata": {
                "name": "worker-static",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "Cloud"
            },
        }

        err, res_obj = test_dispatcher_for_unit_tests(TestUnitAlpha2ToV1Method.__snapshots).alpha2_to_v1(obj)

        self.assertIsNone(err)
        self.assertEqual(res_obj, {
            "apiVersion": "deckhouse.io/v1",
            "kind": "NodeGroup",
            "metadata": {
                "name": "worker-static",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "CloudEphemeral"
            },

        })


    def test_change_node_type_from_hybrid_to_cloud_permanent_for_master_ng(self):
        obj = {
            "apiVersion": "deckhouse.io/v1alpha2",
            "kind": "NodeGroup",
            "metadata": {
                "name": "master",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "Hybrid"
            },
        }

        err, res_obj = test_dispatcher_for_unit_tests(TestUnitAlpha2ToV1Method.__snapshots).alpha2_to_v1(obj)

        self.assertIsNone(err)
        self.assertEqual(res_obj, {
            "apiVersion": "deckhouse.io/v1",
            "kind": "NodeGroup",
            "metadata": {
                "name": "master",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "CloudPermanent"
            },

        })


    def test_change_node_type_from_hybrid_to_cloud_permanent_for_ng_in_provider_cluster_config(self):
        obj = {
            "apiVersion": "deckhouse.io/v1alpha2",
            "kind": "NodeGroup",
            "metadata": {
                "name": "front-nm",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "Hybrid"
            },
        }

        err, res_obj = test_dispatcher_for_unit_tests(TestUnitAlpha2ToV1Method.__snapshots).alpha2_to_v1(obj)

        self.assertIsNone(err)
        self.assertEqual(res_obj, {
            "apiVersion": "deckhouse.io/v1",
            "kind": "NodeGroup",
            "metadata": {
                "name": "front-nm",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "CloudPermanent"
            },

        })


    def test_change_node_type_from_hybrid_to_cloud_static_for_ng_not_in_provider_cluster_config(self):
        obj = {
            "apiVersion": "deckhouse.io/v1alpha2",
            "kind": "NodeGroup",
            "metadata": {
                "name": "another",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "Hybrid"
            },
        }

        err, res_obj = test_dispatcher_for_unit_tests(TestUnitAlpha2ToV1Method.__snapshots).alpha2_to_v1(obj)

        self.assertIsNone(err)
        self.assertEqual(res_obj, {
            "apiVersion": "deckhouse.io/v1",
            "kind": "NodeGroup",
            "metadata": {
                "name": "another",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "CloudStatic"
            },

        })


class TestUnitV1ToAlpha2Method(unittest.TestCase):
    def test_change_node_type_from_cloud_ephemeral_to_cloud(self):
        obj = {
            "apiVersion": "deckhouse.io/v1",
            "kind": "NodeGroup",
            "metadata": {
                "name": "worker-static",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "CloudEphemeral"
            },

        }

        err, res_obj = test_dispatcher_for_unit_tests(None).v1_to_alpha2(obj)

        self.assertIsNone(err)
        self.assertEqual(res_obj, {
            "apiVersion": "deckhouse.io/v1alpha2",
            "kind": "NodeGroup",
            "metadata": {
                "name": "worker-static",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "Cloud"
            },
        })


    def test_change_node_type_from_cloud_permanent_to_hybrid(self):
        obj = {
            "apiVersion": "deckhouse.io/v1",
            "kind": "NodeGroup",
            "metadata": {
                "name": "master",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "CloudPermanent"
            },

        }

        err, res_obj = test_dispatcher_for_unit_tests(None).v1_to_alpha2(obj)

        self.assertIsNone(err)
        self.assertEqual(res_obj, {
            "apiVersion": "deckhouse.io/v1alpha2",
            "kind": "NodeGroup",
            "metadata": {
                "name": "master",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "Hybrid"
            },
        })


    def test_change_node_type_from_cloud_static_to_hybrid(self):
        obj = {
            "apiVersion": "deckhouse.io/v1",
            "kind": "NodeGroup",
            "metadata": {
                "name": "another",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "CloudStatic"
            },

        }

        err, res_obj = test_dispatcher_for_unit_tests(None).v1_to_alpha2(obj)

        self.assertIsNone(err)
        self.assertEqual(res_obj, {
            "apiVersion": "deckhouse.io/v1alpha2",
            "kind": "NodeGroup",
            "metadata": {
                "name": "another",
            },
            "spec": {
                "disruptions": {
                    "approvalMode": "Automatic"
                },
                "cri": {
                    "docker": {
                        "manage": False,
                        "maxConcurrentDownloads": 4
                    }
                },
                "nodeType": "Hybrid"
            },
        })



class TestGroupValidationWebhook(unittest.TestCase):
    def test_should_convert_from_v1_to_alpha2(self):
        ctx = {
            "binding": "v1_to_alpha2",
            "fromVersion": "deckhouse.io/v1",
            "review": {
                "request": {
                    "uid": "76bbb5fd-9289-4175-bd86-182d28a64689",
                    "desiredAPIVersion": "deckhouse.io/v1alpha1",
                    "objects": [
                        {
                            "apiVersion": "deckhouse.io/v1",
                            "kind": "NodeGroup",
                            "metadata": {
                                "creationTimestamp": "2023-10-16T10:03:38Z",
                                "generation": 3,
                                "managedFields": [
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                ".": {},
                                                "f:disruptions": {
                                                    ".": {},
                                                    "f:approvalMode": {}
                                                },
                                                "f:nodeType": {}
                                            }
                                        },
                                        "manager": "kubectl-create",
                                        "operation": "Update",
                                        "time": "2023-10-16T10:03:38Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                "f:nodeTemplate": {
                                                    ".": {},
                                                    "f:labels": {
                                                        ".": {},
                                                        "f:aaaa": {}
                                                    }
                                                }
                                            }
                                        },
                                        "manager": "kubectl-edit",
                                        "operation": "Update",
                                        "time": "2023-10-16T10:06:01Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                "f:kubelet": {
                                                    "f:resourceReservation": {
                                                        "f:mode": {}
                                                    }
                                                }
                                            }
                                        },
                                        "manager": "deckhouse-controller",
                                        "operation": "Update",
                                        "time": "2024-01-30T07:08:03Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:status": {
                                                ".": {},
                                                "f:conditionSummary": {
                                                    ".": {},
                                                    "f:ready": {},
                                                    "f:statusMessage": {}
                                                },
                                                "f:conditions": {},
                                                "f:deckhouse": {
                                                    ".": {},
                                                    "f:observed": {
                                                        ".": {},
                                                        "f:checkSum": {},
                                                        "f:lastTimestamp": {}
                                                    },
                                                    "f:processed": {
                                                        ".": {},
                                                        "f:checkSum": {},
                                                        "f:lastTimestamp": {}
                                                    },
                                                    "f:synced": {}
                                                },
                                                "f:error": {},
                                                "f:kubernetesVersion": {},
                                                "f:nodes": {},
                                                "f:ready": {},
                                                "f:upToDate": {}
                                            }
                                        },
                                        "manager": "deckhouse-controller",
                                        "operation": "Update",
                                        "subresource": "status",
                                        "time": "2024-11-23T18:10:49Z"
                                    }
                                ],
                                "name": "worker-static",
                                "uid": "5a5e3820-5fdd-4fea-a8cf-032586ae0be5"
                            },
                            "spec": {
                                "disruptions": {
                                    "approvalMode": "Automatic"
                                },
                                "kubelet": {
                                    "containerLogMaxFiles": 4,
                                    "containerLogMaxSize": "50Mi",
                                    "resourceReservation": {
                                        "mode": "Off"
                                    }
                                },
                                "nodeTemplate": {
                                    "labels": {
                                        "aaaa": "bbbb"
                                    }
                                },
                                "nodeType": "CloudStatic"
                            },
                            "status": {
                                "conditionSummary": {
                                    "ready": "True",
                                    "statusMessage": ""
                                },
                                "conditions": [
                                    {
                                        "lastTransitionTime": "2023-10-16T10:03:39Z",
                                        "status": "True",
                                        "type": "Ready"
                                    },
                                    {
                                        "lastTransitionTime": "2023-10-26T12:24:02Z",
                                        "status": "False",
                                        "type": "Updating"
                                    },
                                    {
                                        "lastTransitionTime": "2023-10-16T10:03:39Z",
                                        "status": "False",
                                        "type": "WaitingForDisruptiveApproval"
                                    },
                                    {
                                        "lastTransitionTime": "2023-10-16T10:03:39Z",
                                        "status": "False",
                                        "type": "Error"
                                    }
                                ],
                                "deckhouse": {
                                    "observed": {
                                        "checkSum": "8cfb8c1cda6ce98f0b50e52302d5a871",
                                        "lastTimestamp": "2024-11-23T18:10:08Z"
                                    },
                                    "processed": {
                                        "checkSum": "8cfb8c1cda6ce98f0b50e52302d5a871",
                                        "lastTimestamp": "2024-11-23T18:10:49Z"
                                    },
                                    "synced": "True"
                                },
                                "error": "",
                                "kubernetesVersion": "1.30",
                                "nodes": 0,
                                "ready": 0,
                                "upToDate": 0
                            }
                        }
                    ]
                }
            },
            "toVersion": "deckhouse.io/v1alpha2",
            "type": "Conversion"
        }

        out = hook.testrun(main, [ctx])

        def assert_api_version_and_node_type_changed_and_another_not_changed(t: unittest.TestCase, objects: typing.List[dict]):
            t.assertEqual(len(objects), 1)
            o = objects[0]

            tests.assert_common_resource_fields(t, o, "deckhouse.io/v1alpha2", "worker-static")

            obj = DotMap(o)

            # assert nodeTypeChanged
            t.assertEqual(obj.spec.nodeType, "Hybrid")

            # assert some fields cannot changed
            t.assertIn("aaaa", obj.spec.nodeTemplate.labels)
            t.assertEqual(obj.spec.nodeTemplate.labels.aaaa, "bbbb")
            t.assertEqual(obj.status.kubernetesVersion, "1.30")
            t.assertEqual(obj.status.ready, 0)


        tests.assert_conversion(self, out, assert_api_version_and_node_type_changed_and_another_not_changed, None)


    def test_should_convert_from_alpha2_to_alpha1(self):
        ctx = {
            "binding": "alpha2_to_alpha1",
            "fromVersion": "deckhouse.io/v1alpha2",
            "review": {
                "request": {
                    "uid": "93577800-3941-410b-a946-4c370ffc8ee8",
                    "desiredAPIVersion": "deckhouse.io/v1alpha1",
                    "objects": [
                        {
                            "apiVersion": "deckhouse.io/v1alpha2",
                            "kind": "NodeGroup",
                            "metadata": {
                                "creationTimestamp": "2023-06-06T08:23:11Z",
                                "generation": 13,
                                "managedFields": [
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                ".": {},
                                                "f:cloudInstances": {
                                                    ".": {},
                                                    "f:classReference": {
                                                        ".": {},
                                                        "f:kind": {},
                                                        "f:name": {}
                                                    },
                                                    "f:maxSurgePerZone": {},
                                                    "f:maxUnavailablePerZone": {},
                                                    "f:priority": {}
                                                },
                                                "f:cri": {
                                                    ".": {},
                                                    "f:type": {}
                                                },
                                                "f:disruptions": {
                                                    ".": {},
                                                    "f:approvalMode": {}
                                                },
                                                "f:nodeTemplate": {
                                                    ".": {},
                                                    "f:labels": {
                                                        ".": {},
                                                        "f:aaaaa": {}
                                                    }
                                                },
                                                "f:nodeType": {}
                                            }
                                        },
                                        "manager": "kubectl-create",
                                        "operation": "Update",
                                        "time": "2023-06-06T08:23:11Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                "f:cloudInstances": {
                                                    "f:maxPerZone": {},
                                                    "f:minPerZone": {}
                                                }
                                            }
                                        },
                                        "manager": "kubectl-edit",
                                        "operation": "Update",
                                        "time": "2023-06-06T15:12:35Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                "f:kubelet": {
                                                    ".": {},
                                                    "f:containerLogMaxFiles": {},
                                                    "f:containerLogMaxSize": {},
                                                    "f:resourceReservation": {
                                                        ".": {},
                                                        "f:mode": {}
                                                    }
                                                }
                                            }
                                        },
                                        "manager": "deckhouse-controller",
                                        "operation": "Update",
                                        "time": "2023-07-26T07:47:00Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:status": {
                                                ".": {},
                                                "f:conditionSummary": {
                                                    ".": {},
                                                    "f:ready": {},
                                                    "f:statusMessage": {}
                                                },
                                                "f:conditions": {},
                                                "f:deckhouse": {
                                                    ".": {},
                                                    "f:observed": {
                                                        ".": {},
                                                        "f:checkSum": {},
                                                        "f:lastTimestamp": {}
                                                    },
                                                    "f:processed": {
                                                        ".": {},
                                                        "f:checkSum": {},
                                                        "f:lastTimestamp": {}
                                                    },
                                                    "f:synced": {}
                                                },
                                                "f:desired": {},
                                                "f:error": {},
                                                "f:instances": {},
                                                "f:kubernetesVersion": {},
                                                "f:lastMachineFailures": {},
                                                "f:max": {},
                                                "f:min": {},
                                                "f:nodes": {},
                                                "f:ready": {},
                                                "f:upToDate": {}
                                            }
                                        },
                                        "manager": "deckhouse-controller",
                                        "operation": "Update",
                                        "subresource": "status",
                                        "time": "2024-11-23T18:40:42Z"
                                    }
                                ],
                                "name": "p-90",
                                "uid": "83bc46fa-bc40-4829-9414-82099680797b"
                            },
                            "spec": {
                                "cloudInstances": {
                                    "classReference": {
                                        "kind": "OpenStackInstanceClass",
                                        "name": "p-90"
                                    },
                                    "maxPerZone": 0,
                                    "maxSurgePerZone": 0,
                                    "maxUnavailablePerZone": 0,
                                    "minPerZone": 0,
                                    "priority": 90
                                },
                                "cri": {
                                    "type": "Docker",
                                    "docker": {
                                        "manage": False,
                                        "maxConcurrentDownloads": 4
                                    }
                                },
                                "disruptions": {
                                    "approvalMode": "Automatic"
                                },
                                "kubelet": {
                                    "containerLogMaxFiles": 4,
                                    "containerLogMaxSize": "50Mi",
                                    "resourceReservation": {
                                        "mode": "Off"
                                    }
                                },
                                "nodeTemplate": {
                                    "labels": {
                                        "aaaaa": ""
                                    }
                                },
                                "nodeType": "Cloud"
                            },
                            "status": {
                                "conditionSummary": {
                                    "ready": "True",
                                    "statusMessage": ""
                                },
                                "conditions": [
                                    {
                                        "lastTransitionTime": "2023-06-15T12:07:25Z",
                                        "status": "True",
                                        "type": "Ready"
                                    },
                                    {
                                        "lastTransitionTime": "2023-06-15T12:07:25Z",
                                        "status": "False",
                                        "type": "Updating"
                                    },
                                    {
                                        "lastTransitionTime": "2023-06-06T08:23:11Z",
                                        "status": "False",
                                        "type": "WaitingForDisruptiveApproval"
                                    },
                                    {
                                        "lastTransitionTime": "2023-06-15T12:06:51Z",
                                        "status": "False",
                                        "type": "Error"
                                    },
                                    {
                                        "lastTransitionTime": "2023-06-15T12:07:25Z",
                                        "status": "False",
                                        "type": "Scaling"
                                    }
                                ],
                                "deckhouse": {
                                    "observed": {
                                        "checkSum": "fcdb966f01c82f439fecd3c0a3599ee0",
                                        "lastTimestamp": "2024-11-23T18:40:05Z"
                                    },
                                    "processed": {
                                        "checkSum": "fcdb966f01c82f439fecd3c0a3599ee0",
                                        "lastTimestamp": "2024-11-23T18:40:42Z"
                                    },
                                    "synced": "True"
                                },
                                "desired": 0,
                                "error": "",
                                "instances": 0,
                                "kubernetesVersion": "1.30",
                                "lastMachineFailures": [],
                                "max": 0,
                                "min": 0,
                                "nodes": 0,
                                "ready": 0,
                                "upToDate": 0
                            }
                        }
                    ]
                }
            },
            "toVersion": "deckhouse.io/v1alpha1",
            "type": "Conversion"
        }

        out = hook.testrun(main, [ctx])

        def assert_api_version_and_docker_moved_from_cri_and_another_not_changed(t: unittest.TestCase, objects: typing.List[dict]):
            t.assertEqual(len(objects), 1)
            o = objects[0]

            tests.assert_common_resource_fields(t, o, "deckhouse.io/v1alpha1", "p-90")

            obj = DotMap(o)

            # assert docker moved
            t.assertEqual(obj.spec.docker,  DotMap({
                "manage": False,
                "maxConcurrentDownloads": 4
            }))
            t.assertNotIn("Docker", obj.spec.cri)

            t.assertIn("cri", obj.spec)
            t.assertEqual(obj.spec.cri.type, "Docker")

            # assert some fields cannot changed
            t.assertEqual(obj.spec.nodeTemplate.labels.aaaaa, "")
            t.assertEqual(obj.status.kubernetesVersion, "1.30")
            t.assertEqual(obj.status.ready, 0)


        tests.assert_conversion(self, out, assert_api_version_and_docker_moved_from_cri_and_another_not_changed, None)


    def test_should_convert_from_alpha2_to_v1(self):
        ctx = {
            "binding": "alpha2_to_v1",
            "fromVersion": "deckhouse.io/v1alpha2",
            "review": {
                "request": {
                    "uid": "28ab7564-6f8e-4184-8169-ad3d65dda957",
                    "desiredAPIVersion": "deckhouse.io/v1",
                    "objects": [
                        {
                            "apiVersion": "deckhouse.io/v1alpha2",
                            "kind": "NodeGroup",
                            "metadata": {
                                "creationTimestamp": "2024-11-23T11:09:16Z",
                                "generation": 1,
                                "managedFields": [
                                    {
                                        "apiVersion": "deckhouse.io/v1alpha2",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                ".": {},
                                                "f:cloudInstances": {
                                                    ".": {},
                                                    "f:classReference": {
                                                        ".": {},
                                                        "f:kind": {},
                                                        "f:name": {}
                                                    },
                                                    "f:maxPerZone": {},
                                                    "f:maxSurgePerZone": {},
                                                    "f:maxUnavailablePerZone": {},
                                                    "f:minPerZone": {}
                                                },
                                                "f:cri": {
                                                    ".": {},
                                                    "f:type": {}
                                                },
                                                "f:disruptions": {
                                                    ".": {},
                                                    "f:approvalMode": {}
                                                },
                                                "f:nodeType": {}
                                            }
                                        },
                                        "manager": "kubectl-create",
                                        "operation": "Update",
                                        "time": "2024-11-23T11:09:16Z"
                                    }
                                ],
                                "name": "worker-small-a2",
                                "uid": "5389bed8-daeb-4d60-a1e4-48c4f8903f8b"
                            },
                            "spec": {
                                "cloudInstances": {
                                    "classReference": {
                                        "kind": "OpenStackInstanceClass",
                                        "name": "worker-small"
                                    },
                                    "maxPerZone": 0,
                                    "maxSurgePerZone": 0,
                                    "maxUnavailablePerZone": 0,
                                    "minPerZone": 0
                                },
                                "cri": {
                                    "type": "Containerd"
                                },
                                "disruptions": {
                                    "approvalMode": "Automatic"
                                },
                                "nodeType": "Cloud"
                            }
                        }
                    ]
                }
            },
            "snapshots": {
                "cluster_config": [
                    {
                        "filterResult": "YXBpVmVyc2lvbjogZGVja2hvdXNlLmlvL3YxYWxwaGExCmtpbmQ6IE9wZW5TdGFja0NsdXN0ZXJDb25maWd1cmF0aW9uCmxheW91dDogU3RhbmRhcmQKbWFzdGVyTm9kZUdyb3VwOgogIGluc3RhbmNlQ2xhc3M6CiAgICBhZGRpdGlvbmFsU2VjdXJpdHlHcm91cHM6CiAgICAtIHNhbmRib3gKICAgIC0gc2FuZGJveC1mcm9udGVuZAogICAgLSB0c3Qtc2VjLWdyb3VwCiAgICBldGNkRGlza1NpemVHYjogMTAKICAgIGZsYXZvck5hbWU6IG0xLmxhcmdlLTUwZwogICAgaW1hZ2VOYW1lOiB1YnVudHUtMTgtMDQtY2xvdWQtYW1kNjQKICByZXBsaWNhczogMQogIHZvbHVtZVR5cGVNYXA6CiAgICBub3ZhOiBjZXBoLXNzZApub2RlR3JvdXBzOgotIGluc3RhbmNlQ2xhc3M6CiAgICBhZGRpdGlvbmFsU2VjdXJpdHlHcm91cHM6CiAgICAtIHNhbmRib3gKICAgIC0gc2FuZGJveC1mcm9udGVuZAogICAgY29uZmlnRHJpdmU6IGZhbHNlCiAgICBmbGF2b3JOYW1lOiBtMS54c21hbGwKICAgIGltYWdlTmFtZTogdWJ1bnR1LTE4LTA0LWNsb3VkLWFtZDY0CiAgICBtYWluTmV0d29yazogc2FuZGJveAogICAgcm9vdERpc2tTaXplOiAxNQogIG5hbWU6IGZyb250LW5tCiAgbm9kZVRlbXBsYXRlOgogICAgbGFiZWxzOgogICAgICBhYWE6IGFhYWEKICAgICAgY2NjOiBjY2NjCiAgcmVwbGljYXM6IDIKICB2b2x1bWVUeXBlTWFwOgogICAgbm92YTogY2VwaC1zc2QKcHJvdmlkZXI6CiAgYXV0aFVSTDogaHR0cHM6Ly9jbG91ZC5leGFtcGxlLmNvbS92My8KICBkb21haW5OYW1lOiBEZWZhdWx0CiAgcGFzc3dvcmQ6IHBhc3N3b3JkCiAgcmVnaW9uOiByZWcKICB0ZW5hbnROYW1lOiB1c2VyCiAgdXNlcm5hbWU6IHVzZXIKc3NoUHVibGljS2V5OiBzc2gtcnNhIEFBQQpzdGFuZGFyZDoKICBleHRlcm5hbE5ldHdvcmtOYW1lOiBwdWJsaWMKICBpbnRlcm5hbE5ldHdvcmtDSURSOiAxOTIuMTY4LjE5OC4wLzI0CiAgaW50ZXJuYWxOZXR3b3JrRE5TU2VydmVyczoKICAtIDguOC44LjgKICAtIDEuMS4xLjEKICAtIDguOC40LjQKICBpbnRlcm5hbE5ldHdvcmtTZWN1cml0eTogdHJ1ZQp0YWdzOgogIGE6IGIK"
                    }
                ]
            },
            "toVersion": "deckhouse.io/v1",
            "type": "Conversion"
        }


        out = hook.testrun(main, [ctx])

        def assert_api_version_and_change_node_type_and_another_not_changed(t: unittest.TestCase, objects: typing.List[dict]):
            t.assertEqual(len(objects), 1)
            o = objects[0]

            tests.assert_common_resource_fields(t, o, "deckhouse.io/v1", "worker-small-a2")

            obj = DotMap(o)

            # assert docker moved
            t.assertEqual(obj.spec.nodeType, "CloudEphemeral")

            # assert some fields cannot changed
            t.assertEqual(obj.spec.cri.type, "Containerd")
            t.assertEqual(obj.spec.cloudInstances.classReference.name, "worker-small")

        tests.assert_conversion(self, out, assert_api_version_and_change_node_type_and_another_not_changed, None)


    def test_should_convert_from_alpha1_to_alpha2(self):
        ctx = {
            "binding": "alpha1_to_alpha2",
            "fromVersion": "deckhouse.io/v1alpha1",
            "review": {
                "request": {
                    "uid": "e8a2a24c-2232-480d-a188-2589de68c684",
                    "desiredAPIVersion": "deckhouse.io/v1alpha1",
                    "objects": [
                        {
                            "apiVersion": "deckhouse.io/v1alpha1",
                            "kind": "NodeGroup",
                            "metadata": {
                                "creationTimestamp": "2023-10-16T10:03:38Z",
                                "generation": 3,
                                "managedFields": [
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                ".": {},
                                                "f:disruptions": {
                                                    ".": {},
                                                    "f:approvalMode": {}
                                                },
                                                "f:nodeType": {}
                                            }
                                        },
                                        "manager": "kubectl-create",
                                        "operation": "Update",
                                        "time": "2023-10-16T10:03:38Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                "f:nodeTemplate": {
                                                    ".": {},
                                                    "f:labels": {
                                                        ".": {},
                                                        "f:aaaa": {}
                                                    }
                                                }
                                            }
                                        },
                                        "manager": "kubectl-edit",
                                        "operation": "Update",
                                        "time": "2023-10-16T10:06:01Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                "f:kubelet": {
                                                    "f:resourceReservation": {
                                                        "f:mode": {}
                                                    }
                                                }
                                            }
                                        },
                                        "manager": "deckhouse-controller",
                                        "operation": "Update",
                                        "time": "2024-01-30T07:08:03Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:status": {
                                                ".": {},
                                                "f:conditionSummary": {
                                                    ".": {},
                                                    "f:ready": {},
                                                    "f:statusMessage": {}
                                                },
                                                "f:conditions": {},
                                                "f:deckhouse": {
                                                    ".": {},
                                                    "f:observed": {
                                                        ".": {},
                                                        "f:checkSum": {},
                                                        "f:lastTimestamp": {}
                                                    },
                                                    "f:processed": {
                                                        ".": {},
                                                        "f:checkSum": {},
                                                        "f:lastTimestamp": {}
                                                    },
                                                    "f:synced": {}
                                                },
                                                "f:error": {},
                                                "f:kubernetesVersion": {},
                                                "f:nodes": {},
                                                "f:ready": {},
                                                "f:upToDate": {}
                                            }
                                        },
                                        "manager": "deckhouse-controller",
                                        "operation": "Update",
                                        "subresource": "status",
                                        "time": "2024-11-22T16:40:04Z"
                                    }
                                ],
                                "name": "worker-static",
                                "uid": "5a5e3820-5fdd-4fea-a8cf-032586ae0be5"
                            },
                            "spec": {
                                "disruptions": {
                                    "approvalMode": "Manual"
                                },
                                "kubelet": {
                                    "containerLogMaxFiles": 4,
                                    "containerLogMaxSize": "50Mi"
                                },
                                "nodeTemplate": {
                                    "labels": {
                                        "node-role.kubernetes.io/control-plane": "",
                                        "node-role.kubernetes.io/master": ""
                                    },
                                    "taints": [
                                        {
                                            "effect": "NoSchedule",
                                            "key": "node-role.kubernetes.io/control-plane"
                                        }
                                    ]
                                },
                                "docker": {
                                    "manage": False,
                                    "maxConcurrentDownloads": 4
                                },
                                "nodeType": "Hybrid"
                            },
                            "status": {
                                "conditionSummary": {
                                    "ready": "True",
                                    "statusMessage": ""
                                },
                                "error": "",
                                "kubernetesVersion": "1.30",
                                "nodes": 1,
                                "ready": 1,
                                "upToDate": 1
                            }
                        }
                    ]
                }
            },
            "toVersion": "deckhouse.io/v1alpha2",
            "type": "Conversion"
        }

        out = hook.testrun(main, [ctx])

        def assert_api_version_and_docker_move_to_cri_and_remove_k8s_ver_and_another_not_changed(t: unittest.TestCase, objects: typing.List[dict]):
            t.assertEqual(len(objects), 1)
            o = objects[0]

            tests.assert_common_resource_fields(t, o, "deckhouse.io/v1alpha2", "worker-static")

            obj = DotMap(o)

            # assert docker moved
            t.assertIn("cri", obj.spec)
            t.assertIn("docker", obj.spec.cri)
            t.assertEqual(obj.spec.cri.docker, DotMap({
                "manage": False,
                "maxConcurrentDownloads": 4
            }))

            # assert some fields cannot changed
            t.assertEqual(obj.spec.kubelet, DotMap({
                "containerLogMaxFiles": 4,
                "containerLogMaxSize": "50Mi"
            }))
            t.assertEqual(obj.status.ready, 1)

        tests.assert_conversion(self, out, assert_api_version_and_docker_move_to_cri_and_remove_k8s_ver_and_another_not_changed, None)


    def test_should_convert_from_v1_alpha2_multiple_objects(self):
        ctx = {
            "binding": "v1_to_alpha2",
            "fromVersion": "deckhouse.io/v1",
            "review": {
                "request": {
                    "uid": "1f756338-32cd-49b4-81d7-292a770aa1d8",
                    "desiredAPIVersion": "deckhouse.io/v1alpha1",
                    "objects": [
                        {
                            "apiVersion": "deckhouse.io/v1",
                            "kind": "NodeGroup",
                            "metadata": {
                                "creationTimestamp": "2021-03-18T13:46:17Z",
                                "generation": 6,
                                "name": "master",
                                "uid": "7f66a236-6931-478d-b635-69ab8862fa75"
                            },
                            "spec": {
                                "disruptions": {
                                    "approvalMode": "Manual"
                                },
                                "kubelet": {
                                    "containerLogMaxFiles": 4,
                                    "containerLogMaxSize": "50Mi",
                                    "resourceReservation": {
                                        "mode": "Off"
                                    }
                                },
                                "nodeTemplate": {
                                    "labels": {
                                        "node-role.kubernetes.io/control-plane": "",
                                        "node-role.kubernetes.io/master": ""
                                    },
                                    "taints": [
                                        {
                                            "effect": "NoSchedule",
                                            "key": "node-role.kubernetes.io/control-plane"
                                        }
                                    ]
                                },
                                "nodeType": "CloudPermanent"
                            },
                            "status": {
                                "conditionSummary": {
                                    "ready": "True",
                                    "statusMessage": ""
                                },
                                "conditions": [
                                    {
                                        "lastTransitionTime": "2023-05-10T07:47:32Z",
                                        "status": "True",
                                        "type": "Ready"
                                    },
                                    {
                                        "lastTransitionTime": "2024-11-23T16:22:28Z",
                                        "status": "False",
                                        "type": "Updating"
                                    },
                                    {
                                        "lastTransitionTime": "2023-09-05T09:08:55Z",
                                        "status": "False",
                                        "type": "WaitingForDisruptiveApproval"
                                    },
                                    {
                                        "lastTransitionTime": "2023-03-07T20:51:25Z",
                                        "status": "False",
                                        "type": "Error"
                                    }
                                ],
                                "deckhouse": {
                                    "observed": {
                                        "checkSum": "42c757d98445e45283f80870e61d4963",
                                        "lastTimestamp": "2024-11-23T19:50:01Z"
                                    },
                                    "processed": {
                                        "checkSum": "42c757d98445e45283f80870e61d4963",
                                        "lastTimestamp": "2024-11-23T19:31:29Z"
                                    },
                                    "synced": "True"
                                },
                                "error": "",
                                "kubernetesVersion": "1.30",
                                "nodes": 1,
                                "ready": 1,
                                "upToDate": 1
                            }
                        },
                        {
                            "apiVersion": "deckhouse.io/v1",
                            "kind": "NodeGroup",
                            "metadata": {
                                "creationTimestamp": "2024-03-18T17:55:40Z",
                                "generation": 2,
                                "managedFields": [
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                ".": {},
                                                "f:kubelet": {
                                                    ".": {},
                                                    "f:containerLogMaxFiles": {},
                                                    "f:containerLogMaxSize": {},
                                                    "f:resourceReservation": {
                                                        ".": {},
                                                        "f:mode": {}
                                                    }
                                                },
                                                "f:nodeType": {},
                                                "f:staticInstances": {}
                                            }
                                        },
                                        "manager": "kubectl-create",
                                        "operation": "Update",
                                        "time": "2024-03-18T17:55:40Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                "f:staticInstances": {
                                                    "f:count": {}
                                                }
                                            }
                                        },
                                        "manager": "kubectl-edit",
                                        "operation": "Update",
                                        "time": "2024-03-18T17:56:07Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:status": {
                                                ".": {},
                                                "f:conditionSummary": {
                                                    ".": {},
                                                    "f:ready": {},
                                                    "f:statusMessage": {}
                                                },
                                                "f:conditions": {},
                                                "f:deckhouse": {
                                                    ".": {},
                                                    "f:observed": {
                                                        ".": {},
                                                        "f:checkSum": {},
                                                        "f:lastTimestamp": {}
                                                    },
                                                    "f:processed": {
                                                        ".": {},
                                                        "f:checkSum": {},
                                                        "f:lastTimestamp": {}
                                                    },
                                                    "f:synced": {}
                                                },
                                                "f:error": {},
                                                "f:kubernetesVersion": {},
                                                "f:nodes": {},
                                                "f:ready": {},
                                                "f:upToDate": {}
                                            }
                                        },
                                        "manager": "deckhouse-controller",
                                        "operation": "Update",
                                        "subresource": "status",
                                        "time": "2024-11-23T19:40:03Z"
                                    }
                                ],
                                "name": "worker",
                                "uid": "16048331-5bf9-46f8-b535-6ad202380c1a"
                            },
                            "spec": {
                                "kubelet": {
                                    "containerLogMaxFiles": 4,
                                    "containerLogMaxSize": "50Mi",
                                    "resourceReservation": {
                                        "mode": "Auto"
                                    }
                                },
                                "nodeType": "Static",
                                "staticInstances": {
                                    "count": 0
                                }
                            },
                            "status": {
                                "conditionSummary": {
                                    "ready": "True",
                                    "statusMessage": ""
                                },
                                "conditions": [
                                    {
                                        "lastTransitionTime": "2024-03-18T17:56:24Z",
                                        "status": "True",
                                        "type": "Ready"
                                    },
                                    {
                                        "lastTransitionTime": "2024-03-18T17:56:24Z",
                                        "status": "False",
                                        "type": "Updating"
                                    },
                                    {
                                        "lastTransitionTime": "2024-03-18T17:56:24Z",
                                        "status": "False",
                                        "type": "WaitingForDisruptiveApproval"
                                    },
                                    {
                                        "lastTransitionTime": "2024-03-18T17:56:24Z",
                                        "status": "False",
                                        "type": "Error"
                                    }
                                ],
                                "deckhouse": {
                                    "observed": {
                                        "checkSum": "d084ef81a6796c7a7f386762663cdbfb",
                                        "lastTimestamp": "2024-11-23T19:40:03Z"
                                    },
                                    "processed": {
                                        "checkSum": "d084ef81a6796c7a7f386762663cdbfb",
                                        "lastTimestamp": "2024-11-23T19:31:31Z"
                                    },
                                    "synced": "True"
                                },
                                "error": "",
                                "kubernetesVersion": "1.30",
                                "nodes": 0,
                                "ready": 0,
                                "upToDate": 0
                            }
                        },
                        {
                            "apiVersion": "deckhouse.io/v1",
                            "kind": "NodeGroup",
                            "metadata": {
                                "creationTimestamp": "2024-11-23T11:09:16Z",
                                "generation": 1,
                                "name": "worker-small-a2",
                                "uid": "5389bed8-daeb-4d60-a1e4-48c4f8903f8b"
                            },
                            "spec": {
                                "cloudInstances": {
                                    "classReference": {
                                        "kind": "OpenStackInstanceClass",
                                        "name": "worker-small"
                                    },
                                    "maxPerZone": 0,
                                    "maxSurgePerZone": 0,
                                    "maxUnavailablePerZone": 0,
                                    "minPerZone": 0
                                },
                                "cri": {
                                    "type": "Containerd"
                                },
                                "disruptions": {
                                    "approvalMode": "Automatic"
                                },
                                "kubelet": {
                                    "containerLogMaxFiles": 4,
                                    "containerLogMaxSize": "50Mi",
                                    "resourceReservation": {
                                        "mode": "Auto"
                                    }
                                },
                                "nodeType": "CloudEphemeral"
                            },
                            "status": {
                                "conditionSummary": {
                                    "ready": "True",
                                    "statusMessage": ""
                                },
                                "conditions": [
                                    {
                                        "lastTransitionTime": "2024-11-23T11:09:18Z",
                                        "status": "True",
                                        "type": "Ready"
                                    },
                                    {
                                        "lastTransitionTime": "2024-11-23T11:09:18Z",
                                        "status": "False",
                                        "type": "Updating"
                                    },
                                    {
                                        "lastTransitionTime": "2024-11-23T11:09:18Z",
                                        "status": "False",
                                        "type": "WaitingForDisruptiveApproval"
                                    },
                                    {
                                        "lastTransitionTime": "2024-11-23T11:09:18Z",
                                        "status": "False",
                                        "type": "Error"
                                    },
                                    {
                                        "lastTransitionTime": "2024-11-23T11:09:18Z",
                                        "status": "False",
                                        "type": "Scaling"
                                    }
                                ],
                                "deckhouse": {
                                    "observed": {
                                        "checkSum": "ffc0444cc2ba5d39bbf8aeff72102178",
                                        "lastTimestamp": "2024-11-23T19:40:03Z"
                                    },
                                    "processed": {
                                        "checkSum": "ffc0444cc2ba5d39bbf8aeff72102178",
                                        "lastTimestamp": "2024-11-23T19:31:31Z"
                                    },
                                    "synced": "True"
                                },
                                "desired": 0,
                                "error": "",
                                "instances": 0,
                                "kubernetesVersion": "1.30",
                                "lastMachineFailures": [],
                                "max": 0,
                                "min": 0,
                                "nodes": 0,
                                "ready": 0,
                                "upToDate": 0
                            }
                        },
                        {
                            "apiVersion": "deckhouse.io/v1",
                            "kind": "NodeGroup",
                            "metadata": {
                                "creationTimestamp": "2023-10-16T10:03:38Z",
                                "generation": 3,
                                "managedFields": [
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                ".": {},
                                                "f:disruptions": {
                                                    ".": {},
                                                    "f:approvalMode": {}
                                                },
                                                "f:nodeType": {}
                                            }
                                        },
                                        "manager": "kubectl-create",
                                        "operation": "Update",
                                        "time": "2023-10-16T10:03:38Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                "f:nodeTemplate": {
                                                    ".": {},
                                                    "f:labels": {
                                                        ".": {},
                                                        "f:aaaa": {}
                                                    }
                                                }
                                            }
                                        },
                                        "manager": "kubectl-edit",
                                        "operation": "Update",
                                        "time": "2023-10-16T10:06:01Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:spec": {
                                                "f:kubelet": {
                                                    "f:resourceReservation": {
                                                        "f:mode": {}
                                                    }
                                                }
                                            }
                                        },
                                        "manager": "deckhouse-controller",
                                        "operation": "Update",
                                        "time": "2024-01-30T07:08:03Z"
                                    },
                                    {
                                        "apiVersion": "deckhouse.io/v1",
                                        "fieldsType": "FieldsV1",
                                        "fieldsV1": {
                                            "f:status": {
                                                ".": {},
                                                "f:conditionSummary": {
                                                    ".": {},
                                                    "f:ready": {},
                                                    "f:statusMessage": {}
                                                },
                                                "f:conditions": {},
                                                "f:deckhouse": {
                                                    ".": {},
                                                    "f:observed": {
                                                        ".": {},
                                                        "f:checkSum": {},
                                                        "f:lastTimestamp": {}
                                                    },
                                                    "f:processed": {
                                                        ".": {},
                                                        "f:checkSum": {},
                                                        "f:lastTimestamp": {}
                                                    },
                                                    "f:synced": {}
                                                },
                                                "f:error": {},
                                                "f:kubernetesVersion": {},
                                                "f:nodes": {},
                                                "f:ready": {},
                                                "f:upToDate": {}
                                            }
                                        },
                                        "manager": "deckhouse-controller",
                                        "operation": "Update",
                                        "subresource": "status",
                                        "time": "2024-11-23T19:40:04Z"
                                    }
                                ],
                                "name": "worker-static",
                                "uid": "5a5e3820-5fdd-4fea-a8cf-032586ae0be5"
                            },
                            "spec": {
                                "disruptions": {
                                    "approvalMode": "Automatic"
                                },
                                "kubelet": {
                                    "containerLogMaxFiles": 4,
                                    "containerLogMaxSize": "50Mi",
                                    "resourceReservation": {
                                        "mode": "Off"
                                    }
                                },
                                "nodeTemplate": {
                                    "labels": {
                                        "aaaa": "bbbb"
                                    }
                                },
                                "nodeType": "CloudStatic"
                            },
                            "status": {
                                "conditionSummary": {
                                    "ready": "True",
                                    "statusMessage": ""
                                },
                                "conditions": [
                                    {
                                        "lastTransitionTime": "2023-10-16T10:03:39Z",
                                        "status": "True",
                                        "type": "Ready"
                                    },
                                    {
                                        "lastTransitionTime": "2023-10-26T12:24:02Z",
                                        "status": "False",
                                        "type": "Updating"
                                    },
                                    {
                                        "lastTransitionTime": "2023-10-16T10:03:39Z",
                                        "status": "False",
                                        "type": "WaitingForDisruptiveApproval"
                                    },
                                    {
                                        "lastTransitionTime": "2023-10-16T10:03:39Z",
                                        "status": "False",
                                        "type": "Error"
                                    }
                                ],
                                "deckhouse": {
                                    "observed": {
                                        "checkSum": "8cfb8c1cda6ce98f0b50e52302d5a871",
                                        "lastTimestamp": "2024-11-23T19:40:04Z"
                                    },
                                    "processed": {
                                        "checkSum": "8cfb8c1cda6ce98f0b50e52302d5a871",
                                        "lastTimestamp": "2024-11-23T19:31:31Z"
                                    },
                                    "synced": "True"
                                },
                                "error": "",
                                "kubernetesVersion": "1.30",
                                "nodes": 0,
                                "ready": 0,
                                "upToDate": 0
                            }
                        }
                    ]
                }
            },
            "toVersion": "deckhouse.io/v1alpha2",
            "type": "Conversion"
        }



        out = hook.testrun(main, [ctx])

        def assert_api_version_and_change_node_type(t: unittest.TestCase, objects: typing.List[dict]):
            t.assertEqual(len(objects), 4)

            tests.assert_common_resource_fields(t, objects[0], "deckhouse.io/v1alpha2", "master")
            # was CloudPermanent
            t.assertEqual(DotMap(objects[0]).spec.nodeType, "Hybrid")

            tests.assert_common_resource_fields(t, objects[1], "deckhouse.io/v1alpha2", "worker")
            # not changed
            t.assertEqual(DotMap(objects[1]).spec.nodeType, "Static")

            tests.assert_common_resource_fields(t, objects[2], "deckhouse.io/v1alpha2", "worker-small-a2")
            # was CloudEphemeral
            t.assertEqual(DotMap(objects[2]).spec.nodeType, "Cloud")

            tests.assert_common_resource_fields(t, objects[3], "deckhouse.io/v1alpha2", "worker-static")
            # was CloudStatic
            t.assertEqual(DotMap(objects[3]).spec.nodeType, "Hybrid")


        tests.assert_conversion(self, out, assert_api_version_and_change_node_type, None)



if __name__ == '__main__':
    unittest.main()
