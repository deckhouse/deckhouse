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

# Checks what e2e-authn-login-user may do with its ID token.
# Usage: rbac.sh denied|allowed
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

expect="${1:?denied or allowed}"
# A fresh token for every check, so the ID token cannot expire in between.
bash ../../tests/common/scripts/get-token.sh static-user-login "${NS}" e2e-authn-login-client \
  e2e-authn-login-user e2e-authn-login-user@example.com kubernetes
kubeconfig="$(state_dir static-user-login)/rbac.kubeconfig"
token_kubeconfig "$(jq -r .id_token "$(state_dir static-user-login)/e2e-authn-login-user.tokens.json")" "${kubeconfig}"
k() { kubectl --kubeconfig "${kubeconfig}" "$@"; }

case "${expect}" in
  denied)
    answer="$(k auth can-i list pods -n "${NS}" 2>/dev/null || true)"
    [[ "${answer}" == "no" ]] || die "without a binding, can-i list pods answered '${answer}', expected 'no'"
    if out="$(k get pods -n "${NS}" 2>&1)"; then
      die "without a binding, listing pods succeeded"
    fi
    grep -q 'pods is forbidden: User "e2e-authn-login-user@example.com" cannot list resource "pods"' <<<"${out}" \
      || die "unexpected error: ${out}"
    log "Listing pods is forbidden: ${out}"
    ;;
  allowed)
    deadline=$(( $(date +%s) + 120 ))
    until [[ "$(k auth can-i list pods -n "${NS}" 2>/dev/null || true)" == "yes" ]]; do
      (( $(date +%s) < deadline )) || die "with the ClusterRoleBinding to the group, can-i list pods is still not 'yes'"
      sleep 5
    done
    k get pods -n "${NS}" >/dev/null || die "with the ClusterRoleBinding to the group, listing pods failed"
    k get pods -n kube-system >/dev/null || die "the ClusterRoleBinding must allow listing pods in every namespace"
    answer="$(k auth can-i delete pods -n "${NS}" 2>/dev/null || true)"
    [[ "${answer}" == "no" ]] || die "ClusterRole view must not allow deleting pods, can-i answered '${answer}'"
    log "The group binding allows listing pods and still forbids deleting them"
    ;;
  *)
    die "unknown expectation ${expect}"
    ;;
esac
