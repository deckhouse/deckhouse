#!/usr/bin/env bash

#
#Copyright 2021 Flant JSC
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.
#

# Helper - pull all cert-manager crds and mutate their labels and annotations
# also injects certificateOwnerRef patch
#!/usr/bin/env bash
set -euo pipefail

version="v1.17.4"
name="cert-manager"
repo="https://raw.githubusercontent.com/cert-manager/cert-manager/${version}/deploy/crds"
crds=("crd-certificaterequests.yaml crd-certificates.yaml crd-challenges.yaml crd-clusterissuers.yaml crd-issuers.yaml crd-orders.yaml")


DOWNLOAD="${DOWNLOAD:-true}" # Is needed download crds (used for tests in CI)
CRD_PATH="${CRD_PATH:-.}"

mkdir -p "${CRD_PATH}"

# Make variables visible for env() inside yq
export name
export version

# Step 1: Download files (if enabled)
if [[ "${DOWNLOAD}" == "true" ]]; then
  for crd in ${crds[@]}; do
    filepath="${CRD_PATH}/${crd}"
    file_url="${repo}/${crd}"

    curl -s "${file_url}" > "${filepath}"
  done
fi

# Step 2: Modify already existing files in CRD_PATH
for crd in ${crds[@]}; do
  filepath="${CRD_PATH}/${crd}"

  # Normalize labels, annotations and comments
  yq -i '
    del(.metadata.labels)
    | ... comments=""
    | del(.metadata.annotations["helm.sh/resource-policy"])
    | with(.metadata.labels;
        . = {
          "heritage": "deckhouse",
          "app": env(name),
          "module": env(name),
          "app.kubernetes.io/name": env(name),
          "app.kubernetes.io/instance": env(name),
          "app.kubernetes.io/version": env(version)
        }
        | .. style="single"
      )
  ' "${filepath}"

  # Inject certificateOwnerRef
  if [[ "${crd}" == "crd-certificates.yaml" ]]; then
    yq -i '
      .spec.versions[].schema.openAPIV3Schema.properties.spec.properties.certificateOwnerRef = {
        "type": "boolean",
        "x-doc-default": "nil",
        "description": "CertificateOwnerRef is whether to set the certificate resource as an owner of a secret where a TLS certificate is stored. When this option is toggled, the secret will be automatically removed when the certificate resource is deleted. A global owner reference policy will be used by default (controlled by the --enable-certificate-owner-ref flag)."
      }
    ' "${filepath}"
  fi

  # Inject backup labels
  if [[ "${crd}" == "crd-clusterissuers.yaml" || "${crd}" == "crd-issuers.yaml" ]]; then
    yq -i '.metadata.labels["backup.deckhouse.io/cluster-config"] = "true"' "${filepath}"
  fi
done
#                 certificateOwnerRef:
#                   type: boolean
#                   x-doc-default: nil
#                   description: CertificateOwnerRef is whether to set the certificate resource as an owner of a secret where a TLS certificate is stored. When this option is toggled, the secret will be automatically removed when the certificate resource is deleted. A global owner reference policy will be used by default (controlled by the --enable-certificate-owner-ref flag).
#
