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

# Removes, where it can, every other trigger that makes the module render the `kubernetes`
# OAuth2Client (templates/dex/oauth2client.yaml), so the case proves the DexClient-only path.
# The regression guard of that path is the template test template_tests/kubernetes_oauth2client_test.go;
# this case proves it end to end only on a cluster where nothing else keeps the client rendered:
#   - a DexAuthenticator or another DexClient with the allow-access-to-kubernetes annotation set
#     to a true value (the hooks read it with strconv.ParseBool) belongs to other modules and
#     cannot be switched off: the script then logs NOT PROVEN with their names, changes nothing
#     and lets the case check the end-to-end path only;
#   - API publication (the Ingress kube-system/kubernetes-api, which the user-authn hook
#     discover_enabled_publish_api reads) is switched off through ModuleConfig
#     control-plane-manager settings.apiserver.publishAPI.ingress.enabled;
#   - user-authn settings.kubeconfigGenerator is removed.
# Changed ModuleConfigs are saved by mc-settings.sh and restored in the cleanup of the step.
# When it switched triggers off, the client must disappear within 5 minutes; otherwise the case fails.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

truthy='^(1|t|T|TRUE|true|True)$'
annotated() {
  kubectl get "$1" -A -o json \
    | jq -r --arg a "$2" --arg re "${truthy}" '.items[]
        | select((.metadata.annotations[$a] // "") | test($re))
        | "\(.metadata.namespace)/\(.metadata.name)"'
}

authenticators="$(annotated dexauthenticators.deckhouse.io dexauthenticator.deckhouse.io/allow-access-to-kubernetes)"
clients="$(annotated dexclients.deckhouse.io dexclient.deckhouse.io/allow-access-to-kubernetes)"
if [[ -n "${authenticators}${clients}" ]]; then
  log "NOT PROVEN: objects of other modules grant access to Kubernetes and keep the kubernetes OAuth2Client rendered, so this run checks the end-to-end path only and does not prove the DexClient-only path (the template test does): DexAuthenticators [${authenticators//$'\n'/ }] DexClients [${clients//$'\n'/ }]"
  exit 0
fi
log "No DexAuthenticator or DexClient grants access to Kubernetes"

# Every read below lets an API error abort the script (or, inside a wait, retry): a failed read must
# never be taken for an absent object or an unset setting, which would turn the case into a silent
# NOT PROVEN or a false pass.
ingress_absent() {
  local out
  out="$(kubectl -n kube-system get ingress kubernetes-api --ignore-not-found -o name)" || return 1
  [[ -z "${out}" ]]
}
client_absent() {
  local out
  out="$(kubectl -n "${DEX_NAMESPACE}" get oauth2clients.dex.coreos.com "${KUBERNETES_OAUTH2CLIENT}" --ignore-not-found -o name)" || return 1
  [[ -z "${out}" ]]
}

ingress="$(kubectl -n kube-system get ingress kubernetes-api --ignore-not-found -o name)"
if [[ -n "${ingress}" ]]; then
  cpm="$(kubectl get moduleconfig control-plane-manager --ignore-not-found -o json)"
  [[ -n "${cpm}" ]] || cpm='{}'
  enabled="$(jq -r '.spec.settings.apiserver.publishAPI.ingress.enabled // false' <<<"${cpm}")"
  if [[ "${enabled}" != "true" ]]; then
    log "NOT PROVEN: the Ingress kube-system/kubernetes-api exists, but ModuleConfig control-plane-manager does not set apiserver.publishAPI.ingress.enabled, so the case cannot switch it off and checks the end-to-end path only"
    exit 0
  fi
  log "The Kubernetes API is published through Ingress; switching it off for this case"
  bash ../../tests/common/scripts/mc-settings.sh apply '{"apiserver":{"publishAPI":{"ingress":{"enabled":false}}}}' control-plane-manager
  wait_for 300 "the Ingress kube-system/kubernetes-api to be deleted" ingress_absent
fi

authn="$(kubectl get moduleconfig user-authn --ignore-not-found -o json)"
[[ -n "${authn}" ]] || authn='{}'
generator="$(jq -r '.spec.settings.kubeconfigGenerator // [] | length' <<<"${authn}")"
if (( generator > 0 )); then
  log "user-authn settings.kubeconfigGenerator has ${generator} entries; removing it for this case"
  bash ../../tests/common/scripts/mc-settings.sh apply '{"kubeconfigGenerator":null}'
fi

if ! wait_for 300 "the kubernetes OAuth2Client to be absent" client_absent; then
  die "the kubernetes OAuth2Client is still rendered although no trigger is left; the case cannot prove the DexClient-only path"
fi
