#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
# This hook is responsible for generating metrics for node count of each type.
#

from dataclasses import dataclass

from framework import MetricsExporter, bindingcontext

# We do not charge for control plane nodes which are in desired state.
#
# The consumer must subtract the number of tainted control plane nodes from the total number of
# nodes of the same type. Thus we will check one of the following conditions:
#   1. the node is NOT in control plane node group, so we charge for it
#   2. the node is from control plane node group, BUT has no expected taints meaning they were
#      reconfigured by user, so we charge for it
#
# The following metrics are generated:
#   - flant_pricing_count_nodes_by_type         -- DEPRECATED all nodes except CP nodes with
#     expected taints
#   - flant_pricing_nodes                       -- all nodes
#   - flant_pricing_controlplane_nodes          -- CP nodes
#   - flant_pricing_controlplane_tainted_nodes  -- CP nodes with expected taints
#
# All metrics are labeled with "type" which is one of the following:
#   - ephemeral
#   - vm
#   - hard
#   - special
#
# To count the number of nodes to charge for any type, the consumer must subtract the number of
# tainted nodes from the total number of nodes of the same type, e.g.
#
# flant_pricing_nodes{type="ephemeral"} - flant_pricing_controlplane_tainted_nodes{type="ephemeral"}
# flant_pricing_nodes{type="vm"}        - flant_pricing_controlplane_tainted_nodes{type="vm"}
# flant_pricing_nodes{type="hard"}      - flant_pricing_controlplane_tainted_nodes{type="hard"}
# flant_pricing_nodes{type="special"}   - flant_pricing_controlplane_tainted_nodes{type="special"}
#
# flant_pricing_controlplane_tainted_nodes will be non-zero only for one type.


# Node types for pricing from the annotation 'pricing.flant.com/nodeType'. Lowercase versions of
# them are used as labels in metrics
PRICING_EPHEMERAL = "Ephemeral"
PRICING_HARD = "Hard"
PRICING_SPECIAL = "Special"
PRICING_VM = "VM"
PRICING_UNKNOWN = "unknown"  # fallback from filter result


# Node group node types
NG_CLOUDEPHEMERAL = "CloudEphemeral"
NG_CLOUDPERMANENT = "CloudPermanent"
NG_CLOUDSTATIC = "CloudStatic"
NG_STATIC = "Static"


def map_ng_to_pricing_type(ng_node_type, virtualization):
    if ng_node_type == NG_CLOUDEPHEMERAL:
        return PRICING_EPHEMERAL

    if ng_node_type in (NG_CLOUDPERMANENT, NG_CLOUDSTATIC):
        return PRICING_VM

    if ng_node_type == NG_STATIC and virtualization != "unknown":
        return PRICING_VM

    return PRICING_HARD


@dataclass
class NodeGroup:
    # jqFiter: name
    name: str
    # jqFiter: nodeType
    node_type: str


@dataclass
class Node:
    # jqFiter: nodeGroup (name)
    node_group: NodeGroup
    # jqFiter: pricingNodeType
    pricing_node_type: str
    # jqFiter: virtualization
    virtualization: str

    def pricing_type(self):
        if self.pricing_node_type == PRICING_UNKNOWN:
            return map_ng_to_pricing_type(
                self.node_group.node_type,
                self.virtualization,
            )
        return self.pricing_node_type


def parse_nodegroups_by_name(ng_snapshots):
    """
    Collects dict of node groups by name.
    """
    by_name = {}
    for s in ng_snapshots:
        filtered = s["filterResult"]
        if filtered is None:
            # should not happen
            continue
        name, node_type = filtered["name"], filtered["nodeType"]
        by_name[name] = NodeGroup(name=name, node_type=node_type)
    return by_name


def parse_nodes(node_snapshots, nodegroup_by_name):
    """
    Collects list of nodes with nodegroup in them. Skips nodes which are not in node groups.
    """
    nodes = []
    for s in node_snapshots:
        filtered = s["filterResult"]
        if filtered is None:
            # The node did not match the jqFilter, e.g. control-plane node without expected taints
            continue

        ng_name = filtered["nodeGroup"]
        if ng_name not in nodegroup_by_name:
            # we don't charge for nodes which are not in node groups
            continue
        node_group = nodegroup_by_name.get(ng_name)

        nodes.append(
            Node(
                node_group=node_group,
                pricing_node_type=filtered["pricingNodeType"],
                virtualization=filtered["virtualization"],
            )
        )
    return nodes


def main():
    """
    Hook entry point. Used in test.
    """
    metric_group = "group_node_metrics"
    # snap_name, metric_name
    metric_configs = (
        ("nodes", "flant_pricing_count_nodes_by_type"),  # DEPRECATED
        ("nodes_all", "flant_pricing_nodes"),
        ("nodes_cp", "flant_pricing_controlplane_nodes"),
        ("nodes_t_cp", "flant_pricing_controlplane_tainted_nodes"),
    )

    metrics = []
    with bindingcontext("node_metrics.yaml") as ctx:
        snapshots = ctx["snapshots"]
        # Collect node groups to use them in nodes
        ng_by_name = parse_nodegroups_by_name(snapshots["ngs"])
        for snap_name, metric_name in metric_configs:
            # Build node instances
            nodes = parse_nodes(snapshots[snap_name], ng_by_name)
            for m in gen_metric(nodes, metric_group, metric_name):
                metrics.append(m)

    with MetricsExporter() as e:
        e.export({"action": "expire", "group": metric_group})
        for m in metrics:
            e.export(m)


def gen_metric(nodes, metric_group: str, metric_name: str):
    pricing_types = (
        PRICING_EPHEMERAL,
        PRICING_HARD,
        PRICING_SPECIAL,
        PRICING_VM,
    )
    # Count nodes by type
    count_by_type = {t: 0 for t in pricing_types}
    for node in nodes:
        count_by_type[node.pricing_type()] += 1

    # Yield metrics
    for pricing_type, count in count_by_type.items():
        yield {
            "name": metric_name,
            "group": metric_group,
            "set": count,
            "labels": {
                "type": pricing_type.lower(),
            },
        }


if __name__ == "__main__":
    main()
