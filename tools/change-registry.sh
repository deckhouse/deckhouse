#!/usr/bin/env bash

# Copyright 2022 Flant JSC
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

REGISTRY_USER=""
REGISTRY_PASS=""
REGISTRY_SCHEME=""
REGISTRY_ADDRESS=""
REGISTRY_PATH=""
REGISTRY_CAFILE=""

function usage() {
  printf "
 Usage: %s [--user <USER NAME>] [--password <PASSWORD>] [--ca-file <FILENAME>] --registry-url <SCHEME://REGISTRY_URL/PATH>
    --user <USER NAME>
            Registry auth user name.
    --password <PASSWORD>
            Registry auth user password.
    --registry-url <SCHEME://REGISTRY_URL/PATH>
            Registry URL.
    --ca-file <FILENAME>
            File containing CA certificate for validating registry self-signed certificate.
    --help|-h
            Print this message.
" "$0"
}

# get token from registry auth
# bb-rp-get-token
function bb-rp-get-token() {
  local AUTH=""
  local AUTH_HEADER=""
  local AUTH_REALM=""
  local AUTH_SERVICE=""

  AUTH_HEADER="$(curl --retry 3 -sSLi "${REGISTRY_SCHEME}://${REGISTRY_ADDRESS}/v2/" | grep -i "www-authenticate")"
  AUTH_REALM="$(awk -F "," '{split($1,s,"\""); print s[2]}' <<< "${AUTH_HEADER}")"
  AUTH_SERVICE="$(awk -F "," '{split($2,s,"\""); print s[2]}' <<< "${AUTH_HEADER}" | sed "s/ /+/g")"
  curl --retry 3 -fsSL -u ${REGISTRY_USER}:${REGISTRY_PASS} "${AUTH_REALM}?service=${AUTH_SERVICE}&scope=repository:${REGISTRY_PATH#/}:pull" | jq -r '.token'
}

function parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
    --user)
      REGISTRY_USER="$2"
      ;;
    --password)
      REGISTRY_PASS="$2"
      ;;
    --registry-url)
      REGISTRY_URL="$2"
      ;;
    --ca-file)
      REGISTRY_CAFILE="$2"
      ;;
    --help | -h)
      usage
      exit 0
      ;;
    --*)
      echo "Illegal option $1"
      usage
      exit 1
      ;;
    esac
    shift $(($# > 0 ? 1 : 0))
  done

  TEMP_URL="$REGISTRY_URL"
  REGISTRY_SCHEME="$(grep -oE "^https{0,1}://" <<< "$TEMP_URL")"
  TEMP_URL="${TEMP_URL#"$REGISTRY_SCHEME"}"
  REGISTRY_ADDRESS="$(cut -d "/" -f1 <<< "$TEMP_URL")"
  REGISTRY_PATH="${TEMP_URL#"$REGISTRY_ADDRESS"}"
  REGISTRY_SCHEME="$(sed "s/:\/\///" <<< "$REGISTRY_SCHEME")"
  DECKHOUSE_TAG="$(kubectl -n d8-system get deploy deckhouse -o json | jq '.spec.template.spec.containers[] | select(.name=="deckhouse") | .image | split(":")[-1]' -r)"

  if [[ "$REGISTRY_PATH" == "" ]]; then
    >&2 echo "Cannot parse path from registry url: $REGISTRY_URL. Registry url must have at least slash at the end. (for example, https://registry.example.com/ instead of https://registry.example.com)"
    exit 1
  fi

  if [[ "$REGISTRY_SCHEME" == "" ]]; then
    >&2 echo "Cannot parse scheme from registry url: $REGISTRY_URL. Scheme should be 'http' or 'https'."
    exit 1
  fi

  if [[ "$REGISTRY_ADDRESS" == "" ]]; then
    >&2 echo "Cannot parse hostname from registry url: $REGISTRY_URL."
    exit 1
  fi

  domain_validator="^[a-z0-9][-a-z0-9\.]*[a-z](:[0-9]{1,5})?(:\d+)?$"
  if ! [[ $REGISTRY_ADDRESS =~ $domain_validator ]]; then
    >&2 echo "Registry domain doesn't fit the regex ${domain_validator}: $REGISTRY_ADDRESS."
    exit 1
  fi

  if [[ "$REGISTRY_CAFILE" != "" ]] && [[ ! -f "$REGISTRY_CAFILE" ]]; then
    >&2 echo "Cannot find ca file: $REGISTRY_CAFILE."
    exit 1
  fi

  TOKEN="$(bb-rp-get-token)"
  if [[ "$TOKEN" == "" ]]; then
    >&2 echo "Cannot get Bearer token from registry $REGISTRY_URL"
    exit 1
  fi

  URI="${REGISTRY_SCHEME}://${REGISTRY_ADDRESS}/v2${REGISTRY_PATH}/manifests/${DECKHOUSE_TAG}"
  curl --retry 3 -fsSLq -o /dev/null \
       -H "Accept: application/vnd.docker.distribution.manifest.v2+json" \
       -H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" \
       -H "Authorization: Bearer $TOKEN" "$URI" 2>/dev/null
  RESULT=$?
  if [[ $RESULT -ne 0 ]]; then
    >&2 echo "Cannot find image ${REGISTRY_ADDRESS}${REGISTRY_PATH}:${DECKHOUSE_TAG}."
    exit 1
  fi
}

function create_dockerconfigjson() {
  if [[ "$REGISTRY_USER" != "" ]] && [[ "$REGISTRY_PASS" != "" ]]; then
    REGISTRY_AUTH="\"auth\":\"$(echo -n "$REGISTRY_USER:$REGISTRY_PASS" | base64 -w0)\""
  fi
  cat - <<EOF
{
  "auths": {
    "$REGISTRY_ADDRESS": {
        $REGISTRY_AUTH
    }
  }
}
EOF
}

parse_args "$@"

DOCKERCONFIGJSON="$(create_dockerconfigjson | base64 -w0)"
REGISTRY_ADDRESS="$(echo -n $REGISTRY_ADDRESS | base64 -w0)"
REGISTRY_PATH="$(echo -n $REGISTRY_PATH | base64 -w0)"
REGISTRY_SCHEME="$(echo -n $REGISTRY_SCHEME | base64 -w0)"

kubectl -n d8-system patch secret deckhouse-registry -p="{\"data\":{\".dockerconfigjson\": \"${DOCKERCONFIGJSON}\", \"address\": \"${REGISTRY_ADDRESS}\", \"path\": \"${REGISTRY_PATH}\", \"scheme\": \"${REGISTRY_SCHEME}\"}}"
if  [[ "$REGISTRY_CAFILE" != "" ]]; then
  REGISTRY_CA="$(base64 -w0 < ${REGISTRY_CAFILE})"
  kubectl -n d8-system patch secret deckhouse-registry -p="{\"data\":{\"ca\": \"${REGISTRY_CA}\"}}"
fi
