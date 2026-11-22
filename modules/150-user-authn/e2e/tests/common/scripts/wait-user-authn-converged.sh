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

# Waits until the user-authn module has converged: the Deckhouse queue is empty, the Module is
# Ready and the Deployments of Dex and of the controller have finished their rollout.
# With E2E_SKIP_ROLLOUT_WAIT=1 the rollouts are not awaited; the caller checks the replicas itself
# (a rollout to more replicas than the cluster can place would block here without a reason).
#
# Usage: wait-user-authn-converged.sh
set -Eeuo pipefail

script_dir="$(dirname "${BASH_SOURCE[0]}")"

# Give Deckhouse time to notice a ModuleConfig change before the queue is read.
sleep 10
bash "${E2E_LIB_DIR}/wait-deckhouse-queue.sh"
kubectl wait module/user-authn --for=jsonpath='{.status.phase}'=Ready --timeout=15m
if [[ "${E2E_SKIP_ROLLOUT_WAIT:-}" != "1" ]]; then
  kubectl -n d8-user-authn rollout status deployment/dex --timeout=10m
  kubectl -n d8-user-authn rollout status deployment/user-authn-controller --timeout=10m
fi
