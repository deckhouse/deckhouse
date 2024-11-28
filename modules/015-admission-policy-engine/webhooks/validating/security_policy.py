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

config = """
configVersion: v1
kubernetes:
  - name: policies
    apiVersion: deckhouse.io/v1alpha1
    kind: SecurityPolicy
    queue: "securitypolicies"
    group: main
    executeHookOnEvent: []
    executeHookOnSynchronization: false
    keepFullObjectsInMemory: false
    jqFilter: |
      {
        "name": .metadata.name,
        "imageReferences": .spec.policies.verifyImageSignatures.imageReferences
      }
kubernetesValidating:
- name: securitypolicies.deckhouse.io
  group: main
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["securitypolicies"]
    scope:       "Cluster"
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
    references = ctx.review.request.object.spec.policies.verifyImageSignatures.imageReferences
    if references is None:
        output.allow()
        return

    print("New references:", references)
    existing_references = [obj.filterResult for obj in ctx.snapshots.policies if obj.filterResult.imageReferences is not None]
    print("Existing references:", existing_references)

    for exobj in existing_references:
        for exref in exobj.imageReferences:
            for ref in references:
                ref_clean = ref.replace("*",'').strip()
                exref_clean = exref.replace("*",'').strip()
                print("Check1: ", ref_clean)
                print("Check2: ", exref_clean)
                min_length = min(len(ref_clean), len(exref_clean))
                if ref_clean[:min_length] == exref_clean[:min_length]:
                    output.deny(f"ImageReference \"{ref}\" has intersection in the policy \"{exobj.name}\"")
                    return


    # output.deny(f"users.deckhouse.io \"{user_name}\", user \"{user_with_the_same_email[0].name}\" is already using email \"{email}\"")
    output.allow()


def validate_delete(ctx: DotMap, output: hook.ValidationsCollector):
    return


if __name__ == "__main__":
    hook.run(main, config=config)
