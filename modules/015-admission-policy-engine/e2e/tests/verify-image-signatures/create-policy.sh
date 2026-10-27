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

# Creates the SecurityPolicy e2e-ape-sp-signatures that requires the agnhost image to be signed
# with a Cosign key generated here. Nothing signs the image with that key, so the policy must deny
# it. The key pair lives only in a temporary directory and is never written to the repository.
set -Eeuo pipefail
source "../common/scripts/lib.sh"

if ! kubectl get crd verifiers.config.ratify.deislabs.io >/dev/null 2>&1; then
  fail "the Ratify CRDs are absent: the edition does not ship verifyImageSignatures, while the e2e cluster is expected to run FE"
fi

workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT
openssl ecparam -name prime256v1 -genkey -noout -out "${workdir}/cosign.key" 2>/dev/null
openssl ec -in "${workdir}/cosign.key" -pubout -out "${workdir}/cosign.pub" 2>/dev/null

jq -n --rawfile pub "${workdir}/cosign.pub" '{
  apiVersion: "deckhouse.io/v1alpha1",
  kind: "SecurityPolicy",
  metadata: {name: "e2e-ape-sp-signatures"},
  spec: {
    enforcementAction: "Deny",
    match: {namespaceSelector: {matchNames: ["e2e-ape-signatures"]}},
    policies: {
      verifyImageSignatures: [{
        reference: "registry.k8s.io/e2e-test-images/agnhost*",
        publicKeys: [($pub | rtrimstr("\n"))]
      }]
    }
  }
}' | kubectl apply -f -
log "SecurityPolicy e2e-ape-sp-signatures created with a freshly generated public key"
