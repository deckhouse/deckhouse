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

bb-curl-helper-extract-admin-certs() {
  local admin_conf="/etc/kubernetes/admin.conf"
  local cert_file="${TMPDIR}/bb-kube-admin-cert.pem"
  local key_file="${TMPDIR}/bb-kube-admin-key.pem"

  awk '/client-certificate-data:/{print $2}' "$admin_conf" | base64 -d > "$cert_file"
  awk '/client-key-data:/{print $2}' "$admin_conf" | base64 -d > "$key_file"
  chmod 600 "$cert_file" "$key_file"
}
