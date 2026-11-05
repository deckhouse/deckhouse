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

# End-to-end checks of scoped reads of the toy aggregated API on a cluster with
# the patched kube-apiserver. Prerequisites: deploy/ applied, `d8`, `curl`,
# `jq`, and a current kubeconfig with impersonation rights (the
# deploy/50-e2e-runner.yaml service account, or cluster admin).
#
# Requests go to kube-apiserver with curl using the kubeconfig credentials
# plus `Impersonate-User`: kubectl cannot set arbitrary headers. With
# impersonation, kube-apiserver forwards the impersonated user in
# X-Remote-User, so the extension sees the test user.
#
# Never run the denied-user checks with a system:masters identity directly:
# the delegated authorizer always allows that group.
#
# Env:
#   PROJECT          a multitenancy project name for the project:<name> case (optional)
#   REVOKE_DEADLINE  seconds a scoped watch may outlive a revoked RoleBinding (default 15)
#   SKIP_PORT_FORWARD=1  skip the direct-to-Service spoofing checks
set -uo pipefail

GROUP_PATH=/apis/scopetoy.deckhouse.io/v1
NS_A=scope-toy-a NS_B=scope-toy-b
U_NS=scope-toy-ns-reader U_ALL=scope-toy-cluster-reader U_DENIED=scope-toy-denied U_WATCH=scope-toy-watcher
REVOKE_DEADLINE=${REVOKE_DEADLINE:-15}
ALL_SEEDED="default/gizmo-default kube-system/gizmo-kube-system scope-toy-a/gizmo-scope-toy-a scope-toy-b/gizmo-scope-toy-b scope-toy-c/gizmo-scope-toy-c"

K="d8 k"
WORK=$(mktemp -d /tmp/scope-toy-e2e.XXXXXX)
PF_PID="" W2="" RB_DELETED=""
cleanup() {
  [[ -n "$PF_PID" ]] && kill "$PF_PID" 2>/dev/null
  [[ -n "$W2" ]] && kill "$W2" 2>/dev/null
  # Interrupted between revoking and restoring the watcher binding: restore it,
  # or the next run fails on a missing RoleBinding.
  [[ -n "$RB_DELETED" && -s "$WORK/rb.json" ]] && $K apply -f "$WORK/rb.json" >/dev/null
  rm -rf "$WORK"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

PASS=0 FAIL=0
ok()   { PASS=$((PASS + 1)); printf 'PASS  %s\n' "$1"; }
bad()  { FAIL=$((FAIL + 1)); printf 'FAIL  %s\n      %s\n' "$1" "$2"; }
info() { printf -- '----  %s\n' "$*"; }

# --- credentials -------------------------------------------------------------

$K config view --raw --minify -o json > "$WORK/kubeconfig.json" || { echo "cannot read kubeconfig"; exit 2; }
SERVER=$(jq -r '.clusters[0].cluster.server' "$WORK/kubeconfig.json")
jq -r '.clusters[0].cluster."certificate-authority-data" // empty' "$WORK/kubeconfig.json" | base64 -d > "$WORK/ca.crt"
CURL_AUTH=()
if [[ -s "$WORK/ca.crt" ]]; then CURL_CA=(--cacert "$WORK/ca.crt"); else CURL_CA=(-k); fi
CERT=$(jq -r '.users[0].user."client-certificate-data" // empty' "$WORK/kubeconfig.json")
TOKEN=$(jq -r '.users[0].user.token // empty' "$WORK/kubeconfig.json")
if [[ -n "$CERT" ]]; then
  echo "$CERT" | base64 -d > "$WORK/admin.crt"
  jq -r '.users[0].user."client-key-data"' "$WORK/kubeconfig.json" | base64 -d > "$WORK/admin.key"
  CURL_AUTH=(--cert "$WORK/admin.crt" --key "$WORK/admin.key")
elif [[ -n "$TOKEN" ]]; then
  CURL_AUTH=(-H "Authorization: Bearer $TOKEN")
else
  echo "the kubeconfig has neither a client certificate nor a token (exec plugins are not supported here)"; exit 2
fi

# api <base-url> <path> [curl args...]: prints the body, then the HTTP code on the last line.
api() {
  local base=$1 path=$2; shift 2
  curl -sS "${CURL_CA[@]}" "${CURL_AUTH[@]}" -w '\n%{http_code}' "$@" "$base$path"
}
# kapi <user> <scope|-> <path> [curl args...]: a request through kube-apiserver as <user>.
kapi() {
  local user=$1 scope=$2 path=$3; shift 3
  local h=()
  [[ "$user" != "-" ]] && h+=(-H "Impersonate-User: $user")
  [[ "$scope" != "-" ]] && h+=(-H "X-Deckhouse-Scope: $scope")
  api "$SERVER" "$path" "${h[@]}" "$@"
}
code_of() { tail -n1 <<<"$1"; }
body_of() { sed '$d' <<<"$1"; }
names_of() { body_of "$1" | jq -r '[.items[]? | "\(.metadata.namespace)/\(.metadata.name)"] | sort | join(" ")'; }

expect_list() { # <desc> <response> <code> [names]
  local desc=$1 resp=$2 code=$3 want=${4-}
  local got_code; got_code=$(code_of "$resp")
  if [[ "$got_code" != "$code" ]]; then bad "$desc" "status $got_code, want $code: $(body_of "$resp" | head -c 300)"; return; fi
  if [[ $# -ge 4 ]]; then
    local got; got=$(names_of "$resp")
    [[ "$got" == "$want" ]] && ok "$desc" || bad "$desc" "got [$got], want [$want]"
  else
    ok "$desc"
  fi
}
expect_code() { # <desc> <response> <code>
  local got; got=$(code_of "$2")
  [[ "$got" == "$3" ]] && ok "$1" || bad "$1" "status $got, want $3: $(body_of "$2" | head -c 300)"
}

# --- pre-flight --------------------------------------------------------------

info "pre-flight"
allowed=$($K -n kube-system get cm extension-apiserver-authentication -o jsonpath='{.data.requestheader-allowed-names}')
if [[ -z "$allowed" || "$allowed" == "[]" ]]; then
  echo "requestheader-allowed-names is empty: the receiver library trusts nothing in that case, every scoped check would fail. Aborting."; exit 2
fi
info "requestheader-allowed-names: $allowed"
avail=$($K get apiservice v1.scopetoy.deckhouse.io -o jsonpath='{.status.conditions[?(@.type=="Available")].status}')
[[ "$avail" == "True" ]] || { echo "APIService v1.scopetoy.deckhouse.io is not Available ($avail)"; $K get apiservice v1.scopetoy.deckhouse.io -o yaml | tail -20; exit 2; }
ann=$($K get apiservice v1.scopetoy.deckhouse.io -o jsonpath='{.metadata.annotations.scope\.deckhouse\.io/resources}')
[[ "$ann" == *gizmos* ]] || { echo "APIService lacks scope.deckhouse.io/resources: gizmos"; exit 2; }
$K -n scope-toy rollout status deploy/scope-toy --timeout=60s >/dev/null || { echo "toy server not ready"; exit 2; }

# --- scoped lists through kube-apiserver -------------------------------------

info "lists"
expect_list "allowed user, no scope: everything" "$(kapi $U_ALL - $GROUP_PATH/gizmos)" 200 "$ALL_SEEDED"
expect_list "denied user, no scope: 403" "$(kapi $U_DENIED - $GROUP_PATH/gizmos)" 403
resp=$(kapi $U_DENIED accessible $GROUP_PATH/gizmos)
expect_list "denied user, accessible: empty list, not 403" "$resp" 200 ""
if [[ $(code_of "$resp") == 403 ]]; then
  info "hint: a plain 403 here usually means kube-apiserver ignored the opt-in (no scope resolvers: look for 'ignoring the opt-in' at V(2) in the kube-apiserver log) or the patch is not active"
fi
expect_list "namespace reader, accessible: only its namespace" "$(kapi $U_NS accessible $GROUP_PATH/gizmos)" 200 "$NS_A/gizmo-$NS_A"
expect_list "allowed user, accessible: forwarded as is" "$(kapi $U_ALL accessible $GROUP_PATH/gizmos)" 200 "$ALL_SEEDED"
expect_list "namespace reader, system: intersection is empty" "$(kapi $U_NS system $GROUP_PATH/gizmos)" 200 ""
# System namespaces are default, d8-* and kube-* (FILTERED-LISTING.md).
expect_list "allowed user, system: narrowed to system namespaces" "$(kapi $U_ALL system $GROUP_PATH/gizmos)" 200 "default/gizmo-default kube-system/gizmo-kube-system"
expect_list "namespace reader, namespaced list inside the set" "$(kapi $U_NS accessible $GROUP_PATH/namespaces/$NS_A/gizmos)" 200 "$NS_A/gizmo-$NS_A"
expect_list "namespace reader, namespaced list outside the set: empty" "$(kapi $U_NS accessible $GROUP_PATH/namespaces/$NS_B/gizmos)" 200 ""
if [[ -n "${PROJECT:-}" ]]; then
  resp=$(kapi $U_ALL "project:$PROJECT" $GROUP_PATH/gizmos)
  info "allowed user, project:$PROJECT -> $(code_of "$resp") [$(names_of "$resp")] (check against the namespaces of the project)"
fi

resp=$(kapi $U_NS accessible $GROUP_PATH/gizmos -H 'Accept: application/json;as=Table;v=v1;g=meta.k8s.io,application/json')
rows=$(body_of "$resp" | jq -r '[.rows[]?.cells[0]] | join(" ")')
[[ $(code_of "$resp") == 200 && "$rows" == "gizmo-$NS_A" ]] && ok "Table output filtered the same" || bad "Table output filtered the same" "status $(code_of "$resp") rows [$rows]"

if out=$($K get gizmos.scopetoy.deckhouse.io -A --scope=accessible --as=$U_NS --no-headers 2>&1); then
  [[ $(awk '{print $1"/"$2}' <<<"$out" | sort | xargs) == "$NS_A/gizmo-$NS_A" ]] && ok "d8 k get -A --scope=accessible --as" || bad "d8 k get -A --scope=accessible --as" "$out"
else
  bad "d8 k get -A --scope=accessible --as" "$out"
fi

# --- get ---------------------------------------------------------------------

info "get"
expect_code "namespace reader, accessible, get inside the set" "$(kapi $U_NS accessible $GROUP_PATH/namespaces/$NS_A/gizmos/gizmo-$NS_A)" 200
expect_code "namespace reader, accessible, get outside the set: 404 (kube-apiserver)" "$(kapi $U_NS accessible $GROUP_PATH/namespaces/$NS_B/gizmos/gizmo-$NS_B)" 404
expect_code "denied user, accessible, get: 404" "$(kapi $U_DENIED accessible $GROUP_PATH/namespaces/$NS_A/gizmos/gizmo-$NS_A)" 404
expect_code "allowed user, system, get in a non-system namespace: 404 (extension)" "$(kapi $U_ALL system $GROUP_PATH/namespaces/$NS_A/gizmos/gizmo-$NS_A)" 404
expect_code "allowed user, system, get in kube-system" "$(kapi $U_ALL system $GROUP_PATH/namespaces/kube-system/gizmos/gizmo-kube-system)" 200

# --- spoofed headers through kube-apiserver ----------------------------------

info "spoofing through kube-apiserver"
spoof=(-H "X-Deckhouse-Scope-Mode: filtered" -H "X-Deckhouse-Scope-Namespaces: $NS_A")
spoof_variants=(-H "x-deckhouse-scope-mode: filtered" -H "X_Deckhouse_Scope_Namespaces: $NS_A" -H "Connection: X-Deckhouse-Scope-Mode, X-Deckhouse-Scope-Namespaces")
expect_list "allowed user + spoofed headers: stripped, everything" "$(kapi $U_ALL - $GROUP_PATH/gizmos "${spoof[@]}")" 200 "$ALL_SEEDED"
expect_list "allowed user + spoofed spellings and Connection: stripped" "$(kapi $U_ALL - $GROUP_PATH/gizmos "${spoof_variants[@]}")" 200 "$ALL_SEEDED"
expect_list "denied user + spoofed headers: still 403" "$(kapi $U_DENIED - $GROUP_PATH/gizmos "${spoof[@]}")" 403
expect_list "namespace reader, accessible + spoofed wider set: kube-apiserver set wins" \
  "$(kapi $U_NS accessible $GROUP_PATH/gizmos -H "X-Deckhouse-Scope-Mode: filtered" -H "X-Deckhouse-Scope-Namespaces: $NS_A,$NS_B,scope-toy-c")" 200 "$NS_A/gizmo-$NS_A"

# --- spoofed headers sent to the Service directly ----------------------------

if [[ "${SKIP_PORT_FORWARD:-}" != 1 ]]; then
  info "spoofing directly to the Service (port-forward)"
  $K -n scope-toy port-forward svc/scope-toy 18443:443 >"$WORK/pf.log" 2>&1 &
  PF_PID=$!
  for _ in $(seq 1 50); do curl -sk -o /dev/null https://127.0.0.1:18443/ 2>/dev/null && break; sleep 0.2; done
  PROBE_TOKEN=$($K -n scope-toy create token scope-toy-probe --duration=10m)
  direct() { curl -sSk -w '\n%{http_code}' "$@" "https://127.0.0.1:18443$GROUP_PATH/gizmos"; }
  resp=$(direct -H "Authorization: Bearer $PROBE_TOKEN" "${spoof[@]}" -H "X-Remote-User: $U_ALL")
  expect_code "direct: SA token without RBAC + spoofed headers: 403, not filtered" "$resp" 403
  resp=$(direct -H "Authorization: Bearer $PROBE_TOKEN" "${spoof_variants[@]}")
  expect_code "direct: SA token + spoofed spellings: 403" "$resp" 403
  if [[ -n "$CERT" ]]; then
    resp=$(direct --cert "$WORK/admin.crt" --key "$WORK/admin.key" "${spoof[@]}")
    expect_list "direct: admin client certificate + spoofed headers: ignored, everything" "$resp" 200 "$ALL_SEEDED"
  elif [[ -n "$TOKEN" ]]; then
    resp=$(direct -H "Authorization: Bearer $TOKEN" "${spoof[@]}")
    expect_list "direct: admin token + spoofed headers: ignored, everything" "$resp" 200 "$ALL_SEEDED"
  fi
  kill "$PF_PID" 2>/dev/null; PF_PID=""
fi

# --- watch -------------------------------------------------------------------

info "watch filtered per event"
rv=$(body_of "$(kapi - - $GROUP_PATH/gizmos)" | jq -r .metadata.resourceVersion)
curl -sSN "${CURL_CA[@]}" "${CURL_AUTH[@]}" -H "Impersonate-User: $U_NS" -H "X-Deckhouse-Scope: accessible" \
  "$SERVER$GROUP_PATH/gizmos?watch=1&resourceVersion=$rv&timeoutSeconds=20" >"$WORK/watch1.json" 2>"$WORK/watch1.err" &
W1=$!
sleep 2
for ns in $NS_B $NS_A; do
  kapi - - $GROUP_PATH/namespaces/$ns/gizmos -X POST -H 'Content-Type: application/json' \
    -d "{\"apiVersion\":\"scopetoy.deckhouse.io/v1\",\"kind\":\"Gizmo\",\"metadata\":{\"name\":\"e2e-$ns\"}}" >/dev/null
done
sleep 2
kill $W1 2>/dev/null; wait $W1 2>/dev/null
events=$(jq -r '"\(.type) \(.object.metadata.namespace)/\(.object.metadata.name)"' "$WORK/watch1.json" 2>/dev/null | xargs)
[[ "$events" == "ADDED $NS_A/e2e-$NS_A" ]] && ok "scoped watch sees only events of the set" || bad "scoped watch sees only events of the set" "events [$events] $(cat "$WORK/watch1.err")"
for ns in $NS_B $NS_A; do kapi - - $GROUP_PATH/namespaces/$ns/gizmos/e2e-$ns -X DELETE >/dev/null; done

info "watch revocation"
curl -sSN "${CURL_CA[@]}" "${CURL_AUTH[@]}" -H "Impersonate-User: $U_WATCH" -H "X-Deckhouse-Scope: accessible" \
  "$SERVER$GROUP_PATH/gizmos?watch=1&timeoutSeconds=120" >"$WORK/watch2.json" 2>"$WORK/watch2.err" &
W2=$!
sleep 3
if ! kill -0 $W2 2>/dev/null; then
  bad "watch revocation" "the watch ended before the revocation: $(cat "$WORK/watch2.err" "$WORK/watch2.json" | head -c 300)"
else
  $K -n $NS_A get rolebinding scope-toy:watcher -o json | jq 'del(.metadata.resourceVersion,.metadata.uid,.metadata.creationTimestamp,.metadata.managedFields)' >"$WORK/rb.json"
  start=$(date +%s)
  RB_DELETED=1
  $K -n $NS_A delete rolebinding scope-toy:watcher >/dev/null
  ended=""
  for _ in $(seq 1 $((REVOKE_DEADLINE * 5))); do
    kill -0 $W2 2>/dev/null || { ended=$(( $(date +%s) - start )); break; }
    sleep 0.2
  done
  if [[ -n "$ended" ]]; then ok "watch ended ${ended}s after the RoleBinding was revoked"; else bad "watch revocation" "still open after ${REVOKE_DEADLINE}s"; kill $W2 2>/dev/null; fi
  $K apply -f "$WORK/rb.json" >/dev/null && RB_DELETED=""
fi
wait $W2 2>/dev/null
W2=""

metric=$(api "$SERVER" /metrics 2>/dev/null | grep '^apiserver_scopefilter_proxied_watch_closed_total' || true)
info "kube-apiserver (the instance that answered) metrics: ${metric:-none}"

# --- summary -----------------------------------------------------------------

echo
echo "passed: $PASS, failed: $FAIL"
info "toy server log tail:"
$K -n scope-toy logs deploy/scope-toy --since=5m 2>/dev/null | grep -E '"(read|authorize)"|scope:' | tail -n 15
[[ $FAIL -eq 0 ]]
