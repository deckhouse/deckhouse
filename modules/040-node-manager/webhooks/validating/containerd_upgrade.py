#!/usr/bin/env python3

# Copyright 2025 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os
from typing import Optional
from deckhouse import hook
from dotmap import DotMap

config = """
configVersion: v1
kubernetes:
  - name: d8-nodes-with-containerd-custom-conf
    apiVersion: v1
    kind: nodes
    executeHookOnEvent: []
    executeHookOnSynchronization: false
    keepFullObjectsInMemory: false
    labelSelector:
      matchLabels:
        node.deckhouse.io/containerd: custom-config
    jqFilter: |
      {
        "nodeName": .metadata.name,
        "nodeGroup": '.metadata.labels."node.deckhouse.io/group" // "unknown"'
      }
kubernetesValidating:
- name: containerd-allow-upgrade.deckhouse.io
  group: main
  includeSnapshotsFrom: ["d8-nodes-with-containerd-custom-conf"]
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["UPDATE"]
    resources:   ["nodegroups"]
    scope:       "Cluster"
"""

def main(ctx: hook.Context):
    try:
        binding_context = DotMap(ctx.binding_context)
        errmsg, warnings = validate(binding_context)
        if errmsg is None:
            ctx.output.validations.allow(*warnings)
        else:
            ctx.output.validations.deny(errmsg)
    except Exception as e:
        ctx.output.validations.error(str(e))

def validate(ctx: DotMap) -> tuple[Optional[str], list[str]]:
    if ctx.review.request.operation == "UPDATE":
        return validate_update(ctx)
    else:
        raise Exception(f"Unknown operation {ctx.review.request.operation}")

def validate_update(ctx: DotMap) -> tuple[Optional[str], list[str]]:
    print(ctx.review.request.object.spec.cri)
    if ctx.review.request.object.spec.cri.type == "ContainerdV2":
      nodeGroupNameWithChangedCRI = ctx.review.object.metadata.name
      for i in ctx.snapshots.get("d8-nodes-with-containerd-custom-conf",[]):
          node = i["filterResult"]
          nodeName = node.get('nodeName', '')
          nodeGroupName = node.get('nodeGroup', '')

          if nodeGroupName == nodeGroupNameWithChangedCRI:
              nodesWithCustomConf = ", ".join(nodeName)
          
      if nodesWithCustomConf:
          errorMessage = (f"CRI cannot be changed because next nodes are using custom configuration: {nodesWithCustomConf}")
          return errorMessage, []

    return None, []

if __name__ == "__main__":
    hook.run(main, config=config)
