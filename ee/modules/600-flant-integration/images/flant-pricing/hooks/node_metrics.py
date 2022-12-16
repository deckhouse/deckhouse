#!/usr/bin/env micropython
#
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
# This hook is responsible for generating metrics for node count of each type.
#
# It is written in micropython because it lets keep the image size small. Note the difference
# between python and micropython: https://docs.micropython.org/en/latest/genrst/index.html

import json
import os
import sys

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


def main():
    # Hook config
    if len(sys.argv) > 1 and sys.argv[1] == "--config":
        with open("node_metrics.yaml", "r", encoding="utf-8") as f:
            print(f.read())
            return

    # Hook body
    metric_group = "group_node_metrics"
    with open(os.getenv("METRICS_PATH"), "a", encoding="utf-8") as f:
        f.write(json.dumps({"action": "expire", "group": metric_group}))
        f.write("\n")
        for m in collect_metrics(metric_group):
            f.write(json.dumps(m))
            f.write("\n")


def collect_metrics(metric_group):
    ng_filter_results = list(read_filter_results("ngs"))

    def generate(name, metric_name):
        return generate_metric_with_type(
            list(read_filter_results(name)),
            ng_filter_results,
            metric_group,
            metric_name,
        )

    # DEPRECATED all nodes except CP nodes with expected taints
    for m in generate("nodes", "flant_pricing_count_nodes_by_type"):
        yield m

    # all nodes
    for m in generate("nodes_all", "flant_pricing_nodes"):
        yield m

    # CP nodes
    for m in generate("nodes_cp", "flant_pricing_controlplane_nodes"):
        yield m

    # CP nodes with expected taints
    for m in generate("nodes_t_cp", "flant_pricing_controlplane_tainted_nodes"):
        yield m


# Node types for pricing from the annotation 'pricing.flant.com/nodeType'. Lowercase versions of
# them are used as labels in metrics
PRICING_NODE_TYPE_EPHEMERAL = "Ephemeral"
PRICING_NODE_TYPE_HARD = "Hard"
PRICING_NODE_TYPE_SPECIAL = "Special"
PRICING_NODE_TYPE_VM = "VM"
PRICING_NODE_TYPE_UNKNOWN = "unknown"  # fallback from filter result


def generate_metric_with_type(
    node_filter_results,
    ng_filter_results,
    metric_group: str,
    metric_name: str,
):
    pricing_types = (
        PRICING_NODE_TYPE_EPHEMERAL,
        PRICING_NODE_TYPE_HARD,
        PRICING_NODE_TYPE_SPECIAL,
        PRICING_NODE_TYPE_VM,
    )
    count_by_type = {t: 0 for t in pricing_types}

    # Count by nodes type
    for node in node_filter_results:
        node_nodegroup_name = node.get("nodeGroup")
        # We don't bill nodes without NodeGroup
        if node_nodegroup_name is None:
            continue

        virtualization = node["virtualization"]
        pricing_type = node["pricingNodeType"]

        if pricing_type == PRICING_NODE_TYPE_UNKNOWN:
            # Deduce node type from NodeGroup if we can
            for ng in ng_filter_results:
                # Find the relevant NodeGroup snapshot
                if ng["name"] != node_nodegroup_name:
                    continue
                pricing_type = map_node_type_to_pricing_type(
                    ng["nodeType"], virtualization
                )
                break
        count_by_type[pricing_type] += 1

    # Generate metrics
    for pricing_type, count in count_by_type.items():
        # by coincidence, metric label values are lowercase pricing annotation values
        metric_type = pricing_type.lower()
        yield {
            "name": metric_name,
            "group": metric_group,
            "set": count,
            "labels": {"type": metric_type},
        }


# Node group node types
NODEGROUP_NODE_TYPE_CLOUDEPHEMERAL = "CloudEphemeral"
NODEGROUP_NODE_TYPE_CLOUDPERMANENT = "CloudPermanent"
NODEGROUP_NODE_TYPE_CLOUDSTATIC = "CloudStatic"
NODEGROUP_NODE_TYPE_STATIC = "Static"


def map_node_type_to_pricing_type(ng_node_type, virtualization):
    if ng_node_type == NODEGROUP_NODE_TYPE_CLOUDEPHEMERAL:
        return PRICING_NODE_TYPE_EPHEMERAL

    if ng_node_type in (
        NODEGROUP_NODE_TYPE_CLOUDPERMANENT,
        NODEGROUP_NODE_TYPE_CLOUDSTATIC,
    ):
        return PRICING_NODE_TYPE_VM

    if ng_node_type == NODEGROUP_NODE_TYPE_STATIC and virtualization != "unknown":
        return PRICING_NODE_TYPE_VM

    return PRICING_NODE_TYPE_HARD


def read_filter_results(name):
    for s in read_snaphots(name):
        yield s["filterResult"]


def read_snaphots(name):
    """
    Returns the list of snapshots.

    In general, there is only one snapshot, but there can be more than one.

    In generatl, the returned list contains dicts of the following structure:

        {
            "object": { "kind": ..., "metadata": ... } ,
            "filterResult": { ... }
        }

    - `object` is a JSON dump of Kubernetes object.
    - `filterResult`is a JSON result of applying `jqFilter` to the Kubernetes object.

    Keeping dumps for object fields can take a lot of memory. There is a parameter
    `keepFullObjectsInMemory: false` to disable full dumps.

    Note that disabling full objects make sense only if `jqFilter` is defined, as it disables full
    objects in snapshots field, objects field of "Synchronization" binding context and object field
    of "Event" binding context.

    See https://github.com/flant/shell-operator/blob/main/HOOKS.md
    """

    context = read_binding_context()
    return context["snapshots"][name]


def read_binding_context():
    context_path = os.getenv("BINDING_CONTEXT_PATH")
    i = os.getenv("BINDING_CONTEXT_CURRENT_INDEX")
    context = ""
    with open(context_path, "r", encoding="utf-8") as f:
        context = json.load(f)
    return context[i]


if __name__ == "__main__":
    main()
