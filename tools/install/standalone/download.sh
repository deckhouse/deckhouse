#!/usr/bin/env bash

: ${DEBUG:="false"}
: ${STANDALONE_IMAGE:="/install-standalone"}
: ${SCHEME:="https"}
: ${TAG:="stable"}
: ${REGISTRY:="registry.deckhouse.io/deckhouse"}
: ${LOGIN:="license-token"}

# check python version on host machine
checkPython() {
  for pybin in python3 python2 python; do
    if command -v "$pybin" >/dev/null 2>&1; then
      python_binary="$pybin"
      echo "Python is $pybin"
      return 0
    fi
  done
  echo "Python not found, exiting..."
  return 1
}

# get auth token from source registry 
get_token() {
  cat - <<EOF | $python_binary
import requests
import json
import re

try:
    from requests import get
    from re import search, sub
except ImportError as e:
    print(f"Error load module: {e}")

url = '$SCHEME://$REGISTRY_ADDRESS/v2/'
header = requests.get(url, timeout=10).headers.get('Www-Authenticate')
realm = re.search(r'realm="([^"]+)"', header).group(1)
service = re.sub(r' ', '+', re.search(r'service="([^"]+)"', header).group(1))
token_url = f"{realm}?service={service}&scope=repository:$REGISTRY_PATH$EDITION$STANDALONE_IMAGE:pull"

if '$REGISTRY_AUTH':
    try:
        request = requests.get(token_url, timeout=10, auth=('$LOGIN', '$PASSWORD')).json()
        print(request.get('token'))
    except json.JSONDecodeError as e:
        print(f"JSON decode error: {e}")
    except Exception as e:
        print(f"An unexpected error occurred: {e}")
else:
    try:
# No authentication provided
       request = requests.get(token_url, timeout=10).json() 
       print(request.get('token'))
    except json.JSONDecodeError as e:
        print(f"JSON decode error: {e}")
    except Exception as e:
        print(f"An unexpected error occurred: {e}")
EOF
}

# create default temp directory with prefix dhctl-standalone or use custom directory
tmp_dhctl() {
  cat - <<EOF | $python_binary
import tempfile
import os

temp_dir = "$DHCTL_TMP_DIR"

if '$DHCTL_TMP_DIR':
    try:
        if not os.path.exists(temp_dir):
            os.makedirs(temp_dir)
            print(f"{temp_dir}")
        else:
             print(f"{temp_dir}")
    except Exception as e:
        print(f"An unexpected error occurred: {e}")
else:
    try:
        temp_dir = tempfile.mkdtemp(prefix="dhctl-standalone-")
        print(f"{temp_dir}")
    except Exception as e:
        print(f"An unexpected error occurred: {e}")
EOF
}	

# download json manifest from source registry
fetchManifest() {

TOKEN="$(get_token)"
TMP_DIR="$(tmp_dhctl)"
  cat - <<EOF | $python_binary
import requests
import tempfile
import os

try:
   from requests.exceptions import HTTPError
except ImportError as e:
   print(f"Error load module: {e}")

manifest_path = os.path.join("$TMP_DIR", "deckhouse-standalone-manifest.json")
url = '$SCHEME://$REGISTRY_ADDRESS/v2/$REGISTRY_PATH$EDITION$STANDALONE_IMAGE/manifests/$TAG'
headers = { 'Authorization': 'Bearer $TOKEN', 'Accept': 'application/vnd.docker.distribution.manifest.v2+json' }
manifest = requests.get(url, headers=headers, timeout=10, stream=True)
try:
    print(f"Temp directory for dhctl-standalone is $TMP_DIR")
    with open(manifest_path, 'wb') as f:
        f.write(manifest.content)
except HTTPError as e:
    print(f"An unexpected error occurred: {e}")
EOF
}
# get sha256 digest from json manifest
get_digest_from_manifest() {
  cat - <<EOF | $python_binary
import json
file_path = "$TMP_DIR/deckhouse-standalone-manifest.json"
try:
    with open(file_path) as file_path:
        data = json.load(file_path)
    print(data["layers"][-1]["digest"])
except FileNotFoundError:
    print(f"File not found: {file_path}")
except json.JSONDecodeError:
    print(f"Error decoding JSON from file: {file_path}")
except Exception as e:
    print(f"An unexpected error occurred: {e}")
EOF
}

# download and extract dhctl-download to directory
downloadInstall() {

local DIGEST="$(get_digest_from_manifest)"

  cat - <<EOF | $python_binary
import requests
import tarfile
import os
import shutil

try:
   from requests.exceptions import HTTPError
except ImportError as e:
   print(f"Error load module: {e}")

blob_path = os.path.join("$TMP_DIR", "dhctl.x86_64.tar.gz")
url = '$SCHEME://$REGISTRY_ADDRESS/v2/$REGISTRY_PATH$EDITION$STANDALONE_IMAGE/blobs/$DIGEST'
headers = { 'Authorization': 'Bearer $TOKEN' }
output_dir = '$TMP_DIR'

#download dhclt-standalone
def download_dhctl(url, dest_path):
    with requests.get(url, stream=True) as response:
            with open(dest_path, 'wb') as f:
                for chunk in requests.get(url, headers=headers, timeout=10, stream=True).iter_content(chunk_size=8192):  # 8KB chunks
                    f.write(chunk)
#extract dhctl-standalone
def extract_dhctl(archive_path, extract_dir):
    with tarfile.open(archive_path, 'r') as outer_tar:
        outer_tar.extractall(path=extract_dir)
        for member in outer_tar.getmembers():
            if member.name.endswith('.tar.gz'):
                inner_tar_path = os.path.join(extract_dir, member.name)
                origin_tar_path = os.path.join(extract_dir, 'dhctl.x86_64.tar.gz')
                manifest_json_path = os.path.join(extract_dir, 'deckhouse-standalone-manifest.json')
                extract_dhctl(inner_tar_path, extract_dir)
                os.remove(inner_tar_path)
                os.remove(origin_tar_path)
                os.remove(manifest_json_path)
##install dhctl-standalone
try:
    print(f"Download dhctl-standalone in $TMP_DIR")
    download_dhctl(url, blob_path)
    print(f"Downloaded dhctl-standalone saved as {blob_path}")
except HTTPError as e:
    print(f"An unexpected error occurred: {e}")
try:
    print(f"Extract dhctl-standalone to $TMP_DIR")
    extract_dhctl(blob_path, output_dir)
    print(f"Extracted dhctl standalone to {output_dir}")
except Exception as e:
    print(f"An unexpected error occurred: {e}")
EOF
}

# fail_trap is executed if an error occurs.
fail_trap() {
  result=$?
  if [ "$result" != "0" ]; then
    if [[ -n "$INPUT_ARGUMENTS" ]]; then
      echo "Failed to install dhctl-standalone with the arguments provided: $INPUT_ARGUMENTS"
      help
    else
     echo "Failed to install dhctl-standalone"
    fi
  fi
  exit $result
}

# help provides dhctl-standalone installation arguments
help () {
  echo "The available commands for execution are listed below:"
  echo -e "\t[--help|-h ] prints this help"
  echo -e "\t[--tag|-t] source release channel to download dhctl-standalone. default 'stable'."
  echo -e "\t[--tmp-dir|-d] temp diretory to download and extract dhctl-standalone. default '/tmp' directory."
  echo -e "\t[--scheme|-s] scheme to download from source registry. default 'https'."
  echo -e "\t[--edition|-e] edition to download from source registry. default 'ce'."
  echo -e "\t[--user]|-u] login user credentionals. default 'license-token'."
  echo -e "\t[--password|-p] password user credentionals."
  echo -e "\t[--registry|-r] source registry to download dhctl-standalone. default 'registry.deckhouse.io'."
  echo -e "\t[--verbose] DEBUG verbose. default 'false'."
}


# stop execution on any error
trap "fail_trap" EXIT
set -Eeo pipefail

parseArguments() {
export INPUT_ARGUMENTS="${@}"	
  while [[ $# -gt 0 ]]; do
    case $1 in
      -t|--tag)
        TAG="$2"
        shift # past argument
        shift # past value
        ;;
      -d|--tmp-dir)
        DHCTL_TMP_DIR="$2"
        shift # past argument
        shift # past value
        ;;
      -s|--scheme)
        SCHEME="$2"
        shift # past argument
        shift # past value
        ;;
      -e|--edition)
        EDITION="$2"
        shift # past argument
        shift # past value
        ;;
      -u|--user)
        LOGIN="$2"
        shift # past argument
        shift # past value
        ;;
      -p|--password)
        PASSWORD="$2"
        shift # past argument
        shift # past value
        ;;
      -r|--registry)
        REGISTRY="$2"
        shift # past argument
        shift # past value
        ;;
      -h|--help)
	help
	exit 0
        ;;	
      --verbose)
        DEBUG="$2"
        shift # past argument
        shift # past value
        ;;
      -*|--*)
        >&2 echo "Unknown option $1"
        exit 1
        ;;
      *)
    esac
  done
}

validateArguments() {

  if [[ -n "$PASSWORD" ]]; then
      REGISTRY_AUTH="yes"
  else
      REGISTRY_AUTH=""
  fi

  if [ -z "$EDITION" ]; then
    EDITION=""
  else
    EDITION="/$EDITION"
  fi

  REGISTRY_ADDRESS="${REGISTRY%%/*}"
  REGISTRY_PATH="${REGISTRY#*/}"

  if [ -z "${REGISTRY_ADDRESS}" ]; then
    >&2 echo "Registry must have registry address"
    exit 2
  fi

  if [ "${DEBUG}" == "true" ]; then
    set -x
  fi
}

parseArguments "$@"
validateArguments
checkPython
fetchManifest
downloadInstall
