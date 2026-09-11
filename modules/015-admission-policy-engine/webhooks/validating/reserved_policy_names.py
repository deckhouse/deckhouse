#!/usr/bin/python3

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

# A policy that spans both user and system namespaces is rendered as several Gatekeeper
# constraints, because a constraint carries a single enforcementAction. The extra constraints
# are named by prefixing the policy name, so a policy that claims one of those prefixes would
# render a constraint under a name another policy already owns and break the release.

from typing import Optional

from deckhouse import hook
from dotmap import DotMap

RESERVED_NAME_PREFIXES = (
    "d8-system-warn-",
    "d8-system-enforce-",
    # Pod Security Standards render their own constraints under this prefix.
    "d8-pod-security-",
)

config = """
configVersion: v1
kubernetesValidating:
- name: reserved-policy-names.deckhouse.io
  group: main
  rules:
  - apiGroups:   ["deckhouse.io"]
    apiVersions: ["*"]
    operations:  ["CREATE", "UPDATE"]
    resources:   ["securitypolicies", "operationpolicies"]
    scope:       "Cluster"
"""


def main(ctx: hook.Context):
    try:
        binding_context = DotMap(ctx.binding_context)
        validate(binding_context, ctx.output.validations)
    except Exception as e:
        ctx.output.validations.error(str(e))


def validate(ctx: DotMap, output: hook.ValidationsCollector):
    error = check_reserved_name(ctx.review.request.object.metadata.name)
    if error is not None:
        output.deny(error)
        return

    output.allow()


def check_reserved_name(name: str) -> Optional[str]:
    for prefix in RESERVED_NAME_PREFIXES:
        if name.startswith(prefix):
            return (
                f"Name \"{name}\" starts with the reserved prefix \"{prefix}\". "
                "The module uses this prefix for the constraints it derives from a policy "
                "that applies to system namespaces. Choose another name."
            )

    return None


if __name__ == "__main__":
    hook.run(main, config=config)
