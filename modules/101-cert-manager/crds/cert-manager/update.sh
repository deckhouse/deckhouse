#!/bin/bash

# Copyright 2025 Flant JSC
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
current_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
script_path=$(dirname "${BASH_SOURCE[0]}")

MODULE_NAME="cert-manager"
GIT_URL=https://github.com/cert-manager/cert-manager
CRD_FILE_PATH="${script_path:-.}"
CRD_GIT_PATH=deploy/crds
DOWNLOAD="${DOWNLOAD:-true}" # Is needed download crds (used for tests in CI)
MODULE_VERSION=v$(cat $script_path/../../images/cert-manager-controller/werf.inc.yaml | head -1 | sed -n 's/.*"\(.*\)".*/\1/p')
CRDS_FOR_BACKUP=(issuers.cert-manager.io clusterissuers.cert-manager.io)
echo "Update $MODULE_NAME crds"
echo $MODULE_NAME version: $MODULE_VERSION
if [[ "${DOWNLOAD}" == "true" ]]; then
    rm -f $script_path/*.yaml
    git clone --depth 1 --branch  $MODULE_VERSION $GIT_URL /tmp/$MODULE_NAME
    cp /tmp/$MODULE_NAME/$CRD_GIT_PATH/*.yaml "${CRD_FILE_PATH}"
    rm -rf /tmp/$MODULE_NAME
fi

# for file in folder 
for f in $CRD_FILE_PATH/*.yaml; do
    base=$(basename "$f")
    crd_name=$(yq ".metadata.name" $f )
    # Inject backup label
    if [[ " ${CRDS_FOR_BACKUP[@]} " =~ " ${crd_name} " ]]; then
        echo "Inject backup label to $f"
        yq -i '.metadata.labels["backup.deckhouse.io/cluster-config"] = "true"' "${f}"
    fi

    # Inject certificateOwnerRef
    if [[ "${crd_name}" == "certificates.cert-manager.io" ]]; then
    yq -i '
      .spec.versions[].schema.openAPIV3Schema.properties.spec.properties.certificateOwnerRef = {
        "type": "boolean",
        "x-doc-default": "nil",
        "description": "CertificateOwnerRef is whether to set the certificate resource as an owner of a secret where a TLS certificate is stored. When this option is toggled, the secret will be automatically removed when the certificate resource is deleted. A global owner reference policy will be used by default (controlled by the --enable-certificate-owner-ref flag)."
      }
    ' "${f}"    
    fi

done
