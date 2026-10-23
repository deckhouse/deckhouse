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

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional
from dotmap import DotMap


# A policy that spans both user and system namespaces is rendered as several Gatekeeper
# constraints, because a constraint carries a single enforcementAction. The extra constraints
# are named by prefixing the policy name, so a policy that claims one of those prefixes would
# render a constraint under a name another policy already owns and break the release.

RESERVED_NAME_PREFIXES = (
    "d8-system-default-",
    # Pod Security Standards render their own constraints under this prefix, and the module
    # publishes the standards as the SecurityPolicy objects `d8-pod-security-baseline` and
    # `d8-pod-security-restricted`.
    "d8-pod-security-",
    # A denying policy that only warns about the pod-creating controllers checks them in a
    # constraint of its own under this prefix. See policy_controllers in
    # templates/policies/_helpers.tpl. Only a denying variant is split this way, and the
    # `d8-system-default-` one only warns, so the two prefixes never stack on one name.
    "d8-controllers-",
)

# What the module uses each prefix for, quoted in the denial so that the user learns why the name is
# taken: a user editing a published standard needs another answer than one naming a policy.
RESERVED_NAME_PURPOSES = {
    "d8-system-default-": "for the constraints it derives from a policy that applies to system namespaces. "
    "Choose another name.",
    "d8-pod-security-": "for the Pod Security Standards: their constraints and the SecurityPolicy objects "
    "that describe them, which cannot be changed. Configure the standards in the module settings, "
    "or choose another name for your policy.",
    "d8-controllers-": "for the constraints that check the pod-creating controllers of a policy. "
    "Choose another name.",
}

# Deckhouse applies the module's own policies, the published Pod Security Standards among them,
# so it is the one requester the reserved names stay open to.
DECKHOUSE_USERNAME = "system:serviceaccount:d8-system:deckhouse"

# A Kubernetes object name is limited to 253 characters, and a derived constraint carries the
# longest of the prefixes above on top of the policy name. Without this check the policy is
# admitted and the release breaks at the next converge instead.
MAX_NAME_LENGTH = 253 - max(len(prefix) for prefix in RESERVED_NAME_PREFIXES)


def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    # The length check is skipped as well: the module names its own policies, and none of them
    # is split into prefixed constraints.
    if ctx.review.request.userInfo.username == DECKHOUSE_USERNAME:
        return None, True

    name = ctx.review.request.object.metadata.name
    for check in (check_reserved_name, check_name_length):
        error = check(name)
        if error is not None:
            return error, False

    return None, True


def check_reserved_name(name: str) -> Optional[str]:
    for prefix in RESERVED_NAME_PREFIXES:
        if name.startswith(prefix):
            return (
                f"Name \"{name}\" starts with the reserved prefix \"{prefix}\". "
                f"The module uses this prefix {RESERVED_NAME_PURPOSES[prefix]}"
            )

    return None


def check_name_length(name: str) -> Optional[str]:
    if len(name) > MAX_NAME_LENGTH:
        return (
            f"Name \"{name}\" is {len(name)} characters long, which leaves no room for the "
            "prefix the module adds to the constraints it derives from a policy. "
            f"Use a name of at most {MAX_NAME_LENGTH} characters."
        )

    return None
