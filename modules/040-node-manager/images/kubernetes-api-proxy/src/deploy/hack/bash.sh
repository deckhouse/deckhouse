#!/usr/bin/env bash

# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

openssl genrsa -out kap.key 2048
openssl req -new -key kap.key -out kap.csr -subj="/O=system:kubernetes-api-proxy/CN=system:kubernetes-api-proxy"

kubectl delete csr kap-csr || echo "CSR not found"
cat <<EOF | kubectl create -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: kap-csr
spec:
  request: $(cat kap.csr | base64 | tr -d '\n')
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - client auth
EOF

kubectl certificate approve kap-csr
kubectl get csr kap-csr -o jsonpath='{.status.certificate}' | base64 --decode > kap.crt
