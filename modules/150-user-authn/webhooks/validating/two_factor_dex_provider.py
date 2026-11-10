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

# Refuses a DexProvider of the LDAP or Crowd type that turns Basic Authentication to the Kubernetes API on
# (enableBasicAuth) while the user-authn settings ask the users of its type for the second factor (twoFactor).
# Basic Authentication has no page for a code, so it would let the user past the second factor.
#
# webhooks/validating/two_factor_module_config.py refuses the other half of the combination: settings that ask
# for the second factor while such a DexProvider exists.
#
# templates/two-factor-dex-provider-validation-webhook.yaml embeds this file in a ValidationWebhook;
# webhook-operator renders it into a hook that calls validate(). The settings come from the snapshot of its context
# binding on the user-authn ModuleConfig, in the shape of its jqFilter.

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional
from dotmap import DotMap

# The type of a DexProvider and the field of its spec that holds the connector settings.
BASIC_AUTH_TYPES = {"LDAP": "ldap", "Crowd": "crowd"}


def conflict(binding_context: dict) -> Optional[str]:
    """Returns the message that refuses the DexProvider of the request, or None."""
    obj = binding_context["review"]["request"]["object"]
    if (obj.get("metadata") or {}).get("deletionTimestamp"):
        return None
    spec = obj.get("spec") or {}
    provider_type = spec.get("type")
    if provider_type not in BASIC_AUTH_TYPES:
        return None
    # A disabled provider is not in the Dex configuration, so it has no Basic Authentication.
    if spec.get("enabled") is False or not (spec.get(BASIC_AUTH_TYPES[provider_type]) or {}).get("enableBasicAuth"):
        return None
    for item in binding_context["snapshots"]["modulesettings"]:
        two_factor = item["filterResult"]
        if two_factor["enabled"] and provider_type in two_factor["providerTypes"]:
            return (
                f"enableBasicAuth is not possible while twoFactor asks the users of the {provider_type} providers for the "
                "second factor: Basic Authentication to the Kubernetes API cannot give it. Remove the type from "
                "twoFactor.providerTypes of the user-authn settings first"
            )
    return None


def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    message = conflict(ctx.toDict())
    return message, message is None
