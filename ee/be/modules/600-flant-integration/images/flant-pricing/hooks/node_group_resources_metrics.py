#!/usr/bin/env python3
#
# Copyright 2024 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
# This hook is responsible for generating metrics for node count of each type.
#

from dataclasses import dataclass
from typing import List

from shell_operator import hook

def main(ctx: hook.Context):

    for s in ctx.snapshots["nodes"]:
        filtered = s["filterResult"]
        if filtered is None:
            # should not happen
            continue


    for s in ctx.snapshots["ngs"]:
        filtered = s["filterResult"]
        if filtered is None:
            # should not happen
            continue

        name, node_type = filtered["name"], filtered["node_type"]
        by_name[name] = NodeGroup(name=name, node_type=node_type)
