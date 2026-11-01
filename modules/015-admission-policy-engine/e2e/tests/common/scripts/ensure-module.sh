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

# Makes sure admission-policy-engine is enabled, converged and bootstrapped before a test uses it.
#
# testing/e2e/lib/module-enabled.sh enables the module when it is not Ready and records the
# original `spec.enabled` in E2E_STATE_DIR; the deferred command of the Taskfile puts it back
# after the suite with `module-enabled.sh restore`.
#
# The module renders its constraints and webhooks only after the bootstrap hook has seen every
# ConstraintTemplate created, so the script waits for the Gatekeeper Deployments and the
# ValidatingWebhookConfiguration as well.
#
# Every wait shares one budget of 18 minutes (E2E_APE_ENSURE_TIMEOUT), so that the script fails
# with a message inside the 20m timeout of the Chainsaw step instead of being killed by it.
set -Eeuo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

module="admission-policy-engine"
budget="${E2E_APE_ENSURE_TIMEOUT:-1080}"
deadline=$(( $(date +%s) + budget ))

# remaining prints the seconds left of the budget and fails when nothing is left.
remaining() {
  local left=$(( deadline - $(date +%s) ))
  (( left > 0 )) || fail "the module did not converge within ${budget}s: $1"
  echo "${left}"
}

# The module wait takes at most 10 minutes of the budget: 7 for Ready and 3 for the queue.
E2E_MODULE_READY_TIMEOUT="${E2E_MODULE_READY_TIMEOUT:-420}" \
E2E_MODULE_QUEUE_TIMEOUT="${E2E_MODULE_QUEUE_TIMEOUT:-180}" \
  bash "${E2E_LIB_DIR:?E2E_LIB_DIR is not set, run the suite through its Taskfile}/module-enabled.sh" \
  ensure "${module}" "${E2E_STATE_DIR:-/tmp/e2e-ape-state}"

left="$(remaining "${GATEKEEPER_DEPLOY} rollout")"
kubectl -n "${APE_NS}" rollout status "deployment/${GATEKEEPER_DEPLOY}" --timeout="${left}s"
left="$(remaining "${GATEKEEPER_AUDIT_DEPLOY} rollout")"
kubectl -n "${APE_NS}" rollout status "deployment/${GATEKEEPER_AUDIT_DEPLOY}" --timeout="${left}s"

bootstrapped_now=false
until kubectl get validatingwebhookconfiguration "${APE_VWC}" >/dev/null 2>&1; do
  bootstrapped_now=true
  (( $(date +%s) < deadline )) || fail "ValidatingWebhookConfiguration ${APE_VWC} did not appear: the module is not bootstrapped"
  sleep 5
done
log "ValidatingWebhookConfiguration ${APE_VWC} is present, the module is bootstrapped"

if [[ "${bootstrapped_now}" == "true" ]]; then
  # The webhook configuration appears in the same release as the first constraints, but the next
  # converge may still be running; waiting for the queue keeps the first test off it.
  left="$(remaining "the Deckhouse queue after the bootstrap")"
  bash "${E2E_LIB_DIR}/wait-deckhouse-queue.sh" "$(( left < 600 ? left : 600 ))"
fi
