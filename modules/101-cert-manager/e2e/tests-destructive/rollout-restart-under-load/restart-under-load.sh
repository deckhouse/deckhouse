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


# Restarts the cert-manager controller, webhook and cainjector Deployments while
# Certificates are created and deleted every 2 seconds, and fails when any
# kubectl call of the load generator failed (a webhook call error, an x509 error,
# a refused connection) or when a Certificate does not become Ready afterwards.
#
# Usage: restart-under-load.sh <NAMESPACE>
#
# The inner waits add up to about 13 minutes at most (20s warm-up, three restarts with a
# 3 minute rollout timeout and a 15s pause each, 30s after the last restart, 2 minutes for
# readiness), below the 15 minute timeout of the Chainsaw step.
# The load generator stops by itself when this script is gone or after 14 minutes, and
# writes its PID to /tmp/e2e-cm-load-generator.pid, so the step cleanup can stop it after a
# SIGKILL of this script, which skips the EXIT trap.
set -Eeuo pipefail

namespace="${1:?namespace is required}"
work="$(mktemp -d)"
errors="${work}/errors.log"
: > "${errors}"
load_pid=""
pid_file=/tmp/e2e-cm-load-generator.pid

cleanup() {
  if [[ -n "${load_pid}" ]]; then
    kill "${load_pid}" 2>/dev/null || true
    wait "${load_pid}" 2>/dev/null || true
  fi
  rm -f "${pid_file}"
  rm -rf "${work}"
}
trap cleanup EXIT

create_certificate() {
  kubectl -n "${namespace}" create -f - <<CERT
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: e2e-cm-load-$1
  labels:
    e2e-cm-load: "true"
spec:
  secretName: e2e-cm-load-$1-tls
  dnsNames:
    - load-$1.e2e-cm.example.test
  issuerRef:
    group: cert-manager.io
    kind: ClusterIssuer
    name: selfsigned
CERT
}

# Keeps the last few Certificates and deletes older ones, so creation, update by the
# controller and deletion all happen during the restart.
load() {
  local i=0 parent="$$" deadline=$(( $(date +%s) + 840 ))
  while kill -0 "${parent}" 2>/dev/null && (( $(date +%s) < deadline )); do
    i=$(( i + 1 ))
    if ! out="$(create_certificate "${i}" 2>&1)"; then
      echo "$(date -u +%T) create e2e-cm-load-${i}: ${out}" >> "${errors}"
    fi
    if (( i > 5 )); then
      if ! out="$(kubectl -n "${namespace}" delete certificate "e2e-cm-load-$(( i - 5 ))" --wait=false 2>&1)"; then
        echo "$(date -u +%T) delete e2e-cm-load-$(( i - 5 )): ${out}" >> "${errors}"
      fi
    fi
    sleep 2
  done
}

# Detached from the step output, so an orphaned generator cannot hold the pipes of Chainsaw.
load </dev/null >/dev/null 2>&1 &
load_pid=$!
echo "${load_pid}" > "${pid_file}"
echo "Load generator started"
sleep 20

for deployment in cert-manager webhook cainjector; do
  echo "$(date -u +%T) Restarting deployment/${deployment}"
  kubectl -n d8-cert-manager rollout restart "deployment/${deployment}"
  kubectl -n d8-cert-manager rollout status "deployment/${deployment}" --timeout=3m
  echo "$(date -u +%T) deployment/${deployment} rolled out"
  sleep 15
done

# Keep the load running after the last restart: a webhook endpoint that is removed late
# shows up as errors only after the rollout reports success.
sleep 30
kill "${load_pid}"
wait "${load_pid}" 2>/dev/null || true
load_pid=""
rm -f "${pid_file}"
echo "Load generator stopped"

failed=0
if [[ -s "${errors}" ]]; then
  >&2 echo "kubectl calls failed during the restart:"
  >&2 cat "${errors}"
  failed=1
else
  echo "No kubectl call failed during the restart"
fi

# The suite namespace is shared, so only the Certificates of the load generator are checked.
total="$(kubectl -n "${namespace}" get certificate -l e2e-cm-load=true -o name | wc -l)"
echo "Waiting for ${total} remaining load Certificates to become Ready"
if ! kubectl -n "${namespace}" wait certificate -l e2e-cm-load=true --for=condition=Ready --timeout=2m; then
  >&2 kubectl -n "${namespace}" get certificate -l e2e-cm-load=true
  failed=1
fi

exit "${failed}"
