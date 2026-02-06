#!/usr/bin/env bash
set -euo pipefail

# Keep Dex kubernetes-storage CRDs in sync with the Dex version we build.
#
# - Update local CRDs (downloads from GitHub):
#     ./pull_dex_crds.sh
#
# - CI check mode (no network): compares our CRDs with a checked-out Dex source tree:
#     ./pull_dex_crds.sh --check --dex-dir /path/to/dex/scripts/manifests/crds

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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
  connectors.yaml
  devicerequests.yaml
  devicetokens.yaml
  oauth2clients.yaml
  offlinesessionses.yaml
  passwords.yaml
  refreshtokens.yaml
  signingkeies.yaml
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
    left="${script_dir}/${crd}"
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

# Extract "tag: vX.Y.Z" from werf.inc.yaml (dex source checkout).
werf_file="${script_dir}/../images/dex/werf.inc.yaml"
if [[ ! -f "${werf_file}" ]]; then
  echo "Cannot find ${werf_file} to detect Dex version." >&2
  exit 1
fi

dex_tag="$(
  awk 'index($0, "dexidp/dex.git") {found=1} found && $1=="tag:" {print $2; exit}' "${werf_file}"
)"
if [[ -z "${dex_tag}" ]]; then
  echo "Cannot detect Dex tag from ${werf_file}." >&2
  exit 1
fi

repo="https://raw.githubusercontent.com/dexidp/dex/${dex_tag}/scripts/manifests/crds"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

for crd in "${crds[@]}"; do
  url="${repo}/${crd}"
  echo "Downloading ${url}"
  curl -fsSL "${url}" > "${tmp}/${crd}"
  mv "${tmp}/${crd}" "${script_dir}/${crd}"
done

echo "Updated Dex CRDs from Dex ${dex_tag}."
