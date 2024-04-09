#!/usr/bin/env python3
#
# Copyright 2024 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
# This hook is responsible for generating metrics for node count of each type.
#

from dataclasses import dataclass
from typing import List

from shell_operator import hook
from kubernetes import utils


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

        labels = {}
        if ng["name"] in ngs_capacity:
            if "nodeTemplate" in ng:
                if ng["nodeTemplate"] != {}:
                    if "labels" in ng["nodeTemplate"]:
                        labels.update(ng["nodeTemplate"]["labels"])
                    if "taints" in ng["nodeTemplate"]:
                        for taint in ng["nodeTemplate"]["taints"]:
                            taint_labels = {}
                            if "value" in taint:
                                taint_labels.update({'taint-' + taint["key"]: taint["value"]})
                            else:
                                taint_labels.update({'taint-' + taint["key"]: ""})
                            labels.update(taint_labels)
            metric = {
                "name": "flant_pricing_node_group_cpu_cores",
                "group": metric_group,
                "set": ng["name"]["cpu"],
                "labels": labels,
            }

            ctx.metrics.expire(metric_group)
            ctx.metrics.collect(metric)
