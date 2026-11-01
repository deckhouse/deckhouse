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

# Checks D8DenyExecHeritage: exec into a pod labelled `heritage: deckhouse` in a system namespace
# is denied for a cluster administrator, exec into an unlabelled pod next to it is not.
#
# The constraint and its webhook are not rendered by a Deckhouse build whose version is `dev`.
# The script then reports that explicitly and checks that neither is present.
#
# Usage: check-exec-heritage.sh <NAMESPACE>
set -Eeuo pipefail
source "../common/scripts/lib.sh"

ns="${1:?namespace is required}"

if ! webhook_present "deny-exec-heritage.admission-policy-engine.deckhouse.io"; then
  # Absence is correct only for a build whose version is `dev`; anything else is a defect.
  version="$(kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- cat /deckhouse/version 2>&1 | tr -d '[:space:]' || true)"
  [[ "${version}" == "dev" ]] \
    || fail "the deny-exec-heritage webhook is not rendered while the Deckhouse version is '${version}', not 'dev'"
  log "NOT APPLICABLE: the deny-exec-heritage webhook is not rendered, as expected for Deckhouse version 'dev'"
  if kubectl get d8denyexecheritage deny-exec-heritage-pods >/dev/null 2>&1; then
    fail "the D8DenyExecHeritage constraint exists while its webhook does not"
  fi
  exit 0
fi

bash ../common/scripts/wait-constraints.sh 300 d8denyexecheritage/deny-exec-heritage-pods

# The pod labelled heritage: deckhouse can be created only by the Deckhouse service account.
compliant_pod e2e-ape-heritage "${ns}" \
  | jq '.metadata.labels.heritage = "deckhouse"' \
  | kubectl create "${AS_DECKHOUSE[@]}" -f -
compliant_pod e2e-ape-plain "${ns}" | kubectl create -f -
kubectl -n "${ns}" wait pod/e2e-ape-heritage pod/e2e-ape-plain --for=condition=Ready --timeout=180s \
  || log "The exec targets are not Ready; admission is checked before the kubelet is called, so the checks go on"

# Gatekeeper reads the pod labels from its inventory, which is synced asynchronously.
deadline=$(( $(date +%s) + 180 ))
while true; do
  out="$(kubectl -n "${ns}" exec e2e-ape-heritage -c app -- /agnhost --help 2>&1 || true)"
  if grep -q "with label 'heritage: deckhouse' is forbidden" <<<"${out}"; then
    log "Exec into the heritage pod is denied: ${out}"
    break
  fi
  (( $(date +%s) < deadline )) || fail "exec into the heritage pod was not denied: ${out}"
  sleep 5
done

out="$(kubectl -n "${ns}" exec e2e-ape-plain -c app -- /agnhost --help 2>&1 || true)"
if grep -qE "denied the request|is forbidden" <<<"${out}"; then
  fail "exec into a pod without the heritage label was denied: ${out}"
fi
log "Exec into the unlabelled pod is not denied by admission"
