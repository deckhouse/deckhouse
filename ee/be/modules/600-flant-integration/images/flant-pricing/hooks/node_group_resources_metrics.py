#!/usr/bin/env python3
#
# Copyright 2024 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
# This hook is responsible for generating metrics for node group resources.
#

from kubernetes import utils

from shell_operator import hook


def main(ctx: hook.Context):
    metric_group = "group_node_group_resources_metrics"

    ngs_capacity = {}

    for snapshot in ctx.snapshots["nodes"]:
        node = snapshot["filterResult"]
        ng_name = node["node_group"]
        cpu = utils.parse_quantity(node["capacity"]["cpu"])
        ram_in_bytes = utils.parse_quantity(node["capacity"]["memory"])

        if ng_name not in ngs_capacity:
            ngs_capacity[ng_name] = {}

        if "cpu" not in ngs_capacity[node["node_group"]]:
            ngs_capacity[ng_name]["cpu"] = 0

        if "memory" not in ngs_capacity[node["node_group"]]:
            ngs_capacity[ng_name]["memory"] = 0

        ngs_capacity[ng_name]["cpu"] += cpu
        ngs_capacity[ng_name]["memory"] += ram_in_bytes

    for snapshot in ctx.snapshots["ngs"]:
        ng = snapshot["filterResult"]

        labels = {"name": ng["name"]}
        if ng["name"] in ngs_capacity:
            if "nodeTemplate" in ng:
                if "labels" in ng["nodeTemplate"]:
                    for k, v in ng["nodeTemplate"]["labels"].items():
                        labels.update({k.replace(".", "_").replace("/", "__"): v})
                if "taints" in ng["nodeTemplate"]:
                    for taint in ng["nodeTemplate"]["taints"]:
                        taint_labels = {}
                        key = 'taint_' + taint["key"].replace(".", "_").replace("/", "__")
                        if "value" in taint:
                            taint_labels.update({key: taint["value"]})
                        else:
                            taint_labels.update({key: ""})
                        labels.update(taint_labels)
            print(labels)
            ctx.metrics.expire(metric_group)
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
