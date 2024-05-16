# Copyright 2021 Flant JSC
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

bb-sync-file /opt/deckhouse/bin/d8-kubelet-forker - << "EOF"
#!/bin/bash
set -e

# Start sysctl-tuner to set appropriate values to system variables before kubelet start
if [ -x /opt/deckhouse/bin/sysctl-tuner ]; then
  /opt/deckhouse/bin/sysctl-tuner
fi

# Hack, which is necessary for correct start of kubelet service. Since for the enabled fencing feature the value 0 is required, which can be set with sysctl-tuner
sysctl -w kernel.panic=10

$@ &
CHILDREN_PID="$!"

attempt=0
max_attempts=120 # 2min
until ss -nltp4 | grep -qE "127.0.0.1:10248.*pid=$CHILDREN_PID" && d8-curl -s -f http://127.0.0.1:10248/healthz > /dev/null; do
  attempt=$(( attempt + 1 ))

  if ! kill -0 $CHILDREN_PID 2>/dev/null; then
    >&2 echo "d8-kubelet-forker [ERROR] kubelet with PID $CHILDREN_PID is not running."
    exit 1
  fi

  if [ "$attempt" -gt "$max_attempts" ]; then
    >&2 echo "d8-kubelet-forker [ERROR] Could not reach /healthz HTTP-endpoint of kubelet with PID $CHILDREN_PID after $max_attempts attempts. Exiting."
    exit 1
  fi
  echo "d8-kubelet-forker [INFO] Waiting for HTTP 200 response from /healthz endpoing of kubelet with PID $CHILDREN_PID (attempt $attempt of $max_attempts)..."
  sleep 1
done

{{- if eq (dig "fencing" "mode" "" .nodeGroup) "Watchdog" }}
  sysctl -w kernel.panic=0
{{- end }}

EOF
chmod +x /opt/deckhouse/bin/d8-kubelet-forker
