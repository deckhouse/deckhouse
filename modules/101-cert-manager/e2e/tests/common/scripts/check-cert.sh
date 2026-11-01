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


# Inspects the certificate stored in a kubernetes.io/tls Secret with openssl.
#
# Usage: check-cert.sh --namespace <NS> --secret <NAME> [options]
#   --san <TEXT>             the subjectAltName extension contains TEXT, for example
#                            "DNS:host.example.test" or "IP Address:192.0.2.10"; repeatable
#   --issuer-cn <CN>         the issuer of tls.crt has the common name CN
#   --ca-equals <NS/NAME/KEY> ca.crt is the same certificate as the KEY entry of that Secret
#   --verify                 `openssl verify` of tls.crt against ca.crt succeeds
#   --is-ca                  tls.crt has the basic constraint CA:TRUE
#
# Every failed check is reported; the exit code is 1 when any check fails.
set -Eeuo pipefail

namespace=""
secret=""
sans=()
issuer_cn=""
ca_equals=""
verify=false
is_ca=false

while (( $# > 0 )); do
  case "$1" in
    --namespace) namespace="$2"; shift 2 ;;
    --secret) secret="$2"; shift 2 ;;
    --san) sans+=("$2"); shift 2 ;;
    --issuer-cn) issuer_cn="$2"; shift 2 ;;
    --ca-equals) ca_equals="$2"; shift 2 ;;
    --verify) verify=true; shift ;;
    --is-ca) is_ca=true; shift ;;
    *) >&2 echo "unknown argument $1"; exit 2 ;;
  esac
done

[[ -n "${namespace}" && -n "${secret}" ]] || { >&2 echo "--namespace and --secret are required"; exit 2; }

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

secret_key() {
  # $1 namespace, $2 name, $3 data key, $4 output file
  local data
  data="$(kubectl -n "$1" get secret "$2" -o json | jq -r --arg k "$3" '.data[$k] // ""')"
  printf '%s' "${data}" | base64 -d > "$4"
  [[ -s "$4" ]] || { >&2 echo "Secret $1/$2 has no data key $3"; return 1; }
}

fingerprint() {
  openssl x509 -in "$1" -noout -fingerprint -sha256 | cut -d= -f2
}

secret_key "${namespace}" "${secret}" tls.crt "${work}/tls.crt"
secret_key "${namespace}" "${secret}" ca.crt "${work}/ca.crt"

echo "Certificate ${namespace}/${secret}:"
openssl x509 -in "${work}/tls.crt" -noout -subject -issuer -dates -nameopt RFC2253
san_text="$(openssl x509 -in "${work}/tls.crt" -noout -ext subjectAltName 2>/dev/null || true)"
echo "${san_text}"

failed=0
fail() { >&2 echo "FAIL: $*"; failed=1; }

for san in "${sans[@]}"; do
  grep -qF -- "${san}" <<<"${san_text}" || fail "subjectAltName does not contain '${san}'"
done

if [[ -n "${issuer_cn}" ]]; then
  issuer="$(openssl x509 -in "${work}/tls.crt" -noout -issuer -nameopt RFC2253)"
  grep -qE "(^issuer=|,)CN=${issuer_cn}(,|$)" <<<"${issuer}" || fail "issuer '${issuer}' does not have CN=${issuer_cn}"
fi

if [[ -n "${ca_equals}" ]]; then
  IFS=/ read -r ref_ns ref_name ref_key <<<"${ca_equals}"
  secret_key "${ref_ns}" "${ref_name}" "${ref_key}" "${work}/ref.crt"
  [[ "$(fingerprint "${work}/ca.crt")" == "$(fingerprint "${work}/ref.crt")" ]] \
    || fail "ca.crt differs from ${ref_ns}/${ref_name} ${ref_key}"
fi

if [[ "${verify}" == true ]]; then
  openssl verify -CAfile "${work}/ca.crt" "${work}/tls.crt" || fail "openssl verify failed"
fi

if [[ "${is_ca}" == true ]]; then
  openssl x509 -in "${work}/tls.crt" -noout -ext basicConstraints | grep -q "CA:TRUE" || fail "tls.crt is not a CA certificate"
fi

exit "${failed}"
