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

# Refuses a TokenExchangeProvider with a group prefix that can produce the same group name as the groupPrefix of an
# enabled DexProvider: an equal prefix, or one that the other starts with. Without it a provider of tokens could name
# a group like a directory behind a DexProvider and take the rights written for that group.
#
# webhooks/validating/group_prefix_dex_provider.py refuses the other half: a DexProvider whose groupPrefix meets a group
# prefix of a TokenExchangeProvider or the groupPrefix of another DexProvider. The conflicts of two
# TokenExchangeProviders are the business of the user-authn-controller.
#
# templates/group-prefix-token-exchange-validation-webhook.yaml embeds this file in a ValidationWebhook;
# webhook-operator renders it into a hook that calls validate(). The DexProviders come from the snapshot of its
# context binding, in the shape of its jqFilter.

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional
from dotmap import DotMap


def overlap(a: str, b: str) -> bool:
    """Whether two prefixes can produce the same name: equal ones, or one that the other starts with."""
    return a.startswith(b) or b.startswith(a)


def conflict(binding_context: dict) -> Optional[str]:
    """Returns the message that refuses the TokenExchangeProvider of the request, or None."""
    obj = binding_context["review"]["request"]["object"]
    if (obj.get("metadata") or {}).get("deletionTimestamp"):
        return None
    groups = ((obj.get("spec") or {}).get("identity") or {}).get("groups") or []
    prefixes = [g["prefix"] for g in groups if g.get("prefix")]

    for item in binding_context["snapshots"]["dexproviders"]:
        other = item["filterResult"]
        if not other["enabled"] or not other.get("groupPrefix"):
            continue
        for prefix in prefixes:
            if overlap(prefix, other["groupPrefix"]):
                return (
                    f"the group prefix {prefix!r} can name the same groups as groupPrefix {other['groupPrefix']!r} of the "
                    f"DexProvider {other['name']}: the prefixes must differ and neither may start with the other"
                )
    return None


def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    message = conflict(ctx.toDict())
    return message, message is None
