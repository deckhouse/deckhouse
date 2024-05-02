#!/usr/bin/env python3
#
# Copyright 2024 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
# This hook is responsible for generating metrics for node group resources.
#

from collections import defaultdict
from kubernetes import utils

from shell_operator import hook

def main(ctx: hook.Context):
    metric_group = "group_node_group_resources_metrics"
    ctx.metrics.expire(metric_group)

    ngs_capacity = defaultdict(lambda: defaultdict(int))

    for snapshot in ctx.snapshots["nodes"]:
        node = snapshot["filterResult"]
        ng_name = node["node_group"]
        capacity_cpu = node.get("capacity", {}).get("cpu", 0)
        capacity_mem = node.get("capacity", {}).get("memory", 0)
        cpu = utils.parse_quantity(capacity_cpu)
        ram_in_bytes = utils.parse_quantity(capacity_mem)

        ngs_capacity[ng_name]["cpu"] += cpu
        ngs_capacity[ng_name]["memory"] += ram_in_bytes

    capacity = defaultdict(lambda: defaultdict(int))

    for snapshot in ctx.snapshots["ngs"]:
        ng = snapshot["filterResult"]
        label_key = ""
        for taint in ng.get("nodeTemplate", {}).get("taints", []):
            match taint.get("key"):
                case "node-role.kubernetes.io/control-plane" | "node-role.kubernetes.io/master":
                    label_key = "is_master"
                case "dedicated.deckhouse.io":
                    match taint.get("value"):
                        case "system":
                            label_key = "is_system"
                        case "monitoring":
                            label_key = "is_monitoring"
                        case "frontend":
                            label_key = "is_frontend"

        # empty label key matches worker load
        capacity[label_key]["cpu"] += float(ngs_capacity[ng["name"]]["cpu"])
        capacity[label_key]["memory"] += float(ngs_capacity[ng["name"]]["memory"])

    for key, value in capacity.items():
        labels = {
            "is_master": "false",
            "is_system": "false",
            "is_monitoring": "false",
            "is_frontend": "false",
        }

        if key != "":
            labels[key] = "true"

        ctx.metrics.collect({
            "name": "flant_pricing_node_group_cpu_cores",
            "group": metric_group,
            "set": value["cpu"],
            "labels": labels,
        })

        ctx.metrics.collect({
            "name": "flant_pricing_node_group_memory",
            "group": metric_group,
            "set": value["memory"],
            "labels": labels,
        })


if __name__ == "__main__":
    hook.run(main, configpath="node_group_resources_metrics.yaml")
