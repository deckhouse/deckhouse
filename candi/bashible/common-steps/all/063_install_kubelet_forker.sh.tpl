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
  if ! /opt/deckhouse/bin/sysctl-tuner; then
    >&2 echo "d8-kubelet-forker [ERROR] sysctl-tuner exited with code $?"
    exit 1
  fi
fi

$@ &
CHILDREN_PID="$!"

attempt=0
max_attempts=120 # 2min
until ss -nltp4 | grep -qE "127.0.0.1:10248.*pid=$CHILDREN_PID" && /opt/deckhouse/bin/d8-curl --connect-timeout 10 -s -f http://127.0.0.1:10248/healthz > /dev/null; do
  attempt=$(( attempt + 1 ))

  if ! kill -0 $CHILDREN_PID 2>/dev/null; then
    >&2 echo "d8-kubelet-forker [ERROR] kubelet (PID $CHILDREN_PID) is not running"
    exit 1
  fi

  if [ "$attempt" -gt "$max_attempts" ]; then
    >&2 echo "d8-kubelet-forker [ERROR] kubelet (PID $CHILDREN_PID) /healthz did not return 200 after $max_attempts attempts, giving up"
    exit 1
  fi
  echo "d8-kubelet-forker [INFO] Waiting for /healthz on kubelet (PID $CHILDREN_PID) to return 200 (attempt $attempt of $max_attempts)"
  sleep 1
done

EOF
chmod +x /opt/deckhouse/bin/d8-kubelet-forker
