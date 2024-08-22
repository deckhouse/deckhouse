# Copyright 2024 Flant JSC
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
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail

#
# We restart all cilium pods order to clear any errors in logs that may have occurred during the initial setup.
#
kubectl -n d8-cni-cilium annotate pod -l app=agent safe-agent-updater-daemonset-generation- && kubectl -n d8-cni-cilium rollout restart ds safe-agent-updater
cilium_daemonset_generation=$(kubectl -n d8-cni-cilium get ds agent -o 'jsonpath={..metadata.annotations.safe-agent-updater-daemonset-generation}') 2>/dev/null
>&2 echo "Current Cilium daemonset generation is "$cilium_daemonset_generation
cilium_daemonset_count_desired=$(kubectl -n d8-cni-cilium get ds agent -o 'jsonpath={..status.desiredNumberScheduled}') 2>/dev/null
>&2 echo "The number of desired Cilium Pods is "$cilium_daemonset_count_desired

sleep 30

testRunAttempts=$cilium_daemonset_count_desired
for ((i=1; i<=$testRunAttempts; i++)); do
  >&2 echo "Check Cilium Pods readiness..."
  cilium_daemonset_count_ready=$((kubectl -n d8-cni-cilium get pods -l app=agent -o=jsonpath='{range .items[*]}{..metadata.annotations.safe-agent-updater-daemonset-generation}{..status.conditions[?(@.type=="Ready")].status}{..status.phase}{"\n"}{end}' | grep $cilium_daemonset_generation"TrueRunning" || true) | wc -l) 2>/dev/null
  >&2 echo "The number of ready Cilium Pods is "$cilium_daemonset_count_ready" / "$cilium_daemonset_count_desired
  if [[ "$cilium_daemonset_count_desired" == "$cilium_daemonset_count_ready" ]]; then
    test_failed=""
    >&2 echo "All Cilium Pods are ready."
    break
  fi

  if [[ $i < $testRunAttempts ]]; then
    >&2 echo -n "  Cilium Pods not ready. Attempt $i/$testRunAttempts failed. Sleep for 30 seconds..."
    sleep 30
  else
    test_failed="true"
    >&2 echo -n "  Cilium Pods not ready. Attempt $i/$testRunAttempts failed."
  fi

done

if [[ $test_failed == "true" ]] ; then
  exit 1
fi

#
# Start cilium tests
#
mkdir -p cilium-junits
#
d8 cilium status --wait

#
# We are launching test pods that establish "long-lived" connections with each other
#
d8 cilium connectivity test \
--include-conn-disrupt-test \
--conn-disrupt-test-setup \
--conn-disrupt-test-restarts-path "./cilium-conn-disrupt-restarts" \
--conn-disrupt-dispatch-interval 0ms
#
# We are running a set of common tests.
#
d8 cilium connectivity test \
--include-unsafe-tests \
--collect-sysdump-on-failure \
--sysdump-quick \
--include-conn-disrupt-test \
--conn-disrupt-test-restarts-path "./cilium-conn-disrupt-restarts" \
--flush-ct \
--expected-drop-reasons="+No egress gateway found" \
--expected-drop-reasons="+CT: Unknown L4 protocol" \
--expected-drop-reasons="+No mapping for NAT masquerade" \
--expected-drop-reasons="+Unsupported L2 protocol" \
--sysdump-hubble-flows-count=1000000 \
--sysdump-hubble-flows-timeout=5m \
--sysdump-output-filename "cilium-sysdump-common-e2e-<ts>" \
--junit-file "cilium-junits/common-e2e.xml" \
--junit-property github_job_step="Run tests common-e2e" \
--request-timeout 30s \
--external-target one.one.one.one.
