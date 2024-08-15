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

mkdir -p cilium-junits

d8 cilium status --wait

d8 cilium connectivity test \
--include-conn-disrupt-test \
--conn-disrupt-test-setup \
--conn-disrupt-test-restarts-path "./cilium-conn-disrupt-restarts" \
--conn-disrupt-dispatch-interval 0ms

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
--sysdump-hubble-flows-count=1000000 \
--sysdump-hubble-flows-timeout=5m \
--sysdump-output-filename "cilium-sysdump-common-e2e-<ts>" \
--junit-file "cilium-junits/common-e2e.xml" \
--junit-property github_job_step="Run tests common-e2e" \
--request-timeout 30s
