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

# Shared helpers of the user-authn e2e suite. Source the file, do not execute it:
#
#   source ../../tests/common/scripts/lib.sh
#
# The helpers never print passwords, password hashes or tokens. Secrets that a later
# step needs are kept in files under E2E_STATE_DIR, which only root can read.

E2E_STATE_DIR="${E2E_STATE_DIR:-/tmp/e2e-authn-state}"
DEX_NAMESPACE="d8-user-authn"
# Name of the OAuth2Client object of the privileged `kubernetes` client (ToFnvLikeDex("kubernetes")).
KUBERNETES_OAUTH2CLIENT="nn2wezlsnzsxizltzpzjzzeeeirsk"
# Redirect URI registered in the DexClient objects of the suite. Nothing listens there:
# the helpers read the authorization code from the Location header and never follow it.
TEST_REDIRECT_URI="http://127.0.0.1:65535/callback"
# federated:id makes Dex put federated_claims.connector_id into the ID token; user-api requires it.
TEST_SCOPE="openid email groups profile offline_access federated:id"
KUBERNETES_AUDIENCE_SCOPE="audience:server:client_id:kubernetes"

umask 077
mkdir -p "${E2E_STATE_DIR}/tmp"

# tmp_file prints a new file under E2E_STATE_DIR, which restore-modules.sh removes after the suite.
tmp_file() {
  mktemp -p "${E2E_STATE_DIR}/tmp"
}

# secret_file <VALUE> stores a secret in a temporary file and prints its path, so that the value
# is passed to curl as `name@file` and never appears on a command line.
secret_file() {
  local file
  file="$(tmp_file)"
  printf '%s' "$1" > "${file}"
  printf '%s' "${file}"
}

die() {
  >&2 echo "FAIL: $*"
  exit 1
}

log() {
  echo ">>> $*"
}

# state_dir <TEST_NAME> prints the per-test directory for secrets and intermediate results.
state_dir() {
  local dir="${E2E_STATE_DIR}/${1:?test name is required}"
  mkdir -p "${dir}"
  printf '%s' "${dir}"
}

# wait_for <TIMEOUT_SECONDS> <DESCRIPTION> <COMMAND...> retries the command every 5 seconds.
wait_for() {
  local timeout="$1" description="$2"
  shift 2
  local deadline=$(( $(date +%s) + timeout ))
  until "$@" >/dev/null 2>&1; do
    if (( $(date +%s) >= deadline )); then
      >&2 echo "Timed out after ${timeout}s waiting for: ${description}"
      return 1
    fi
    sleep 5
  done
  log "Done: ${description}"
}

urlencode() {
  jq -rn --arg v "$1" '$v | @uri'
}

urldecode() {
  local s="${1//+/ }"
  printf '%b' "${s//%/\\x}"
}

# query_param <URL> <NAME> prints the decoded value of a query parameter.
query_param() {
  local query="${1#*\?}" kv
  [[ "${query}" == "$1" ]] && return 1
  local IFS='&'
  for kv in ${query}; do
    if [[ "${kv%%=*}" == "$2" ]]; then
      urldecode "${kv#*=}"
      return 0
    fi
  done
  return 1
}

# html_text <FILE> prints the visible text of an HTML page on one line, for diagnostics and matching.
html_text() {
  sed -e 's/<[^>]*>/ /g' "$1" | tr -s ' \t\r\n' ' ' | head -c 2000
}

# gen_password prints a random password that satisfies every predefined complexity level of
# passwordPolicy (upper and lower case, a digit, a special character, no character three times in a row).
gen_password() {
  local p
  while :; do
    p="Ea7!$(openssl rand -hex 12)Zq"
    grep -Eq '(.)\1\1' <<<"${p}" || break
  done
  printf '%s' "${p}"
}

# hash_password <PASSWORD> prints a raw bcrypt hash ($2...) of the password.
# The hash is computed locally by `d8 iam user create --dry-run` (the same code the d8 CLI uses to
# create users), or by glibc crypt() through perl as a fallback.
hash_password() {
  local password="$1" out=""
  if command -v d8 >/dev/null 2>&1; then
    out="$(printf '%s' "${password}" \
      | d8 iam user create e2e-authn-hash --email e2e-authn-hash@example.com --password-stdin --dry-run -o json 2>/dev/null \
      | jq -r '.spec.password // empty' 2>/dev/null || true)"
    if [[ -n "${out}" && "${out}" != '$2'* ]]; then
      out="$(printf '%s' "${out}" | base64 -d 2>/dev/null || true)"
    fi
    if [[ "${out}" == '$2'* ]]; then
      printf '%s' "${out}"
      return 0
    fi
  fi
  local salt
  salt="$(openssl rand -base64 48 | tr -dc 'A-Za-z0-9./' | head -c 21)e"
  out="$(PW="${password}" SALT="${salt}" perl -e 'print crypt($ENV{PW}, "\$2b\$10\$" . $ENV{SALT})' 2>/dev/null || true)"
  if [[ "${out}" == '$2b$10$'* && ${#out} -eq 60 ]]; then
    printf '%s' "${out}"
    return 0
  fi
  die "cannot compute a bcrypt hash: neither 'd8 iam user create --dry-run' nor perl crypt() with bcrypt support is available"
}

# create_user <TEST_NAME> <USER_NAME> <EMAIL> [TTL] creates a User with a random password and keeps
# the password in <state dir>/<USER_NAME>.password.
create_user() {
  local test="$1" name="$2" email="$3" ttl="${4:-}"
  local dir password hash
  dir="$(state_dir "${test}")"
  password="$(gen_password)"
  hash="$(hash_password "${password}")"
  printf '%s' "${password}" > "${dir}/${name}.password"
  # The hash goes to jq through stdin, not through argv.
  printf '%s' "${hash}" | jq -Rs --arg name "${name}" --arg email "${email}" --arg ttl "${ttl}" '. as $hash | {
      apiVersion: "deckhouse.io/v1",
      kind: "User",
      metadata: {name: $name, labels: {"e2e.user-authn.deckhouse.io/suite": "true"}},
      spec: ({email: $email, password: $hash} + (if $ttl == "" then {} else {ttl: $ttl} end))
    }' | kubectl create -f - >/dev/null
  log "Created User ${name} (${email})"
}

# user_password <TEST_NAME> <USER_NAME> prints the password stored by create_user.
user_password() {
  cat "$(state_dir "$1")/$2.password"
}

# dex_discover sets DEX_ISSUER (without the trailing slash), DEX_HOST and DEX_RESOLVE. The issuer is
# the public Dex URL that the module hands to the API server; requests go to the ClusterIP of the dex
# Service with the issuer host name, so no DNS record or Ingress is needed.
dex_discover() {
  DEX_ISSUER="$(kubectl -n "${DEX_NAMESPACE}" get configmap control-plane-configurator -o jsonpath='{.data.oidcIssuerURL}')"
  [[ -n "${DEX_ISSUER}" ]] || die "ConfigMap ${DEX_NAMESPACE}/control-plane-configurator has no oidcIssuerURL"
  DEX_ISSUER="${DEX_ISSUER%/}"
  DEX_HOST="${DEX_ISSUER#https://}"
  DEX_HOST="${DEX_HOST%%/*}"
  local ip
  ip="$(kubectl -n "${DEX_NAMESPACE}" get service dex -o jsonpath='{.spec.clusterIP}')"
  [[ -n "${ip}" ]] || die "Service ${DEX_NAMESPACE}/dex has no ClusterIP"
  [[ "${ip}" == *:* ]] && ip="[${ip}]"
  DEX_RESOLVE="${DEX_HOST}:443:${ip}"
}

dex_curl() {
  curl -sS -k --max-time 30 --resolve "${DEX_RESOLVE}" "$@"
}

# dex_authorize <EMAIL> <PASSWORD> <CLIENT_ID> <REDIRECT_URI> <SCOPE> [CURL_ARGS...]
# Runs the authorization code flow of the local connector the way a browser does: it opens
# /auth/local (bypassing the connector choice page), follows the redirect to the login form and
# posts the credentials. It never follows the redirect to the client. Results:
#   AUTH_STATUS    HTTP code of the credentials POST (or of the first request when no form was reached);
#   AUTH_LOCATION  absolute redirect URL of that response, empty when there is none;
#   AUTH_CODE      authorization code when Dex redirected to REDIRECT_URI with a code;
#   AUTH_ERROR     OAuth2 error when Dex redirected to REDIRECT_URI with an error;
#   AUTH_BODY      file with the response body; AUTH_JAR - the cookie jar.
dex_authorize() {
  local email="$1" password="$2" client_id="$3" redirect_uri="$4" scope="$5"
  shift 5
  AUTH_JAR="$(tmp_file)"
  AUTH_BODY="$(tmp_file)"
  AUTH_CODE=""
  AUTH_ERROR=""
  AUTH_LOCATION=""
  AUTH_STATUS=""
  local state out login_url
  state="$(openssl rand -hex 8)"
  local url="${DEX_ISSUER}/auth/local?client_id=$(urlencode "${client_id}")&redirect_uri=$(urlencode "${redirect_uri}")&response_type=code&scope=$(urlencode "${scope}")&state=${state}"
  out="$(dex_curl "$@" -b "${AUTH_JAR}" -c "${AUTH_JAR}" -o "${AUTH_BODY}" -w '%{http_code} %{redirect_url}' "${url}")" || return 1
  AUTH_STATUS="${out%% *}"
  login_url="${out#* }"
  if [[ "${login_url}" != *"/auth/local/login"* ]]; then
    AUTH_LOCATION="${login_url}"
    auth_parse_redirect "${redirect_uri}" "${state}"
    return 0
  fi
  out="$(dex_curl "$@" -b "${AUTH_JAR}" -c "${AUTH_JAR}" -o "${AUTH_BODY}" -w '%{http_code} %{redirect_url}' \
    --data-urlencode "login=${email}" --data-urlencode "password@$(secret_file "${password}")" "${login_url}")" || return 1
  AUTH_STATUS="${out%% *}"
  AUTH_LOCATION="${out#* }"
  auth_parse_redirect "${redirect_uri}" "${state}"
}

auth_parse_redirect() {
  local redirect_uri="$1" state="$2"
  [[ -n "${AUTH_LOCATION}" && "${AUTH_LOCATION}" == "${redirect_uri}"* ]] || return 0
  if [[ "$(query_param "${AUTH_LOCATION}" state || true)" != "${state}" ]]; then
    AUTH_ERROR="state mismatch"
    return 0
  fi
  AUTH_CODE="$(query_param "${AUTH_LOCATION}" code || true)"
  AUTH_ERROR="$(query_param "${AUTH_LOCATION}" error || true)"
}

# auth_describe prints the outcome of the last dex_authorize call without the authorization code.
auth_describe() {
  local location="${AUTH_LOCATION%%\?*}"
  echo "HTTP ${AUTH_STATUS}, redirect to '${location}', OAuth2 error '${AUTH_ERROR}', page: $(html_text "${AUTH_BODY}" | head -c 300)"
}

# dex_login_code <EMAIL> <PASSWORD> <CLIENT_ID> <REDIRECT_URI> <SCOPE> retries dex_authorize until
# Dex issues an authorization code (Dex restarts after a configuration change), for at most
# DEX_LOGIN_TIMEOUT seconds (300 by default). The code is left in AUTH_CODE.
dex_login_code() {
  local deadline=$(( $(date +%s) + ${DEX_LOGIN_TIMEOUT:-300} ))
  while :; do
    if dex_authorize "$@" && [[ -n "${AUTH_CODE}" ]]; then
      return 0
    fi
    if (( $(date +%s) >= deadline )); then
      >&2 echo "Dex did not issue an authorization code: $(auth_describe)"
      return 1
    fi
    sleep 5
  done
}

# dex_token <CURL_ARGS...> posts to the token endpoint. Sets TOKEN_STATUS and TOKEN_BODY (file).
dex_token() {
  TOKEN_BODY="$(tmp_file)"
  TOKEN_STATUS="$(dex_curl -o "${TOKEN_BODY}" -w '%{http_code}' "$@" "${DEX_ISSUER}/token")"
}

# dex_exchange_code <CODE> <CLIENT_ID> <CLIENT_SECRET> [REDIRECT_URI] exchanges an authorization code
# and fails unless Dex answers 200 with an ID token.
dex_exchange_code() {
  dex_token --data-urlencode "grant_type=authorization_code" --data-urlencode "code=$1" \
    --data-urlencode "redirect_uri=${4:-${TEST_REDIRECT_URI}}" \
    --data-urlencode "client_id=$2" --data-urlencode "client_secret@$(secret_file "$3")"
  if [[ "${TOKEN_STATUS}" != "200" ]] || ! jq -e '.id_token' "${TOKEN_BODY}" >/dev/null 2>&1; then
    >&2 echo "Token endpoint answered HTTP ${TOKEN_STATUS}: $(jq -c '{error, error_description}' "${TOKEN_BODY}" 2>/dev/null || echo "not JSON")"
    return 1
  fi
}

# dex_refresh <REFRESH_TOKEN> <CLIENT_ID> <CLIENT_SECRET> sets TOKEN_STATUS and TOKEN_BODY.
dex_refresh() {
  dex_token --data-urlencode "grant_type=refresh_token" --data-urlencode "refresh_token@$(secret_file "$1")" \
    --data-urlencode "client_id=$2" --data-urlencode "client_secret@$(secret_file "$3")"
}

# jwt_claims <JWT> prints the decoded payload of a JWT.
jwt_claims() {
  local payload
  payload="$(cut -d. -f2 <<<"$1" | tr '_-' '/+')"
  while (( ${#payload} % 4 )); do payload+="="; done
  base64 -d <<<"${payload}"
}

# dex_client_secret <NAMESPACE> <DEXCLIENT_NAME> prints the client secret the module generated.
dex_client_secret() {
  kubectl -n "$1" get secret "dex-client-$2" -o jsonpath='{.data.clientSecret}' | base64 -d
}

dex_client_id() {
  printf 'dex-client-%s@%s' "$2" "$1"
}

oauth2client_exists() {
  kubectl -n "${DEX_NAMESPACE}" get oauth2clients.dex.coreos.com -o json \
    | jq -e --arg id "$1" 'any(.items[]; .id == $id)' >/dev/null
}

kubernetes_client_trusts() {
  kubectl -n "${DEX_NAMESPACE}" get oauth2clients.dex.coreos.com "${KUBERNETES_OAUTH2CLIENT}" -o json \
    | jq -e --arg id "$1" '(.trustedPeers // []) | index($id) != null' >/dev/null
}

# wait_dex_client <NAMESPACE> <DEXCLIENT_NAME> waits for the Secret and the OAuth2Client of a DexClient.
wait_dex_client() {
  local id
  id="$(dex_client_id "$1" "$2")"
  wait_for 300 "Secret $1/dex-client-$2" kubectl -n "$1" get secret "dex-client-$2"
  wait_for 300 "OAuth2Client ${id}" oauth2client_exists "${id}"
}

# token_kubeconfig <TOKEN> <FILE> writes a kubeconfig that authenticates with the token only,
# against the API server of the current kubeconfig.
token_kubeconfig() {
  local token="$1" file="$2" server ca ca_file tls
  server="$(kubectl config view --raw --minify -o jsonpath='{.clusters[0].cluster.server}')"
  ca="$(kubectl config view --raw --minify -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')"
  ca_file="$(kubectl config view --raw --minify -o jsonpath='{.clusters[0].cluster.certificate-authority}')"
  if [[ -z "${ca}" && -n "${ca_file}" && -r "${ca_file}" ]]; then
    ca="$(base64 -w0 < "${ca_file}")"
  fi
  if [[ -n "${ca}" ]]; then
    tls="certificate-authority-data: ${ca}"
  else
    tls="insecure-skip-tls-verify: true"
  fi
  cat > "${file}" <<KUBECONFIG
apiVersion: v1
kind: Config
clusters:
- name: e2e
  cluster:
    server: ${server}
    ${tls}
users:
- name: e2e
  user:
    token: ${token}
contexts:
- name: e2e
  context:
    cluster: e2e
    user: e2e
current-context: e2e
KUBECONFIG
}

# public_domain <NAME> renders the global publicDomainTemplate for the name.
public_domain() {
  local template
  template="$(kubectl get moduleconfig global -o jsonpath='{.spec.settings.modules.publicDomainTemplate}')"
  [[ "${template}" == *"%s"* ]] || die "ModuleConfig global has no publicDomainTemplate with %s"
  printf '%s' "${template//%s/$1}"
}

cluster_domain() {
  local domain
  domain="$(kubectl -n kube-system get secret d8-cluster-configuration -o json 2>/dev/null \
    | jq -r '.data."cluster-configuration.yaml" // empty' | base64 -d 2>/dev/null \
    | awk '/^clusterDomain:/ {gsub(/"/, "", $2); print $2}')"
  printf '%s' "${domain:-cluster.local}"
}

# ingress_discover finds an IngressNginxController and a Ready controller pod to send requests to.
# Sets ING_CLASS, ING_IP and ING_CURL_ARGS. Fails with an explicit message when there is none.
ingress_discover() {
  local controllers name inlet pods
  controllers="$(kubectl get ingressnginxcontrollers.deckhouse.io -o json 2>/dev/null || true)"
  [[ -n "${controllers}" ]] && [[ "$(jq '.items | length' <<<"${controllers}")" -gt 0 ]] \
    || die "the cluster has no IngressNginxController: this case needs the ingress-nginx module with at least one controller (see README, prerequisites)"
  name="$(jq -r '.items[0].metadata.name' <<<"${controllers}")"
  ING_CLASS="$(jq -r '.items[0].spec.ingressClass // "nginx"' <<<"${controllers}")"
  inlet="$(jq -r '.items[0].spec.inlet // ""' <<<"${controllers}")"
  pods="$(kubectl -n d8-ingress-nginx get pods -o json)"
  ING_IP="$(jq -r --arg n "${name}" '
      [.items[]
       | select((.metadata.labels.app == "controller" and .metadata.labels.name == $n) or (.metadata.name | startswith("controller-" + $n + "-")))
       | select(any(.status.conditions[]?; .type == "Ready" and .status == "True"))
       | .status.podIP] | first // empty' <<<"${pods}")"
  [[ -n "${ING_IP}" ]] || die "IngressNginxController ${name} has no Ready controller pod in d8-ingress-nginx"
  [[ "${ING_IP}" == *:* ]] && ING_IP="[${ING_IP}]"
  ING_CURL_ARGS=()
  if [[ "${inlet}" == *ProxyProtocol* ]]; then
    ING_CURL_ARGS+=(--haproxy-protocol)
  fi
  log "Ingress controller ${name}: class ${ING_CLASS}, inlet ${inlet:-unknown}, pod IP ${ING_IP}"
}

# totp_code <BASE32_SECRET> [UNIX_TIME] prints the RFC 6238 code (SHA1, 6 digits, 30 seconds).
totp_code() {
  local secret="${1^^}" now="${2:-$(date +%s)}"
  secret="${secret//=/}"
  while (( ${#secret} % 8 )); do secret+="="; done
  local key_hex counter_hex mac offset b0 b1 b2 b3
  key_hex="$(printf '%s' "${secret}" | base32 -d | od -An -v -tx1 | tr -d ' \n')"
  counter_hex="$(printf '%016x' $(( now / 30 )))"
  mac="$(printf "$(sed 's/../\\x&/g' <<<"${counter_hex}")" \
    | openssl dgst -sha1 -mac HMAC -macopt "hexkey:${key_hex}" -binary | od -An -v -tx1 | tr -d ' \n')"
  offset=$(( 16#${mac:39:1} ))
  b0=$(( 16#${mac:offset*2:2} & 0x7f ))
  b1=$(( 16#${mac:offset*2+2:2} ))
  b2=$(( 16#${mac:offset*2+4:2} ))
  b3=$(( 16#${mac:offset*2+6:2} ))
  printf '%06d' $(( ((b0 << 24) | (b1 << 16) | (b2 << 8) | b3) % 1000000 ))
}

# create_user_operation <NAME> <JSON_SPEC> creates a UserOperation. The spec, which may hold a
# password hash, goes to jq through stdin, not through argv.
create_user_operation() {
  printf '%s' "$2" | jq --arg name "$1" '{
      apiVersion: "deckhouse.io/v1",
      kind: "UserOperation",
      metadata: {name: $name, labels: {"e2e.user-authn.deckhouse.io/suite": "true"}},
      spec: .
    }' | kubectl create -f - >/dev/null
  log "Created UserOperation $1"
}

# wait_user_operation <NAME> <PHASE> waits until the UserOperation reaches the phase.
wait_user_operation() {
  local name="$1" phase="$2"
  if ! kubectl wait "useroperations.deckhouse.io/${name}" --for=jsonpath='{.status.phase}'="${phase}" --timeout=180s >/dev/null; then
    >&2 kubectl get "useroperations.deckhouse.io/${name}" -o jsonpath='{.status}'
    die "UserOperation ${name} did not reach phase ${phase}"
  fi
  log "UserOperation ${name} is ${phase}"
}

# dex_config prints the Dex configuration the module rendered.
dex_config() {
  kubectl -n "${DEX_NAMESPACE}" get secret dex -o jsonpath='{.data.config\.yaml}' | base64 -d
}

# wait_dex_config <TIMEOUT_SECONDS> <EXTENDED_REGEX> waits until the rendered Dex configuration
# matches the expression and the Dex Deployment has rolled out with it.
wait_dex_config() {
  local timeout="$1" regex="$2"
  # The configuration is read into a variable first: with pipefail, `grep -q` exiting on the first
  # match would make the writer of the pipe fail with SIGPIPE and the check would never pass.
  dex_config_matches() { local config; config="$(dex_config)" && grep -Eq "${regex}" <<<"${config}"; }
  wait_for "${timeout}" "Dex configuration matching '${regex}'" dex_config_matches
  kubectl -n "${DEX_NAMESPACE}" rollout status deployment/dex --timeout="${timeout}s"
}

# wait_dex_config_absent <TIMEOUT_SECONDS> <EXTENDED_REGEX> is the reverse of wait_dex_config.
wait_dex_config_absent() {
  local timeout="$1" regex="$2"
  dex_config_clean() { local config; config="$(dex_config)" && ! grep -Eq "${regex}" <<<"${config}"; }
  wait_for "${timeout}" "Dex configuration without '${regex}'" dex_config_clean
  kubectl -n "${DEX_NAMESPACE}" rollout status deployment/dex --timeout="${timeout}s"
}
