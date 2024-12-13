#!/usr/bin/env python3

# Copyright 2024 Flant JSC
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

from typing import Optional
from deckhouse import hook
from dotmap import DotMap
import json

prefix = "propagated_"

config = """
configVersion: v1
kubernetesValidating:
- name: clusterobservabilitypropagateddashboards-policy.deckhouse.io
  group: main
  rules:
  - apiGroups:   ["observability.deckhouse.io"]
    apiVersions: ["v1alpha1"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["clusterobservabilitypropagateddashboards"]
    scope:       "Cluster"
kubernetes:
- name: clusterobservabilitypropagateddashboards
  group: main
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  apiVersion: observability.deckhouse.io/v1alpha1
  kind: ClusterObservabilityPropagatedDashboard
  jqFilter: |
    {
      "name": .metadata.name,
      "definition": .spec.definition
    }
"""


def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        errmsg, warnings = validate(binding_context)
        if errmsg is None:
            ctx.output.validations.allow(*warnings)
        else:
            print("test")
            ctx.output.validations.deny(f"test")
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap) -> tuple[Optional[str], list[str]]:
    operation = ctx.review.request.operation
    if operation == "CREATE" or operation == "UPDATE":
        return validate_creation_or_update(ctx)
    elif operation == "DELETE":
        return validate_delete(ctx)
    else:
        raise Exception(f"Unknown operation {ctx.operation}")

def validate_creation_or_update(ctx: DotMap) -> tuple[Optional[str], list[str]]:
    warnings = []

    name = ctx.review.request.object.metadata.name
    definition = json.loads(ctx.review.request.object.spec.definition)
    uid = definition["uid"]
    if not uid.startswith(prefix):
        return "not allowed", warnings

    return None, warnings


def validate_delete(ctx: DotMap) -> tuple[Optional[str], list[str]]:
    warnings = []
    return None, warnings


if __name__ == "__main__":
    hook.run(main, config=config)
