#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#

from node_group_resources_metrics import main
from shell_operator import hook


def test_node_node_group_resources_metrics():
    out = hook.testrun(main, binding_context)

    assert out.metrics.data == expected_metrics
    assert not out.kube_operations.data

binding_context = [
    {
        "binding": "main",
        "type": "Group",
        "snapshots": {
            "nodes": [
                {
                    "filterResult": {
                      "node_group": "application-nodes",
                      "capacity": {
                        "cpu": "8",
                        "ephemeral-storage": "39359968Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "16351048Ki",
                        "pods": "150"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "application-nodes",
                      "capacity": {
                        "cpu": "8",
                        "ephemeral-storage": "39359968Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "16351056Ki",
                        "pods": "150"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "d8-loki",
                      "capacity": {
                        "cpu": "4",
                        "ephemeral-storage": "49677140Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "8105804Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "dam",
                      "capacity": {
                        "cpu": "16",
                        "ephemeral-storage": "39359968Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "3983184Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "dam",
                      "capacity": {
                        "cpu": "16",
                        "ephemeral-storage": "39359968Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "3983192Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "dam",
                      "capacity": {
                        "cpu": "16",
                        "ephemeral-storage": "39359968Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "3983184Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "dam",
                      "capacity": {
                        "cpu": "16",
                        "ephemeral-storage": "39359968Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "3983192Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "loadbalancer",
                      "capacity": {
                        "cpu": "2",
                        "ephemeral-storage": "49967180Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "4001708Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "loadbalancer",
                      "capacity": {
                        "cpu": "2",
                        "ephemeral-storage": "49967180Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "4001700Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "master",
                      "capacity": {
                        "cpu": "8",
                        "ephemeral-storage": "49677140Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "16384592Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "odfe-common",
                      "capacity": {
                        "cpu": "4",
                        "ephemeral-storage": "29042796Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "8105812Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "odfe-common",
                      "capacity": {
                        "cpu": "4",
                        "ephemeral-storage": "29042796Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "8105804Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "opendistro",
                      "capacity": {
                        "cpu": "2",
                        "ephemeral-storage": "29042796Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "10157908Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "opendistro",
                      "capacity": {
                        "cpu": "2",
                        "ephemeral-storage": "29042796Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "10157900Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "router",
                      "capacity": {
                        "cpu": "2",
                        "ephemeral-storage": "29324176Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "4030560Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "system",
                      "capacity": {
                        "cpu": "4",
                        "ephemeral-storage": "49677140Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "16351056Ki",
                        "pods": "110"
                      }
                    }
                },
                {
                    "filterResult": {
                      "node_group": "system",
                      "capacity": {
                        "cpu": "4",
                        "ephemeral-storage": "49677140Ki",
                        "hugepages-1Gi": "0",
                        "hugepages-2Mi": "0",
                        "memory": "16351056Ki",
                        "pods": "110"
                      }
                    }
                },
            ],
            "ngs": [
                {
                    "filterResult": {
                      "name": "application-nodes",
                      "nodeTemplate": {
                        "labels": {
                          "node-role/application": ""
                        }
                      }
                    }
                },
                {
                    "filterResult": {
                      "name": "d8-loki",
                      "nodeTemplate": {
                        "labels": {
                          "node-role/d8-loki": ""
                        },
                        "taints": [
                          {
                            "effect": "NoExecute",
                            "key": "dedicated",
                            "value": "d8-loki"
                          }
                        ]
                      }
                    }
                },
                {
                    "filterResult": {
                      "name": "dam",
                      "nodeTemplate": {
                        "labels": {
                          "node-role/application": "",
                          "node-role/dam": ""
                        },
                        "taints": [
                          {
                            "effect": "NoExecute",
                            "key": "dedicated",
                            "value": "dam"
                          }
                        ]
                      }
                    }
                },
                {
                    "filterResult": {
                      "name": "loadbalancer",
                      "nodeTemplate": {
                        "labels": {
                          "node-role/loadbalancer": ""
                        },
                        "taints": [
                          {
                            "effect": "NoExecute",
                            "key": "node-role/loadbalancer",
                            "value": ""
                          }
                        ]
                      }
                    }
                },
                {
                    "filterResult": {
                      "name": "master",
                      "nodeTemplate": {
                        "labels": {
                          "node-role.kubernetes.io/control-plane": "",
                          "node-role.kubernetes.io/master": ""
                        },
                        "taints": [
                          {
                            "effect": "NoSchedule",
                            "key": "node-role.kubernetes.io/master"
                          },
                          {
                            "effect": "NoSchedule",
                            "key": "node-role.kubernetes.io/control-plane"
                          }
                        ]
                      }
                    }
                },
                {
                    "filterResult": {
                      "name": "odfe-common",
                      "nodeTemplate": {
                        "labels": {
                          "node-role/opendistro-common": ""
                        },
                        "taints": [
                          {
                            "effect": "NoSchedule",
                            "key": "dedicated",
                            "value": "opendistro-common"
                          }
                        ]
                      }
                    }
                },
                {
                    "filterResult": {
                      "name": "opendistro",
                      "nodeTemplate": {
                        "labels": {
                          "node-role/opendistro": ""
                        },
                        "taints": [
                          {
                            "effect": "NoSchedule",
                            "key": "dedicated",
                            "value": "opendistro"
                          }
                        ]
                      }
                    }
                },
                {
                    "filterResult": {
                      "name": "router",
                      "nodeTemplate": {
                        "labels": {
                            "node-role.deckhouse.io/frontend": ""
                        },
                        "taints": [
                          {
                            "effect": "NoExecute",
                            "key": "dedicated.deckhouse.io",
                            "value": "frontend"
                          }
                        ]
                      }
                    }
                },
                {
                    "filterResult": {
                      "name": "system",
                      "nodeTemplate": {
                        "labels": {
                          "node-role.deckhouse.io/system": "",
                          "node-role/logging": "",
                          "node-role/monitoring": ""
                        },
                        "taints": [
                          {
                            "effect": "NoExecute",
                            "key": "dedicated.deckhouse.io",
                            "value": "system"
                          }
                        ]
                      }
                    }
                },
            ],
        },
    }
]

expected_metrics = [
    {"action": "expire", "group": "group_node_group_resources_metrics"},
    {
        "name": "flant_pricing_node_group_cpu_cores",
        "group": "group_node_group_resources_metrics",
         "labels": {"is_frontend": "false", "is_master": "false", "is_monitoring": "false", "is_system": "false"},
         "set": 100.0,
    },
    {
        "name": "flant_pricing_node_group_memory",
        "group": "group_node_group_resources_metrics",
         "labels": {"is_frontend": "false", "is_master": "false", "is_monitoring": "false", "is_system": "false"},
         "set": 103702007808.0,
    },
    {
        "name": "flant_pricing_node_group_cpu_cores",
        "group": "group_node_group_resources_metrics",
         "labels": {"is_frontend": "false", "is_master": "true", "is_monitoring": "false", "is_system": "false"},
         "set": 8.0,
    },
    {
        "name": "flant_pricing_node_group_memory",
        "group": "group_node_group_resources_metrics",
         "labels": {"is_frontend": "false", "is_master": "true", "is_monitoring": "false", "is_system": "false"},
         "set": 16777822208.0,
    },
    {
        "name": "flant_pricing_node_group_cpu_cores",
        "group": "group_node_group_resources_metrics",
         "labels": {"is_frontend": "true", "is_master": "false", "is_monitoring": "false", "is_system": "false"},
         "set": 2.0,
    },
    {
        "name": "flant_pricing_node_group_memory",
        "group": "group_node_group_resources_metrics",
         "labels": {"is_frontend": "true", "is_master": "false", "is_monitoring": "false", "is_system": "false"},
         "set": 4127293440.0,
    },
    {
        "name": "flant_pricing_node_group_cpu_cores",
        "group": "group_node_group_resources_metrics",
         "labels": {"is_frontend": "false", "is_master": "false", "is_monitoring": "false", "is_system": "true"},
         "set": 8.0,
    },
    {
        "name": "flant_pricing_node_group_memory",
        "group": "group_node_group_resources_metrics",
         "labels": {"is_frontend": "false", "is_master": "false", "is_monitoring": "false", "is_system": "true"},
         "set": 33486962688.0,
    },
]
