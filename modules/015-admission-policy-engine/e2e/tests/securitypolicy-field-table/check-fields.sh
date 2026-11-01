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

# Runs the field table of the SecurityPolicy e2e-ape-sp-fields: for every field one pod that
# breaks only that field must be denied with the message of the matching constraint template.
# The rules themselves are covered by the gator tests of the constraint templates; this table
# proves that each field reaches Gatekeeper as a constraint routed by the webhook.
set -Eeuo pipefail

probe="../common/scripts/probe.sh"
ns="e2e-ape-sp-fields"
failed=()
# The constraints are already enforced when the table starts. Only the first probe gets time for
# the webhook to load them; every other row is a short retry, so the whole table stays well within
# the step timeout and the summary below is always printed.
row_timeout=120

# name # jq patch of the compliant pod # expected message (ERE); jq filters contain `|`, hence `#`
table=(
  "privileged#.spec.containers[0].securityContext |= (.privileged = true | del(.allowPrivilegeEscalation))#Privileged container is not allowed"
  "privilege-escalation#.spec.containers[0].securityContext.allowPrivilegeEscalation = true#Privilege escalation container is not allowed"
  "host-network#.spec.hostNetwork = true#The hostNetwork or hostPort are not allowed"
  "host-pid#.spec.hostPID = true#Sharing the hostPID namespace is not allowed"
  "host-ipc#.spec.hostIPC = true#Sharing the hostIPC namespace is not allowed"
  "host-path#.spec.volumes = [{name: \"host\", hostPath: {path: \"/etc\"}}] | .spec.containers[0].volumeMounts = [{name: \"host\", mountPath: \"/host\", readOnly: true}]#HostPath volume is not allowed"
  "capability#.spec.containers[0].securityContext.capabilities.add = [\"SYS_ADMIN\"]#container has a disallowed capability"
  "required-drop#.spec.containers[0].securityContext.capabilities.drop = [\"NET_RAW\"]#container is not dropping all required capabilities"
  "run-as-user#.spec.securityContext.runAsUser = 500#is attempting to run as disallowed user"
  "seccomp#.spec.securityContext.seccompProfile = {type: \"Unconfined\"}#Seccomp profile is not allowed"
  "writable-root#.spec.containers[0].securityContext.readOnlyRootFilesystem = false#only read-only root filesystem container is allowed"
  "automount-token#.spec.automountServiceAccountToken = true#Automounting service account token is disallowed"
)

for row in "${table[@]}"; do
  IFS='#' read -r name patch message <<<"${row}"
  echo "=== ${name}: expect deny with '${message}'"
  if ! bash "${probe}" --expect deny --namespace "${ns}" --name "bad-${name}" --timeout "${row_timeout}" \
      --patch "${patch}" --match "\\[e2e-ape-sp-fields\\]" --match "${message}"; then
    failed+=("${name}")
  fi
  row_timeout=30
done

echo "=== allowed host path: a read-only mount under the allowed prefix is admitted"
if ! bash "${probe}" --expect allow --namespace "${ns}" --name good-host-path --timeout 30 \
    --patch '.spec.volumes = [{name: "host", hostPath: {path: "/var/e2e-ape/data"}}] | .spec.containers[0].volumeMounts = [{name: "host", mountPath: "/data", readOnly: true}]'; then
  failed+=("good-host-path")
fi

echo "=== allowed capability: NET_BIND_SERVICE is admitted"
if ! bash "${probe}" --expect allow --namespace "${ns}" --name good-capability --timeout 30 \
    --patch '.spec.containers[0].securityContext.capabilities.add = ["NET_BIND_SERVICE"]'; then
  failed+=("good-capability")
fi

echo "=== compliant pod is admitted"
if ! bash "${probe}" --expect allow --namespace "${ns}" --name good-compliant --timeout 30; then
  failed+=("good-compliant")
fi

if (( ${#failed[@]} > 0 )); then
  echo "Failed rows: ${failed[*]}"
  exit 1
fi
echo "All rows of the field table hold"
