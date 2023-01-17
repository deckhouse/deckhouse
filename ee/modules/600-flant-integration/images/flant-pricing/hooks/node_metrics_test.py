#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#

from node_metrics import main
from shell_operator import hook


def test_node_metrics():
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
                        "is_controlplane": True,
                        "is_controlplane_tainted": True,
                        "is_legacy_counted": False,
                        "is_managed": True,
                        "node_group": "master",
                        "pricing_node_type": "unknown",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "is_controlplane": True,
                        "is_controlplane_tainted": True,
                        "is_legacy_counted": False,
                        "is_managed": True,
                        "node_group": "master",
                        "pricing_node_type": "unknown",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "is_controlplane": True,
                        "is_controlplane_tainted": True,
                        "is_legacy_counted": False,
                        "is_managed": True,
                        "node_group": "master",
                        "pricing_node_type": "unknown",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "is_controlplane": False,
                        "is_controlplane_tainted": False,
                        "is_legacy_counted": True,
                        "is_managed": True,
                        "node_group": "worker-medium",
                        "pricing_node_type": "unknown",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "is_controlplane": False,
                        "is_controlplane_tainted": False,
                        "is_legacy_counted": True,
                        "is_managed": True,
                        "node_group": "worker-small",
                        "pricing_node_type": "unknown",
                        "virtualization": "kvm",
                    }
                },
            ],
            "ngs": [
                {
                    "filterResult": {
                        "name": "master",
                        "node_type": "CloudPermanent",
                    }
                },
                {
                    "filterResult": {
                        "name": "worker-large",
                        "node_type": "CloudEphemeral",
                    }
                },
                {
                    "filterResult": {
                        "name": "worker-medium",
                        "node_type": "CloudEphemeral",
                    }
                },
                {
                    "filterResult": {
                        "name": "worker-small",
                        "node_type": "CloudEphemeral",
                    }
                },
            ],
        },
    }
]


expected_metrics = [
    {"action": "expire", "group": "group_node_metrics"},
    {
        "name": "flant_pricing_count_nodes_by_type",
        "group": "group_node_metrics",
        "set": 2,
        "labels": {"type": "ephemeral"},
    },
    {
        "name": "flant_pricing_count_nodes_by_type",
        "group": "group_node_metrics",
        "set": 0,
        "labels": {"type": "hard"},
    },
    {
        "name": "flant_pricing_count_nodes_by_type",
        "group": "group_node_metrics",
        "set": 0,
        "labels": {"type": "special"},
    },
    {
        "name": "flant_pricing_count_nodes_by_type",
        "group": "group_node_metrics",
        "set": 0,
        "labels": {"type": "vm"},
    },
    {
        "name": "flant_pricing_nodes",
        "group": "group_node_metrics",
        "set": 2,
        "labels": {"type": "ephemeral"},
    },
    {
        "name": "flant_pricing_nodes",
        "group": "group_node_metrics",
        "set": 0,
        "labels": {"type": "hard"},
    },
    {
        "name": "flant_pricing_nodes",
        "group": "group_node_metrics",
        "set": 0,
        "labels": {"type": "special"},
    },
    {
        "name": "flant_pricing_nodes",
        "group": "group_node_metrics",
        "set": 3,
        "labels": {"type": "vm"},
    },
    {
        "name": "flant_pricing_controlplane_nodes",
        "group": "group_node_metrics",
        "set": 0,
        "labels": {"type": "ephemeral"},
    },
    {
        "name": "flant_pricing_controlplane_nodes",
        "group": "group_node_metrics",
        "set": 0,
        "labels": {"type": "hard"},
    },
    {
        "name": "flant_pricing_controlplane_nodes",
        "group": "group_node_metrics",
        "set": 0,
        "labels": {"type": "special"},
    },
    {
        "name": "flant_pricing_controlplane_nodes",
        "group": "group_node_metrics",
        "set": 3,
        "labels": {"type": "vm"},
    },
    {
        "name": "flant_pricing_controlplane_tainted_nodes",
        "group": "group_node_metrics",
        "set": 0,
        "labels": {"type": "ephemeral"},
    },
    {
        "name": "flant_pricing_controlplane_tainted_nodes",
        "group": "group_node_metrics",
        "set": 0,
        "labels": {"type": "hard"},
    },
    {
        "name": "flant_pricing_controlplane_tainted_nodes",
        "group": "group_node_metrics",
        "set": 0,
        "labels": {"type": "special"},
    },
    {
        "name": "flant_pricing_controlplane_tainted_nodes",
        "group": "group_node_metrics",
        "set": 3,
        "labels": {"type": "vm"},
    },
]
