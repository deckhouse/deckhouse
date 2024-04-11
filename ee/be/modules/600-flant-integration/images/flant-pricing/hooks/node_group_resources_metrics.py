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

    for snapshot in ctx.snapshots["ngs"]:
        ng = snapshot["filterResult"]

        is_master = "false"
        is_system = "false"
        is_monitoring = "false"
        is_frontend = "false"

        taints = ng.get("nodeTemplate", {}).get("taints", [])
        if taints:
            for taint in taints:
                if taint.get("key") == "node-role.kubernetes.io/control-plane":
                    is_master = "true"
                if taint.get("key") == "node-role.kubernetes.io/master":
                    is_master = "true"
                if taint.get("key") == "dedicated.deckhouse.io" and taint.get("value") == "system":
                    is_system = "true"
                if taint.get("key") == "dedicated.deckhouse.io" and taint.get("value") == "monitoring":
                    is_monitoring = "true"
                if taint.get("key") == "dedicated.deckhouse.io" and taint.get("value") == "frontend":
                    is_frontend = "true"

        labels = {
            "is_master": is_master,
            "is_system": is_system,
            "is_monitoring": is_monitoring,
            "is_frontend": is_frontend,
        }

        ctx.metrics.collect({
            "name": "flant_pricing_node_group_cpu_cores",
            "group": metric_group,
            "set": float(ngs_capacity[ng["name"]]["cpu"]),
            "labels": labels,
        })

        ctx.metrics.collect({
            "name": "flant_pricing_node_group_memory",
            "group": metric_group,
            "set": float(ngs_capacity[ng["name"]]["memory"]),
            "labels": labels,
        })


if __name__ == "__main__":
    hook.run(main, configpath="node_group_resources_metrics.yaml")
