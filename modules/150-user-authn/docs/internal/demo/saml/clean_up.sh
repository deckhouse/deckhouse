#!/bin/sh

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

# Delete DexProvider
d8 k delete dexprovider saml-demo --ignore-not-found

# Delete Dex connector object from storage
d8 k -n d8-user-authn delete connectors.dex.coreos.com saml-demo --ignore-not-found

# Delete refresh tokens created by the SAML connector
for token in $(d8 k -n d8-user-authn get refreshtokens.dex.coreos.com -o json 2>/dev/null | \
  jq -r '.items[] | select(.connectorID == "saml-demo") | .metadata.name'); do
  d8 k -n d8-user-authn delete refreshtokens.dex.coreos.com "${token}" --ignore-not-found
done

# Delete offline sessions created by the SAML connector
for session in $(d8 k -n d8-user-authn get offlinesessionses.dex.coreos.com -o json 2>/dev/null | \
  jq -r '.items[] | select(.connID == "saml-demo") | .metadata.name'); do
  d8 k -n d8-user-authn delete offlinesessionses.dex.coreos.com "${session}" --ignore-not-found
done

# Delete Keycloak namespace (includes Deployment, Service, Ingress)
d8 k delete ns saml-demo --ignore-not-found
