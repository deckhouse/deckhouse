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

bb-curl-helper-extract-client-certs() {
  local kubeconfig="$1"
  local cert_file="$2"
  local key_file="$3"

  awk '/client-certificate-data:/{print $2}' "$kubeconfig" | base64 -d > "$cert_file"
  awk '/client-key-data:/{print $2}' "$kubeconfig" | base64 -d > "$key_file"
  chmod 600 "$cert_file" "$key_file"
}

bb-curl-helper-extract-admin-certs() {
  bb-curl-helper-extract-client-certs \
    "/etc/kubernetes/admin.conf" \
    "${TMPDIR}/bb-kube-admin-cert.pem" \
    "${TMPDIR}/bb-kube-admin-key.pem"
}

bb-curl-helper-extract-super-admin-certs() {
  bb-curl-helper-extract-client-certs \
    "/etc/kubernetes/super-admin.conf" \
    "${TMPDIR}/bb-kube-super-admin-cert.pem" \
    "${TMPDIR}/bb-kube-super-admin-key.pem"
}
