#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#


import framework
from node_metrics import handle


def test_node_metrics():
    metrics = framework.MetricsExporter(mem_store=True)
    kubernetes = framework.KubernetesModifier(mem_store=True)
    for ctx in binding_context:
        hook_ctx = framework.HookContext(ctx, metrics=metrics, kubernetes=kubernetes)
        handle(hook_ctx)

    assert metrics.storage.data == expected_metrics


binding_context = [
    {
        "binding": "main",
        "type": "Group",
        "snapshots": {
            "nodes_t_cp": [
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "master",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "master",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "master",
                        "virtualization": "kvm",
                    }
                },
                {"filterResult": None},
                {"filterResult": None},
            ],
            "nodes": [
                {"filterResult": None},
                {"filterResult": None},
                {"filterResult": None},
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "worker-medium",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "worker-small",
                        "virtualization": "kvm",
                    }
                },
            ],
            "ngs": [
                {"filterResult": {"name": "master", "nodeType": "CloudPermanent"}},
                {
                    "filterResult": {
                        "name": "worker-large",
                        "nodeType": "CloudEphemeral",
                    }
                },
                {
                    "filterResult": {
                        "name": "worker-medium",
                        "nodeType": "CloudEphemeral",
                    }
                },
                {
                    "filterResult": {
                        "name": "worker-small",
                        "nodeType": "CloudEphemeral",
                    }
                },
            ],
            "nodes_all": [
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "master",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "master",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "master",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "worker-medium",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "worker-small",
                        "virtualization": "kvm",
                    }
                },
            ],
            "nodes_cp": [
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "master",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "master",
                        "virtualization": "kvm",
                    }
                },
                {
                    "filterResult": {
                        "pricingNodeType": "unknown",
                        "nodeGroup": "master",
                        "virtualization": "kvm",
                    }
                },
                {"filterResult": None},
                {"filterResult": None},
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
