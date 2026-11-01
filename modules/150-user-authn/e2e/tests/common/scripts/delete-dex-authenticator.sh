#!/usr/bin/env bash

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

# Deletes a DexAuthenticator and waits until the module has removed the objects it rendered
# for it, so that the test namespace is not deleted while Deckhouse still writes into it.
#
# Usage: delete-dex-authenticator.sh <NAMESPACE> <NAME>
set -Eeuo pipefail

ns="${1:?namespace is required}"
name="${2:?name is required}"

kubectl -n "${ns}" delete dexauthenticators.deckhouse.io "${name}" --ignore-not-found --wait=true --timeout=2m
deadline=$(( $(date +%s) + 300 ))
while (( $(date +%s) < deadline )); do
  left="$(kubectl -n "${ns}" get deployment,service,ingress -l "deckhouse.io/dex-authenticator-for=${name}" -o name 2>/dev/null || true)"
  if [[ -z "${left}" ]]; then
    echo "Objects of DexAuthenticator ${ns}/${name} are gone"
    exit 0
  fi
  sleep 5
done
>&2 echo "Objects of DexAuthenticator ${ns}/${name} are still present: ${left}"
exit 1
