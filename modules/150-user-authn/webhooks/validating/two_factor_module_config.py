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

# Refuses the settings of the user-authn ModuleConfig that turn the second factor (twoFactor) on for a type of
# provider while a DexProvider of that type has Basic Authentication to the Kubernetes API (enableBasicAuth).
# Basic Authentication has no page for a code, so it would let the user past the second factor.
#
# webhooks/validating/two_factor_dex_provider.py refuses the other half of the combination: a DexProvider that
# turns Basic Authentication on under settings that already ask for the second factor.
#
# templates/two-factor-module-config-validation-webhook.yaml embeds this file in a ValidationWebhook; webhook-operator
# renders it into a hook that calls validate(). The DexProviders come from the snapshot of its context binding.

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional
from dotmap import DotMap

MODULE = "user-authn"
# The second factor covers the providers of these types, which have a Basic Authentication.
BASIC_AUTH_TYPES = ("LDAP", "Crowd")


def settings_of(obj: dict) -> dict:
    """The twoFactor settings of a ModuleConfig as the module gets them. Deckhouse converts the settings of a version
    before 4 (openapi/conversions/v4.yaml) and puts staticUsers2FA in place of twoFactor, so a ModuleConfig of version 3
    with providerTypes written into staticUsers2FA covers the providers of those types. A ModuleConfig without a version
    is converted too."""
    spec = obj.get("spec") or {}
    settings = spec.get("settings") or {}
    if (spec.get("version") or 0) < 4 and "staticUsers2FA" in settings:
        return settings.get("staticUsers2FA") or {}
    return settings.get("twoFactor") or {}


def conflicting_providers(binding_context: dict) -> tuple[list, list]:
    """Returns the provider types that twoFactor of the request covers and have a Basic Authentication, and the names of
    the DexProviders whose Basic Authentication the settings would break."""
    obj = binding_context["review"]["request"]["object"]
    if (obj.get("metadata") or {}).get("name") != MODULE or (obj.get("metadata") or {}).get("deletionTimestamp"):
        return [], []
    two_factor = settings_of(obj)
    if not two_factor.get("enabled"):
        return [], []
    types = two_factor.get("providerTypes") or ["Local"]
    covered = [t for t in BASIC_AUTH_TYPES if t in types]
    if not covered:
        return [], []
    return covered, sorted(
        item["filterResult"]["name"]
        for item in binding_context["snapshots"]["dexproviders"]
        if item["filterResult"]["type"] in covered
        and item["filterResult"]["enabled"]
        and item["filterResult"]["basicAuth"]
    )


def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    covered, names = conflicting_providers(ctx.toDict())
    if not names:
        return None, True
    return (
        f"twoFactor asks the users of the {' and '.join(covered)} providers for the second factor, which Basic Authentication to the "
        "Kubernetes API cannot give: turn enableBasicAuth off in the DexProvider objects "
        + ", ".join(f"'{name}'" for name in names)
        + " or remove their type from twoFactor.providerTypes"
    ), False
