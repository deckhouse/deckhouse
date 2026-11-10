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

# Refuses a DexProvider whose groupPrefix can produce the same group name as the groupPrefix of another enabled
# DexProvider or as a group prefix of a TokenExchangeProvider: an equal prefix, or one that the other starts with.
# Dex does not prefix a group that already starts with the prefix of its provider, so two such providers could
# assert each other's groups, and a rule written for a group of one would apply to the users of the other.
#
# webhooks/validating/group_prefix_token_exchange.py refuses the other half: a TokenExchangeProvider whose group prefix
# meets the groupPrefix of a DexProvider.
#
# templates/group-prefix-dex-provider-validation-webhook.yaml embeds this file in a ValidationWebhook;
# webhook-operator renders it into a hook that calls validate(). The other providers come from the snapshots of its
# context bindings, in the shape of their jqFilters.

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional
from dotmap import DotMap


def overlap(a: str, b: str) -> bool:
    """Whether two prefixes can produce the same name: equal ones, or one that the other starts with."""
    return a.startswith(b) or b.startswith(a)


def conflict(binding_context: dict) -> Optional[str]:
    """Returns the message that refuses the DexProvider of the request, or None."""
    obj = binding_context["review"]["request"]["object"]
    metadata = obj.get("metadata") or {}
    if metadata.get("deletionTimestamp"):
        return None
    spec = obj.get("spec") or {}
    prefix = spec.get("groupPrefix")
    # A disabled provider is not in the Dex configuration, so its prefix is not applied. The check runs again when
    # it is enabled.
    if not prefix or spec.get("enabled") is False:
        return None

    name = metadata.get("name")
    for item in binding_context["snapshots"]["dexproviders"]:
        other = item["filterResult"]
        if other["name"] == name or not other["enabled"] or not other.get("groupPrefix"):
            continue
        if overlap(prefix, other["groupPrefix"]):
            return (
                f"groupPrefix {prefix!r} can name the same groups as groupPrefix {other['groupPrefix']!r} of the "
                f"DexProvider {other['name']}: the prefixes must differ and neither may start with the other"
            )
    for item in binding_context["snapshots"]["tokenexchangeproviders"]:
        other = item["filterResult"]
        for other_prefix in other["groupPrefixes"]:
            if overlap(prefix, other_prefix):
                return (
                    f"groupPrefix {prefix!r} can name the same groups as the group prefix {other_prefix!r} of the "
                    f"TokenExchangeProvider {other['name']}: the prefixes must differ and neither may start with the other"
                )
    return None


def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    message = conflict(ctx.toDict())
    return message, message is None
