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


# Scrapes the cert-manager controller metrics through the kube-rbac-proxy sidecar
# (https://<POD_IP>:9404/metrics) of every controller pod with a ServiceAccount token,
# and succeeds when one pod exposes every expected line. Only the leader runs the
# certificates controller, so in HA mode the other pods lack per-certificate metrics.
#
# Usage: scrape-metrics.sh <SA_NAMESPACE> <SA_NAME> <TIMEOUT_SECONDS> <EXTENDED_REGEX>...
set -Eeuo pipefail

sa_namespace="${1:?}"; sa_name="${2:?}"; timeout="${3:?}"; shift 3
patterns=("$@")

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT
body="${work}/body"
token="$(kubectl -n "${sa_namespace}" create token "${sa_name}" --duration=10m)"
[[ -n "${token}" ]] || { >&2 echo "kubectl create token returned an empty token"; exit 1; }
# The token goes to curl through a config file, not the command line.
printf 'header = "Authorization: Bearer %s"\n' "${token}" > "${work}/curlrc"
unset token
deadline=$(( $(date +%s) + timeout ))

while (( $(date +%s) < deadline )); do
  for ip in $(kubectl -n d8-cert-manager get pod -l app=cert-manager \
      -o jsonpath='{range .items[?(@.status.podIP)]}{.status.podIP}{"\n"}{end}'); do
    [[ "${ip}" == *:* ]] && host="[${ip}]" || host="${ip}"
    code="$(curl -sk -o "${body}" -w '%{http_code}' --max-time 15 \
      -K "${work}/curlrc" "https://${host}:9404/metrics" || true)"
    if [[ "${code}" != "200" ]]; then
      echo "${ip}: HTTP ${code}"
      continue
    fi
    missing=0
    for pattern in "${patterns[@]}"; do
      if ! grep -Eq -- "${pattern}" "${body}"; then
        echo "${ip}: no line matches ${pattern}"
        missing=1
      fi
    done
    if (( missing == 0 )); then
      echo "${ip}: all expected metrics found"
      for pattern in "${patterns[@]}"; do grep -E -- "${pattern}" "${body}"; done
      exit 0
    fi
  done
  sleep 10
done

>&2 echo "No cert-manager pod exposed the expected metrics within ${timeout}s"
exit 1
