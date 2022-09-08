#!/bin/bash

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
  if [[ "$REGISTRY_SCHEME" == "" ]]; then
    >&2 echo "Cannot parse scheme from registry url: $URL. Scheme should be 'http' or 'https'."
    exit 1
  fi

  if [[ "$REGISTRY_ADDRESS" == "" ]]; then
    >&2 echo "Cannot parse hostname from registry url: $REGISTRY_URL."
    exit 1
  fi

  if [[ "$REGISTRY_CAFILE" != "" ]] && [[ ! -f "$REGISTRY_CAFILE" ]]; then
    >&2 echo "Cannot find ca file: $REGISTRY_CAFILE."
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
if  [[ "$REGISTRY_CAFILE" != "" ]]; then
  REGISTRY_CAFILE="$(base64 -w0 < $REGISTRY_CAFILE)"
fi

kubectl -n d8-system patch secret deckhouse-registry -p="{\"data\":{\".dockerconfigjson\": \"${DOCKERCONFIGJSON}\", \"address\": \"${REGISTRY_ADDRESS}\", \"path\": \"${REGISTRY_PATH}\", \"scheme\": \"${REGISTRY_SCHEME}\"}}"
if  [[ "$REGISTRY_CAFILE" != "" ]]; then
  REGISTRY_CAFILE="$(base64 -w0 < $REGISTRY_CAFILE)"
  kubectl -n d8-system patch secret deckhouse-registry -p="{\"data\":{\"ca\": \"${REGISTRY_CAFILE}\"}}"
fi
