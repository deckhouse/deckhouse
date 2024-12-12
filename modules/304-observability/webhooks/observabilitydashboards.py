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

from deckhouse import hook
from dotmap import DotMap
import json

prefixes = ["propagated_", "cluster_"]

config = """
configVersion: v1
kubernetesValidating:
- name: observabilitydashboards-policy.deckhouse.io
  group: main
  rules:
  - apiGroups:   ["observability.deckhouse.io"]
    apiVersions: ["v1alpha1"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["observabilitydashboards"]
    scope:       "Namespaced"
kubernetes:
- name: observabilitydashboards
  group: main
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  apiVersion: observability.deckhouse.io/v1alpha1
  kind: ObservabilityDashboard
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
        validate(binding_context, ctx.output.validations)
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap, output: hook.ValidationsCollector):
    operation = ctx.review.request.operation
    if operation == "CREATE" or operation == "UPDATE":
        validate_creation_or_update(ctx, output)
    elif operation == "DELETE":
        validate_delete(ctx, output)
    else:
        raise Exception(f"Unknown operation {ctx.operation}")


def validate_creation_or_update(ctx: DotMap, output: hook.ValidationsCollector):
    definition = json.loads(ctx.review.request.object.spec.definition)

    uid = definition["uid"]
    for prefix in prefixes:
        if uid.startswith(prefix):
            output.deny("\".spec.definition\" must not contain uid with \"{prefix}\" prefix.")
            return

    output.allow()


def validate_delete(ctx: DotMap, output: hook.ValidationsCollector):
    output.allow()


if __name__ == "__main__":
    hook.run(main, config=config)
