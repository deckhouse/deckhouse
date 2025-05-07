#!/usr/bin/python3
from typing import Optional

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
        "references": [.spec.policies.verifyImageSignatures[]?.reference]
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
    error = check_verify_image_signatures(ctx)
    if error is not None:
        output.deny(error)
        return

    output.allow()


def validate_delete(ctx: DotMap, output: hook.ValidationsCollector):
    return


# check that all image references don't have intersection, it's required by ratify
# https://ratify.dev/docs/plugins/verifier/cosign/#scopes
def check_verify_image_signatures(ctx: DotMap) -> Optional[str]:
    references = [item.reference for item in ctx.review.request.object.spec.policies.verifyImageSignatures]
    if len(references) == 0:
        return None

    existing_references = [obj.filterResult for obj in ctx.snapshots.policies if obj.filterResult.references is not None]

    for exobj in existing_references:
        # On update skip self intersection
        if exobj.name == ctx.review.request.object.metadata.name:
            continue
        for exref in exobj.references:
            for ref in references:
                ref_clean = ref.replace("*",'').strip()
                exref_clean = exref.replace("*",'').strip()
                if ref_clean == "" or exref_clean == "":
                    # Skip `*` references, they are treat as default
                    continue
                min_length = min(len(ref_clean), len(exref_clean))
                # Check intersection but ignore fully equal references
                if ref_clean[:min_length] == exref_clean[:min_length] and ref_clean != exref_clean:
                    return f"ImageReference \"{ref}\" has intersection in the SecurityPolicy \"{exobj.name}\""

    return None


if __name__ == "__main__":
    hook.run(main, config=config)
