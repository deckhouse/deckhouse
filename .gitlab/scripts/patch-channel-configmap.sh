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

#
# Update the "release-channels-data" ConfigMap (consumed by the deckhouse-web
# site/API) on the deckhouse-web clusters: patch only the version of the
# release channel that was just deployed (RELEASE_CHANNEL), leave the other
# four channels untouched, then apply via `werf converge`.
#
# Clusters are configured through optional base64 kubeconfig variables:
#   KUBECONFIG_BASE64_PROD_25 (namespace deckhouse-web-production)
#   KUBECONFIG_BASE64_DEV     (namespace deckhouse-web-stage)
# A cluster whose variable is unset is skipped with a warning, not a failure.
#
# Required env: CI_COMMIT_TAG, RELEASE_CHANNEL.

set -Euo pipefail

if [[ -z "${CI_COMMIT_TAG:-}" ]]; then
  echo "CI_COMMIT_TAG is not set." >&2
  exit 1
fi
if [[ -z "${RELEASE_CHANNEL:-}" ]]; then
  echo "RELEASE_CHANNEL is not set." >&2
  exit 1
fi

CM_NAME="release-channels-data"
CM_KEY="channels.yaml"
DEFAULT_TEMPLATE='groups:
- name: "v1"
  channels:
    - name: alpha
      version: v0.0.0
    - name: beta
      version: v0.0.0
    - name: ea
      version: v0.0.0
    - name: stable
      version: v0.0.0
    - name: rock-solid
      version: v0.0.0
'

# Map RELEASE_CHANNEL to the short channel key used inside channels.yaml.
case "${RELEASE_CHANNEL}" in
  early-access) CHANNEL_KEY="ea" ;;
  *) CHANNEL_KEY="${RELEASE_CHANNEL}" ;;
esac

VERSION="${CI_COMMIT_TAG#v}"

TOOLS_PATH="${HOME}/.bin"
mkdir -p "${TOOLS_PATH}"
export PATH="${TOOLS_PATH}:${PATH}"

if ! command -v yq >/dev/null 2>&1; then
  echo "Installing yq..."
  curl -Lsf -o "${TOOLS_PATH}/yq" "https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64"
  chmod +x "${TOOLS_PATH}/yq"
fi
if ! command -v kubectl >/dev/null 2>&1; then
  echo "Installing kubectl..."
  KUBECTL_VERSION="$(curl -Lsf https://dl.k8s.io/release/stable.txt)"
  curl -Lsf -o "${TOOLS_PATH}/kubectl" "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
  chmod +x "${TOOLS_PATH}/kubectl"
fi

mkdir -p publish-channels/.helm/templates
cat > publish-channels/werf.yaml <<EOF
project: deckhouse-channels
configVersion: 1
EOF
cat > publish-channels/.helm/templates/configmap.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: release-channels-data
data:
  channels.yaml: |
{{ .Files.Get "channels.yaml" | indent 4 }}
EOF

deploy_cluster() {
  local KUBECONF_VAR="$1" NAMESPACE="$2" WERF_ENV_NAME="$3"
  local KUBECONF64="${!KUBECONF_VAR:-}"

  if [[ -z "${KUBECONF64}" ]]; then
    echo "⚠️ [$(date -u)] ${KUBECONF_VAR} is not set, skipping cluster '${WERF_ENV_NAME}'."
    return 0
  fi

  local KUBECONFIG_FILE
  KUBECONFIG_FILE="$(mktemp)"
  trap 'rm -f "${KUBECONFIG_FILE}"' RETURN
  echo "${KUBECONF64}" | base64 -d > "${KUBECONFIG_FILE}"

  local CURRENT_YAML
  if ! CURRENT_YAML="$(kubectl --kubeconfig="${KUBECONFIG_FILE}" -n "${NAMESPACE}" get configmap "${CM_NAME}" -o jsonpath="{.data.${CM_KEY//./\\.}}" 2>/dev/null)" || [[ -z "${CURRENT_YAML}" ]]; then
    echo "⚠️ [$(date -u)] Could not read existing ConfigMap '${CM_NAME}' in '${NAMESPACE}', starting from a default template."
    CURRENT_YAML="${DEFAULT_TEMPLATE}"
  fi

  echo "${CURRENT_YAML}" | yq eval "(.groups[0].channels[] | select(.name == \"${CHANNEL_KEY}\") | .version) = \"v${VERSION}\"" - \
    > publish-channels/.helm/channels.yaml

  echo "⚓️ 💫 [$(date -u)] Updated channels.yaml for cluster '${WERF_ENV_NAME}' (channel '${CHANNEL_KEY}' -> v${VERSION}):"
  cat publish-channels/.helm/channels.yaml

  export WERF_KUBE_CONFIG_BASE64="${KUBECONF64}"
  export WERF_NAMESPACE="${NAMESPACE}"
  export WERF_DIR="publish-channels"
  export WERF_ENV="${WERF_ENV_NAME}"
  type werf && source "$(werf ci-env gitlab --verbose --as-file)"
  werf converge

  unset WERF_KUBE_CONFIG_BASE64 WERF_NAMESPACE WERF_DIR WERF_ENV
}

deploy_cluster KUBECONFIG_BASE64_PROD_25 "deckhouse-web-production" "web-production"
deploy_cluster KUBECONFIG_BASE64_DEV "deckhouse-web-stage" "web-stage"
