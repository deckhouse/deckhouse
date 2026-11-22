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

# Creates the DexAuthenticator and the Ingress of the application. Both depend on the
# cluster: the public domain template, the Ingress class of the controller and the
# cluster domain used in the auth-url annotation.
set -Eeuo pipefail
source ../../tests/common/scripts/lib.sh

ingress_discover
app_host="$(public_domain e2e-authn-app)"
dir="$(state_dir dex-authenticator-app)"
printf '%s' "${app_host}" > "${dir}/app-host"
auth_url="https://e2e-authn-app-dex-authenticator.${NS}.svc.$(cluster_domain)/dex-authenticator/auth"
log "Application host ${app_host}, auth-url ${auth_url}"

kubectl -n "${NS}" apply -f - <<MANIFESTS
apiVersion: deckhouse.io/v1
kind: DexAuthenticator
metadata:
  name: e2e-authn-app
spec:
  applicationDomain: ${app_host}
  applicationIngressClassName: ${ING_CLASS}
  allowedGroups:
    - e2e-authn-app-group
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: e2e-authn-app
  annotations:
    nginx.ingress.kubernetes.io/auth-signin: https://\$host/dex-authenticator/sign_in
    nginx.ingress.kubernetes.io/auth-url: ${auth_url}
    nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
spec:
  ingressClassName: ${ING_CLASS}
  rules:
    - host: ${app_host}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: e2e-authn-app
                port:
                  name: http
MANIFESTS
