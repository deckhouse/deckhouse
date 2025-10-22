#!/usr/bin/python3

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

INSTANCE_CLASS_NAME = os.getenv("INSTANCE_CLASS_NAME", "instanceclasses")

config = f"""
configVersion: v1
kubernetesValidating:
- name: instanceclass-in-nodegroups.deckhouse.io
  group: main
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["DELETE"]
    resources:   ["{INSTANCE_CLASS_NAME}"]
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
    if ctx.review.request.operation == "DELETE":
        return validate_delete(ctx)
    else:
        raise Exception(f"Unknown operation {ctx.review.request.operation}")

def validate_delete(ctx: DotMap) -> tuple[Optional[str], list[str]]:
    class_to_delete = ctx.review.request.name
    node_group_consumers = ctx.review.request.oldObject.status.nodeGroupConsumers
    resource_kind = ctx.review.request.kind.kind

    if node_group_consumers:
        node_groups = ", ".join(node_group_consumers)
        error_message = (f"{resource_kind}/{class_to_delete} cannot be deleted "
                         f"because it is being used by NodeGroup: {node_groups}")
        return error_message, []

    return None, []

if __name__ == "__main__":
    hook.run(main, config=config)
