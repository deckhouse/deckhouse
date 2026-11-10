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

# Warns about the allowedProviders of a DexClient that name no DexProvider. It never refuses a DexClient: a GitOps tool
# may apply the client before its provider, and a provider may be removed later, which the Dex client survives (nobody
# signs in through a name without a connector).
#
# templates/dex-client-validation-webhook.yaml embeds this file in a ValidationWebhook; webhook-operator renders it
# into a hook that calls validate(). The DexProviders come from the snapshot of its context binding.

# The operator template imports these as well; they make the file importable by the tests.
from typing import Optional
from dotmap import DotMap

# The connector of the built-in password database; the other connectors are named after their DexProvider.
LOCAL_PROVIDER = "local"


def unknown_providers_warning(binding_context: dict) -> Optional[str]:
    """Returns a warning that names the allowedProviders no DexProvider has, or None."""
    obj = binding_context["review"]["request"]["object"]
    names = (obj.get("spec") or {}).get("allowedProviders") or []
    if not names or (obj.get("metadata") or {}).get("deletionTimestamp"):
        return None
    known = {LOCAL_PROVIDER} | {(item.get("filterResult") or {}).get("name")
                                for item in binding_context["snapshots"]["dexproviders"]}
    unknown = [name for name in names if name not in known]
    if not unknown:
        return None
    return ("allowedProviders names no DexProvider: " + ", ".join(f"'{name}'" for name in unknown)
            + "; nobody can sign in through them until the provider is created")


def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    return unknown_providers_warning(ctx.toDict()), True
