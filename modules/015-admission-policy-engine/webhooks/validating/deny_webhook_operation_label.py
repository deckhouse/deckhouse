#!/usr/bin/python3
from typing import Optional

# Copyright 2026 Flant JSC
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

LABEL_KEY = "gatekeeper.sh/operation"
LABEL_VALUE = "webhook"

config = """
configVersion: v1
kubernetesValidating:
- name: deny-webhook-operation-label.deckhouse.io
  group: main
  rules:
  - apiGroups:   [""]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["pods"]
    scope:       "Namespaced"
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
        return
    else:
        raise Exception(f"Unknown operation {ctx.operation}")


def validate_creation_or_update(ctx: DotMap, output: hook.ValidationsCollector):
    error = check_forbidden_label(ctx)
    if error is not None:
        output.deny(error)
        return

    output.allow()


def check_forbidden_label(ctx: DotMap) -> Optional[str]:
    # Make it similar to `heritage` protection policy:
    # allow only Deckhouse service accounts (d8-* namespaces).
    username = ctx.review.request.userInfo.username
    if isinstance(username, str) and username.startswith("system:serviceaccount:d8-"):
        return None

    obj = ctx.review.request.object
    labels = getattr(obj.metadata, "labels", None)
    if labels is None:
        return None

    value = labels.get(LABEL_KEY) if hasattr(labels, "get") else None
    if value != LABEL_VALUE:
        return None

    namespace = obj.metadata.namespace
    name = obj.metadata.name

    return (
        f"Creating/updating a Pod with the `{LABEL_KEY}: {LABEL_VALUE}` label is forbidden for user {username}. "
        f"Only Deckhouse service accounts are allowed."
    )


if __name__ == "__main__":
    hook.run(main, config=config)
