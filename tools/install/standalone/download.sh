#!/usr/bin/env bash

export LANG=C LC_NUMERIC=C
set -Eeo pipefail

edition="ce"
release_chanel="stable"
tag=""
registry="registry.deckhouse.io"
registry_address="registry.deckhouse.io"
registry_path="registry.deckhouse.io"
user=""
token=""
output_dir=""
scheme="https"

function parse_arguments() {
  while [[ $# -gt 0 ]]; do
    case $1 in
      -t|--tag)
        tag="$2"
        shift # past argument
        shift # past value
        ;;
      -s|--scheme)
        scheme="$2"
        shift # past argument
        shift # past value
        ;;
      -e|--edition)
        edition="$2"
        shift # past argument
        shift # past value
        ;;
      -u|--user)
        user="$2"
        shift # past argument
        shift # past value
        ;;
      -p|--password)
        token="$2"
        shift # past argument
        shift # past value
        ;;
      -r|--registry)
        registry="$2"
        shift # past argument
        shift # past value
        ;;
      -*|--*)
        >&2 echo "Unknown option $1"
        exit 1
        ;;
      *)
        if [ -n "$output_dir" ]; then
          >&2 echo "Output dir was already passed. Use one argument to pass output directory"
          exit 1
        fi
        output_dir="$1"
        shift # past argument
        ;;
    esac
  done
}

function check_arguments() {
  if [ -z "$output_dir" ]; then
    >&2 echo "Output dir was not provided"
    exit 2
  fi

  if [ -z "$tag" ]; then
    # drop release channel if tag was provided
    release_chanel=""
  fi

  registry_address="${registry%@*}"
  registry_path="${registry#*@}"

  if [ -z "${registry_address}" ]; then
    >&2 echo "Registry must have registry address"
    exit 2
  fi

  if [ -z "${registry_path}" ]; then
    registry_path=""
  fi
  registry_path="/${registry_path}"
}

function get_token() {
  local registry_auth="$1"

  local auth=""
  local auth_header=""
  local auth_realm=""
  local auth_service=""

  if [[ -n "${registry_auth}" ]]; then
    auth="yes"
  fi

  auth_header="$(bb-rp-curl -sSLi "${scheme}://${registry_address}/v2/" | grep -i "www-authenticate")"
  auth_realm="$(grep -oE 'Bearer realm="http[s]{0,1}://[a-z0-9\.\:\/\-]+"' <<< "${auth_header}" | cut -d '"' -f2)"
  auth_service="$(grep -oE 'service="[[:print:]]+"' <<< "${auth_header}" | cut -d '"' -f2 | sed 's/ /+/g')"
  if [ -z "${auth_realm}" ]; then
    >&2 echo "couldn't find bearer realm parameter, consider enabling bearer token auth in your registry, returned header: ${auth_header}"
    exit 3
  fi
  # Remove leading / from REGISTRY_PATH due to scope format -> scope=repository:deckhouse/fe:pull
  curl -fsSL ${auth:+-u "$REGISTRY_AUTH"} "${auth_realm}?service=${auth_service}&scope=repository:${registry_path#/}:pull" | jq -r '.token'
}

function fetch_manifest() {
  local token="$1"
  local tag="$2"
  local manifest_file="$3"

  local url="${scheme}://${registry_address}/v2${registry_path}/manifests/${tag}"

  bb-rp-curl -fsSL --create-dirs \
    -H "Authorization: Bearer ${token}" \
    -H 'Accept: application/vnd.docker.distribution.manifest.v2+json' \
    -o "$manifest_file" \
    "${url}"
}

function download_image() {
  local token=""
  local auth=""
  if [ -n "$user" ]; then
    auth="${user}:${token}"
  fi

  token="$(get_token "$auth")"
  manifest_file="$(mktemp /tmp/deckhouse-standalone-manifest.XXXXXX)"
  fetch_manifest "$token"


  rm -f "$manifest_file"
}

function extract_installer() {
  exit 0
}

parse_arguments "$@"
check_arguments
download_image
extract_installer
