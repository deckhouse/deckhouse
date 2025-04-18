#!/bin/bash

# Copyright 2025 Flant JSC
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

namespace="d8-openvpn"
statefulset="openvpn"
secrets=(
  openvpn-pki-ca
  openvpn-pki-crl
  openvpn-pki-index-txt
  openvpn-pki-server
)

echo "🔍 [1/3] Checking OpenVPN StatefulSet readiness..."

for i in {1..20}; do
  sleep 30
  replicas=$(kubectl -n "$namespace" get statefulset "$statefulset" -o jsonpath='{.spec.replicas}' || echo "0")
  ready=$(kubectl -n "$namespace" get statefulset "$statefulset" -o jsonpath='{.status.readyReplicas}' || echo "0")
  echo "[$i/20] StatefulSet ready: $ready/$replicas"

  if [[ "$replicas" == "$ready" && "$replicas" -gt 0 ]]; then
    echo "✅ StatefulSet is fully ready."
    break
  fi

  if [[ "$i" == 20 ]]; then
    echo "❌ StatefulSet did not become ready in time."
    exit 1
  fi
done

echo ""
echo "🔍 [2/3] Checking OpenVPN container logs for common errors..."

log=$(kubectl -n "$namespace" logs "$statefulset-0" -c openvpn-tcp 2>&1 || true)
if echo "$log" | grep -Eiq "(error|fail|denied|not permitted|operation not permitted)"; then
  echo "❌ Detected suspicious entries in OpenVPN logs:"
  echo "$log" | grep -Ei "(error|fail|denied|not permitted|operation not permitted)"
  exit 1
else
  echo "✅ No suspicious log entries found."
fi

echo ""
echo "🔍 [3/3] Checking required OpenVPN Secrets..."

missing=()
for secret in "${secrets[@]}"; do
  if ! kubectl -n "$namespace" get secret "$secret" >/dev/null 2>&1; then
    echo "❌ Missing secret: $secret"
    missing+=("$secret")
  else
    echo "✅ Secret exists: $secret"
  fi
done

if [[ ${#missing[@]} -ne 0 ]]; then
  echo "❌ One or more required secrets are missing: ${missing[*]}"
  exit 1
fi

echo ""
echo "🎉 All OpenVPN readiness checks passed successfully."

