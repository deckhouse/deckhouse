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

# Copies the NodeConfig CRD from the nodelet repository, the single source of its
# schema, into crds/nodeconfig.yaml, under the module's own header. The tag is the
# nodelet the release ships, $nodeletVersion of the nodelet image.
# Usage: hack/sync-nodeconfig-crd.sh

set -euo pipefail

repo="${NODELET_REPO:-git@fox.flant.com:deckhouse/container-base/nodelet.git}"
source_path="crds/internal.deckhouse.io_nodeconfigs.yaml"
crd_name="nodeconfigs.internal.deckhouse.io"

module_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="${module_dir}/crds/nodeconfig.yaml"

# A werf template, not YAML: the version is the one line that assigns it.
werf_file="${module_dir}/../../images/platform/registrypackages/nodelet/werf.inc.yaml"
tag="$(sed -n 's/.*\$nodeletVersion := "\([^"]*\)".*/\1/p' "${werf_file}")"
if [[ -z "${tag}" ]]; then
  echo "no \$nodeletVersion in ${werf_file}" >&2
  exit 1
fi

workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT

git archive --remote="${repo}" "${tag}" "${source_path}" | tar -x -C "${workdir}"
source_file="${workdir}/${source_path}"

if ! grep -q "^  name: ${crd_name}$" "${source_file}"; then
  echo "${repo}@${tag}:${source_path} is not the ${crd_name} CRD" >&2
  exit 1
fi
if ! grep -q '^spec:$' "${source_file}"; then
  echo "${repo}@${tag}:${source_path} has no top-level spec" >&2
  exit 1
fi

# The header is the module's: labels addon-operator installs the CRD by, and no
# controller-gen annotations. Everything from spec on is nodelet's, byte for byte.
{
  cat <<EOF
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  labels:
    heritage: deckhouse
    module: node-manager
  name: ${crd_name}
EOF
  sed -n '/^spec:$/,$p' "${source_file}"
} > "${target}.tmp"
mv "${target}.tmp" "${target}"

echo "wrote ${target} from ${repo}@${tag}"
