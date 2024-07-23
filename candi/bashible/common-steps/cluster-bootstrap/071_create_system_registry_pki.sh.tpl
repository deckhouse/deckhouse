# Copyright 2024 Flant JSC
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

# Upload pki for system-registry


{{- if and .registry.registryMode (ne .registry.registryMode "Direct") }}

registry_pki_path="/etc/kubernetes/system-registry/pki"
etcd_pki_path="/etc/kubernetes/pki/etcd"

bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf get ns d8-system || bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf create ns d8-system
bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf -n d8-system delete secret d8-system-registry-pki || true
bb-kubectl --kubeconfig=/etc/kubernetes/admin.conf -n d8-system create secret generic d8-system-registry-pki \
  --from-file=etcd-ca.key=$etcd_pki_path/ca.key \
  --from-file=etcd-ca.crt=$etcd_pki_path/ca.crt \
  --from-file=registry-ca.key=$registry_pki_path/ca.key \
  --from-file=registry-ca.crt=$registry_pki_path/ca.crt \
  --from-file=seaweedfs.key=$registry_pki_path/seaweedfs.key \
  --from-file=seaweedfs.crt=$registry_pki_path/seaweedfs.crt \
  --from-file=auth.key=$registry_pki_path/auth.key \
  --from-file=auth.crt=$registry_pki_path/auth.crt \
  --from-file=distribution.key=$registry_pki_path/distribution.key \
  --from-file=distribution.crt=$registry_pki_path/distribution.crt \
{{- end }}
