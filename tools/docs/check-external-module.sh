#!/usr/bin/env bash

set -euo pipefail

DEFAULT_HUGO_IMAGE="ghcr.io/gohugoio/hugo:v0.163.3"
DEFAULT_DECKHOUSE_REF="main"
DEFAULT_DECKHOUSE_REPO_URL="https://github.com/deckhouse/deckhouse.git"

MODULE_PATH="${MODULE_PATH:-}"
CHANNEL="${CHANNEL:-alpha}"
MODULE_VERSION="${MODULE_VERSION:-}"
OUTPUT_DIR="${OUTPUT_DIR:-}"
HUGO_IMAGE="${HUGO_IMAGE:-${DEFAULT_HUGO_IMAGE}}"
DECKHOUSE_REPO="${DECKHOUSE_REPO:-}"
DECKHOUSE_REF="${DECKHOUSE_REF:-${DEFAULT_DECKHOUSE_REF}}"
DECKHOUSE_REPO_URL="${DECKHOUSE_REPO_URL:-${DEFAULT_DECKHOUSE_REPO_URL}}"
KEEP_WORKDIR="${KEEP_WORKDIR:-0}"

OUTPUT_DIR_EXPLICIT=0

usage() {
  cat <<'EOF'
Usage: check-external-module.sh [options]

Build documentation for a Deckhouse external module with Hugo and verify that
the rendered pages exist. Uses docker to run Hugo — no local Hugo install is
required.

Options (CLI flags override the matching environment variables):
  --module-path <path>       Path to the module repository (default: current directory).
  --channel <name>           Release channel to render (default: alpha).
  --version <ver>            Module version label (default: git describe of --module-path, or v0.0.0-dev).
  --output <dir>             Directory for the rendered site (default: temporary directory, removed on exit).
  --hugo-image <image>       Hugo docker image (default: ghcr.io/gohugoio/hugo:v0.163.3).
  --deckhouse-repo <path>    Existing checkout of the deckhouse repo. Skips the sparse clone below.
  --deckhouse-ref <ref>      Branch/tag to fetch when cloning deckhouse (default: main).
  --repo-url <url>           deckhouse repo URL used for the sparse clone
                             (default: https://github.com/deckhouse/deckhouse.git).
  --keep                     Do not remove the temporary directories on exit.
  -h, --help                 Show this help.

Required on PATH: bash, docker, yq. git is required only when --deckhouse-repo is not provided.
EOF
}

die() {
  echo "check-external-module: $*" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --module-path) MODULE_PATH="${2:?}"; shift 2 ;;
    --module-path=*) MODULE_PATH="${1#*=}"; shift ;;
    --channel) CHANNEL="${2:?}"; shift 2 ;;
    --channel=*) CHANNEL="${1#*=}"; shift ;;
    --version) MODULE_VERSION="${2:?}"; shift 2 ;;
    --version=*) MODULE_VERSION="${1#*=}"; shift ;;
    --output) OUTPUT_DIR="${2:?}"; OUTPUT_DIR_EXPLICIT=1; shift 2 ;;
    --output=*) OUTPUT_DIR="${1#*=}"; OUTPUT_DIR_EXPLICIT=1; shift ;;
    --hugo-image) HUGO_IMAGE="${2:?}"; shift 2 ;;
    --hugo-image=*) HUGO_IMAGE="${1#*=}"; shift ;;
    --deckhouse-repo) DECKHOUSE_REPO="${2:?}"; shift 2 ;;
    --deckhouse-repo=*) DECKHOUSE_REPO="${1#*=}"; shift ;;
    --deckhouse-ref) DECKHOUSE_REF="${2:?}"; shift 2 ;;
    --deckhouse-ref=*) DECKHOUSE_REF="${1#*=}"; shift ;;
    --repo-url) DECKHOUSE_REPO_URL="${2:?}"; shift 2 ;;
    --repo-url=*) DECKHOUSE_REPO_URL="${1#*=}"; shift ;;
    --keep) KEEP_WORKDIR=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

if [[ -n "${OUTPUT_DIR}" && "${OUTPUT_DIR_EXPLICIT}" -eq 0 ]]; then
  OUTPUT_DIR_EXPLICIT=1
fi

if [[ -z "${MODULE_PATH}" ]]; then
  MODULE_PATH="$(pwd)"
fi

if [[ ! -d "${MODULE_PATH}" ]]; then
  die "module path is not a directory: ${MODULE_PATH}"
fi
MODULE_PATH="$(cd -- "${MODULE_PATH}" && pwd)"

command -v docker >/dev/null 2>&1 || die "docker is required and was not found in PATH."

if ! command -v yq >/dev/null 2>&1; then
  cat >&2 <<'EOF'
check-external-module: yq is required and was not found in PATH.
Install one of:
  brew install yq              # macOS
  snap install yq              # Ubuntu (snap)
  apt install yq               # Debian/Ubuntu (repo-provided)
Or download a release from https://github.com/mikefarah/yq/releases
EOF
  exit 1
fi

if [[ -z "${DECKHOUSE_REPO}" ]] && ! command -v git >/dev/null 2>&1; then
  die "git is required to fetch the Deckhouse repository; install git or pass --deckhouse-repo."
fi

[[ -f "${MODULE_PATH}/module.yaml" ]] || die "required file is missing: ${MODULE_PATH}/module.yaml"
[[ -d "${MODULE_PATH}/docs" ]] || die "required directory is missing: ${MODULE_PATH}/docs"

if [[ -z "${MODULE_VERSION}" ]]; then
  if [[ -d "${MODULE_PATH}/.git" ]] && command -v git >/dev/null 2>&1; then
    MODULE_VERSION="$(git -C "${MODULE_PATH}" describe --tags --always --dirty 2>/dev/null || true)"
  fi
  if [[ -z "${MODULE_VERSION}" ]]; then
    MODULE_VERSION="v0.0.0-dev"
  fi
fi

MODULE_NAME="$(yq eval -r '.name' "${MODULE_PATH}/module.yaml")"
if [[ -z "${MODULE_NAME}" || "${MODULE_NAME}" == "null" ]]; then
  die "unable to read module name from ${MODULE_PATH}/module.yaml"
fi

TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/check-external-module.XXXXXX")"
CLONED_DECKHOUSE=""
CREATED_OUTPUT_DIR=""

cleanup() {
  local status=$?
  if [[ "${KEEP_WORKDIR}" -eq 1 ]]; then
    echo "Working directory kept: ${TMP_ROOT}" >&2
    if [[ -n "${CREATED_OUTPUT_DIR}" ]]; then
      echo "Rendered output kept: ${CREATED_OUTPUT_DIR}" >&2
    fi
    return "${status}"
  fi
  rm -rf "${TMP_ROOT}"
  if [[ -n "${CREATED_OUTPUT_DIR}" && "${OUTPUT_DIR_EXPLICIT}" -eq 0 ]]; then
    rm -rf "${CREATED_OUTPUT_DIR}"
  fi
  return "${status}"
}
trap cleanup EXIT

if [[ -z "${DECKHOUSE_REPO}" ]]; then
  CLONED_DECKHOUSE="${TMP_ROOT}/deckhouse"
  echo "Fetching Deckhouse docs template (${DECKHOUSE_REF}) from ${DECKHOUSE_REPO_URL}..."
  git clone --quiet --filter=blob:none --sparse --depth 1 \
    --branch "${DECKHOUSE_REF}" \
    "${DECKHOUSE_REPO_URL}" "${CLONED_DECKHOUSE}"
  git -C "${CLONED_DECKHOUSE}" sparse-checkout set \
    tools/docs \
    docs/site/backends/docs-builder-template >/dev/null
  DECKHOUSE_REPO="${CLONED_DECKHOUSE}"
else
  if [[ ! -d "${DECKHOUSE_REPO}" ]]; then
    die "--deckhouse-repo points to a missing directory: ${DECKHOUSE_REPO}"
  fi
  DECKHOUSE_REPO="$(cd -- "${DECKHOUSE_REPO}" && pwd)"
fi

CANONICAL_SCRIPT="${DECKHOUSE_REPO}/tools/docs/external-module-docs.sh"
TEMPLATE_DIR="${DECKHOUSE_REPO}/docs/site/backends/docs-builder-template"
[[ -x "${CANONICAL_SCRIPT}" || -f "${CANONICAL_SCRIPT}" ]] \
  || die "canonical script not found in Deckhouse checkout: ${CANONICAL_SCRIPT}"
[[ -d "${TEMPLATE_DIR}" ]] \
  || die "docs template not found in Deckhouse checkout: ${TEMPLATE_DIR}"

if [[ -z "${OUTPUT_DIR}" ]]; then
  OUTPUT_DIR="${TMP_ROOT}/output"
fi
mkdir -p "${OUTPUT_DIR}"
OUTPUT_DIR="$(cd -- "${OUTPUT_DIR}" && pwd)"
CREATED_OUTPUT_DIR="${OUTPUT_DIR}"

echo "Building module documentation:"
echo "  module:    ${MODULE_NAME}"
echo "  channel:   ${CHANNEL}"
echo "  version:   ${MODULE_VERSION}"
echo "  source:    ${MODULE_PATH}"
echo "  output:    ${OUTPUT_DIR}"
echo "  hugo:      ${HUGO_IMAGE}"
echo "  deckhouse: ${DECKHOUSE_REPO}"

MODULE_PATH="${MODULE_PATH}" \
CHANNEL="${CHANNEL}" \
MODULE_VERSION="${MODULE_VERSION}" \
MODE=build \
HUGO_IMAGE="${HUGO_IMAGE}" \
OUTPUT_DIR="${OUTPUT_DIR}" \
bash "${CANONICAL_SCRIPT}"

missing_pages=()
for lang in en ru; do
  page="${OUTPUT_DIR}/${lang}/modules/${MODULE_NAME}/${CHANNEL}/readme.html"
  if [[ ! -s "${page}" ]]; then
    missing_pages+=("${page}")
  fi
done

if (( ${#missing_pages[@]} > 0 )); then
  echo "Hugo build did not produce the expected rendered pages:" >&2
  for page in "${missing_pages[@]}"; do
    echo "  ${page}" >&2
  done
  exit 1
fi

echo "Module documentation build succeeded."
echo "  en: ${OUTPUT_DIR}/en/modules/${MODULE_NAME}/${CHANNEL}/readme.html"
echo "  ru: ${OUTPUT_DIR}/ru/modules/${MODULE_NAME}/${CHANNEL}/readme.html"
