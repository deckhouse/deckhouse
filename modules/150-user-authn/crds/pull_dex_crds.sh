#!/usr/bin/env bash
set -euo pipefail

#
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
#

# Keep the Dex kubernetes-storage CRDs in sync with the Dex we build: the release from oss.yaml
# with the patches of images/dex/patches applied. The patches change the manifests
# (100-storage-schema adds useridentities.yaml and authsessions.yaml), so the CRDs come from a
# patched source tree, never from the upstream release alone.
#
# - Update local CRDs; clones the Dex release and applies the patches (needs git and network):
#     ./pull_dex_crds.sh
#
# - Update local CRDs from a patched Dex source tree (no network):
#     ./pull_dex_crds.sh --dex-dir /path/to/dex/scripts/manifests/crds
#
# - CI check mode (no network): compares our CRDs with a patched Dex source tree:
#     ./pull_dex_crds.sh --check --dex-dir /path/to/dex/scripts/manifests/crds

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
dex_crds_dir="${script_dir}/external"

mode="update"
dex_dir=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check)
      mode="check"
      shift
      ;;
    --dex-dir)
      dex_dir="${2:-}"
      shift 2
      ;;
    *)
      echo "Unknown arg: $1" >&2
      exit 2
      ;;
  esac
done

crds=(
  authcodes.yaml
  authrequests.yaml
  authsessions.yaml
  connectors.yaml
  devicerequests.yaml
  devicetokens.yaml
  oauth2clients.yaml
  offlinesessionses.yaml
  passwords.yaml
  refreshtokens.yaml
  signingkeies.yaml
  useridentities.yaml
)

if [[ "${mode}" == "check" ]]; then
  if [[ -z "${dex_dir}" ]]; then
    echo "--dex-dir is required in --check mode" >&2
    exit 2
  fi
  if [[ ! -d "${dex_dir}" ]]; then
    echo "Dex CRD dir not found: ${dex_dir}" >&2
    exit 2
  fi

  for crd in "${crds[@]}"; do
    left="${dex_crds_dir}/${crd}"
    right="${dex_dir}/${crd}"

    if [[ ! -f "${left}" ]]; then
      echo "Missing local CRD file: ${left}" >&2
      exit 1
    fi
    if [[ ! -f "${right}" ]]; then
      echo "Missing Dex CRD file: ${right}" >&2
      exit 1
    fi

    if ! diff -u "${right}" "${left}" >/dev/null; then
      echo "Dex CRD mismatch for ${crd}." >&2
      echo "Run: ${script_dir}/pull_dex_crds.sh" >&2
      exit 1
    fi
  done

  echo "OK: Dex CRDs match."
  exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

source_desc="${dex_dir}"
if [[ -z "${dex_dir}" ]]; then
  oss_file="${script_dir}/../oss.yaml"
  dex_version="$(
    awk '$1=="-" && $2=="name:" {in_dex=0} $1=="id:" && $2=="dex" {in_dex=1} in_dex && $1=="version:" {print $2; exit}' "${oss_file}"
  )"
  if [[ -z "${dex_version}" ]]; then
    echo "Cannot detect the Dex version from ${oss_file}." >&2
    exit 1
  fi

  echo "Cloning Dex v${dex_version}"
  git clone --quiet --depth 1 --branch "v${dex_version}" https://github.com/dexidp/dex.git "${tmp}/dex"
  for patch in "${script_dir}"/../images/dex/patches/*.patch; do
    echo "Applying $(basename "${patch}")"
    git -C "${tmp}/dex" apply "${patch}"
  done
  dex_dir="${tmp}/dex/scripts/manifests/crds"
  source_desc="Dex v${dex_version} with the patches of images/dex/patches"
fi

if [[ ! -d "${dex_dir}" ]]; then
  echo "Dex CRD dir not found: ${dex_dir}" >&2
  exit 2
fi

mkdir -p "${dex_crds_dir}"
for crd in "${crds[@]}"; do
  if [[ ! -f "${dex_dir}/${crd}" ]]; then
    echo "Missing Dex CRD file: ${dex_dir}/${crd}" >&2
    exit 1
  fi
  cp "${dex_dir}/${crd}" "${dex_crds_dir}/${crd}"
done

echo "Updated Dex CRDs from ${source_desc}."
